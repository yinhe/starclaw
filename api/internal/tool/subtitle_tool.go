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

// SubtitleTool adds subtitles (SRT) to videos without voiceover.
// Decoupled from DubbingTool for independent use: subtitle-only scenarios,
// foreign-language captions, accessibility, etc.
type SubtitleTool struct {
	db *gorm.DB
}

func NewSubtitleTool(db *gorm.DB) *SubtitleTool {
	return &SubtitleTool{db: db}
}

func (t *SubtitleTool) Name() string { return "subtitle" }

func (t *SubtitleTool) Description() string {
	return `字幕工具，为视频添加 SRT 字幕（不含配音）。适用于静音字幕、外语翻译字幕、无障碍字幕等场景。
操作：
- add_subtitles: 为视频烧录字幕
- generate_srt: 仅生成 SRT 字幕文件（不烧录到视频）

字幕自适应视频方向（横屏/竖屏/方屏），自动调整字号和边距。
样式选项：auto（默认自适应）、small（小字）、large（大字）、none（不显示）。

使用示例：
1. 先用 generate_srt 预览字幕时间轴
2. 满意后用 add_subtitles 烧录到视频

如果需要配音+字幕，请使用 dubbing 工具。`
}

func (t *SubtitleTool) Parameters() interface{} {
	return &JSONSchema{
		Type: "object",
		Properties: map[string]Property{
			"action":         {Type: "string", Description: "Action: add_subtitles, generate_srt"},
			"video_id":       {Type: "string", Description: "Video record ID to add subtitles to (required for add_subtitles)"},
			"narrations":     {Type: "string", Description: "JSON array of subtitle segments: [{\"text\":\"字幕文字\",\"start\":0,\"end\":5},{\"text\":\"第二条\",\"start\":5,\"end\":10}]"},
			"subtitle_style": {Type: "string", Description: "Subtitle style: auto (default), small, large, none"},
		},
		Required: []string{"action", "narrations"},
	}
}

type subtitleArgs struct {
	Action        string `json:"action"`
	VideoID       string `json:"video_id"`
	Narrations    string `json:"narrations"`
	SubtitleStyle string `json:"subtitle_style"`
}

func (t *SubtitleTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	var args subtitleArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %v", err)
	}
	switch args.Action {
	case "add_subtitles":
		return t.addSubtitles(ctx, args)
	case "generate_srt":
		return t.generateSRTOnly(args)
	default:
		return "", fmt.Errorf("unknown action: %s. Use: add_subtitles, generate_srt", args.Action)
	}
}

func (t *SubtitleTool) addSubtitles(ctx context.Context, args subtitleArgs) (string, error) {
	if args.VideoID == "" {
		return "", fmt.Errorf("video_id is required")
	}
	if args.Narrations == "" {
		return "", fmt.Errorf("narrations JSON array is required")
	}

	var segments []NarrationSegment
	if err := json.Unmarshal([]byte(args.Narrations), &segments); err != nil {
		return "", fmt.Errorf("invalid narrations JSON: %v", err)
	}

	userID := ""
	if uid, ok := ctx.Value(CtxKeyUserID).(string); ok {
		userID = uid
	}

	videoRecPtr, err := findVideoRecord(t.db, args.VideoID, userID)
	if err != nil {
		return "", err
	}
	videoRec := *videoRecPtr

	tmpDir, err := os.MkdirTemp("", "subtitles-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	videoPath, err := resolveVideoFile(videoRec.VideoURL, tmpDir, "source.mp4")
	if err != nil {
		return "", err
	}

	subStyle := GetSubtitleStyle(videoPath, args.SubtitleStyle)
	if !subStyle.Enabled {
		return "", fmt.Errorf("subtitle_style is set to 'none'")
	}

	srtPath := filepath.Join(tmpDir, "subtitles.srt")
	if err := GenerateSRT(segments, srtPath); err != nil {
		return "", err
	}

	outputID := uuid.New().String()
	outputDir := "/app/merged_videos"
	os.MkdirAll(outputDir, 0755)
	outputPath := filepath.Join(outputDir, outputID+".mp4")

	escapedSrt := strings.ReplaceAll(srtPath, "\\", "/")
	escapedSrt = strings.ReplaceAll(escapedSrt, ":", "\\:")
	vf := fmt.Sprintf("subtitles='%s':force_style='%s'", escapedSrt, subStyle.ForceStyleString())

	log.Printf("[SubtitleTool] Burning %d subtitles into video %s", len(segments), args.VideoID)
	cmd := exec.CommandContext(ctx, "ffmpeg", "-y", "-i", videoPath,
		"-vf", vf, "-c:v", "libx264", "-preset", "fast", "-c:a", "copy", outputPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("ffmpeg failed: %v\n%s", err, string(out))
	}

	downloadURL := fmt.Sprintf("/v1/videos/merged/%s.mp4", outputID)
	fi, _ := os.Stat(outputPath)
	sizeMB := float64(0)
	if fi != nil {
		sizeMB = float64(fi.Size()) / 1024 / 1024
	}

	// Save subtitled record
	convID := ""
	if cid, ok := ctx.Value(CtxKeyConversationID).(string); ok {
		convID = cid
	}
	sourceIDJSON, _ := json.Marshal([]string{videoRec.ID})
	subtitledRecord := model.VideoRecord{
		UserID: userID, ConversationID: convID,
		Model: "subtitled", Prompt: fmt.Sprintf("字幕: %d条", len(segments)),
		VideoURL: downloadURL, Size: videoRec.Size, Duration: videoRec.Duration,
		Status: "succeeded", Type: "subtitled", ClipIDs: string(sourceIDJSON),
	}
	t.db.Create(&subtitledRecord)

	return toJSON(map[string]interface{}{
		"action": "add_subtitles", "status": "success",
		"video_id": args.VideoID, "segments": len(segments),
		"download_url": downloadURL, "size_mb": fmt.Sprintf("%.1f", sizeMB),
		"message": fmt.Sprintf("字幕添加完成！%d条字幕。", len(segments)),
	}), nil
}

func (t *SubtitleTool) generateSRTOnly(args subtitleArgs) (string, error) {
	if args.Narrations == "" {
		return "", fmt.Errorf("narrations JSON array is required")
	}

	var segments []NarrationSegment
	if err := json.Unmarshal([]byte(args.Narrations), &segments); err != nil {
		return "", fmt.Errorf("invalid narrations JSON: %v", err)
	}

	// Generate SRT content in memory
	var sb strings.Builder
	for i, seg := range segments {
		startH := int(seg.Start) / 3600
		startM := (int(seg.Start) % 3600) / 60
		startS := int(seg.Start) % 60
		startMs := int((seg.Start - float64(int(seg.Start))) * 1000)

		endH := int(seg.End) / 3600
		endM := (int(seg.End) % 3600) / 60
		endS := int(seg.End) % 60
		endMs := int((seg.End - float64(int(seg.End))) * 1000)

		sb.WriteString(fmt.Sprintf("%d\n", i+1))
		sb.WriteString(fmt.Sprintf("%02d:%02d:%02d,%03d --> %02d:%02d:%02d,%03d\n", startH, startM, startS, startMs, endH, endM, endS, endMs))
		sb.WriteString(seg.Text + "\n\n")
	}

	return toJSON(map[string]interface{}{
		"action":   "generate_srt",
		"segments": len(segments),
		"srt":      sb.String(),
		"message":  fmt.Sprintf("已生成 %d 条字幕的 SRT 内容。可用 add_subtitles 烧录到视频。", len(segments)),
	}), nil
}
