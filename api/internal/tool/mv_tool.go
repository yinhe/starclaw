package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/yinhe/starclaw/internal/model"
	"gorm.io/gorm"
)

// MVTool composes music videos by combining video clips with a music track and optional lyrics subtitles.
type MVTool struct {
	db *gorm.DB
}

func NewMVTool(db *gorm.DB) *MVTool {
	return &MVTool{db: db}
}

func (t *MVTool) Name() string { return "mv_production" }

func (t *MVTool) Description() string {
	return `MV（音乐视频）合成工具，将视频画面与音乐轨道合成为最终MV。
操作：
- compose_mv: 基础合成，将视频片段与音乐合并（简单拼接）
- compose_pro: 专业合成，支持逐镜头精确时长裁剪、xfade/flash/beat_cut转场、节拍同步剪辑。格莱美级MV用此操作。

compose_pro 接受 scenes JSON数组，每个场景包含 video_id、trim_duration（精确裁剪秒数）、transition（转场类型：cut/crossfade/flash/fadewhite/wipeleft/fadeblack）、transition_duration（转场时长秒数）。
音频来源：music_id（已生成音乐）或 audio_url（上传的音频文件路径）。`
}

func (t *MVTool) Parameters() interface{} {
	return &JSONSchema{
		Type: "object",
		Properties: map[string]Property{
			"action":         {Type: "string", Description: "Action: compose_mv, compose_pro"},
			"music_id":       {Type: "string", Description: "Music record ID from music_generation tool"},
			"audio_url":      {Type: "string", Description: "Direct audio file URL/path (alternative to music_id, e.g. uploaded .wav file)"},
			"video_id":       {Type: "string", Description: "For compose_mv: specific video record ID to use."},
			"scenes":         {Type: "string", Description: `For compose_pro: JSON array of scenes. Each scene: {"video_id":"xxx", "trim_duration":5.0, "transition":"crossfade", "transition_duration":0.5}. video_id accepts: record_id (from check_status), task_id, or scene label (e.g. "scene_1"). Transitions: cut (hard cut, default), crossfade (dissolve), flash (white flash), fadewhite, fadeblack, wipeleft, slideleft, circlecrop. transition_duration defaults to 0.5s.`},
			"lyrics_srt":     {Type: "string", Description: "SRT-format lyrics for subtitle burning. Optional."},
			"subtitle_style": {Type: "string", Description: "Subtitle style: auto (default), small, large, none"},
		},
		Required: []string{"action"},
	}
}

type mvArgs struct {
	Action        string `json:"action"`
	MusicID       string `json:"music_id"`
	AudioURL      string `json:"audio_url"`
	VideoID       string `json:"video_id"`
	Scenes        string `json:"scenes"`
	LyricsSRT     string `json:"lyrics_srt"`
	SubtitleStyle string `json:"subtitle_style"`
}

func (t *MVTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	var args mvArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %v", err)
	}
	switch args.Action {
	case "compose_mv":
		return t.composeMV(ctx, args)
	case "compose_pro":
		return t.composePro(ctx, args)
	default:
		return "", fmt.Errorf("unknown action: %s. Use: compose_mv, compose_pro", args.Action)
	}
}

func (t *MVTool) composeMV(ctx context.Context, args mvArgs) (string, error) {
	userID := ""
	if uid, ok := ctx.Value(CtxKeyUserID).(string); ok {
		userID = uid
	}
	convID := ""
	if cid, ok := ctx.Value(CtxKeyConversationID).(string); ok {
		convID = cid
	}

	if args.MusicID == "" {
		return "", fmt.Errorf("music_id is required for compose_mv")
	}
	if convID == "" && args.VideoID == "" {
		return "", fmt.Errorf("compose_mv requires a conversation context or video_id")
	}

	// Resolve music file
	var music model.MusicRecord
	if err := t.db.Where("id = ?", args.MusicID).First(&music).Error; err != nil {
		return "", fmt.Errorf("music record not found: %s", args.MusicID)
	}
	if music.Status != "succeeded" {
		return "", fmt.Errorf("music not ready (status: %s)", music.Status)
	}

	tmpDir, err := os.MkdirTemp("", "compose-mv-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	musicPath, err := ResolveMusicPath(t.db, args.MusicID, tmpDir)
	if err != nil {
		return "", err
	}

	// Get video
	videoPath := ""
	totalDuration := 0
	totalClips := 0

	if args.VideoID != "" {
		var video model.VideoRecord
		if err := t.db.Where("id = ?", args.VideoID).First(&video).Error; err != nil {
			return "", fmt.Errorf("video not found: %s", args.VideoID)
		}
		if strings.HasPrefix(video.VideoURL, "/v1/videos/merged/") {
			fn := strings.TrimPrefix(video.VideoURL, "/v1/videos/merged/")
			videoPath = filepath.Join(MergedVideosDir(), fn)
		} else {
			videoPath = filepath.Join(tmpDir, "input_video.mp4")
			if err := DownloadFile(video.VideoURL, videoPath); err != nil {
				return "", fmt.Errorf("failed to download video: %v", err)
			}
		}
		totalDuration = video.Duration
		totalClips = 1
	} else {
		// Merge RAW clips from conversation (video_url, NOT narrated_url)
		var clips []model.VideoRecord
		query := t.db.Where("conversation_id = ? AND (type = 'clip' OR type = '') AND status = 'succeeded'", convID)
		if userID != "" {
			query = query.Where("user_id = ?", userID)
		}
		query.Order("scene ASC, created_at ASC").Find(&clips)
		if len(clips) == 0 {
			return "", fmt.Errorf("no completed video clips found in this conversation")
		}
		totalClips = len(clips)

		var clipPaths []string
		for i, clip := range clips {
			clipPath := filepath.Join(tmpDir, fmt.Sprintf("clip_%03d.mp4", i))
			resolved, err := ResolveClipToLocal(clip.VideoURL, clipPath)
			if err != nil {
				return "", fmt.Errorf("failed to resolve clip %d: %v", i+1, err)
			}
			clipPaths = append(clipPaths, resolved)
			totalDuration += clip.Duration
		}

		if len(clipPaths) == 1 {
			videoPath = clipPaths[0]
		} else {
			listPath := filepath.Join(tmpDir, "filelist.txt")
			var listContent strings.Builder
			for _, p := range clipPaths {
				listContent.WriteString(fmt.Sprintf("file '%s'\n", p))
			}
			os.WriteFile(listPath, []byte(listContent.String()), 0644)

			videoPath = filepath.Join(tmpDir, "merged_raw.mp4")
			cmd := hiddenCmdCtx(ctx, "ffmpeg", "-y", "-f", "concat", "-safe", "0",
				"-i", listPath, "-c", "copy", videoPath)
			if out, err := cmd.CombinedOutput(); err != nil {
				cmd2 := hiddenCmdCtx(ctx, "ffmpeg", "-y", "-f", "concat", "-safe", "0",
					"-i", listPath, "-c:v", "libx264", "-c:a", "aac", "-preset", "fast", videoPath)
				if out2, err2 := cmd2.CombinedOutput(); err2 != nil {
					return "", fmt.Errorf("merge failed: %v\n%s\n%s", err2, string(out), string(out2))
				}
			}
		}
	}

	if _, err := os.Stat(videoPath); os.IsNotExist(err) {
		return "", fmt.Errorf("video file not found: %s", videoPath)
	}

	// Compose video + music + optional lyrics
	outputDir := MergedVideosDir()
	outputFilename := fmt.Sprintf("mv_%s.mp4", uuid.New().String()[:8])
	outputPath := filepath.Join(outputDir, outputFilename)

	var ffmpegArgs []string
	if args.LyricsSRT != "" {
		srtPath := filepath.Join(tmpDir, "lyrics.srt")
		os.WriteFile(srtPath, []byte(args.LyricsSRT), 0644)

		escapedSRT := strings.ReplaceAll(srtPath, "\\", "/")
		escapedSRT = strings.ReplaceAll(escapedSRT, ":", "\\:")

		mvSubStyle := GetSubtitleStyle(videoPath, args.SubtitleStyle)
		mvForceStyle := mvSubStyle.ForceStyleString() + ",Alignment=2"
		ffmpegArgs = []string{
			"-y", "-i", videoPath, "-i", musicPath,
			"-filter_complex",
			fmt.Sprintf("[0:v]subtitles=%s:force_style='%s'[v]", escapedSRT, mvForceStyle),
			"-map", "[v]", "-map", "1:a",
			"-c:v", "libx264", "-preset", "fast",
			"-c:a", "aac", "-b:a", "192k",
			"-shortest", outputPath,
		}
	} else {
		ffmpegArgs = []string{
			"-y", "-i", videoPath, "-i", musicPath,
			"-map", "0:v", "-map", "1:a",
			"-c:v", "copy", "-c:a", "aac", "-b:a", "192k",
			"-shortest", outputPath,
		}
	}

	log.Printf("[MVTool] composing: video=%s, music=%s, clips=%d", videoPath, musicPath, totalClips)
	cmd := hiddenCmdCtx(ctx, "ffmpeg", ffmpegArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("FFmpeg compose failed: %v\n%s", err, string(out))
	}

	fi, _ := os.Stat(outputPath)
	sizeMB := float64(0)
	if fi != nil {
		sizeMB = float64(fi.Size()) / (1024 * 1024)
	}
	downloadURL := fmt.Sprintf("/v1/videos/merged/%s", outputFilename)

	mvRecord := model.VideoRecord{
		UserID: userID, ConversationID: convID,
		Model: "mv", Prompt: fmt.Sprintf("MV: %d个场景+音乐, %d秒", totalClips, totalDuration),
		VideoURL: downloadURL, Size: "1280*720", Duration: totalDuration,
		Status: "succeeded", Type: "mv",
	}
	t.db.Create(&mvRecord)
	ExtractThumbnail(t.db, mvRecord.ID, outputPath)

	return toJSON(map[string]interface{}{
		"action": "compose_mv", "status": "success",
		"mv_id": mvRecord.ID, "download_url": downloadURL,
		"size_mb": fmt.Sprintf("%.1f", sizeMB), "clips": totalClips, "duration": totalDuration,
		"message": fmt.Sprintf("MV合成完成！%d个场景+音乐，共%d秒。下载: %s (%.1f MB)", totalClips, totalDuration, downloadURL, sizeMB),
	}), nil
}

// ── compose_pro: Professional MV assembly with per-clip trimming + transitions ──

type proScene struct {
	VideoID            string  `json:"video_id"`
	TrimDuration       float64 `json:"trim_duration"`       // seconds to keep from clip (0 = use full clip)
	Transition         string  `json:"transition"`          // cut, crossfade, flash, fadewhite, fadeblack, wipeleft, slideleft, circlecrop
	TransitionDuration float64 `json:"transition_duration"` // transition length in seconds (default 0.5)
}

func (t *MVTool) composePro(ctx context.Context, args mvArgs) (string, error) {
	userID := ""
	if uid, ok := ctx.Value(CtxKeyUserID).(string); ok {
		userID = uid
	}
	convID := ""
	if cid, ok := ctx.Value(CtxKeyConversationID).(string); ok {
		convID = cid
	}

	if args.Scenes == "" {
		return "", fmt.Errorf("scenes JSON array is required for compose_pro")
	}

	var scenes []proScene
	if err := json.Unmarshal([]byte(args.Scenes), &scenes); err != nil {
		return "", fmt.Errorf("invalid scenes JSON: %v", err)
	}
	if len(scenes) == 0 {
		return "", fmt.Errorf("at least one scene is required")
	}

	tmpDir, err := os.MkdirTemp("", "compose-pro-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	// Resolve audio (music_id or audio_url)
	audioPath, err := t.resolveAudioURL(args, tmpDir)
	if err != nil {
		return "", err
	}

	log.Printf("[MVTool] compose_pro: %d scenes, audio=%s", len(scenes), audioPath)

	// Step 1: Resolve and trim each clip
	var trimmedPaths []string
	var trimmedDurations []float64
	for i, scene := range scenes {
		if scene.VideoID == "" {
			return "", fmt.Errorf("scene %d: video_id is required", i+1)
		}
		video, err := t.resolveVideoRecord(scene.VideoID, convID, userID)
		if err != nil {
			return "", fmt.Errorf("scene %d: %v", i+1, err)
		}

		// Download/resolve clip
		clipPath := filepath.Join(tmpDir, fmt.Sprintf("raw_%03d.mp4", i))
		if strings.HasPrefix(video.VideoURL, "/v1/videos/merged/") || strings.HasPrefix(video.VideoURL, "/v1/videos/clips/") {
			var src string
			if strings.HasPrefix(video.VideoURL, "/v1/videos/clips/") {
				fn := strings.TrimPrefix(video.VideoURL, "/v1/videos/clips/")
				src = filepath.Join(VideosDir(), fn)
			} else {
				fn := strings.TrimPrefix(video.VideoURL, "/v1/videos/merged/")
				src = filepath.Join(MergedVideosDir(), fn)
			}
			if _, err := os.Stat(src); err == nil {
				clipPath = src
			}
		}
		if clipPath == filepath.Join(tmpDir, fmt.Sprintf("raw_%03d.mp4", i)) {
			resolved, err := ResolveClipToLocal(video.VideoURL, clipPath)
			if err != nil {
				return "", fmt.Errorf("scene %d: failed to resolve video: %v", i+1, err)
			}
			clipPath = resolved
		}

		// Trim to exact duration if specified
		finalPath := clipPath
		clipDur := float64(video.Duration)
		if scene.TrimDuration > 0 && scene.TrimDuration < clipDur {
			trimPath := filepath.Join(tmpDir, fmt.Sprintf("trim_%03d.mp4", i))
			trimCmd := hiddenCmdCtx(ctx, "ffmpeg", "-y",
				"-i", clipPath,
				"-t", fmt.Sprintf("%.3f", scene.TrimDuration),
				"-c:v", "libx264", "-preset", "fast", "-an",
				trimPath,
			)
			if out, err := trimCmd.CombinedOutput(); err != nil {
				return "", fmt.Errorf("scene %d: trim failed: %v\n%s", i+1, err, string(out))
			}
			finalPath = trimPath
			clipDur = scene.TrimDuration
		}

		trimmedPaths = append(trimmedPaths, finalPath)
		trimmedDurations = append(trimmedDurations, clipDur)
	}

	// Step 2: Assemble with transitions using ffmpeg xfade filter chain
	var assembledPath string
	if len(trimmedPaths) == 1 {
		assembledPath = trimmedPaths[0]
	} else {
		assembledPath, err = t.assembleWithTransitions(ctx, tmpDir, trimmedPaths, trimmedDurations, scenes)
		if err != nil {
			return "", fmt.Errorf("assembly failed: %v", err)
		}
	}

	// Step 3: Combine with audio + optional subtitles
	outputDir := MergedVideosDir()
	outputFilename := fmt.Sprintf("mvpro_%s.mp4", uuid.New().String()[:8])
	outputPath := filepath.Join(outputDir, outputFilename)

	var ffmpegArgs []string
	if args.LyricsSRT != "" {
		srtPath := filepath.Join(tmpDir, "lyrics.srt")
		os.WriteFile(srtPath, []byte(args.LyricsSRT), 0644)
		escapedSRT := strings.ReplaceAll(srtPath, "\\", "/")
		escapedSRT = strings.ReplaceAll(escapedSRT, ":", "\\:")
		mvSubStyle := GetSubtitleStyle(assembledPath, args.SubtitleStyle)
		mvForceStyle := mvSubStyle.ForceStyleString() + ",Alignment=2"
		ffmpegArgs = []string{
			"-y", "-i", assembledPath, "-i", audioPath,
			"-filter_complex",
			fmt.Sprintf("[0:v]subtitles=%s:force_style='%s'[v]", escapedSRT, mvForceStyle),
			"-map", "[v]", "-map", "1:a",
			"-c:v", "libx264", "-preset", "fast",
			"-c:a", "aac", "-b:a", "192k",
			"-shortest", outputPath,
		}
	} else {
		ffmpegArgs = []string{
			"-y", "-i", assembledPath, "-i", audioPath,
			"-map", "0:v", "-map", "1:a",
			"-c:v", "copy", "-c:a", "aac", "-b:a", "192k",
			"-shortest", outputPath,
		}
	}

	log.Printf("[MVTool] compose_pro final: video=%s, audio=%s", assembledPath, audioPath)
	cmd := hiddenCmdCtx(ctx, "ffmpeg", ffmpegArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Fallback: re-encode video if copy fails
		ffmpegArgs[len(ffmpegArgs)-3] = "libx264"
		cmd2 := hiddenCmdCtx(ctx, "ffmpeg", ffmpegArgs...)
		if out2, err2 := cmd2.CombinedOutput(); err2 != nil {
			return "", fmt.Errorf("final compose failed: %v\n%s\n%s", err2, string(out), string(out2))
		}
	}

	fi, _ := os.Stat(outputPath)
	sizeMB := float64(0)
	if fi != nil {
		sizeMB = float64(fi.Size()) / (1024 * 1024)
	}

	// Calculate total duration
	totalDur := 0.0
	for _, d := range trimmedDurations {
		totalDur += d
	}
	// Subtract transition overlaps
	for i := 1; i < len(scenes); i++ {
		td := scenes[i].TransitionDuration
		if td > 0 && scenes[i].Transition != "" && scenes[i].Transition != "cut" {
			totalDur -= td
		}
	}

	downloadURL := fmt.Sprintf("/v1/videos/merged/%s", outputFilename)
	mvRecord := model.VideoRecord{
		UserID: userID, ConversationID: convID,
		Model:    "mv-pro",
		Prompt:   fmt.Sprintf("Pro MV: %d scenes, %.0fs", len(scenes), totalDur),
		VideoURL: downloadURL, Size: "1280*720",
		Duration: int(totalDur),
		Status:   "succeeded", Type: "mv",
	}
	t.db.Create(&mvRecord)
	ExtractThumbnail(t.db, mvRecord.ID, outputPath)

	// Build transition summary
	transitionCounts := make(map[string]int)
	for _, s := range scenes {
		tr := s.Transition
		if tr == "" {
			tr = "cut"
		}
		transitionCounts[tr]++
	}
	var trSummary []string
	for k, v := range transitionCounts {
		trSummary = append(trSummary, fmt.Sprintf("%s×%d", k, v))
	}

	return toJSON(map[string]interface{}{
		"action": "compose_pro", "status": "success",
		"mv_id": mvRecord.ID, "download_url": downloadURL,
		"size_mb":     fmt.Sprintf("%.1f", sizeMB),
		"clips":       len(scenes),
		"duration":    int(totalDur),
		"transitions": strings.Join(trSummary, ", "),
		"message": fmt.Sprintf("专业MV合成完成！%d个镜头，%d秒，转场: %s。下载: %s (%.1f MB)",
			len(scenes), int(totalDur), strings.Join(trSummary, "/"), downloadURL, sizeMB),
	}), nil
}

// assembleWithTransitions chains clips with xfade transitions
func (t *MVTool) assembleWithTransitions(ctx context.Context, tmpDir string, clips []string, durations []float64, scenes []proScene) (string, error) {
	if len(clips) < 2 {
		return clips[0], nil
	}

	// For many clips, chain pairwise: merge 0+1 → tmp, tmp+2 → tmp2, ...
	// This avoids ultra-complex filter_complex chains that fail with many inputs
	current := clips[0]
	currentDur := durations[0]

	for i := 1; i < len(clips); i++ {
		next := clips[i]
		nextDur := durations[i]

		transition := "cut"
		transDur := 0.0
		if i < len(scenes) {
			transition = scenes[i].Transition
			transDur = scenes[i].TransitionDuration
		}
		if transition == "" {
			transition = "cut"
		}
		if transDur <= 0 && transition != "cut" {
			transDur = 0.5
		}

		outPath := filepath.Join(tmpDir, fmt.Sprintf("chain_%03d.mp4", i))

		if transition == "cut" {
			// Simple concat
			listPath := filepath.Join(tmpDir, fmt.Sprintf("list_%03d.txt", i))
			os.WriteFile(listPath, []byte(fmt.Sprintf("file '%s'\nfile '%s'\n", current, next)), 0644)
			cmd := hiddenCmdCtx(ctx, "ffmpeg", "-y",
				"-f", "concat", "-safe", "0", "-i", listPath,
				"-c:v", "libx264", "-preset", "fast", "-an",
				outPath,
			)
			if out, err := cmd.CombinedOutput(); err != nil {
				return "", fmt.Errorf("concat %d failed: %v\n%s", i, err, string(out))
			}
			currentDur = currentDur + nextDur
		} else if transition == "flash" {
			// Flash = very short fadewhite
			if transDur > 0.3 {
				transDur = 0.15
			}
			offset := currentDur - transDur
			if offset < 0 {
				offset = 0
			}
			cmd := hiddenCmdCtx(ctx, "ffmpeg", "-y",
				"-i", current, "-i", next,
				"-filter_complex",
				fmt.Sprintf("[0:v][1:v]xfade=transition=fadewhite:duration=%.3f:offset=%.3f[v]", transDur, offset),
				"-map", "[v]", "-c:v", "libx264", "-preset", "fast", "-an",
				outPath,
			)
			if out, err := cmd.CombinedOutput(); err != nil {
				return "", fmt.Errorf("flash transition %d failed: %v\n%s", i, err, string(out))
			}
			currentDur = currentDur + nextDur - transDur
		} else {
			// xfade transition (crossfade, fadewhite, fadeblack, wipeleft, slideleft, circlecrop, etc.)
			xfadeType := transition
			if xfadeType == "crossfade" {
				xfadeType = "fade"
			}
			offset := currentDur - transDur
			if offset < 0 {
				offset = 0
			}
			cmd := hiddenCmdCtx(ctx, "ffmpeg", "-y",
				"-i", current, "-i", next,
				"-filter_complex",
				fmt.Sprintf("[0:v][1:v]xfade=transition=%s:duration=%.3f:offset=%.3f[v]", xfadeType, transDur, offset),
				"-map", "[v]", "-c:v", "libx264", "-preset", "fast", "-an",
				outPath,
			)
			if out, err := cmd.CombinedOutput(); err != nil {
				// Fallback to simple concat if xfade fails
				log.Printf("[MVTool] xfade %s failed for scene %d, falling back to concat: %v", xfadeType, i, err)
				listPath := filepath.Join(tmpDir, fmt.Sprintf("list_%03d.txt", i))
				os.WriteFile(listPath, []byte(fmt.Sprintf("file '%s'\nfile '%s'\n", current, next)), 0644)
				cmd2 := hiddenCmdCtx(ctx, "ffmpeg", "-y",
					"-f", "concat", "-safe", "0", "-i", listPath,
					"-c:v", "libx264", "-preset", "fast", "-an",
					outPath,
				)
				if out2, err2 := cmd2.CombinedOutput(); err2 != nil {
					return "", fmt.Errorf("fallback concat %d failed: %v\n%s\n%s", i, err2, string(out), string(out2))
				}
				currentDur = currentDur + nextDur
			} else {
				currentDur = currentDur + nextDur - transDur
			}
		}

		current = outPath
	}

	return current, nil
}

// resolveAudioURL resolves music from music_id or audio_url
func (t *MVTool) resolveAudioURL(args mvArgs, tmpDir string) (string, error) {
	if args.MusicID != "" {
		return ResolveMusicPath(t.db, args.MusicID, tmpDir)
	}
	if args.AudioURL != "" {
		// Local upload path
		if strings.HasPrefix(args.AudioURL, "/v1/uploads/") {
			filename := strings.TrimPrefix(args.AudioURL, "/v1/uploads/")
			localPath := filepath.Join(UploadsDir(), filename)
			if _, err := os.Stat(localPath); err == nil {
				return localPath, nil
			}
		}
		if _, err := os.Stat(args.AudioURL); err == nil {
			return args.AudioURL, nil
		}
		// Music path
		if strings.HasPrefix(args.AudioURL, "/v1/music/") {
			filename := strings.TrimPrefix(args.AudioURL, "/v1/music/")
			localPath := filepath.Join(MusicDir(), filename)
			if _, err := os.Stat(localPath); err == nil {
				return localPath, nil
			}
		}
		if strings.Contains(args.AudioURL, "/v1/music/") {
			idx := strings.Index(args.AudioURL, "/v1/music/")
			localPath := filepath.Join(MusicDir(), args.AudioURL[idx+len("/v1/music/"):])
			if _, err := os.Stat(localPath); err == nil {
				return localPath, nil
			}
		}
		// Relative paths like "data/music/xxx.mp3" that LLMs often generate
		if strings.Contains(args.AudioURL, "music/") {
			idx := strings.LastIndex(args.AudioURL, "music/")
			filename := args.AudioURL[idx+len("music/"):]
			localPath := filepath.Join(MusicDir(), filename)
			if _, err := os.Stat(localPath); err == nil {
				log.Printf("[MVTool] resolved relative audio path %q → %s", args.AudioURL, localPath)
				return localPath, nil
			}
		}
		// Remote URL
		if strings.HasPrefix(args.AudioURL, "http://") || strings.HasPrefix(args.AudioURL, "https://") {
			ext := ".wav"
			if strings.Contains(args.AudioURL, ".mp3") {
				ext = ".mp3"
			}
			dlPath := filepath.Join(tmpDir, "audio"+ext)
			if err := DownloadFile(args.AudioURL, dlPath); err != nil {
				return "", fmt.Errorf("failed to download audio: %v", err)
			}
			return dlPath, nil
		}
		// Direct local path
		if _, err := os.Stat(args.AudioURL); err == nil {
			return args.AudioURL, nil
		}
		return "", fmt.Errorf("cannot resolve audio_url: %s", args.AudioURL)
	}
	return "", fmt.Errorf("music_id or audio_url is required")
}

// resolveVideoRecord flexibly looks up a video by id, task_id, scene label,
// or direct video URL/path. This allows compose_pro to work regardless of
// what identifier the LLM passes as video_id.
func (t *MVTool) resolveVideoRecord(ref, convID, userID string) (model.VideoRecord, error) {
	var video model.VideoRecord

	// 1. Try exact match by record ID (UUID)
	if t.db.Where("id = ?", ref).First(&video).Error == nil {
		if video.Status != "succeeded" {
			return video, fmt.Errorf("video %s status is '%s' (not ready). Use video_generation.check_status to check progress, or use a different video", ref, video.Status)
		}
		return video, nil
	}

	// 2. Try by task_id (fal.ai request_id or DashScope task_id)
	if t.db.Where("task_id = ?", ref).First(&video).Error == nil {
		if video.Status != "succeeded" {
			return video, fmt.Errorf("video task %s status is '%s' (not ready). Use video_generation.check_status to check progress, or use a different video", ref, video.Status)
		}
		return video, nil
	}

	// 3. Try by scene label within the same conversation (e.g. "scene_1", "scene_01")
	if convID != "" {
		q := t.db.Where("conversation_id = ? AND scene = ? AND status = 'succeeded'", convID, ref)
		if userID != "" {
			q = q.Where("user_id = ?", userID)
		}
		if q.Order("created_at DESC").First(&video).Error == nil {
			return video, nil
		}
		// Also try with underscore normalization: "scene_1" ↔ "scene_01"
		normalized := ref
		if parts := strings.SplitN(ref, "_", 2); len(parts) == 2 {
			if n, err := strconv.Atoi(parts[1]); err == nil {
				normalized = fmt.Sprintf("%s_%02d", parts[0], n)
			}
		}
		if normalized != ref {
			q2 := t.db.Where("conversation_id = ? AND scene = ? AND status = 'succeeded'", convID, normalized)
			if userID != "" {
				q2 = q2.Where("user_id = ?", userID)
			}
			if q2.Order("created_at DESC").First(&video).Error == nil {
				return video, nil
			}
		}
	}

	// 4. Try by video_url path (e.g. "/v1/videos/merged/fal_xxx.mp4")
	if strings.HasPrefix(ref, "/v1/videos/") || strings.HasPrefix(ref, "http") {
		if t.db.Where("video_url = ? AND status = 'succeeded'", ref).First(&video).Error == nil {
			return video, nil
		}
	}

	// 5. Try matching by video_url containing the ref as filename
	if t.db.Where("video_url LIKE ? AND status = 'succeeded'", "%"+ref+"%").First(&video).Error == nil {
		return video, nil
	}

	return video, fmt.Errorf("video not found: %s (tried: id, task_id, scene label, video_url)", ref)
}

// suppress unused import
var _ = strconv.Itoa
