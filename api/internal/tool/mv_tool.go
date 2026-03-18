package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
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
	return `MV（音乐视频）合成工具，将视频画面与音乐轨道合成为最终MV。支持歌词字幕烧录，自适应视频方向。
操作：compose_mv
使用前需要先用 music_generation 生成音乐，用 video_generation 生成视频片段。compose_mv 会将视频的原始音频替换为音乐，可选添加歌词字幕。`
}

func (t *MVTool) Parameters() interface{} {
	return &JSONSchema{
		Type: "object",
		Properties: map[string]Property{
			"action":         {Type: "string", Description: "Action: compose_mv"},
			"music_id":       {Type: "string", Description: "Music record ID from music_generation tool (required)"},
			"video_id":       {Type: "string", Description: "Specific video record ID to use. If empty, merges all clips in current conversation."},
			"lyrics_srt":     {Type: "string", Description: "SRT-format lyrics for subtitle burning. Optional."},
			"subtitle_style": {Type: "string", Description: "Subtitle style: auto (default), small, large, none"},
		},
		Required: []string{"action"},
	}
}

type mvArgs struct {
	Action        string `json:"action"`
	MusicID       string `json:"music_id"`
	VideoID       string `json:"video_id"`
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
	default:
		return "", fmt.Errorf("unknown action: %s. Use: compose_mv", args.Action)
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
			videoPath = "/app/merged_videos/" + fn
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
			cmd := exec.CommandContext(ctx, "ffmpeg", "-y", "-f", "concat", "-safe", "0",
				"-i", listPath, "-c", "copy", videoPath)
			if out, err := cmd.CombinedOutput(); err != nil {
				cmd2 := exec.CommandContext(ctx, "ffmpeg", "-y", "-f", "concat", "-safe", "0",
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
	outputDir := "/app/merged_videos"
	os.MkdirAll(outputDir, 0755)
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
	cmd := exec.CommandContext(ctx, "ffmpeg", ffmpegArgs...)
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
