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
	"time"

	"gorm.io/gorm"

	"github.com/yinhe/starclaw/internal/model"
)

// MusicTool allows agents to generate music/songs via fal.ai models
type MusicTool struct {
	db *gorm.DB
}

func NewMusicTool(db *gorm.DB) *MusicTool {
	return &MusicTool{db: db}
}

func (t *MusicTool) Name() string { return "music_generation" }

func (t *MusicTool) Description() string {
	return `AI 音乐/歌曲生成工具，通过 fal.ai 平台调用多种音乐模型。支持的模型：
- ace-step: 歌词转歌曲，支持 [verse]/[chorus]/[bridge] 结构标签，时长 5-240 秒，风格通过 tags 控制
- minimax-music-v2: 高质量音乐生成，支持歌词 + 风格描述，专业音质
- diffrhythm: 歌词转歌曲（带时间戳），极快生成，支持 95 秒或 285 秒
- stable-audio: 纯音乐/音效生成，最长 47 秒
操作：generate_music（生成音乐）、check_status（检查状态）、list_music（列出已生成音乐）`
}

func (t *MusicTool) Parameters() interface{} {
	return &JSONSchema{
		Type: "object",
		Properties: map[string]Property{
			"action":   {Type: "string", Description: "Action: generate_music, check_status, list_music"},
			"model":    {Type: "string", Description: "Model: ace-step (default, lyrics+tags, 5-240s), minimax-music-v2 (lyrics+prompt), diffrhythm (timestamped lyrics, 95s/285s), stable-audio (instrumental only, ≀7s)"},
			"prompt":   {Type: "string", Description: "Style/mood description. For ace-step: comma-separated genre tags like 'pop, ballad, chinese, female vocal'. For minimax-music-v2: natural language description. For stable-audio: audio description."},
			"lyrics":   {Type: "string", Description: "Song lyrics. For ace-step: use [verse], [chorus], [bridge] tags. For minimax-music-v2: use [Verse], [Chorus], [Bridge], [Outro] tags. For diffrhythm: use timestamps like [00:10.00]lyrics. Leave empty or use [inst] for instrumental."},
			"duration": {Type: "string", Description: "Duration in seconds. ace-step: 5-240 (default 60). diffrhythm: 95 or 285. stable-audio: 1-47 (default 30)."},
			"music_id": {Type: "string", Description: "For check_status: the music record ID to check."},
		},
		Required: []string{"action"},
	}
}

type musicArgs struct {
	Action   string `json:"action"`
	Model    string `json:"model"`
	Prompt   string `json:"prompt"`
	Lyrics   string `json:"lyrics"`
	Duration string `json:"duration"`
	MusicID  string `json:"music_id"`
}

func (t *MusicTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	var args musicArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %v", err)
	}

	switch args.Action {
	case "generate_music":
		return t.generateMusic(ctx, args)
	case "check_status":
		return t.checkStatus(ctx, args)
	case "list_music":
		return t.listMusic(ctx)
	default:
		return "", fmt.Errorf("unknown action: %s. Use: generate_music, check_status, list_music", args.Action)
	}
}

// getFalAPIKey retrieves the fal.ai API key from the user's model config
func (t *MusicTool) getFalAPIKey(userID string) string {
	return GetFalAPIKey(t.db, userID)
}

// getFalAPIKeyCtx checks StarAI provider first
func (t *MusicTool) getFalAPIKeyCtx(ctx context.Context, userID string) string {
	return GetFalAPIKeyCtx(ctx, t.db, userID)
}

func (t *MusicTool) generateMusic(ctx context.Context, args musicArgs) (string, error) {
	userID := ""
	if uid, ok := ctx.Value(CtxKeyUserID).(string); ok {
		userID = uid
	}
	apiKey := t.getFalAPIKeyCtx(ctx, userID)
	if apiKey == "" {
		return "", fmt.Errorf("no fal.ai API key found. Please configure a fal provider or use StarAI")
	}

	convID := ""
	if cid, ok := ctx.Value(CtxKeyConversationID).(string); ok {
		convID = cid
	}

	if args.Model == "" {
		args.Model = "ace-step"
	}

	// Build model-specific request body
	var endpoint string
	var body map[string]interface{}
	duration := 60

	switch args.Model {
	case "ace-step":
		endpoint = "fal-ai/ace-step"
		if args.Duration != "" {
			fmt.Sscanf(args.Duration, "%d", &duration)
			if duration < 5 {
				duration = 5
			}
			if duration > 240 {
				duration = 240
			}
		}
		body = map[string]interface{}{
			"tags":     args.Prompt,
			"duration": float64(duration),
		}
		if args.Lyrics != "" {
			body["lyrics"] = args.Lyrics
		}

	case "minimax-music-v2":
		endpoint = "fal-ai/minimax-music/v2"
		if args.Prompt == "" {
			return "", fmt.Errorf("prompt is required for minimax-music-v2 (style/mood description)")
		}
		if args.Lyrics == "" {
			return "", fmt.Errorf("lyrics is required for minimax-music-v2")
		}
		body = map[string]interface{}{
			"prompt":        args.Prompt,
			"lyrics_prompt": args.Lyrics,
		}

	case "diffrhythm":
		endpoint = "fal-ai/diffrhythm"
		if args.Lyrics == "" {
			return "", fmt.Errorf("lyrics with timestamps is required for diffrhythm (e.g. [00:10.00]lyrics)")
		}
		dur := "95s"
		if args.Duration == "285" {
			dur = "285s"
			duration = 285
		} else {
			duration = 95
		}
		body = map[string]interface{}{
			"lyrics":         args.Lyrics,
			"music_duration": dur,
		}
		if args.Prompt != "" {
			body["style_prompt"] = args.Prompt
		}

	case "stable-audio":
		endpoint = "fal-ai/stable-audio"
		if args.Prompt == "" {
			return "", fmt.Errorf("prompt is required for stable-audio")
		}
		if args.Duration != "" {
			fmt.Sscanf(args.Duration, "%d", &duration)
			if duration > 47 {
				duration = 47
			}
		} else {
			duration = 30
		}
		body = map[string]interface{}{
			"prompt":        args.Prompt,
			"seconds_total": duration,
		}

	default:
		return "", fmt.Errorf("unsupported model: %s. Use: ace-step, minimax-music-v2, diffrhythm, stable-audio", args.Model)
	}

	// Submit to fal.ai queue API
	requestID, err := t.submitToFal(apiKey, endpoint, body)
	if err != nil {
		return "", fmt.Errorf("failed to submit music generation: %v", err)
	}

	log.Printf("[MusicTool] Submitted %s task: request_id=%s, duration=%ds", args.Model, requestID, duration)

	// Save record
	record := model.MusicRecord{
		UserID:         userID,
		ConversationID: convID,
		RequestID:      requestID,
		Model:          args.Model,
		Prompt:         args.Prompt,
		Lyrics:         args.Lyrics,
		Duration:       duration,
		Status:         "running",
	}
	t.db.Create(&record)

	// Audit log for reconciliation
	genLogID := LogGeneration(t.db, apiKey, GenLogOpts{
		UserID: userID, ConversationID: convID,
		Provider: "fal", Model: args.Model, Type: "audio",
		TaskID: requestID, RecordID: record.ID,
		Prompt: args.Prompt, Status: "running",
	})

	// Poll in background
	go t.pollAndDownload(apiKey, endpoint, requestID, record.ID, userID, genLogID)

	return toJSON(map[string]interface{}{
		"action":     "generate_music",
		"status":     "submitted",
		"model":      args.Model,
		"music_id":   record.ID,
		"request_id": requestID,
		"duration":   duration,
		"message":    fmt.Sprintf("音乐生成已提交（模型: %s, 时长: %d秒）。请稍候，可用 check_status 查看进度。", args.Model, duration),
	}), nil
}

func (t *MusicTool) checkStatus(ctx context.Context, args musicArgs) (string, error) {
	if args.MusicID == "" {
		return "", fmt.Errorf("music_id is required for check_status")
	}

	// Block and poll DB until terminal status or timeout (5 min)
	deadline := time.Now().Add(5 * time.Minute)
	var rec model.MusicRecord
	for time.Now().Before(deadline) {
		if err := t.db.Where("id = ?", args.MusicID).First(&rec).Error; err != nil {
			return "", fmt.Errorf("music record not found: %s", args.MusicID)
		}
		if rec.LocalURL != "" || rec.Status == "failed" || rec.Status == "succeeded" {
			break
		}
		log.Printf("[MusicTool] check_status waiting for %s (status=%s)", args.MusicID, rec.Status)
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}

	result := map[string]interface{}{
		"action":   "check_status",
		"music_id": rec.ID,
		"model":    rec.Model,
		"status":   rec.Status,
	}
	if rec.LocalURL != "" {
		result["audio_url"] = rec.LocalURL
		// Probe actual audio duration with ffprobe
		actualDuration := t.probeAudioDuration(rec.LocalURL)
		if actualDuration > 0 {
			result["duration_seconds"] = actualDuration
			// Update DB if duration changed
			if int(actualDuration) != rec.Duration {
				t.db.Model(&model.MusicRecord{}).Where("id = ?", rec.ID).Update("duration", int(actualDuration))
			}
		} else {
			result["duration_seconds"] = rec.Duration
		}
		result["message"] = fmt.Sprintf("音乐生成完成！模型: %s, 实际时长: %.1f秒, 音频地址: %s。现在可以根据此时长规划视频分镜。", rec.Model, actualDuration, rec.LocalURL)
	} else if rec.Status == "failed" {
		result["error"] = rec.ErrorMsg
		result["message"] = fmt.Sprintf("音乐生成失败: %s", rec.ErrorMsg)
	} else {
		result["message"] = "音乐生成超时，请稍后重试 check_status"
	}
	return toJSON(result), nil
}

// probeAudioDuration uses ffprobe to get the actual duration of a local audio file
func (t *MusicTool) probeAudioDuration(localURL string) float64 {
	// Convert local URL to file path
	filePath := ""
	if strings.HasPrefix(localURL, "/v1/music/") {
		filename := strings.TrimPrefix(localURL, "/v1/music/")
		filePath = "/app/music/" + filename
	}
	if filePath == "" {
		return 0
	}
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return 0
	}

	cmd := exec.CommandContext(context.Background(), "ffprobe", "-v", "quiet",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1", filePath)
	out, err := cmd.Output()
	if err != nil {
		log.Printf("[MusicTool] ffprobe failed for %s: %v", filePath, err)
		return 0
	}
	d, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil {
		return 0
	}
	return d
}

func (t *MusicTool) listMusic(ctx context.Context) (string, error) {
	userID := ""
	if uid, ok := ctx.Value(CtxKeyUserID).(string); ok {
		userID = uid
	}
	convID := ""
	if cid, ok := ctx.Value(CtxKeyConversationID).(string); ok {
		convID = cid
	}

	var records []model.MusicRecord
	q := t.db.Where("user_id = ?", userID)
	if convID != "" {
		q = q.Where("conversation_id = ?", convID)
	}
	q.Order("created_at DESC").Limit(20).Find(&records)

	items := make([]map[string]interface{}, len(records))
	for i, r := range records {
		item := map[string]interface{}{
			"id":     r.ID,
			"model":  r.Model,
			"status": r.Status,
			"prompt": r.Prompt,
		}
		if r.LocalURL != "" {
			item["audio_url"] = r.LocalURL
		}
		items[i] = item
	}
	return toJSON(map[string]interface{}{
		"action": "list_music",
		"count":  len(records),
		"items":  items,
	}), nil
}

// submitToFal submits a request to fal.ai queue API and returns the request_id
func (t *MusicTool) submitToFal(apiKey, endpoint string, body map[string]interface{}) (string, error) {
	bodyBytes, _ := json.Marshal(body)
	url := fmt.Sprintf("https://queue.fal.run/%s", endpoint)

	req, err := http.NewRequest("POST", url, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Key "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("fal API error (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %s", string(respBody))
	}

	requestID, _ := result["request_id"].(string)
	if requestID == "" {
		return "", fmt.Errorf("no request_id in response: %s", string(respBody))
	}
	return requestID, nil
}

// pollAndDownload polls fal.ai queue status and downloads the result
func (t *MusicTool) pollAndDownload(apiKey, endpoint, requestID, recordID, userID, genLogID string) {
	deadline := time.Now().Add(15 * time.Minute)
	interval := 5 * time.Second

	for time.Now().Before(deadline) {
		time.Sleep(interval)

		// Check status
		statusURL := fmt.Sprintf("https://queue.fal.run/%s/requests/%s/status?logs=1", endpoint, requestID)
		req, _ := http.NewRequest("GET", statusURL, nil)
		req.Header.Set("Authorization", "Key "+apiKey)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Printf("[MusicTool] Poll error for %s: %v", requestID, err)
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var status map[string]interface{}
		json.Unmarshal(body, &status)

		statusStr, _ := status["status"].(string)
		log.Printf("[MusicTool] Poll %s: status=%s", requestID, statusStr)

		if statusStr == "COMPLETED" {
			// Fetch result
			audioURL, err := t.fetchResult(apiKey, endpoint, requestID)
			if err != nil {
				log.Printf("[MusicTool] Failed to fetch result for %s: %v", requestID, err)
				t.db.Model(&model.MusicRecord{}).Where("id = ?", recordID).Updates(map[string]interface{}{
					"status":    "failed",
					"error_msg": err.Error(),
				})
				UpdateGenLog(t.db, genLogID, "failed", "", err.Error())
				return
			}

			// Download audio file locally
			localURL := t.downloadAudio(audioURL, recordID)

			updates := map[string]interface{}{
				"status":    "succeeded",
				"audio_url": audioURL,
			}
			if localURL != "" {
				updates["local_url"] = localURL
			}
			t.db.Model(&model.MusicRecord{}).Where("id = ?", recordID).Updates(updates)
			UpdateGenLog(t.db, genLogID, "succeeded", localURL, "")
			log.Printf("[MusicTool] Music %s completed: %s", recordID, localURL)
			return
		}

		if statusStr != "IN_QUEUE" && statusStr != "IN_PROGRESS" {
			// Unknown/error status
			t.db.Model(&model.MusicRecord{}).Where("id = ?", recordID).Updates(map[string]interface{}{
				"status":    "failed",
				"error_msg": fmt.Sprintf("unexpected status: %s", statusStr),
			})
			UpdateGenLog(t.db, genLogID, "failed", "", fmt.Sprintf("unexpected status: %s", statusStr))
			return
		}
	}

	// Timeout
	t.db.Model(&model.MusicRecord{}).Where("id = ?", recordID).Updates(map[string]interface{}{
		"status":    "failed",
		"error_msg": "generation timed out (15 min)",
	})
	UpdateGenLog(t.db, genLogID, "failed", "", "generation timed out (15 min)")
}

// fetchResult gets the completed result from fal.ai
func (t *MusicTool) fetchResult(apiKey, endpoint, requestID string) (string, error) {
	url := fmt.Sprintf("https://queue.fal.run/%s/requests/%s", endpoint, requestID)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Key "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("fal API error (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	json.Unmarshal(body, &result)

	// Different models return audio in different fields
	// ace-step: result.audio.url
	// minimax-music-v2: result.audio.url
	// diffrhythm: result.audio.url
	// stable-audio: result.audio_file.url
	if audio, ok := result["audio"].(map[string]interface{}); ok {
		if url, ok := audio["url"].(string); ok && url != "" {
			return url, nil
		}
	}
	if audioFile, ok := result["audio_file"].(map[string]interface{}); ok {
		if url, ok := audioFile["url"].(string); ok && url != "" {
			return url, nil
		}
	}

	return "", fmt.Errorf("no audio URL in result: %s", string(body))
}

// downloadAudio downloads the audio file to local storage and returns the local serve URL
func (t *MusicTool) downloadAudio(remoteURL, recordID string) string {
	musicDir := "/app/music"
	os.MkdirAll(musicDir, 0755)

	// Determine extension from URL or default to .mp3
	ext := ".mp3"
	if strings.Contains(remoteURL, ".wav") {
		ext = ".wav"
	} else if strings.Contains(remoteURL, ".flac") {
		ext = ".flac"
	}
	filename := recordID + ext
	localPath := filepath.Join(musicDir, filename)

	resp, err := http.Get(remoteURL)
	if err != nil {
		log.Printf("[MusicTool] Failed to download audio: %v", err)
		return ""
	}
	defer resp.Body.Close()

	f, err := os.Create(localPath)
	if err != nil {
		log.Printf("[MusicTool] Failed to create file: %v", err)
		return ""
	}
	defer f.Close()

	written, err := io.Copy(f, resp.Body)
	if err != nil {
		log.Printf("[MusicTool] Failed to write audio: %v", err)
		return ""
	}

	log.Printf("[MusicTool] Downloaded audio: %s (%.1f MB)", localPath, float64(written)/(1024*1024))
	return fmt.Sprintf("/v1/music/%s", filename)
}
