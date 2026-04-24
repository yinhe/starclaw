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
	return `配音工具，为视频添加人声旁白（TTS）。支持阿里云 CosyVoice 和火山引擎豆包 2.0 双引擎音色，适用于视频解说、漫剧配音、广告旁白等场景。
操作：
- add_voiceover: 为视频添加配音+字幕（最常用）
- list_voices: 查看所有可用音色

音色分类：
【CosyVoice 阿里云】
- 女声：longyuan（温柔知性）、longxiaochun（活泼甜美）、longshu（故事旁白）、longwan（端庄大气）
- 男声：longhua（沉稳大方）、longjing（播音腔）、longshuo（年轻活力）、longfei（浑厚低沉）
【豆包 2.0 火山引擎】支持 instruction 语音指令控制情绪
- 女声：撒娇学妹(sajiaoxuemei)、甜美桃子(tianmeitaozi)、俏皮女声(qiaopinv)、邻家女孩(linjianvhai)、萌丫头(mengyatou)、高冷御姐(gaolengyujie)、弯弯小何(wanwanxiaohe)
- 男声：小坚(xiaojian)、爽朗少年(shuanglangshaonian)、天才童声(tiancaitongsheng)、小男生(xiaonansheng)

豆包2.0音色支持 instruction 字段：在 narrations 每段中加 "instruction":"用害怕的语气说" 可逐段控制情绪。
配音时自动添加字幕（可通过 subtitle_style=none 关闭）。
如需仅添加字幕不配音，请使用 subtitle 工具。`
}

func (t *DubbingTool) Parameters() interface{} {
	return &JSONSchema{
		Type: "object",
		Properties: map[string]Property{
			"action":         {Type: "string", Description: "Action: add_voiceover, list_voices"},
			"video_id":       {Type: "string", Description: "Video record ID to add voiceover to"},
			"narrations":     {Type: "string", Description: "JSON array of narration segments: [{\"text\":\"旁白文字\",\"start\":0,\"end\":5,\"voice\":\"longyuan\"},{\"text\":\"第二段\",\"start\":5,\"end\":10,\"voice\":\"zh_female_sajiaoxuemei_uranus_bigtts\",\"instruction\":\"用害怕的语气说\"}]"},
			"voice":          {Type: "string", Description: "TTS voice: CosyVoice—longyuan(女,默认)/longxiaochun(女)/longhua(男)/longfei(男); 豆包2.0—zh_female_sajiaoxuemei_uranus_bigtts(撒娇学妹)/zh_female_tianmeitaozi_uranus_bigtts(甜美桃子)/zh_male_xiaojian_mars_bigtts(小坚) 等。用 list_voices 查看完整列表"},
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
	Provider    string `json:"provider,omitempty"` // "dashscope" (default) or "volcengine"
}

var availableVoices = []voiceDef{
	// ── DashScope CosyVoice (阿里云) ──
	{ID: "longyuan", Name: "龙媛", Gender: "女", Style: "温柔知性", Description: "适合旁白解说、故事讲述、知识科普"},
	{ID: "longxiaochun", Name: "龙小淳", Gender: "女", Style: "活泼甜美", Description: "适合年轻化内容、短视频、广告"},
	{ID: "longshu", Name: "龙书", Gender: "女", Style: "故事旁白", Description: "适合有声读物、童话故事"},
	{ID: "longwan", Name: "龙婉", Gender: "女", Style: "端庄大气", Description: "适合纪录片、品牌宣传"},
	{ID: "longhua", Name: "龙华", Gender: "男", Style: "沉稳大方", Description: "适合新闻播报、企业宣传"},
	{ID: "longjing", Name: "龙靖", Gender: "男", Style: "播音腔", Description: "适合正式场合、纪录片旁白"},
	{ID: "longshuo", Name: "龙硕", Gender: "男", Style: "年轻活力", Description: "适合短视频、游戏解说"},
	{ID: "longfei", Name: "龙飞", Gender: "男", Style: "浑厚低沉", Description: "适合电影预告、悬疑内容"},
	// ── Volcengine 豆包语音合成模型 2.0 (火山引擎) ──
	// 支持 context_texts 语音指令控制情绪/语气/风格，通过 narration 的 instruction 字段传递
	{ID: "zh_female_sajiaoxuemei_uranus_bigtts", Name: "撒娇学妹", Gender: "女", Style: "撒娇甜美", Description: "角色扮演型，完美适合傻白甜/可爱/任性型女角", Provider: "volcengine"},
	{ID: "zh_female_tianmeitaozi_uranus_bigtts", Name: "甜美桃子", Gender: "女", Style: "甜美清新", Description: "甜美清新，适合旁白/短视频/广告", Provider: "volcengine"},
	{ID: "zh_female_qiaopinv_uranus_bigtts", Name: "俏皮女声", Gender: "女", Style: "俏皮活泼", Description: "俏皮活泼，适合轻松趣味内容", Provider: "volcengine"},
	{ID: "zh_female_linjianvhai_uranus_bigtts", Name: "邻家女孩", Gender: "女", Style: "邻家亲切", Description: "亲切自然，适合日常/生活场景", Provider: "volcengine"},
	{ID: "zh_female_mengyatou_uranus_bigtts", Name: "萌丫头", Gender: "女", Style: "萌萌可爱", Description: "萌系可爱，适合Q版/动画/儿童", Provider: "volcengine"},
	{ID: "zh_female_gaolengyujie_uranus_bigtts", Name: "高冷御姐", Gender: "女", Style: "高冷成熟", Description: "高冷御姐，适合都市/职场/悬疑", Provider: "volcengine"},
	{ID: "zh_male_xiaojian_mars_bigtts", Name: "小坚", Gender: "男", Style: "沉稳叙述", Description: "沉稳叙述型，适合科技/商业/纪录片", Provider: "volcengine"},
	{ID: "zh_male_shuanglangshaonian_tob", Name: "爽朗少年", Gender: "男", Style: "阳光少年", Description: "阳光爽朗，适合短视频/游戏/青春", Provider: "volcengine"},
	{ID: "zh_male_tiancaitongsheng_uranus_bigtts", Name: "天才童声", Gender: "男", Style: "小男生童声", Description: "8岁聪明小男生，适合AI角色/宠物/吉祥物配音", Provider: "volcengine"},
	{ID: "zh_male_xiaonansheng_uranus_bigtts", Name: "小男生", Gender: "男", Style: "活泼男童", Description: "活泼可爱男童声，适合动画/儿童内容", Provider: "volcengine"},
	{ID: "zh_female_wanwanxiaohe_uranus_bigtts", Name: "弯弯小何", Gender: "女", Style: "温柔姐姐", Description: "温柔邻家姐姐声，适合旁白/故事", Provider: "volcengine"},
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

func displayVoiceName(voice string) string {
	if voice == "" {
		return ""
	}
	for _, v := range availableVoices {
		if v.ID == voice {
			return fmt.Sprintf("%s (%s, %s)", v.Name, v.ID, v.Gender)
		}
	}
	return voice
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
	volcAPIKey := GetVolcengineTTSAPIKey(t.db, userID)

	// Check if any segment uses a Volcengine voice
	needVolc := false
	for _, seg := range segments {
		if isVolcengineVoice(strings.TrimSpace(seg.Voice)) {
			needVolc = true
			break
		}
	}
	if !needVolc && isVolcengineVoice(args.Voice) {
		needVolc = true
	}
	if needVolc && volcAPIKey == "" {
		return "", fmt.Errorf("Volcengine TTS API key required for 豆包音色. Configure provider 'volcengine-tts' in model settings or set VOLCENGINE_TTS_API_KEY")
	}
	if !needVolc && apiKey == "" {
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
	usedVoices := make([]string, 0, len(segments))
	for i, seg := range segments {
		segmentVoice := strings.TrimSpace(seg.Voice)
		if segmentVoice == "" {
			segmentVoice = voice
		}
		audioPath := filepath.Join(tmpDir, fmt.Sprintf("tts_%03d.mp3", i))
		if isVolcengineVoice(segmentVoice) {
			if volcAPIKey == "" {
				return "", fmt.Errorf("TTS segment %d: Volcengine API key required for voice %s", i+1, segmentVoice)
			}
			if err := GenerateVolcengineTTS(volcAPIKey, seg.Text, segmentVoice, seg.Instruction, audioPath); err != nil {
				return "", fmt.Errorf("TTS failed for segment %d: %v", i+1, err)
			}
		} else {
			if err := GenerateTTS(apiKey, seg.Text, segmentVoice, audioPath); err != nil {
				return "", fmt.Errorf("TTS failed for segment %d: %v", i+1, err)
			}
		}
		rawAudioFiles = append(rawAudioFiles, audioPath)
		usedVoices = append(usedVoices, segmentVoice)
	}

	// Uniform speech rate: calculate a single global tempo ratio across all segments
	// so that every clip in the video sounds like it's spoken at the same pace.
	var rawDurations []float64
	var totalRawDur, totalWindowDur float64
	for i, seg := range segments {
		dur := ProbeDuration(rawAudioFiles[i])
		if dur <= 0 {
			dur = seg.End - seg.Start
		}
		rawDurations = append(rawDurations, dur)
		totalRawDur += dur
		window := seg.End - seg.Start
		if window <= 0 {
			window = 3.0
		}
		totalWindowDur += window
	}
	globalRatio := 1.0
	if totalWindowDur > 0 && totalRawDur > totalWindowDur {
		globalRatio = totalRawDur / totalWindowDur
		if globalRatio > maxTempoSpeedup {
			globalRatio = maxTempoSpeedup
		}
	} else if totalWindowDur > 0 && totalRawDur < totalWindowDur*0.5 {
		globalRatio = totalRawDur / totalWindowDur
		if globalRatio < maxTempoSlowdown {
			globalRatio = maxTempoSlowdown
		}
	}
	log.Printf("[DubbingTool] Uniform tempo: totalRaw=%.1fs totalWindow=%.1fs globalRatio=%.3f", totalRawDur, totalWindowDur, globalRatio)

	var audioFiles []string
	var audioDurations []float64
	for i, seg := range segments {
		window := seg.End - seg.Start
		if window <= 0 {
			window = 3.0
		}
		fittedPath, fittedDur, err := FitTTSUniform(rawAudioFiles[i], rawDurations[i], globalRatio, window, tmpDir, i)
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
		// Dynamic ducking: BGM at 70% normally, auto-ducks to ~15-20% when narration plays
		// Uses sidechaincompress with asplit (asplit required: sidechain consumes the stream)
		filterParts = append(filterParts,
			"[narration]asplit=2[narr_mix][narr_sc]",
			"[0:a]volume=0.7[bgaudio]",
			"[bgaudio][narr_sc]sidechaincompress=threshold=0.008:ratio=6:attack=80:release=600:level_sc=1.0:level_in=1.0[ducked]",
			"[ducked][narr_mix]amix=inputs=2:duration=first:dropout_transition=2:normalize=0[finalaudio]",
		)
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

	voiceSummary := displayVoiceName(voice)
	voiceSet := map[string]struct{}{}
	voiceLabels := make([]string, 0, len(usedVoices))
	for _, usedVoice := range usedVoices {
		if _, ok := voiceSet[usedVoice]; ok {
			continue
		}
		voiceSet[usedVoice] = struct{}{}
		voiceLabels = append(voiceLabels, displayVoiceName(usedVoice))
	}
	if len(voiceLabels) > 0 {
		voiceSummary = strings.Join(voiceLabels, " / ")
	}
	log.Printf("[DubbingTool] Running ffmpeg voiceover: %d segments, voice=%s", len(segments), voiceSummary)
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
		Model: "narrated", Prompt: fmt.Sprintf("配音视频 %s (音色: %s, %d段配音)", videoRec.Prompt, voiceSummary, len(segments)),
		VideoURL: downloadURL, Size: videoRec.Size, Duration: videoRec.Duration,
		Status: "succeeded", Type: "narrated", ClipIDs: string(sourceIDJSON),
	}
	t.db.Create(&narratedRecord)

	// Also update narrated_url on original record
	t.db.Model(&model.VideoRecord{}).Where("id = ?", videoRec.ID).Update("narrated_url", downloadURL)

	return toJSON(map[string]interface{}{
		"action": "add_voiceover", "status": "success",
		"video_id": args.VideoID, "narrated_id": narratedRecord.ID,
		"segments": len(segments), "voice": voiceSummary,
		"download_url": downloadURL, "size_mb": fmt.Sprintf("%.1f", sizeMB),
		"message": fmt.Sprintf("配音完成！%d段配音，音色: %s。可在视频画廊查看。", len(segments), voiceSummary),
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
