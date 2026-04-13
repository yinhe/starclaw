package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/yinhe/starclaw/internal/model"
)

// ── Resolution helpers ──

// probeResolution returns width, height of a video file using ffprobe.
func probeResolution(path string) (int, int, error) {
	out, err := hiddenCmd("ffprobe", "-v", "error", "-select_streams", "v:0",
		"-show_entries", "stream=width,height", "-of", "csv=p=0:s=x", path).Output()
	if err != nil {
		return 0, 0, err
	}
	parts := strings.Split(strings.TrimSpace(string(out)), "x")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("unexpected ffprobe output: %s", string(out))
	}
	w, _ := strconv.Atoi(parts[0])
	h, _ := strconv.Atoi(parts[1])
	return w, h, nil
}

// ffmpegMergeClips merges clips with crossfade transitions between scenes.
// Uses xfade filter for smooth dissolve transitions (0.5s default).
// Auto-detects and normalizes different resolutions to majority resolution.
func ffmpegMergeClips(ctx context.Context, clipPaths []string, outputPath string) error {
	if len(clipPaths) == 0 {
		return fmt.Errorf("no clips to merge")
	}
	if len(clipPaths) == 1 {
		// Single clip: just copy
		cmd := hiddenCmdCtx(ctx, "ffmpeg", "-y", "-i", clipPaths[0], "-c", "copy", outputPath)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("ffmpeg copy failed: %v\n%s", err, string(out))
		}
		return nil
	}

	// Probe all resolutions
	type res struct{ w, h int }
	resolutions := make([]res, len(clipPaths))
	resCounts := make(map[res]int)
	for i, p := range clipPaths {
		w, h, err := probeResolution(p)
		if err != nil {
			log.Printf("[VideoMerge] ffprobe failed for clip %d: %v, defaulting to 1280x720", i, err)
			w, h = 1280, 720
		}
		resolutions[i] = res{w, h}
		resCounts[res{w, h}]++
	}

	// Find majority resolution
	var targetRes res
	maxCount := 0
	for r, c := range resCounts {
		if c > maxCount {
			maxCount = c
			targetRes = r
		}
	}

	tmpDir := filepath.Dir(clipPaths[0])
	if tmpDir == "" {
		tmpDir = os.TempDir()
	}

	// Step 1: Normalize all clips to same resolution + codec (required for xfade)
	var normalizedPaths []string
	for i, p := range clipPaths {
		normPath := filepath.Join(tmpDir, fmt.Sprintf("norm_%03d.mp4", i))
		if resolutions[i] == targetRes && len(resCounts) == 1 {
			// Same resolution, but still need to re-encode for xfade compatibility
			cmd := hiddenCmdCtx(ctx, "ffmpeg", "-y", "-i", p,
				"-c:v", "libx264", "-c:a", "aac", "-preset", "fast", "-r", "30",
				"-pix_fmt", "yuv420p", normPath)
			if out, err := cmd.CombinedOutput(); err != nil {
				log.Printf("[VideoMerge] re-encode clip %d failed: %s, trying copy", i, string(out))
				hiddenCmdCtx(ctx, "cp", p, normPath).Run()
			}
		} else {
			filter := fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2:color=black",
				targetRes.w, targetRes.h, targetRes.w, targetRes.h)
			cmd := hiddenCmdCtx(ctx, "ffmpeg", "-y", "-i", p,
				"-vf", filter, "-c:v", "libx264", "-c:a", "aac", "-preset", "fast", "-r", "30",
				"-pix_fmt", "yuv420p", normPath)
			if out, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("ffmpeg normalize clip %d failed: %v\n%s", i, err, string(out))
			}
		}
		normalizedPaths = append(normalizedPaths, normPath)
	}

	log.Printf("[VideoMerge] %d clips normalized to %dx%d, applying crossfade transitions", len(normalizedPaths), targetRes.w, targetRes.h)

	// Step 2: Merge with xfade crossfade transitions using a SINGLE filter_complex
	// This is much faster than pairwise chaining (one ffmpeg process vs N-1).
	const xfadeDuration = 0.5

	// Probe durations for calculating xfade offsets
	durations := make([]float64, len(normalizedPaths))
	for i, p := range normalizedPaths {
		durations[i] = ProbeDuration(p)
		if durations[i] <= 0 {
			durations[i] = 5.0
		}
	}

	// Build all -i inputs
	var ffmpegArgs []string
	ffmpegArgs = append(ffmpegArgs, "-y")
	for _, p := range normalizedPaths {
		ffmpegArgs = append(ffmpegArgs, "-i", p)
	}

	// Build single filter_complex with chained xfade
	// Video: [0:v][1:v]xfade=...[v01]; [v01][2:v]xfade=...[v012]; ...
	// Audio: [0:a][1:a]acrossfade=...[a01]; [a01][2:a]acrossfade=...[a012]; ...
	n := len(normalizedPaths)
	transitions := []string{"fade", "dissolve", "smoothleft", "fadeblack"}

	var filterParts []string
	accumulatedDuration := durations[0]

	// Video xfade chain
	prevVideoLabel := "[0:v]"
	for i := 1; i < n; i++ {
		offset := accumulatedDuration - xfadeDuration
		if offset < 0.1 {
			offset = 0.1
		}
		transition := transitions[i%len(transitions)]
		outLabel := fmt.Sprintf("[v%d]", i)
		if i == n-1 {
			outLabel = "[outv]"
		}
		filterParts = append(filterParts,
			fmt.Sprintf("%s[%d:v]xfade=transition=%s:duration=%.2f:offset=%.2f%s",
				prevVideoLabel, i, transition, xfadeDuration, offset, outLabel))
		prevVideoLabel = outLabel
		accumulatedDuration = accumulatedDuration + durations[i] - xfadeDuration
	}

	// Audio acrossfade chain
	prevAudioLabel := "[0:a]"
	for i := 1; i < n; i++ {
		outLabel := fmt.Sprintf("[a%d]", i)
		if i == n-1 {
			outLabel = "[outa]"
		}
		filterParts = append(filterParts,
			fmt.Sprintf("%s[%d:a]acrossfade=d=%.2f:c1=tri:c2=tri%s",
				prevAudioLabel, i, xfadeDuration, outLabel))
		prevAudioLabel = outLabel
	}

	filterComplex := strings.Join(filterParts, ";")
	ffmpegArgs = append(ffmpegArgs,
		"-filter_complex", filterComplex,
		"-map", "[outv]", "-map", "[outa]",
		"-c:v", "libx264", "-preset", "fast", "-pix_fmt", "yuv420p",
		"-c:a", "aac", "-b:a", "192k", outputPath)

	cmd := hiddenCmdCtx(ctx, "ffmpeg", ffmpegArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Fallback 1: video-only xfade (no audio crossfade)
		log.Printf("[VideoMerge] xfade with audio failed: %s, trying video-only", string(out))
		var vOnlyParts []string
		prevVideoLabel = "[0:v]"
		accumulatedDuration = durations[0]
		for i := 1; i < n; i++ {
			offset := accumulatedDuration - xfadeDuration
			if offset < 0.1 {
				offset = 0.1
			}
			transition := transitions[i%len(transitions)]
			outLabel := fmt.Sprintf("[v%d]", i)
			if i == n-1 {
				outLabel = "[outv]"
			}
			vOnlyParts = append(vOnlyParts,
				fmt.Sprintf("%s[%d:v]xfade=transition=%s:duration=%.2f:offset=%.2f%s",
					prevVideoLabel, i, transition, xfadeDuration, offset, outLabel))
			prevVideoLabel = outLabel
			accumulatedDuration = accumulatedDuration + durations[i] - xfadeDuration
		}
		var args2 []string
		args2 = append(args2, "-y")
		for _, p := range normalizedPaths {
			args2 = append(args2, "-i", p)
		}
		args2 = append(args2,
			"-filter_complex", strings.Join(vOnlyParts, ";"),
			"-map", "[outv]",
			"-c:v", "libx264", "-preset", "fast", "-pix_fmt", "yuv420p",
			"-an", outputPath)
		cmd2 := hiddenCmdCtx(ctx, "ffmpeg", args2...)
		out2, err2 := cmd2.CombinedOutput()
		if err2 != nil {
			// Fallback 2: simple concat
			log.Printf("[VideoMerge] video-only xfade also failed: %s, falling back to concat", string(out2))
			return ffmpegSimpleConcat(ctx, normalizedPaths, outputPath)
		}
	}

	log.Printf("[VideoMerge] Crossfade merge complete: %d clips → %s", len(clipPaths), outputPath)
	return nil
}

// ffmpegSimpleConcat is a fallback that concatenates clips without transitions.
func ffmpegSimpleConcat(ctx context.Context, clipPaths []string, outputPath string) error {
	tmpDir := filepath.Dir(clipPaths[0])
	listPath := filepath.Join(tmpDir, "filelist_fallback.txt")
	var listContent strings.Builder
	for _, p := range clipPaths {
		listContent.WriteString(fmt.Sprintf("file '%s'\n", p))
	}
	os.WriteFile(listPath, []byte(listContent.String()), 0644)

	cmd := hiddenCmdCtx(ctx, "ffmpeg", "-y", "-f", "concat", "-safe", "0",
		"-i", listPath, "-c:v", "libx264", "-c:a", "aac", "-preset", "fast", outputPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg simple concat failed: %v\n%s", err, string(out))
	}
	log.Printf("[VideoMerge] Simple concat fallback complete: %d clips", len(clipPaths))
	return nil
}

// ── Merge Videos ──

func (t *VideoTool) mergeVideos(ctx context.Context, args videoArgs) (string, error) {
	userID := ""
	if uid, ok := ctx.Value(CtxKeyUserID).(string); ok {
		userID = uid
	}
	convID := ""
	if cid, ok := ctx.Value(CtxKeyConversationID).(string); ok {
		convID = cid
	}

	var records []model.VideoRecord
	if args.TaskIDs != "" {
		ids := strings.Split(args.TaskIDs, ",")
		for i := range ids {
			ids[i] = strings.TrimSpace(ids[i])
		}
		// Search by both record ID and DashScope task_id (agent may pass either)
		q := t.db.Where("(id IN ? OR task_id IN ?) AND status = ?", ids, ids, "succeeded")
		if userID != "" {
			q = q.Where("user_id = ?", userID)
		}
		q.Find(&records)
		idOrder := make(map[string]int)
		for i, id := range ids {
			idOrder[id] = i
		}
		sort.Slice(records, func(i, j int) bool {
			oi := idOrder[records[i].ID]
			if oj, ok := idOrder[records[i].TaskID]; ok && oj < oi {
				oi = oj
			}
			oj := idOrder[records[j].ID]
			if ojt, ok := idOrder[records[j].TaskID]; ok && ojt < oj {
				oj = ojt
			}
			return oi < oj
		})
	} else if convID != "" {
		// Use method chaining instead of raw SQL OR to avoid GORM parsing issues
		result := t.db.Where("conversation_id = ?", convID).
			Where("type IN ?", []string{"clip", ""}).
			Where("status = ?", "succeeded").
			Order("scene ASC, created_at ASC").Find(&records)
		log.Printf("[VideoTool] merge query: user=%s conv=%s found=%d err=%v", userID, convID, len(records), result.Error)
	} else {
		return "", fmt.Errorf("no conversation context and no task_ids provided")
	}

	if len(records) == 0 {
		// Provide diagnostic info: what videos exist in this conversation?
		var allRecords []model.VideoRecord
		if convID != "" {
			t.db.Where("conversation_id = ?", convID).
				Order("created_at DESC").Limit(20).Find(&allRecords)
		}
		if len(allRecords) > 0 {
			var diag []string
			for _, r := range allRecords {
				p := r.Prompt
				if len(p) > 60 {
					p = p[:60] + "..."
				}
				diag = append(diag, fmt.Sprintf("  - id=%s scene=%s status=%s type=%s prompt=%q", r.ID, r.Scene, r.Status, r.Type, p))
			}
			return "", fmt.Errorf("no completed (status=succeeded, type=clip) videos found to merge. Found %d records in this conversation:\n%s\nHint: use check_status to check pending videos, or use list_videos to see all videos",
				len(allRecords), strings.Join(diag, "\n"))
		}
		return "", fmt.Errorf("no videos found in this conversation. Generate videos first with action=generate_video")
	}

	// Auto-dedup: if multiple clips share the same scene, keep the latest succeeded clip per scene.
	// This matches what list_videos promises: "merge_videos 会自动去重（每个场景保留最新的成功片段）"
	if len(records) > 1 && args.TaskIDs == "" {
		sceneClips := make(map[string][]model.VideoRecord)
		for _, r := range records {
			scene := r.Scene
			if scene == "" {
				scene = "_no_scene"
			}
			sceneClips[scene] = append(sceneClips[scene], r)
		}
		hasDups := false
		for _, clips := range sceneClips {
			if len(clips) > 1 {
				hasDups = true
				break
			}
		}
		if hasDups {
			// Keep latest clip per scene (sorted by created_at ASC, so last = newest)
			var deduped []model.VideoRecord
			dedupInfo := make(map[string]int) // scene → count of duplicates removed
			for scene, clips := range sceneClips {
				best := clips[len(clips)-1] // last = newest (query ordered by created_at ASC)
				deduped = append(deduped, best)
				if len(clips) > 1 {
					dedupInfo[scene] = len(clips) - 1
				}
			}
			// Re-sort deduped by scene ASC
			sort.Slice(deduped, func(i, j int) bool {
				return deduped[i].Scene < deduped[j].Scene
			})
			log.Printf("[VideoTool] merge auto-dedup: %d records → %d (removed duplicates from %d scenes)", len(records), len(deduped), len(dedupInfo))
			records = deduped
		}
	}

	if len(records) == 1 {
		return toJSON(map[string]interface{}{
			"action": "merge_videos", "status": "success",
			"video_url": records[0].VideoURL, "message": "只有1个视频片段，无需合成。",
		}), nil
	}

	tmpDir, err := os.MkdirTemp("", "video-merge-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	var clipPaths []string
	for i, rec := range records {
		clipPath := filepath.Join(tmpDir, fmt.Sprintf("clip_%03d.mp4", i))
		// Use narrated version if available
		dlURL := rec.VideoURL
		if rec.NarratedURL != "" {
			dlURL = rec.NarratedURL
		}
		resolved, err := ResolveClipToLocal(dlURL, clipPath)
		if err != nil {
			return "", fmt.Errorf("failed to resolve clip %d: %v", i+1, err)
		}
		clipPaths = append(clipPaths, resolved)
	}

	mergeID := uuid.New().String()
	outputDir := MergedVideosDir()
	outputPath := filepath.Join(outputDir, mergeID+".mp4")

	if err := ffmpegMergeClips(ctx, clipPaths, outputPath); err != nil {
		return "", err
	}

	fi, _ := os.Stat(outputPath)
	sizeMB := float64(0)
	if fi != nil {
		sizeMB = float64(fi.Size()) / 1024 / 1024
	}
	downloadURL := fmt.Sprintf("/v1/videos/merged/%s.mp4", mergeID)

	clipIDList := make([]string, len(records))
	totalDuration := 0
	for i, r := range records {
		clipIDList[i] = r.ID
		totalDuration += r.Duration
	}
	clipIDsJSON, _ := json.Marshal(clipIDList)

	// Remove any existing merged videos for this conversation to avoid duplicates
	if convID != "" {
		var oldMerges []model.VideoRecord
		t.db.Where("user_id = ? AND conversation_id = ? AND type = ?", userID, convID, "merged").Find(&oldMerges)
		for _, om := range oldMerges {
			if om.VideoURL != "" {
				// Try removing from both merged and clips dirs
				os.Remove(filepath.Join(MergedVideosDir(), filepath.Base(om.VideoURL)))
			}
		}
		if len(oldMerges) > 0 {
			t.db.Unscoped().Where("user_id = ? AND conversation_id = ? AND type = ?", userID, convID, "merged").Delete(&model.VideoRecord{})
			log.Printf("[VideoTool] merge_videos: deleted %d old merged records for conversation %s", len(oldMerges), convID)
		}
	}

	mergedRecord := model.VideoRecord{
		UserID: userID, ConversationID: convID,
		Model: "merged", Prompt: fmt.Sprintf("合成视频: %d个片段, 共%d秒", len(records), totalDuration),
		VideoURL: downloadURL, Size: records[0].Size, Duration: totalDuration,
		Status: "succeeded", Type: "merged", ClipIDs: string(clipIDsJSON),
	}
	t.db.Create(&mergedRecord)

	return toJSON(map[string]interface{}{
		"action": "merge_videos", "status": "success",
		"clips_count": len(records), "download_url": downloadURL,
		"size_mb": fmt.Sprintf("%.1f", sizeMB),
		"message": fmt.Sprintf("视频合成成功！共%d个片段。下载: %s (%.1f MB)", len(records), downloadURL, sizeMB),
	}), nil
}

// ── TryAutoMerge ──

// TryAutoMerge checks if all clips in a conversation are complete, and auto-merges them.
func (t *VideoTool) TryAutoMerge(userID, convID string) {
	t.mergeMu.Lock()
	if t.merging[convID] {
		t.mergeMu.Unlock()
		return
	}
	t.merging[convID] = true
	t.mergeMu.Unlock()
	defer func() {
		t.mergeMu.Lock()
		delete(t.merging, convID)
		t.mergeMu.Unlock()
	}()

	var totalClips, succeededClips, runningClips int64
	t.db.Model(&model.VideoRecord{}).Where("user_id = ? AND conversation_id = ? AND (type = 'clip' OR type = '')", userID, convID).Count(&totalClips)
	t.db.Model(&model.VideoRecord{}).Where("user_id = ? AND conversation_id = ? AND (type = 'clip' OR type = '') AND status = 'succeeded'", userID, convID).Count(&succeededClips)
	t.db.Model(&model.VideoRecord{}).Where("user_id = ? AND conversation_id = ? AND (type = 'clip' OR type = '') AND status IN ('running','pending')", userID, convID).Count(&runningClips)

	if totalClips < 2 || runningClips > 0 || succeededClips < 2 {
		return
	}

	// Grace period: wait 90s for the agent to submit more scenes.
	// Between scene N completing and scene N+1 being submitted, runningClips=0
	// which would falsely trigger a premature merge.
	log.Printf("[VideoTool] Auto-merge: %d clips ready, waiting 90s grace period for more scenes...", succeededClips)
	time.Sleep(90 * time.Second)

	// Re-check after grace period: new clips may have appeared
	var newRunning, newSucceeded int64
	t.db.Model(&model.VideoRecord{}).Where("user_id = ? AND conversation_id = ? AND (type = 'clip' OR type = '') AND status IN ('running','pending')", userID, convID).Count(&newRunning)
	t.db.Model(&model.VideoRecord{}).Where("user_id = ? AND conversation_id = ? AND (type = 'clip' OR type = '') AND status = 'succeeded'", userID, convID).Count(&newSucceeded)
	if newRunning > 0 {
		log.Printf("[VideoTool] Auto-merge deferred: %d clips still running after grace period", newRunning)
		return
	}

	// Check if a merge already exists with the same clip count (another goroutine beat us)
	var existingMerge model.VideoRecord
	hasMerge := t.db.Where("user_id = ? AND conversation_id = ? AND type = ?", userID, convID, "merged").
		Order("created_at DESC").First(&existingMerge).Error == nil

	if hasMerge {
		// Count clips in the existing merge
		var existingClipIDs []string
		json.Unmarshal([]byte(existingMerge.ClipIDs), &existingClipIDs)
		if int64(len(existingClipIDs)) >= newSucceeded {
			return // existing merge already has all clips
		}
		// New clips available since last merge — delete old merge, re-merge with all clips
		log.Printf("[VideoTool] Re-merge: %d clips now vs %d in previous merge", newSucceeded, len(existingClipIDs))
		// Delete old merged file
		if existingMerge.VideoURL != "" {
			oldFile := filepath.Join(MergedVideosDir(), filepath.Base(existingMerge.VideoURL))
			os.Remove(oldFile)
		}
		t.db.Unscoped().Delete(&existingMerge)
	}

	log.Printf("[VideoTool] Auto-merge triggered: %d clips, conversation %s", newSucceeded, convID)

	var records []model.VideoRecord
	t.db.Where("user_id = ? AND conversation_id = ? AND (type = 'clip' OR type = '') AND status = 'succeeded'", userID, convID).
		Order("scene ASC, created_at ASC").Find(&records)
	if len(records) < 2 {
		return
	}

	tmpDir, err := os.MkdirTemp("", "auto-merge-*")
	if err != nil {
		return
	}
	defer os.RemoveAll(tmpDir)

	var clipPaths []string
	var usedRecords []model.VideoRecord
	for i, rec := range records {
		clipPath := filepath.Join(tmpDir, fmt.Sprintf("clip_%03d.mp4", i))
		dlURL := rec.VideoURL
		if rec.NarratedURL != "" {
			dlURL = rec.NarratedURL
		}
		resolved, err := ResolveClipToLocal(dlURL, clipPath)
		if err != nil {
			log.Printf("[VideoTool] Auto-merge: resolve clip %d (%s) failed: %v, skipping", i+1, rec.Scene, err)
			continue
		}
		clipPaths = append(clipPaths, resolved)
		usedRecords = append(usedRecords, rec)
	}
	if len(clipPaths) < 2 {
		log.Printf("[VideoTool] Auto-merge: only %d clips resolved (need ≥2), aborting", len(clipPaths))
		return
	}
	if len(clipPaths) < len(records) {
		log.Printf("[VideoTool] Auto-merge: %d/%d clips resolved (some skipped)", len(clipPaths), len(records))
	}

	mergeID := uuid.New().String()
	outputDir := MergedVideosDir()
	outputPath := filepath.Join(outputDir, mergeID+".mp4")

	ctx := context.Background()
	if err := ffmpegMergeClips(ctx, clipPaths, outputPath); err != nil {
		log.Printf("[VideoTool] Auto-merge failed: %v", err)
		return
	}

	downloadURL := fmt.Sprintf("/v1/videos/merged/%s.mp4", mergeID)
	clipIDList := make([]string, len(usedRecords))
	totalDuration := 0
	for i, r := range usedRecords {
		clipIDList[i] = r.ID
		totalDuration += r.Duration
	}
	clipIDsJSON, _ := json.Marshal(clipIDList)
	mergedRecord := model.VideoRecord{
		UserID: userID, ConversationID: convID,
		Model: "merged", Prompt: fmt.Sprintf("合成视频: %d个片段, 共%d秒", len(usedRecords), totalDuration),
		VideoURL: downloadURL, Size: usedRecords[0].Size, Duration: totalDuration,
		Status: "succeeded", Type: "merged", ClipIDs: string(clipIDsJSON),
	}
	t.db.Create(&mergedRecord)
	ExtractThumbnail(t.db, mergedRecord.ID, downloadURL)
	log.Printf("[VideoTool] Auto-merge succeeded: %d clips, %ds", len(usedRecords), totalDuration)
}

// ── List Videos ──

// listVideos returns existing video records for the current conversation or user,
// so the AI can discover already-generated videos and avoid regenerating them.
func (t *VideoTool) listVideos(ctx context.Context, _ videoArgs) (string, error) {
	userID := ""
	if uid, ok := ctx.Value(CtxKeyUserID).(string); ok {
		userID = uid
	}
	convID := ""
	if cid, ok := ctx.Value(CtxKeyConversationID).(string); ok {
		convID = cid
	}

	var records []model.VideoRecord
	q := t.db.Order("created_at DESC").Limit(50)

	if convID != "" {
		// Show videos in current conversation first
		q = q.Where("conversation_id = ?", convID)
	} else if userID != "" {
		q = q.Where("user_id = ?", userID)
	} else {
		return "", fmt.Errorf("no user or conversation context")
	}
	q.Find(&records)

	type videoSummary struct {
		ID        string `json:"id"`
		TaskID    string `json:"task_id"`
		Model     string `json:"model"`
		Scene     string `json:"scene"`
		Status    string `json:"status"`
		Type      string `json:"type"`
		Duration  int    `json:"duration"`
		VideoURL  string `json:"video_url,omitempty"`
		Prompt    string `json:"prompt_preview"`
		CreatedAt string `json:"created_at"`
	}

	var summaries []videoSummary
	succeeded := 0
	running := 0
	failed := 0
	// Track scenes for duplicate detection
	sceneCount := make(map[string]int)
	sceneBest := make(map[string]string) // scene → best (latest succeeded) record ID
	for _, r := range records {
		promptPreview := r.Prompt
		if len(promptPreview) > 100 {
			promptPreview = promptPreview[:100] + "..."
		}
		s := videoSummary{
			ID: r.ID, TaskID: r.TaskID, Model: r.Model,
			Scene: r.Scene, Status: r.Status, Type: r.Type,
			Duration: r.Duration, Prompt: promptPreview,
			CreatedAt: r.CreatedAt.Format("2006-01-02 15:04:05"),
		}
		if r.Status == "succeeded" {
			s.VideoURL = r.VideoURL
			succeeded++
			if r.Scene != "" {
				sceneBest[r.Scene] = r.ID // last = newest
			}
		} else if r.Status == "running" || r.Status == "pending" {
			running++
		} else {
			failed++
		}
		if r.Scene != "" {
			sceneCount[r.Scene]++
		}
		summaries = append(summaries, s)
	}

	// Detect duplicates
	var duplicateScenes []string
	for scene, count := range sceneCount {
		if count > 1 {
			duplicateScenes = append(duplicateScenes, fmt.Sprintf("%s(%d个)", scene, count))
		}
	}

	result := map[string]interface{}{
		"action":    "list_videos",
		"total":     len(summaries),
		"succeeded": succeeded,
		"running":   running,
		"failed":    failed,
		"videos":    summaries,
	}

	msg := fmt.Sprintf("找到 %d 个视频记录（%d 完成, %d 进行中, %d 失败）。",
		len(summaries), succeeded, running, failed)
	if len(duplicateScenes) > 0 {
		msg += fmt.Sprintf("\n⚠️ 发现重复场景: %s。merge_videos 会自动去重（每个场景保留最新的成功片段）。", strings.Join(duplicateScenes, ", "))
		result["duplicate_scenes"] = duplicateScenes
		result["recommended_clips"] = sceneBest
		msg += "\n建议：直接调用 merge_videos 合成，系统会自动选择每个场景的最佳片段。无需重新生成。"
	}
	if succeeded > 0 {
		msg += " 已完成的视频可直接用于 merge_videos 或 compose_pro。"
	}
	result["message"] = msg

	return toJSON(result), nil
}

// ── DashScope Task Polling ──

func (t *VideoTool) pollDashScopeTask(ctx context.Context, apiKey, baseHost, taskID string, timeout time.Duration) (string, string, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return "", "", ctx.Err()
		case <-time.After(5 * time.Second):
		}
		status, videoURL, err := t.getDashScopeTaskStatus(ctx, apiKey, baseHost, taskID)
		if err != nil {
			return "", "", err
		}
		switch status {
		case "SUCCEEDED":
			return videoURL, status, nil
		case "FAILED":
			return "", status, fmt.Errorf("video generation failed")
		case "CANCELED":
			return "", status, fmt.Errorf("video generation canceled")
		}
	}
	return "", "", fmt.Errorf("polling timeout after %v", timeout)
}

func (t *VideoTool) getDashScopeTaskStatus(ctx context.Context, apiKey, baseHost, taskID string) (string, string, error) {
	var reqURL string
	var client *http.Client
	if isStarAIKey(apiKey) {
		reqURL = StarAIProxyURL("dashscope", "/api/v1/tasks/"+taskID)
		c, _ := GetStarAIClient()
		if c == nil {
			return "", "", fmt.Errorf("StarAI proxy not initialized")
		}
		client = c
	} else {
		reqURL = fmt.Sprintf("https://%s/api/v1/tasks/%s", baseHost, taskID)
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return "", "", err
	}
	if !isStarAIKey(apiKey) {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var result map[string]interface{}
	json.Unmarshal(body, &result)
	output, _ := result["output"].(map[string]interface{})
	status, _ := output["task_status"].(string)

	videoURL := ""
	if results, ok := output["results"].([]interface{}); ok && len(results) > 0 {
		if r, ok := results[0].(map[string]interface{}); ok {
			videoURL, _ = r["url"].(string)
		}
	}
	if videoURL == "" {
		videoURL, _ = output["video_url"].(string)
	}
	return status, videoURL, nil
}

// ── Workflow & Thumbnails ──

func (t *VideoTool) ensureVideoWorkflow(userID, convID string) {
	if convID == "" {
		return
	}
	var count int64
	t.db.Model(&model.Workflow{}).Where("user_id = ? AND description LIKE ?", userID, "%conv:"+convID+"%").Count(&count)
	if count > 0 {
		return
	}
	convTitle := "视频制作"
	var conv model.Conversation
	if err := t.db.Where("id = ?", convID).First(&conv).Error; err == nil && conv.Title != "" {
		convTitle = conv.Title
	}

	type nodeDef struct {
		ID       string                 `json:"id"`
		Type     string                 `json:"type"`
		Position map[string]float64     `json:"position"`
		Data     map[string]interface{} `json:"data"`
	}
	type edgeDef struct {
		ID     string `json:"id"`
		Source string `json:"source"`
		Target string `json:"target"`
	}
	nodes := []nodeDef{
		{ID: "start-1", Type: "start", Position: map[string]float64{"x": 300, "y": 50}, Data: map[string]interface{}{"label": "开始"}},
		{ID: "step-1", Type: "llm", Position: map[string]float64{"x": 300, "y": 150}, Data: map[string]interface{}{"label": "编写脚本"}},
		{ID: "step-2", Type: "tool", Position: map[string]float64{"x": 300, "y": 270}, Data: map[string]interface{}{"label": "生成视频片段", "toolName": "video_generation"}},
		{ID: "step-3", Type: "tool", Position: map[string]float64{"x": 300, "y": 390}, Data: map[string]interface{}{"label": "自动合成", "toolName": "video_generation"}},
		{ID: "end-1", Type: "end", Position: map[string]float64{"x": 300, "y": 510}, Data: map[string]interface{}{"label": "完成"}},
	}
	edges := []edgeDef{
		{ID: "e-s1", Source: "start-1", Target: "step-1"},
		{ID: "e-12", Source: "step-1", Target: "step-2"},
		{ID: "e-23", Source: "step-2", Target: "step-3"},
		{ID: "e-3e", Source: "step-3", Target: "end-1"},
	}
	defJSON, _ := json.Marshal(map[string]interface{}{"nodes": nodes, "edges": edges})
	wf := model.Workflow{
		ID: uuid.New().String(), UserID: userID, Name: convTitle,
		Description: fmt.Sprintf("视频制作工作浀[conv:%s]", convID),
		Definition:  string(defJSON),
	}
	t.db.Create(&wf)
}

// GenerateMissingThumbnails finds videos without thumbnails and generates them.
func (t *VideoTool) GenerateMissingThumbnails() {
	var records []model.VideoRecord
	t.db.Where("status = 'succeeded' AND video_url != '' AND (img_url = '' OR img_url IS NULL)").
		Order("created_at DESC").Limit(50).Find(&records)
	if len(records) == 0 {
		return
	}
	log.Printf("[VideoTool] Generating thumbnails for %d videos", len(records))
	generated := 0
	for _, rec := range records {
		source := rec.VideoURL
		if (rec.Type == "clip" || rec.Type == "") && rec.NarratedURL != "" {
			if strings.HasPrefix(rec.NarratedURL, "/v1/videos/merged/") {
				source = rec.NarratedURL
			}
		}
		if strings.HasPrefix(source, "https://") {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			req, _ := http.NewRequestWithContext(ctx, "HEAD", source, nil)
			resp, err := http.DefaultClient.Do(req)
			cancel()
			if err != nil || resp.StatusCode != 200 {
				continue
			}
		}
		if url := ExtractThumbnail(t.db, rec.ID, source); url != "" {
			generated++
		}
	}
	if generated > 0 {
		log.Printf("[VideoTool] Generated %d thumbnails", generated)
	}
}
