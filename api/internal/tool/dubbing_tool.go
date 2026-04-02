package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/yinhe/starclaw/internal/model"
	"gorm.io/gorm"
)

// findVideoRecord flexibly looks up a video record by id, task_id, or video_url.
func findVideoRecord(db *gorm.DB, videoID, userID string) (*model.VideoRecord, error) {
	var rec model.VideoRecord
	// 1) By primary ID
	if err := db.Where("id = ? AND user_id = ?", videoID, userID).First(&rec).Error; err == nil {
		return &rec, nil
	}
	// 2) By task_id
	if err := db.Where("task_id = ? AND user_id = ?", videoID, userID).First(&rec).Error; err == nil {
		return &rec, nil
	}
	// 3) By video_url variants
	urlVariants := []string{videoID}
	if !strings.HasPrefix(videoID, "/") {
		urlVariants = append(urlVariants, "/v1/videos/merged/"+videoID+".mp4", "/v1/videos/merged/"+videoID)
	}
	for _, u := range urlVariants {
		if err := db.Where("video_url = ? AND user_id = ?", u, userID).First(&rec).Error; err == nil {
			return &rec, nil
		}
	}
	// 4) Fallback: without user_id filter (for merged videos created by system)
	if err := db.Where("id = ? OR task_id = ?", videoID, videoID).First(&rec).Error; err == nil {
		return &rec, nil
	}
	return nil, fmt.Errorf("video not found: %s", videoID)
}

// DubbingTool adds voiceover (TTS) and subtitles to videos.
// Supports multiple voices from Alibaba Cloud CosyVoice.
type DubbingTool struct {
	db *gorm.DB
}

func NewDubbingTool(db *gorm.DB) *DubbingTool {
	return &DubbingTool{db: db}
}

func (t *DubbingTool) Name() string { return "dubbing" }

func (t *DubbingTool) Description() string {
	return `配音工具，为视频添加人声旁白（TTS）。支持阿里云 CosyVoice 多种音色，适用于视频解说、漫剧配音、广告旁白等场景。
操作：
- add_voiceover: 为视频添加配音+字幕（最常用）
- list_voices: 查看所有可用音色

音色分类：
- 女声：longyuan（温柔知性）、longxiaochun（活泼甜美）、longshu（故事旁白）、longwan（端庄大气）
- 男声：longhua（沉稳大方）、longjing（播音腔）、longshuo（年轻活力）、longfei（浑厚低沉）

配音时自动添加字幕（可通过 subtitle_style=none 关闭）。
如需仅添加字幕不配音，请使用 subtitle 工具。`
}

func (t *DubbingTool) Parameters() interface{} {
	return &JSONSchema{
		Type: "object",
		Properties: map[string]Property{
			"action":         {Type: "string", Description: "Action: add_voiceover, list_voices"},
			"video_id":       {Type: "string", Description: "Video record ID to add voiceover to"},
			"narrations":     {Type: "string", Description: "JSON array of narration segments: [{\"text\":\"旁白文字\",\"start\":0,\"end\":5},{\"text\":\"第二段\",\"start\":5,\"end\":10}]"},
			"voice":          {Type: "string", Description: "TTS voice: longyuan(女,默认), longxiaochun(女), longshu(女), longwan(女), longhua(男), longjing(男,播音), longshuo(男), longfei(男)"},
			"subtitle_style": {Type: "string", Description: "Subtitle style: auto (default), small, large, none"},
		},
		Required: []string{"action"},
	}
}

type dubbingArgs struct {
	Action        string `json:"action"`
	VideoID       string `json:"video_id"`
	Narrations    string `json:"narrations"`
	Voice         string `json:"voice"`
	SubtitleStyle string `json:"subtitle_style"`
}

// Voice definitions
type voiceDef struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Gender      string `json:"gender"`
	Style       string `json:"style"`
	Description string `json:"description"`
}

var availableVoices = []voiceDef{
	{ID: "longyuan", Name: "龙媛", Gender: "女", Style: "温柔知性", Description: "适合旁白解说、故事讲述、知识科普"},
	{ID: "longxiaochun", Name: "龙小淳", Gender: "女", Style: "活泼甜美", Description: "适合年轻化内容、短视频、广告"},
	{ID: "longshu", Name: "龙书", Gender: "女", Style: "故事旁白", Description: "适合有声读物、童话故事"},
	{ID: "longwan", Name: "龙婉", Gender: "女", Style: "端庄大气", Description: "适合纪录片、品牌宣传"},
	{ID: "longhua", Name: "龙华", Gender: "男", Style: "沉稳大方", Description: "适合新闻播报、企业宣传"},
	{ID: "longjing", Name: "龙靖", Gender: "男", Style: "播音腔", Description: "适合正式场合、纪录片旁白"},
	{ID: "longshuo", Name: "龙硕", Gender: "男", Style: "年轻活力", Description: "适合短视频、游戏解说"},
	{ID: "longfei", Name: "龙飞", Gender: "男", Style: "浑厚低沉", Description: "适合电影预告、悬疑内容"},
}

func (t *DubbingTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	var args dubbingArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %v", err)
	}
	switch args.Action {
	case "add_voiceover":
		return t.addVoiceover(ctx, args)
	case "list_voices":
		return t.listVoices()
	default:
		return "", fmt.Errorf("unknown action: %s. Use: add_voiceover, list_voices. For subtitle-only, use the subtitle tool.", args.Action)
	}
}

func (t *DubbingTool) listVoices() (string, error) {
	return toJSON(map[string]interface{}{
		"action": "list_voices",
		"voices": availableVoices,
		"tips":   "默认音色: longyuan（女声，温柔知性）。可根据内容风格选择合适的音色。",
	}), nil
}

func (t *DubbingTool) addVoiceover(ctx context.Context, args dubbingArgs) (string, error) {
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
	if len(segments) == 0 {
		return "", fmt.Errorf("narrations array is empty")
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
	if videoRec.VideoURL == "" {
		return "", fmt.Errorf("video has no URL (not yet completed?)")
	}

	apiKey, _ := GetDashScopeAPIKeyCtx(ctx, t.db, userID)
	if apiKey == "" {
		return "", fmt.Errorf("no DashScope API key found for TTS. Please use StarAI channel")
	}

	voice := args.Voice
	if voice == "" {
		voice = "longyuan"
	}

	tmpDir, err := os.MkdirTemp("", "dubbing-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Download source video
	videoPath, err := resolveVideoFile(videoRec.VideoURL, tmpDir, "source.mp4")
	if err != nil {
		return "", err
	}

	// Generate TTS audio for each segment
	var rawAudioFiles []string
	for i, seg := range segments {
		audioPath := filepath.Join(tmpDir, fmt.Sprintf("tts_%03d.mp3", i))
		if err := GenerateTTS(apiKey, seg.Text, voice, audioPath); err != nil {
			return "", fmt.Errorf("TTS failed for segment %d: %v", i+1, err)
		}
		rawAudioFiles = append(rawAudioFiles, audioPath)
	}

	// Fit TTS audio to segment windows (prevent overlap, adjust speed)
	var audioFiles []string
	var audioDurations []float64
	for i, seg := range segments {
		window := seg.End - seg.Start
		if window <= 0 {
			window = 3.0 // fallback 3s window
		}
		fittedPath, fittedDur, err := FitTTSToWindow(rawAudioFiles[i], window, tmpDir, i)
		if err != nil {
			log.Printf("[DubbingTool] FitTTS warning seg %d: %v, using raw audio", i, err)
			fittedPath = rawAudioFiles[i]
			fittedDur = window
		}
		audioFiles = append(audioFiles, fittedPath)
		audioDurations = append(audioDurations, fittedDur)
	}

	// Recalculate segment timings based on actual fitted TTS durations
	totalDur := float64(videoRec.Duration)
	if totalDur <= 0 {
		totalDur = 30 // fallback
	}
	fittedSegments := FitNarrationSegments(segments, audioDurations, totalDur)

	// Generate SRT subtitle file from fitted timings
	subStyle := GetSubtitleStyle(videoPath, args.SubtitleStyle)
	srtPath := filepath.Join(tmpDir, "subtitles.srt")
	if subStyle.Enabled {
		if err := GenerateSRT(fittedSegments, srtPath); err != nil {
			return "", fmt.Errorf("failed to generate subtitles: %v", err)
		}
	}

	// Check if source has audio
	hasAudio := ProbeHasAudio(videoPath)

	// Build filter_complex
	var filterParts []string

	// Video: burn subtitles
	if subStyle.Enabled {
		escapedSrt := strings.ReplaceAll(srtPath, "\\", "/")
		escapedSrt = strings.ReplaceAll(escapedSrt, ":", "\\:")
		filterParts = append(filterParts,
			fmt.Sprintf("[0:v]subtitles='%s':force_style='%s'[finalvideo]", escapedSrt, subStyle.ForceStyleString()))
	} else {
		filterParts = append(filterParts, "[0:v]copy[finalvideo]")
	}

	// Audio: create silent base + delay each TTS + mix
	filterParts = append(filterParts, fmt.Sprintf("anullsrc=r=44100:cl=stereo:d=%d[silence]", videoRec.Duration+5))
	mixInputs := "[silence]"
	inputArgs := []string{"-y", "-i", videoPath}

	for i, seg := range fittedSegments {
		inputArgs = append(inputArgs, "-i", audioFiles[i])
		delayMs := int(seg.Start * 1000)
		filterParts = append(filterParts, fmt.Sprintf("[%d]adelay=%d|%d[a%d]", i+1, delayMs, delayMs, i))
		mixInputs += fmt.Sprintf("[a%d]", i)
	}

	filterParts = append(filterParts, fmt.Sprintf("%samix=inputs=%d:duration=first:normalize=0[narration_raw]", mixInputs, len(segments)+1))
	// Boost narration volume (normalize=0 keeps original levels, but we add a safety boost)
	filterParts = append(filterParts, "[narration_raw]volume=1.8[narration]")

	if hasAudio {
		filterParts = append(filterParts, "[0:a]volume=0.25[bgaudio];[bgaudio][narration]amix=inputs=2:duration=first:normalize=0[finalaudio]")
	} else {
		filterParts = append(filterParts, "[narration]acopy[finalaudio]")
	}

	filterComplex := strings.Join(filterParts, ";")

	// Output
	outputID := uuid.New().String()
	outputDir := MergedVideosDir()
	outputPath := filepath.Join(outputDir, outputID+".mp4")

	ffmpegArgs := append(inputArgs,
		"-filter_complex", filterComplex,
		"-map", "[finalvideo]", "-map", "[finalaudio]",
		"-c:v", "libx264", "-preset", "fast",
		"-c:a", "aac", "-b:a", "192k",
		"-shortest", outputPath,
	)

	log.Printf("[DubbingTool] Running ffmpeg voiceover: %d segments, voice=%s", len(segments), voice)
	cmd := hiddenCmdCtx(ctx, "ffmpeg", ffmpegArgs...)
	cmdOutput, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("ffmpeg failed: %v\n%s", err, string(cmdOutput))
	}

	fi, _ := os.Stat(outputPath)
	sizeMB := float64(0)
	if fi != nil {
		sizeMB = float64(fi.Size()) / 1024 / 1024
	}
	downloadURL := fmt.Sprintf("/v1/videos/merged/%s.mp4", outputID)

	// Save narrated record
	convID := ""
	if cid, ok := ctx.Value(CtxKeyConversationID).(string); ok {
		convID = cid
	}
	sourceIDJSON, _ := json.Marshal([]string{videoRec.ID})
	narratedRecord := model.VideoRecord{
		UserID: userID, ConversationID: convID,
		Model: "narrated", Prompt: fmt.Sprintf("配音牀 %s (音色: %s, %d段旁癀", videoRec.Prompt, voice, len(segments)),
		VideoURL: downloadURL, Size: videoRec.Size, Duration: videoRec.Duration,
		Status: "succeeded", Type: "narrated", ClipIDs: string(sourceIDJSON),
	}
	t.db.Create(&narratedRecord)

	// Also update narrated_url on original record
	t.db.Model(&model.VideoRecord{}).Where("id = ?", videoRec.ID).Update("narrated_url", downloadURL)

	voiceName := voice
	for _, v := range availableVoices {
		if v.ID == voice {
			voiceName = fmt.Sprintf("%s (%s, %s)", v.Name, v.ID, v.Gender)
			break
		}
	}

	return toJSON(map[string]interface{}{
		"action": "add_voiceover", "status": "success",
		"video_id": args.VideoID, "narrated_id": narratedRecord.ID,
		"segments": len(segments), "voice": voiceName,
		"download_url": downloadURL, "size_mb": fmt.Sprintf("%.1f", sizeMB),
		"message": fmt.Sprintf("配音完成！%d段旁白，音色: %s。可在视频画廊查看。", len(segments), voiceName),
	}), nil
}

// resolveVideoFile resolves a video URL to a local file path
func resolveVideoFile(videoURL, tmpDir, filename string) (string, error) {
	if strings.HasPrefix(videoURL, "/v1/videos/clips/") {
		fn := strings.TrimPrefix(videoURL, "/v1/videos/clips/")
		localPath := filepath.Join(VideosDir(), fn)
		if _, err := os.Stat(localPath); err == nil {
			return localPath, nil
		}
	}
	if strings.HasPrefix(videoURL, "/v1/videos/merged/") {
		fn := strings.TrimPrefix(videoURL, "/v1/videos/merged/")
		localPath := filepath.Join(MergedVideosDir(), fn)
		if _, err := os.Stat(localPath); err == nil {
			return localPath, nil
		}
	}
	dest := filepath.Join(tmpDir, filename)
	if err := DownloadFile(videoURL, dest); err != nil {
		return "", fmt.Errorf("failed to download video: %v", err)
	}
	return dest, nil
}
