package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/yinhe/starclaw/internal/model"
)

// ── DashScope Wan Provider ──

func (t *VideoTool) generateVideoWan(ctx context.Context, userID, convID string, args videoArgs, duration int) (string, error) {
	apiKey, baseHost := GetDashScopeAPIKeyCtx(ctx, t.db, userID)
	if apiKey == "" {
		return "", fmt.Errorf("no DashScope API key found. Please configure a qwen model or use StarAI")
	}

	input := map[string]interface{}{"prompt": args.Prompt}
	if args.ImgURL != "" {
		input["img_url"] = args.ImgURL
	}
	body := map[string]interface{}{
		"model": args.Model, "input": input,
		"parameters": map[string]interface{}{"size": args.Size, "duration": duration},
	}
	bodyBytes, _ := json.Marshal(body)

	var reqURL string
	var client *http.Client
	if isStarAIKey(apiKey) {
		reqURL = StarAIProxyURL("dashscope", "/api/v1/services/aigc/video-generation/video-synthesis")
		c, _ := GetStarAIClient()
		if c == nil {
			return "", fmt.Errorf("StarAI proxy not initialized")
		}
		client = c
		log.Printf("[StarAI] Wan submit via proxy: %s", reqURL)
	} else {
		reqURL = fmt.Sprintf("https://%s/api/v1/services/aigc/video-generation/video-synthesis", baseHost)
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, "POST", reqURL, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return "", err
	}
	if !isStarAIKey(apiKey) {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-DashScope-Async", "enable")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("API request failed: %v", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("API error (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	json.Unmarshal(respBody, &result)
	output, _ := result["output"].(map[string]interface{})
	taskID, _ := output["task_id"].(string)
	if taskID == "" {
		return "", fmt.Errorf("no task_id in response: %s", string(respBody))
	}

	log.Printf("[VideoTool] Wan submitted: %s (model=%s, dur=%ds, scene=%s)", taskID, args.Model, duration, args.Scene)

	category := args.Category
	if category == "" {
		category = "general"
	}
	record := model.VideoRecord{
		UserID: userID, ConversationID: convID, TaskID: taskID,
		Model: args.Model, Prompt: args.Prompt, ImgURL: args.ImgURL,
		Size: args.Size, Duration: duration, Scene: args.Scene, Status: "running",
		Category: category,
	}
	t.db.Create(&record)
	if args.Scene != "" {
		t.ensureVideoWorkflow(userID, convID)
	}

	// Audit log for reconciliation
	genLogID := LogGeneration(t.db, apiKey, GenLogOpts{
		UserID: userID, ConversationID: convID,
		Provider: "dashscope", Model: args.Model, Type: "video",
		TaskID: taskID, RecordID: record.ID,
		Prompt: args.Prompt, Status: "running",
	})

	go func() {
		videoURL, _, err := t.pollDashScopeTask(context.Background(), apiKey, baseHost, taskID, 10*time.Minute)
		if err != nil {
			log.Printf("[VideoTool] Task %s failed: %v", taskID, err)
			t.db.Model(&model.VideoRecord{}).Where("task_id = ?", taskID).Updates(map[string]interface{}{"status": "failed"})
			UpdateGenLog(t.db, genLogID, "failed", "", err.Error())

			// Auto-fallback: retry with a faster model
			if fallback, ok := videoFallbackChain[args.Model]; ok {
				log.Printf("[VideoTool] Auto-fallback: %s → %s for prompt=%q", args.Model, fallback, args.Prompt[:min(len(args.Prompt), 60)])
				fallbackArgs := args
				fallbackArgs.Model = fallback
				go func() {
					if _, retryErr := t.generateVideo(context.Background(), fallbackArgs); retryErr != nil {
						log.Printf("[VideoTool] Fallback %s also failed: %v", fallback, retryErr)
					}
				}()
			}
			return
		}
		// Save clip locally to prevent CDN URL expiration during merge
		savedURL := videoURL
		if localURL, err := SaveClipLocally(videoURL); err == nil {
			savedURL = localURL
		} else {
			log.Printf("[VideoTool] Task %s: local save failed (will use CDN URL): %v", taskID, err)
		}
		t.db.Model(&model.VideoRecord{}).Where("task_id = ?", taskID).Updates(map[string]interface{}{
			"video_url": savedURL, "status": "succeeded",
		})
		UpdateGenLog(t.db, genLogID, "succeeded", savedURL, "")
		var rec model.VideoRecord
		if t.db.Where("task_id = ?", taskID).First(&rec).Error == nil {
			ExtractThumbnail(t.db, rec.ID, savedURL)
		}
		if convID != "" {
			t.TryAutoMerge(userID, convID)
		}
	}()

	return toJSON(map[string]interface{}{
		"action": "generate_video", "status": "submitted", "task_id": taskID,
		"model": args.Model, "scene": args.Scene,
		"message": fmt.Sprintf("视频任务已提交。任务ID: %s，模型: %s。可用 check_status 查看进度。", taskID, args.Model),
	}), nil
}

// ── fal.ai Provider ──

func (t *VideoTool) generateVideoFal(ctx context.Context, userID, convID string, args videoArgs, duration int) (string, error) {
	apiKey := GetFalAPIKeyCtx(ctx, t.db, userID)
	if apiKey == "" {
		return "", fmt.Errorf("no fal.ai API key found. Please configure a fal provider or use StarAI")
	}
	endpoint, ok := falVideoEndpoints[args.Model]
	if !ok {
		return "", fmt.Errorf("unknown fal.ai video model: %s", args.Model)
	}

	// Switch to i2v endpoint when image is provided
	if args.Model == "kling-v3" && args.ImgURL != "" {
		endpoint = "fal-ai/kling-video/v3/pro/image-to-video"
	}
	if args.Model == "luma" && args.ImgURL != "" {
		endpoint = "fal-ai/luma-dream-machine/ray-2/image-to-video"
	}
	if args.Model == "sora2" && args.ImgURL != "" {
		endpoint = "fal-ai/sora-2/image-to-video/pro"
	}
	if args.Model == "veo3.1" && args.ImgURL != "" {
		endpoint = "fal-ai/veo3.1/image-to-video"
	}

	body := map[string]interface{}{
		"prompt": args.Prompt,
	}

	// Per-model duration formatting
	switch args.Model {
	case "kling-v3":
		// kling-v3 accepts "3" through "15" as string
		if duration < 3 {
			duration = 5
		}
		if duration > 15 {
			duration = 15
		}
		body["duration"] = fmt.Sprintf("%d", duration)
		body["generate_audio"] = true
		body["negative_prompt"] = "blur, distort, and low quality"
		body["cfg_scale"] = 0.5
	case "veo3":
		// veo3 auto-determines duration, don't send it
	case "veo3.1":
		// veo3.1 accepts "4s", "6s", "8s"
		if duration <= 4 {
			body["duration"] = "4s"
		} else if duration <= 6 {
			body["duration"] = "6s"
		} else {
			body["duration"] = "8s"
		}
		body["generate_audio"] = true
	case "sora2":
		// sora2 accepts "5s", "10s", "15s", "20s"
		switch {
		case duration <= 5:
			body["duration"] = "5s"
		case duration <= 10:
			body["duration"] = "10s"
		case duration <= 15:
			body["duration"] = "15s"
		default:
			body["duration"] = "20s"
		}
		body["generate_audio"] = true
	case "luma":
		// Luma Ray-2: uses aspect_ratio, no duration/width/height
		delete(body, "duration")
	default:
		body["duration"] = fmt.Sprintf("%d", duration)
	}

	if args.ImgURL != "" {
		switch args.Model {
		case "kling-v3":
			body["start_image_url"] = args.ImgURL
		case "luma":
			body["image_url"] = args.ImgURL
		default:
			body["image_url"] = args.ImgURL
		}
	}

	// Per-model resolution/aspect_ratio handling
	switch args.Model {
	case "kling-v3", "luma", "veo3.1", "sora2":
		// These models use aspect_ratio instead of width/height
		sizeParts := strings.Split(args.Size, "*")
		if len(sizeParts) == 2 {
			w, _ := strconv.Atoi(sizeParts[0])
			h, _ := strconv.Atoi(sizeParts[1])
			if w > h {
				body["aspect_ratio"] = "16:9"
			} else if h > w {
				body["aspect_ratio"] = "9:16"
			} else {
				body["aspect_ratio"] = "1:1"
			}
		}
	default:
		sizeParts := strings.Split(args.Size, "*")
		if len(sizeParts) == 2 {
			if w, err := strconv.Atoi(sizeParts[0]); err == nil {
				body["width"] = w
			}
			if h, err := strconv.Atoi(sizeParts[1]); err == nil {
				body["height"] = h
			}
		}
	}

	requestID, _, err := SubmitToFal(apiKey, endpoint, body)
	if err != nil {
		return "", fmt.Errorf("fal.ai submit failed: %v", err)
	}

	log.Printf("[VideoTool] fal.ai submitted: %s (model=%s, scene=%s)", requestID, args.Model, args.Scene)

	falCategory := args.Category
	if falCategory == "" {
		falCategory = "general"
	}
	record := model.VideoRecord{
		UserID: userID, ConversationID: convID, TaskID: requestID,
		Model: args.Model, Prompt: args.Prompt, ImgURL: args.ImgURL,
		Size: args.Size, Duration: duration, Scene: args.Scene, Status: "running",
		Category: falCategory,
	}
	t.db.Create(&record)
	if args.Scene != "" {
		t.ensureVideoWorkflow(userID, convID)
	}

	// Audit log for reconciliation
	genLogID := LogGeneration(t.db, apiKey, GenLogOpts{
		UserID: userID, ConversationID: convID,
		Provider: "fal", Model: args.Model, Type: "video",
		TaskID: requestID, RecordID: record.ID,
		Prompt: args.Prompt, Status: "running",
	})

	go func() {
		log.Printf("[VideoTool] fal.ai polling started: %s (model=%s, endpoint=%s)", requestID, args.Model, endpoint)
		result, err := PollFalStatus(apiKey, endpoint, requestID, 10*time.Minute)
		if err != nil {
			log.Printf("[VideoTool] fal.ai %s polling failed: %v", requestID, err)
			t.db.Model(&model.VideoRecord{}).Where("task_id = ?", requestID).Updates(map[string]interface{}{"status": "failed"})
			UpdateGenLog(t.db, genLogID, "failed", "", err.Error())

			// Auto-fallback: retry with a faster model
			if fallback, ok := videoFallbackChain[args.Model]; ok {
				log.Printf("[VideoTool] Auto-fallback: %s → %s for prompt=%q", args.Model, fallback, args.Prompt[:min(len(args.Prompt), 60)])
				fallbackArgs := args
				fallbackArgs.Model = fallback
				go func() {
					if _, retryErr := t.generateVideo(context.Background(), fallbackArgs); retryErr != nil {
						log.Printf("[VideoTool] Fallback %s also failed: %v", fallback, retryErr)
					}
				}()
			}
			return
		}
		log.Printf("[VideoTool] fal.ai %s poll completed, extracting video URL", requestID)
		videoURL := extractFalVideoURL(result)
		if videoURL == "" {
			log.Printf("[VideoTool] fal.ai %s: extractFalVideoURL returned empty, marking failed", requestID)
			t.db.Model(&model.VideoRecord{}).Where("task_id = ?", requestID).Updates(map[string]interface{}{"status": "failed"})
			UpdateGenLog(t.db, genLogID, "failed", "", "no video URL in result")
			return
		}
		log.Printf("[VideoTool] fal.ai %s: got video URL, downloading to local...", requestID)
		// Download to local storage (clips go to videos/ dir)
		outputDir := VideosDir()
		localFile := fmt.Sprintf("fal_%s.mp4", uuid.New().String()[:8])
		localPath := filepath.Join(outputDir, localFile)
		savedURL := videoURL
		if err := DownloadFile(videoURL, localPath); err == nil {
			savedURL = VideoClipURL(localFile)
		}
		t.db.Model(&model.VideoRecord{}).Where("task_id = ?", requestID).Updates(map[string]interface{}{
			"video_url": savedURL, "status": "succeeded",
		})
		UpdateGenLog(t.db, genLogID, "succeeded", savedURL, "")
		var rec model.VideoRecord
		if t.db.Where("task_id = ?", requestID).First(&rec).Error == nil {
			ExtractThumbnail(t.db, rec.ID, savedURL)
		}
		if convID != "" {
			t.TryAutoMerge(userID, convID)
		}
	}()

	return toJSON(map[string]interface{}{
		"action": "generate_video", "status": "submitted", "task_id": requestID,
		"model": args.Model, "scene": args.Scene,
		"message": fmt.Sprintf("视频任务已提交。任务ID: %s，模型: %s。可用 check_status 查看进度。", requestID, args.Model),
	}), nil
}

func extractFalVideoURL(result map[string]interface{}) string {
	if video, ok := result["video"].(map[string]interface{}); ok {
		if u, ok := video["url"].(string); ok {
			return u
		}
	}
	if u, ok := result["video_url"].(string); ok {
		return u
	}
	if output, ok := result["output"].(map[string]interface{}); ok {
		if u, ok := output["url"].(string); ok {
			return u
		}
	}
	if videos, ok := result["videos"].([]interface{}); ok && len(videos) > 0 {
		if v, ok := videos[0].(map[string]interface{}); ok {
			if u, ok := v["url"].(string); ok {
				return u
			}
		}
	}
	// Debug: log the full result structure when no video URL found
	resJSON, _ := json.Marshal(result)
	log.Printf("[VideoTool] extractFalVideoURL: no video URL found in result keys=%v snippet=%.500s", keysOf(result), string(resJSON))
	return ""
}

func keysOf(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
