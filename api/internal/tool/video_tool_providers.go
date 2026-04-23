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

	isWan27 := strings.HasPrefix(args.Model, "wan2.7")

	input := map[string]interface{}{"prompt": args.Prompt}
	params := map[string]interface{}{"duration": duration}

	if isWan27 {
		// wan2.7 uses media[] array for image/audio inputs + resolution parameter
		if args.ImgURL != "" {
			media := []map[string]interface{}{{"type": "first_frame", "url": args.ImgURL}}
			if args.RefAudioURL != "" {
				media = append(media, map[string]interface{}{"type": "driving_audio", "url": args.RefAudioURL})
			}
			input["media"] = media
		}
		res := args.Resolution
		if res == "" {
			res = "720P"
		}
		params["resolution"] = res
		params["prompt_extend"] = true
	} else {
		// wan2.6 uses img_url + size
		if args.ImgURL != "" {
			input["img_url"] = args.ImgURL
		}
		params["size"] = args.Size
	}

	body := map[string]interface{}{
		"model": args.Model, "input": input,
		"parameters": params,
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

func (t *VideoTool) generateVideoSeedance(ctx context.Context, userID, convID string, args videoArgs, duration int) (string, error) {
	apiKey, baseURL := GetVolcengineAPIKeyCtx(ctx, t.db, userID)
	if apiKey == "" {
		return "", fmt.Errorf("no Volcengine API key found. Please configure volcengine or use StarAI")
	}
	// args.Model is already the exact Volcengine model name (e.g. doubao-seedance-2-0-fast-260128)
	if duration == 0 || duration < -1 {
		duration = 5
	}
	content := []map[string]interface{}{{"type": "text", "text": args.Prompt}}
	if args.ImgURL != "" {
		for _, u := range parseImageURLList(args.ImgURL) {
			if strings.TrimSpace(u) == "" {
				continue
			}
			resolvedURL, err := ResolveImageInputForProvider(u)
			if err != nil {
				return "", fmt.Errorf("invalid Seedance image input %q: %v", u, err)
			}
			content = append(content, map[string]interface{}{
				"role": "reference_image",
				"type": "image_url",
				"image_url": map[string]interface{}{
					"url": resolvedURL,
				},
			})
		}
	}
	if args.RefVideoURL != "" {
		for _, u := range parseImageURLList(args.RefVideoURL) {
			if strings.TrimSpace(u) == "" {
				continue
			}
			content = append(content, map[string]interface{}{
				"role": "reference_video",
				"type": "video_url",
				"video_url": map[string]interface{}{
					"url": strings.TrimSpace(u),
				},
			})
		}
	}
	if args.RefAudioURL != "" {
		for _, u := range parseImageURLList(args.RefAudioURL) {
			if strings.TrimSpace(u) == "" {
				continue
			}
			content = append(content, map[string]interface{}{
				"role": "reference_audio",
				"type": "audio_url",
				"audio_url": map[string]interface{}{
					"url": strings.TrimSpace(u),
				},
			})
		}
	}
	body := map[string]interface{}{
		"model":      args.Model,
		"content":    content,
		"ratio":      aspectRatioFromSize(args.Size),
		"resolution": resolutionFromSize(args.Size),
		"duration":   duration,
	}
	if args.GenerateAudio != nil {
		body["generate_audio"] = *args.GenerateAudio
	}
	if args.Watermark != nil {
		body["watermark"] = *args.Watermark
	}
	if args.ReturnLastFrame != nil {
		body["return_last_frame"] = *args.ReturnLastFrame
	}
	bodyBytes, _ := json.Marshal(body)

	var reqURL string
	var client *http.Client
	if isStarAIKey(apiKey) {
		reqURL = StarAIProxyURL("volcengine", "/contents/generations/tasks")
		c, _ := GetStarAIClient()
		if c == nil {
			return "", fmt.Errorf("StarAI proxy not initialized")
		}
		client = c
		log.Printf("[StarAI] Seedance submit via proxy: %s", reqURL)
	} else {
		reqURL = strings.TrimRight(baseURL, "/") + "/contents/generations/tasks"
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

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("Seedance API request failed: %v", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("Seedance API error (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	json.Unmarshal(respBody, &result)
	taskID, _ := result["id"].(string)
	if taskID == "" {
		if v, ok := result["task_id"].(string); ok {
			taskID = v
		}
	}
	if taskID == "" {
		return "", fmt.Errorf("no task id in Seedance response: %s", string(respBody))
	}

	log.Printf("[VideoTool] Seedance submitted: %s (model=%s, dur=%ds, scene=%s)", taskID, args.Model, duration, args.Scene)

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

	genLogID := LogGeneration(t.db, apiKey, GenLogOpts{
		UserID: userID, ConversationID: convID,
		Provider: "volcengine", Model: args.Model, Type: "video",
		TaskID: taskID, RecordID: record.ID,
		Prompt: args.Prompt, Status: "running",
	})

	go func() {
		videoURL, err := t.pollVolcengineTask(context.Background(), apiKey, baseURL, taskID, 20*time.Minute)
		if err != nil {
			log.Printf("[VideoTool] Seedance task %s failed: %v", taskID, err)
			t.db.Model(&model.VideoRecord{}).Where("task_id = ?", taskID).Updates(map[string]interface{}{"status": "failed"})
			UpdateGenLog(t.db, genLogID, "failed", "", err.Error())
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
		savedURL := videoURL
		// short_drama 专用路径：直接用 StarAI/Seedance 回传的公网 URL，
		// 不做 SaveClipLocally，保证下一场 ref_video_url 仍然是 Seedance 可抓的公网地址。
		// 其他 category（通用视频生成）继续走本地持久化以便 ffmpeg 合并。
		if args.Category != "short_drama" {
			if localURL, lerr := SaveClipLocally(videoURL); lerr == nil {
				savedURL = localURL
			} else {
				log.Printf("[VideoTool] Seedance task %s: local save failed (will use remote URL): %v", taskID, lerr)
			}
		} else {
			log.Printf("[VideoTool] Seedance task %s (short_drama): keep public URL for ref chain: %s", taskID, savedURL)
		}
		updates := map[string]interface{}{"video_url": savedURL, "status": "succeeded"}
		t.db.Model(&model.VideoRecord{}).Where("task_id = ?", taskID).Updates(updates)
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
		"message": fmt.Sprintf("Seedance 视频任务已提交。任务ID: %s，模型: %s。可用 check_status 查看进度。", taskID, args.Model),
	}), nil
}

func (t *VideoTool) pollVolcengineTask(ctx context.Context, apiKey, baseURL, taskID string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		result, err := t.getVolcengineTaskStatus(ctx, apiKey, baseURL, taskID)
		if err != nil {
			return "", err
		}
		status := normalizeVolcengineTaskStatus(result)
		if videoURL := extractVolcengineVideoURL(result); videoURL != "" && status != "FAILED" {
			return videoURL, nil
		}
		if status == "FAILED" {
			resJSON, _ := json.Marshal(result)
			return "", fmt.Errorf("Seedance task failed: %s", string(resJSON))
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
	return "", fmt.Errorf("Seedance polling timeout after %v", timeout)
}

func (t *VideoTool) getVolcengineTaskStatus(ctx context.Context, apiKey, baseURL, taskID string) (map[string]interface{}, error) {
	var reqURL string
	var client *http.Client
	if isStarAIKey(apiKey) {
		reqURL = StarAIProxyURL("volcengine", "/contents/generations/tasks/"+taskID)
		c, _ := GetStarAIClient()
		if c == nil {
			return nil, fmt.Errorf("StarAI proxy not initialized")
		}
		client = c
	} else {
		reqURL = strings.TrimRight(baseURL, "/") + "/contents/generations/tasks/" + taskID
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	if !isStarAIKey(apiKey) {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Seedance status HTTP %d: %s", resp.StatusCode, string(body))
	}
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("Seedance status parse failed: %v", err)
	}
	return result, nil
}

func normalizeVolcengineTaskStatus(result map[string]interface{}) string {
	for _, key := range []string{"status", "task_status", "state"} {
		if raw, ok := result[key].(string); ok {
			s := strings.ToUpper(strings.TrimSpace(raw))
			switch s {
			case "SUCCEEDED", "SUCCESS", "COMPLETED", "DONE":
				return "SUCCEEDED"
			case "FAILED", "FAIL", "ERROR", "CANCELLED", "CANCELED":
				return "FAILED"
			case "RUNNING", "PENDING", "QUEUED", "PROCESSING", "IN_PROGRESS", "CREATED":
				return "RUNNING"
			}
		}
	}
	if extractVolcengineVideoURL(result) != "" {
		return "SUCCEEDED"
	}
	return "RUNNING"
}

func extractVolcengineVideoURL(result map[string]interface{}) string {
	if u := findVolcengineVideoURL(result); u != "" {
		return u
	}
	resJSON, _ := json.Marshal(result)
	log.Printf("[VideoTool] extractVolcengineVideoURL: no video URL found in result keys=%v snippet=%.500s", keysOf(result), string(resJSON))
	return ""
}

func findVolcengineVideoURL(v interface{}) string {
	switch val := v.(type) {
	case map[string]interface{}:
		for _, key := range []string{"video_url", "result_url", "download_url", "file_url", "play_url"} {
			if raw, ok := val[key].(string); ok && raw != "" {
				return raw
			}
		}
		if contentMap, ok := val["content"].(map[string]interface{}); ok {
			if raw := findVolcengineVideoURL(contentMap); raw != "" {
				return raw
			}
		}
		if content, ok := val["content"].([]interface{}); ok {
			for _, item := range content {
				if m, ok := item.(map[string]interface{}); ok {
					if tpe, _ := m["type"].(string); strings.Contains(strings.ToLower(tpe), "video") {
						for _, key := range []string{"url", "video_url", "result_url", "download_url", "file_url"} {
							if raw, ok := m[key].(string); ok && raw != "" {
								return raw
							}
						}
					}
				}
			}
		}
		if video, ok := val["video"].(map[string]interface{}); ok {
			if raw, ok := video["url"].(string); ok && raw != "" {
				return raw
			}
		}
		for _, nested := range []string{"output", "result", "data", "payload", "response"} {
			if child, ok := val[nested]; ok {
				if raw := findVolcengineVideoURL(child); raw != "" {
					return raw
				}
			}
		}
		if raw, ok := val["url"].(string); ok && strings.Contains(strings.ToLower(raw), "mp4") {
			return raw
		}
	case []interface{}:
		for _, item := range val {
			if raw := findVolcengineVideoURL(item); raw != "" {
				return raw
			}
		}
	}
	return ""
}

func aspectRatioFromSize(size string) string {
	parts := strings.Split(size, "*")
	if len(parts) != 2 {
		return "16:9"
	}
	w, errW := strconv.Atoi(parts[0])
	h, errH := strconv.Atoi(parts[1])
	if errW != nil || errH != nil || w <= 0 || h <= 0 {
		return "16:9"
	}
	if w > h {
		return "16:9"
	}
	if h > w {
		return "9:16"
	}
	return "1:1"
}

func resolutionFromSize(size string) string {
	parts := strings.Split(size, "*")
	if len(parts) != 2 {
		return "720p"
	}
	w, errW := strconv.Atoi(parts[0])
	h, errH := strconv.Atoi(parts[1])
	if errW != nil || errH != nil || w <= 0 || h <= 0 {
		return "720p"
	}
	maxDim := w
	if h > maxDim {
		maxDim = h
	}
	if maxDim >= 1920 {
		return "1080p"
	}
	return "720p"
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
