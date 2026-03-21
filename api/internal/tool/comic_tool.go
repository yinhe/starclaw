package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/yinhe/starclaw/internal/model"
	"gorm.io/gorm"
)

// ComicTool assembles images into comic drama (漫剧) videos with Ken Burns effects,
// multi-character TTS voiceover, subtitles, and optional background music.
type ComicTool struct {
	db *gorm.DB
}

func NewComicTool(db *gorm.DB) *ComicTool {
	return &ComicTool{db: db}
}

func (t *ComicTool) Name() string { return "comic_production" }

func (t *ComicTool) Description() string {
	return `漫剧视频制作工具，将一组图片组装成漫剧视频。支持 Ken Burns 动效（缩放/平移）或 AI 视频动画（wan2.6-i2v），多角色配音，自适应字幕，可选BGM。
操作：compose_comic
使用前先用 image_generation 生成所有分镜图片。
每个分镜(panel)包含：
- image_id: 图片记录ID
- narrations: 角色台词数组 [{text, voice, character}]
- duration: 持续秒数（默认5）
- effect: 镜头效果（ken_burns/push_in/pull_out/pan_left/pan_right/crane_up/crane_down/dramatic_zoom/slow_reveal）
- motion: AI视频模式下的动作描述

视频模式：
- ken_burns（默认）: 图片+镜头动效，快速生成
- ai_video: 用wan2.6-i2v将图片动画化，画质更高但耗时更长`
}

func (t *ComicTool) Parameters() interface{} {
	return &JSONSchema{
		Type: "object",
		Properties: map[string]Property{
			"action":         {Type: "string", Description: "Action: compose_comic"},
			"panels":         {Type: "string", Description: "JSON array of panel definitions. Each: {\"image_id\":\"xxx\", \"narrations\":[{\"text\":\"台词\", \"voice\":\"longyuan\", \"character\":\"旁白\"}], \"duration\":5, \"effect\":\"push_in\", \"motion\":\"...\"}"},
			"comic_size":     {Type: "string", Description: "Output video size. Default: 720*1280 (portrait)"},
			"video_mode":     {Type: "string", Description: "'ken_burns' (default, fast) or 'ai_video' (wan2.6-i2v, high quality)"},
			"music_id":       {Type: "string", Description: "Optional: music record ID for background music"},
			"subtitle_style": {Type: "string", Description: "Subtitle style: auto (default), small, large, none"},
		},
		Required: []string{"action"},
	}
}

type comicArgs struct {
	Action        string `json:"action"`
	Panels        string `json:"panels"`
	ComicSize     string `json:"comic_size"`
	VideoMode     string `json:"video_mode"`
	MusicID       string `json:"music_id"`
	SubtitleStyle string `json:"subtitle_style"`
}

// ComicPanel defines one panel in a comic drama
type ComicPanel struct {
	ImageID    string           `json:"image_id"`
	Narrations []ComicNarration `json:"narrations"`
	Duration   int              `json:"duration"`
	Effect     string           `json:"effect"`
	Motion     string           `json:"motion"`
}

// ComicNarration defines one line of dialogue/narration in a panel
type ComicNarration struct {
	Text      string `json:"text"`
	Voice     string `json:"voice"`
	Character string `json:"character"`
}

func (t *ComicTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	var args comicArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %v", err)
	}
	switch args.Action {
	case "compose_comic":
		return t.composeComic(ctx, args)
	default:
		return "", fmt.Errorf("unknown action: %s. Use: compose_comic", args.Action)
	}
}

func (t *ComicTool) composeComic(ctx context.Context, args comicArgs) (string, error) {
	if args.Panels == "" {
		return "", fmt.Errorf("panels JSON array is required")
	}

	var panels []ComicPanel
	if err := json.Unmarshal([]byte(args.Panels), &panels); err != nil {
		return "", fmt.Errorf("invalid panels JSON: %v", err)
	}
	if len(panels) == 0 {
		return "", fmt.Errorf("panels array is empty")
	}

	userID := ""
	if uid, ok := ctx.Value(CtxKeyUserID).(string); ok {
		userID = uid
	}
	convID := ""
	if cid, ok := ctx.Value(CtxKeyConversationID).(string); ok {
		convID = cid
	}

	apiKey, _ := GetDashScopeAPIKey(t.db, userID)
	if apiKey == "" {
		return "", fmt.Errorf("no DashScope API key found for TTS")
	}

	outSize := args.ComicSize
	if outSize == "" {
		outSize = "720*1280"
	}
	sizeParts := strings.Split(outSize, "*")
	outW, outH := 720, 1280
	if len(sizeParts) == 2 {
		if w, err := strconv.Atoi(sizeParts[0]); err == nil && w > 0 {
			outW = w
		}
		if h, err := strconv.Atoi(sizeParts[1]); err == nil && h > 0 {
			outH = h
		}
	}

	tmpDir, err := os.MkdirTemp("", "compose-comic-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	videoMode := args.VideoMode
	if videoMode == "" {
		videoMode = "ken_burns"
	}

	_, baseHost := GetDashScopeAPIKey(t.db, userID)
	log.Printf("[ComicTool] %d panels, %dx%d, mode=%s", len(panels), outW, outH, videoMode)

	// Process each panel ↀindividual video clip
	var panelClips []string
	totalDuration := 0

	if videoMode == "ai_video" {
		panelClips, totalDuration, err = t.processAIVideoPanels(ctx, panels, apiKey, baseHost, tmpDir, outSize, args.SubtitleStyle)
	} else {
		panelClips, totalDuration, err = t.processKenBurnsPanels(ctx, panels, apiKey, tmpDir, outW, outH, args.SubtitleStyle)
	}
	if err != nil {
		return "", err
	}

	// Concatenate all panel clips
	concatList := filepath.Join(tmpDir, "concat.txt")
	var listContent strings.Builder
	for _, p := range panelClips {
		listContent.WriteString(fmt.Sprintf("file '%s'\n", p))
	}
	os.WriteFile(concatList, []byte(listContent.String()), 0644)

	mergedPath := filepath.Join(tmpDir, "merged.mp4")
	concatCmd := exec.CommandContext(ctx, "ffmpeg", "-y", "-f", "concat", "-safe", "0",
		"-i", concatList, "-c:v", "libx264", "-c:a", "aac", "-preset", "fast", mergedPath)
	if out, err := concatCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("concat failed: %v\n%s", err, string(out))
	}

	// Add background music if provided
	finalPath := mergedPath
	if args.MusicID != "" {
		musicPath, err := ResolveMusicPath(t.db, args.MusicID, tmpDir)
		if err != nil {
			log.Printf("[ComicTool] BGM resolve failed: %v (continuing without)", err)
		} else {
			bgmPath := filepath.Join(tmpDir, "final_bgm.mp4")
			bgmCmd := exec.CommandContext(ctx, "ffmpeg", "-y",
				"-i", mergedPath, "-i", musicPath,
				"-filter_complex", "[1:a]volume=0.15[bgm];[0:a][bgm]amix=inputs=2:duration=first[outa]",
				"-map", "0:v", "-map", "[outa]",
				"-c:v", "copy", "-c:a", "aac", "-b:a", "192k", "-shortest", bgmPath)
			if out, err := bgmCmd.CombinedOutput(); err != nil {
				log.Printf("[ComicTool] BGM mixing failed: %s", string(out))
			} else {
				finalPath = bgmPath
			}
		}
	}

	// Save output
	outputDir := MergedVideosDir()
	outputFilename := fmt.Sprintf("comic_%s.mp4", uuid.New().String()[:8])
	outputPath := filepath.Join(outputDir, outputFilename)

	cpCmd := exec.CommandContext(ctx, "cp", finalPath, outputPath)
	if out, err := cpCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("copy failed: %v\n%s", err, string(out))
	}

	fi, _ := os.Stat(outputPath)
	sizeMB := float64(0)
	if fi != nil {
		sizeMB = float64(fi.Size()) / (1024 * 1024)
	}
	downloadURL := fmt.Sprintf("/v1/videos/merged/%s", outputFilename)

	comicRecord := model.VideoRecord{
		UserID: userID, ConversationID: convID,
		Model: "comic", Prompt: fmt.Sprintf("漫剧: %d个分镜 %d秒", len(panels), totalDuration),
		VideoURL: downloadURL, Size: outSize, Duration: totalDuration,
		Status: "succeeded", Type: "comic",
	}
	t.db.Create(&comicRecord)
	ExtractThumbnail(t.db, comicRecord.ID, outputPath)

	return toJSON(map[string]interface{}{
		"action": "compose_comic", "status": "success",
		"comic_id": comicRecord.ID, "download_url": downloadURL,
		"size_mb": fmt.Sprintf("%.1f", sizeMB), "panels": len(panels), "duration": totalDuration,
		"message": fmt.Sprintf("漫剧合成完成！%d个分镜，共%d秒。下载: %s (%.1f MB)", len(panels), totalDuration, downloadURL, sizeMB),
	}), nil
}

// ── Ken Burns Mode ──

func (t *ComicTool) processKenBurnsPanels(ctx context.Context, panels []ComicPanel, apiKey, tmpDir string, outW, outH int, subStyleHint string) ([]string, int, error) {
	var clips []string
	totalDuration := 0

	for i, panel := range panels {
		dur := panel.Duration
		if dur <= 0 {
			dur = 5
		}
		totalDuration += dur

		imgPath, err := ResolveImagePath(t.db, panel.ImageID, tmpDir, i)
		if err != nil {
			return nil, 0, fmt.Errorf("panel %d: %v", i+1, err)
		}

		var ttsPath, allText string
		if len(panel.Narrations) > 0 {
			ttsPath, allText, err = t.generatePanelTTS(apiKey, panel.Narrations, tmpDir, i)
			if err != nil {
				log.Printf("[ComicTool] panel %d TTS failed: %v", i+1, err)
			}
		}

		effect := panel.Effect
		if effect == "" {
			effect = "ken_burns"
		}
		clipPath := filepath.Join(tmpDir, fmt.Sprintf("panel_%03d.mp4", i))
		if err := buildKenBurnsClip(ctx, imgPath, clipPath, outW, outH, dur, effect, ttsPath, allText, subStyleHint); err != nil {
			return nil, 0, fmt.Errorf("panel %d: %v", i+1, err)
		}
		clips = append(clips, clipPath)
	}
	return clips, totalDuration, nil
}

// ── AI Video Mode ──

func (t *ComicTool) processAIVideoPanels(ctx context.Context, panels []ComicPanel, apiKey, baseHost, tmpDir, outSize, subStyleHint string) ([]string, int, error) {
	type i2vTask struct {
		panelIdx int
		taskID   string
		duration int
		panel    ComicPanel
	}
	var tasks []i2vTask

	for i, panel := range panels {
		dur := panel.Duration
		if dur <= 0 {
			dur = 5
		}
		imgURL, err := ResolveImageURL(t.db, panel.ImageID)
		if err != nil {
			return nil, 0, fmt.Errorf("panel %d: %v", i+1, err)
		}
		motion := panel.Motion
		if motion == "" {
			motion = "subtle cinematic motion, gentle character movement, soft camera sway"
		}
		taskID, err := submitI2VTask(ctx, apiKey, baseHost, imgURL, motion, outSize, dur)
		if err != nil {
			return nil, 0, fmt.Errorf("panel %d: i2v submit failed: %v", i+1, err)
		}
		tasks = append(tasks, i2vTask{panelIdx: i, taskID: taskID, duration: dur, panel: panel})
		if i < len(panels)-1 {
			time.Sleep(500 * time.Millisecond)
		}
	}

	// Poll all tasks concurrently
	type i2vResult struct {
		videoURL string
		err      error
	}
	results := make([]i2vResult, len(tasks))
	var wg sync.WaitGroup
	for j, task := range tasks {
		wg.Add(1)
		go func(idx int, tid string) {
			defer wg.Done()
			vURL, _, err := pollDashScope(context.Background(), apiKey, baseHost, tid, 10*time.Minute)
			results[idx] = i2vResult{videoURL: vURL, err: err}
		}(j, task.taskID)
	}
	wg.Wait()

	var clips []string
	totalDuration := 0

	for j, task := range tasks {
		res := results[j]
		if res.err != nil {
			return nil, 0, fmt.Errorf("panel %d: i2v failed: %v", task.panelIdx+1, res.err)
		}

		rawClipPath := filepath.Join(tmpDir, fmt.Sprintf("i2v_%03d.mp4", task.panelIdx))
		if err := DownloadFile(res.videoURL, rawClipPath); err != nil {
			return nil, 0, fmt.Errorf("panel %d: download failed: %v", task.panelIdx+1, err)
		}

		var ttsPath, allText string
		if len(task.panel.Narrations) > 0 {
			var ttsErr error
			ttsPath, allText, ttsErr = t.generatePanelTTS(apiKey, task.panel.Narrations, tmpDir, task.panelIdx)
			if ttsErr != nil {
				log.Printf("[ComicTool] panel %d TTS failed: %v", task.panelIdx+1, ttsErr)
			}
		}

		finalClipPath := filepath.Join(tmpDir, fmt.Sprintf("panel_%03d.mp4", task.panelIdx))
		if ttsPath != "" {
			if err := overlayTTSOnClip(ctx, rawClipPath, ttsPath, allText, finalClipPath, subStyleHint); err != nil {
				log.Printf("[ComicTool] panel %d overlay failed: %v, using raw", task.panelIdx+1, err)
				exec.CommandContext(ctx, "cp", rawClipPath, finalClipPath).Run()
			}
		} else {
			exec.CommandContext(ctx, "cp", rawClipPath, finalClipPath).Run()
		}

		clips = append(clips, finalClipPath)
		totalDuration += task.duration
	}
	return clips, totalDuration, nil
}

// ── Panel TTS ──

func (t *ComicTool) generatePanelTTS(apiKey string, narrations []ComicNarration, tmpDir string, panelIdx int) (string, string, error) {
	if len(narrations) == 0 {
		return "", "", nil
	}

	var audioPaths []string
	var allText string

	for j, narr := range narrations {
		voice := narr.Voice
		if voice == "" {
			voice = "longyuan"
		}
		audioPath := filepath.Join(tmpDir, fmt.Sprintf("tts_p%03d_n%03d.mp3", panelIdx, j))
		if err := GenerateTTS(apiKey, narr.Text, voice, audioPath); err != nil {
			return "", "", fmt.Errorf("TTS failed for narration %d: %v", j+1, err)
		}
		audioPaths = append(audioPaths, audioPath)
		if narr.Character != "" {
			allText += narr.Character + "：" + narr.Text + " "
		} else {
			allText += narr.Text + " "
		}
	}

	if len(audioPaths) == 1 {
		return audioPaths[0], strings.TrimSpace(allText), nil
	}

	concatList := filepath.Join(tmpDir, fmt.Sprintf("tts_concat_p%03d.txt", panelIdx))
	var sb strings.Builder
	for _, p := range audioPaths {
		sb.WriteString(fmt.Sprintf("file '%s'\n", p))
	}
	os.WriteFile(concatList, []byte(sb.String()), 0644)

	combinedPath := filepath.Join(tmpDir, fmt.Sprintf("tts_combined_p%03d.mp3", panelIdx))
	cmd := exec.CommandContext(context.Background(), "ffmpeg", "-y", "-f", "concat", "-safe", "0",
		"-i", concatList, "-c", "copy", combinedPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		cmd2 := exec.CommandContext(context.Background(), "ffmpeg", "-y", "-f", "concat", "-safe", "0",
			"-i", concatList, "-c:a", "libmp3lame", "-b:a", "192k", combinedPath)
		if out2, err2 := cmd2.CombinedOutput(); err2 != nil {
			return "", "", fmt.Errorf("concat TTS failed: %v\n%s\n%s", err2, string(out), string(out2))
		}
	}
	return combinedPath, strings.TrimSpace(allText), nil
}

// ── Shared Helpers ──

// submitI2VTask submits an image-to-video task to DashScope
func submitI2VTask(ctx context.Context, apiKey, baseHost, imgURL, motion, size string, duration int) (string, error) {
	if duration > 5 {
		duration = 5
	}
	body := map[string]interface{}{
		"model":      "wan2.6-i2v",
		"input":      map[string]interface{}{"prompt": motion, "img_url": imgURL},
		"parameters": map[string]interface{}{"size": size, "duration": duration},
	}
	bodyBytes, _ := json.Marshal(body)
	url := fmt.Sprintf("https://%s/api/v1/services/aigc/video-generation/video-synthesis", baseHost)

	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-DashScope-Async", "enable")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("DashScope i2v error (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	json.Unmarshal(respBody, &result)
	output, _ := result["output"].(map[string]interface{})
	taskID, _ := output["task_id"].(string)
	if taskID == "" {
		return "", fmt.Errorf("no task_id: %s", string(respBody))
	}
	return taskID, nil
}

// pollDashScope polls a DashScope task (package-level helper for concurrent use)
func pollDashScope(ctx context.Context, apiKey, baseHost, taskID string, timeout time.Duration) (string, string, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return "", "", ctx.Err()
		case <-time.After(5 * time.Second):
		}

		url := fmt.Sprintf("https://%s/api/v1/tasks/%s", baseHost, taskID)
		req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
		req.Header.Set("Authorization", "Bearer "+apiKey)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

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

		switch status {
		case "SUCCEEDED":
			return videoURL, status, nil
		case "FAILED":
			return "", status, fmt.Errorf("generation failed")
		case "CANCELED":
			return "", status, fmt.Errorf("generation canceled")
		}
	}
	return "", "", fmt.Errorf("timeout after %v", timeout)
}

// overlayTTSOnClip adds TTS audio and optional subtitles to a video clip
func overlayTTSOnClip(ctx context.Context, videoPath, ttsPath, subtitleText, outputPath, subStyleHint string) error {
	videoDur := ProbeDuration(videoPath)
	if videoDur <= 0 {
		videoDur = 5.0
	}
	ttsDur := ProbeDuration(ttsPath)
	if ttsDur <= 0 {
		ttsDur = videoDur
	}

	finalTTS := ttsPath
	if ttsDur > videoDur*0.95 {
		speedRatio := ttsDur / (videoDur * 0.9)
		if speedRatio > 1.8 {
			speedRatio = 1.8
		}
		if speedRatio > 1.05 {
			speedPath := filepath.Join(filepath.Dir(outputPath), fmt.Sprintf("speed_%s.mp3", filepath.Base(outputPath)))
			spdCmd := exec.CommandContext(ctx, "ffmpeg", "-y", "-i", ttsPath,
				"-filter:a", fmt.Sprintf("atempo=%.3f", speedRatio), "-vn", speedPath)
			if _, err := spdCmd.CombinedOutput(); err == nil {
				finalTTS = speedPath
			}
		}
	}

	videoFilter := ""
	if subtitleText != "" && subStyleHint != "none" {
		srtPath := filepath.Join(filepath.Dir(outputPath), fmt.Sprintf("sub_%s.srt", filepath.Base(outputPath)))
		w, h := ProbeVideoDimensions(videoPath)
		maxChars := 18
		if h > w {
			maxChars = 10
		}
		segments := SplitNarrationToSegments(subtitleText, 0, videoDur, maxChars)
		if err := GenerateSRT(segments, srtPath); err == nil {
			subStyle := GetSubtitleStyle(videoPath, subStyleHint)
			if h > w {
				subStyle.FontSize = 14
				subStyle.MarginV = 80
				subStyle.MarginL = 40
				subStyle.MarginR = 40
			}
			escapedSrt := strings.ReplaceAll(srtPath, "\\", "/")
			escapedSrt = strings.ReplaceAll(escapedSrt, ":", "\\:")
			videoFilter = fmt.Sprintf("subtitles='%s':force_style='%s'", escapedSrt, subStyle.ForceStyleString())
		}
	}

	var ffmpegArgs []string
	if videoFilter != "" {
		ffmpegArgs = []string{
			"-y", "-i", videoPath, "-i", finalTTS,
			"-vf", videoFilter,
			"-c:v", "libx264", "-preset", "fast", "-pix_fmt", "yuv420p",
			"-c:a", "aac", "-b:a", "192k",
			"-map", "0:v", "-map", "1:a",
			"-t", fmt.Sprintf("%.1f", videoDur), "-shortest", outputPath,
		}
	} else {
		ffmpegArgs = []string{
			"-y", "-i", videoPath, "-i", finalTTS,
			"-c:v", "copy", "-c:a", "aac", "-b:a", "192k",
			"-map", "0:v", "-map", "1:a",
			"-t", fmt.Sprintf("%.1f", videoDur), "-shortest", outputPath,
		}
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", ffmpegArgs...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg overlay failed: %v\n%s", err, string(out))
	}
	return nil
}

// buildKenBurnsClip creates a video clip from an image with Ken Burns effect
func buildKenBurnsClip(ctx context.Context, imgPath, outputPath string, outW, outH, duration int, effect, ttsPath, subtitleText, subStyleHint string) error {
	fps := 30
	totalFrames := duration * fps
	d := totalFrames

	var zoomFilter string
	switch effect {
	case "zoom_in", "push_in", "dolly_in":
		zoomFilter = fmt.Sprintf("zoompan=z='1+0.3*(on/%d)':x='trunc(iw/2-(iw/zoom/2))':y='trunc(ih/2-(ih/zoom/2))':d=%d:s=%dx%d:fps=%d", d, d, outW, outH, fps)
	case "zoom_out", "pull_out":
		zoomFilter = fmt.Sprintf("zoompan=z='1.35-0.35*(on/%d)':x='trunc(iw/2-(iw/zoom/2))':y='trunc(ih/2-(ih/zoom/2))':d=%d:s=%dx%d:fps=%d", d, d, outW, outH, fps)
	case "pan_left":
		zoomFilter = fmt.Sprintf("zoompan=z='1.3':x='trunc(iw*0.25*(1-on/%d))':y='trunc(ih/2-(ih/zoom/2))':d=%d:s=%dx%d:fps=%d", d, d, outW, outH, fps)
	case "pan_right":
		zoomFilter = fmt.Sprintf("zoompan=z='1.3':x='trunc(iw*0.25*(on/%d))':y='trunc(ih/2-(ih/zoom/2))':d=%d:s=%dx%d:fps=%d", d, d, outW, outH, fps)
	case "pan_up", "crane_up":
		zoomFilter = fmt.Sprintf("zoompan=z='1.3':x='trunc(iw/2-(iw/zoom/2))':y='trunc(ih*0.25*(1-on/%d))':d=%d:s=%dx%d:fps=%d", d, d, outW, outH, fps)
	case "pan_down", "crane_down":
		zoomFilter = fmt.Sprintf("zoompan=z='1.3':x='trunc(iw/2-(iw/zoom/2))':y='trunc(ih*0.25*(on/%d))':d=%d:s=%dx%d:fps=%d", d, d, outW, outH, fps)
	case "dramatic_zoom":
		zoomFilter = fmt.Sprintf("zoompan=z='1+0.5*(on/%d)':x='trunc(iw/2-(iw/zoom/2)+iw*0.08*(on/%d))':y='trunc(ih/2-(ih/zoom/2))':d=%d:s=%dx%d:fps=%d", d, d, d, outW, outH, fps)
	case "slow_reveal":
		zoomFilter = fmt.Sprintf("zoompan=z='1.5-0.5*(on/%d)':x='trunc(iw/2-(iw/zoom/2))':y='trunc(ih*0.15*(on/%d))':d=%d:s=%dx%d:fps=%d", d, d, d, outW, outH, fps)
	default: // ken_burns
		zoomFilter = fmt.Sprintf("zoompan=z='1+0.2*(on/%d)':x='trunc(iw/2-(iw/zoom/2)+iw*0.08*(on/%d))':y='trunc(ih/2-(ih/zoom/2))':d=%d:s=%dx%d:fps=%d", d, d, d, outW, outH, fps)
	}

	fadeFilter := fmt.Sprintf("fade=t=in:st=0:d=0.3,fade=t=out:st=%d:d=0.3", duration-1)

	var ffmpegArgs []string
	if ttsPath != "" {
		ttsDur := ProbeDuration(ttsPath)
		if ttsDur <= 0 {
			ttsDur = float64(duration)
		}
		finalTTS := ttsPath
		if ttsDur > float64(duration)*0.95 {
			speedRatio := ttsDur / (float64(duration) * 0.9)
			if speedRatio > 1.8 {
				speedRatio = 1.8
			}
			if speedRatio > 1.05 {
				speedPath := filepath.Join(filepath.Dir(outputPath), fmt.Sprintf("speed_%s.mp3", filepath.Base(outputPath)))
				spdCmd := exec.CommandContext(ctx, "ffmpeg", "-y", "-i", ttsPath,
					"-filter:a", fmt.Sprintf("atempo=%.3f", speedRatio), "-vn", speedPath)
				if _, err := spdCmd.CombinedOutput(); err == nil {
					finalTTS = speedPath
				}
			}
		}

		videoFilter := zoomFilter + "," + fadeFilter
		if subtitleText != "" && subStyleHint != "none" {
			srtPath := filepath.Join(filepath.Dir(outputPath), fmt.Sprintf("sub_%s.srt", filepath.Base(outputPath)))
			isPortrait := outH > outW
			maxChars := 18
			if isPortrait {
				maxChars = 10
			}
			segments := SplitNarrationToSegments(subtitleText, 0, float64(duration), maxChars)
			if err := GenerateSRT(segments, srtPath); err == nil {
				subStyle := GetSubtitleStyle(imgPath, subStyleHint)
				if isPortrait {
					subStyle.FontSize = 14
					subStyle.MarginV = 80
					subStyle.MarginL = 40
					subStyle.MarginR = 40
				}
				escapedSrt := strings.ReplaceAll(srtPath, "\\", "/")
				escapedSrt = strings.ReplaceAll(escapedSrt, ":", "\\:")
				videoFilter += fmt.Sprintf(",subtitles='%s':force_style='%s'", escapedSrt, subStyle.ForceStyleString())
			}
		}

		ffmpegArgs = []string{
			"-y", "-loop", "1", "-i", imgPath, "-i", finalTTS,
			"-vf", videoFilter,
			"-c:v", "libx264", "-preset", "fast", "-pix_fmt", "yuv420p",
			"-c:a", "aac", "-b:a", "192k",
			"-t", strconv.Itoa(duration), "-shortest", outputPath,
		}
	} else {
		videoFilter := zoomFilter + "," + fadeFilter
		ffmpegArgs = []string{
			"-y", "-loop", "1", "-i", imgPath,
			"-f", "lavfi", "-i", fmt.Sprintf("anullsrc=r=44100:cl=stereo:d=%d", duration),
			"-vf", videoFilter,
			"-c:v", "libx264", "-preset", "fast", "-pix_fmt", "yuv420p",
			"-c:a", "aac", "-t", strconv.Itoa(duration), outputPath,
		}
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", ffmpegArgs...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg Ken Burns failed: %v\n%s", err, string(out))
	}
	return nil
}
