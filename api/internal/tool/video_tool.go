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
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"github.com/google/uuid"
	"github.com/yinhe/starclaw/internal/model"
)

// VideoTool generates videos via multiple providers: DashScope Wan and fal.ai (Veo3, Sora2, Kling, etc.)
type VideoTool struct {
	db      *gorm.DB
	mergeMu sync.Mutex
	merging map[string]bool
}

func NewVideoTool(db *gorm.DB) *VideoTool {
	return &VideoTool{db: db, merging: make(map[string]bool)}
}

func (t *VideoTool) Name() string { return "video_generation" }

func (t *VideoTool) Description() string {
	return `AI 视频生成工具，支持多种视频模型和场景衔接。支持的模型：
- wan2.6-t2v: 阿里云万相文生视频（默认），最高10秒
- wan2.6-i2v: 阿里云万相图生视频，需要img_url
- veo3: Google Veo 3 文生视频 (fal.ai)，电影级画质
- sora2: OpenAI Sora 2 文生视频 (fal.ai)
- kling-v2: 快手可灵 v2 文生视频 (fal.ai)
- minimax-video: MiniMax 视频生成 (fal.ai)
- luma: Luma Dream Machine (fal.ai)

操作：generate_video、check_status、merge_videos、list_models、extract_last_frame
制作流程：1) 编写脚本 2) 逐场景调用 generate_video（用 ref_video_id 衔接上一场景尾帧，用 style_prefix 统一风格） 3) 所有场景完成后自动合成最终视频（带 crossfade 转场效果）。`
}

func (t *VideoTool) Parameters() interface{} {
	return &JSONSchema{
		Type: "object",
		Properties: map[string]Property{
			"action":       {Type: "string", Description: "Action: generate_video, check_status, merge_videos, list_models, extract_last_frame"},
			"prompt":       {Type: "string", Description: "Text prompt describing the video scene. Be detailed about motion, camera angle, style."},
			"model":        {Type: "string", Description: "Model: wan2.6-t2v (default), wan2.6-i2v (requires img_url), veo3, sora2, kling-v2, minimax-video, luma"},
			"img_url":      {Type: "string", Description: "Image URL for image-to-video models (wan2.6-i2v). Tip: use extract_last_frame to get the last frame of the previous scene for continuity."},
			"size":         {Type: "string", Description: "Video resolution: 1280*720 (landscape), 720*1280 (portrait), 960*960 (square). Default: 1280*720"},
			"duration":     {Type: "string", Description: "Video duration in seconds: 5 or 10. Default: 5"},
			"task_id":      {Type: "string", Description: "Task ID or record ID for check_status / extract_last_frame"},
			"scene":        {Type: "string", Description: "Scene label for multi-scene projects (e.g. 'scene_1')"},
			"task_ids":     {Type: "string", Description: "For merge_videos: comma-separated task_ids to merge in order. If empty, merges all in current conversation."},
			"style_prefix": {Type: "string", Description: "Shared style prefix prepended to all scene prompts for visual consistency (e.g. 'cinematic film style, warm color grading, shallow depth of field'). Stored with the record."},
			"ref_video_id": {Type: "string", Description: "Previous scene's video record ID. Auto-extracts its last frame as img_url for i2v, ensuring visual continuity between scenes."},
		},
		Required: []string{"action"},
	}
}

type videoArgs struct {
	Action      string `json:"action"`
	Prompt      string `json:"prompt"`
	Model       string `json:"model"`
	ImgURL      string `json:"img_url"`
	Size        string `json:"size"`
	Duration    string `json:"duration"`
	TaskID      string `json:"task_id"`
	Scene       string `json:"scene"`
	TaskIDs     string `json:"task_ids"`
	StylePrefix string `json:"style_prefix"`
	RefVideoID  string `json:"ref_video_id"`
}

// fal.ai video model endpoints
var falVideoEndpoints = map[string]string{
	"veo3":          "fal-ai/veo3",
	"sora2":         "fal-ai/minimax-video/video-01-live",
	"kling-v2":      "fal-ai/kling-video/v2.1/master/text-to-video",
	"minimax-video": "fal-ai/minimax-video/video-01-live",
	"luma":          "fal-ai/luma-dream-machine",
}

func (t *VideoTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	var args videoArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %v", err)
	}
	switch args.Action {
	case "generate_video":
		return t.generateVideo(ctx, args)
	case "check_status":
		return t.checkStatus(ctx, args)
	case "merge_videos":
		return t.mergeVideos(ctx, args)
	case "list_models":
		return t.listModels()
	case "extract_last_frame":
		return t.extractLastFrame(ctx, args)
	default:
		return "", fmt.Errorf("unknown action: %s. Use: generate_video, check_status, merge_videos, list_models, extract_last_frame", args.Action)
	}
}

func isFalVideoModel(m string) bool {
	_, ok := falVideoEndpoints[m]
	return ok
}

func (t *VideoTool) generateVideo(ctx context.Context, args videoArgs) (string, error) {
	if args.Prompt == "" {
		return "", fmt.Errorf("prompt is required")
	}
	userID := ""
	if uid, ok := ctx.Value(CtxKeyUserID).(string); ok {
		userID = uid
	}
	convID := ""
	if cid, ok := ctx.Value(CtxKeyConversationID).(string); ok {
		convID = cid
	}
	if args.Model == "" {
		args.Model = "wan2.6-t2v"
	}
	if args.Size == "" {
		args.Size = "1280*720"
	}
	duration := 5
	if args.Duration == "10" {
		duration = 10
	}

	// Prepend style_prefix to prompt for visual consistency across scenes
	if args.StylePrefix != "" {
		args.Prompt = args.StylePrefix + ", " + args.Prompt
	}

	// ref_video_id: auto-extract last frame from previous scene → switch to i2v for continuity
	if args.RefVideoID != "" && args.ImgURL == "" {
		frameURL, err := t.getLastFrameURL(args.RefVideoID)
		if err != nil {
			log.Printf("[VideoTool] ref_video_id %s last frame extraction failed: %v, continuing with t2v", args.RefVideoID, err)
		} else {
			args.ImgURL = frameURL
			// Auto-switch to i2v model for DashScope
			if args.Model == "wan2.6-t2v" {
				args.Model = "wan2.6-i2v"
				log.Printf("[VideoTool] Auto-switched to i2v with last frame from %s", args.RefVideoID)
			}
		}
	}

	if isFalVideoModel(args.Model) {
		return t.generateVideoFal(ctx, userID, convID, args, duration)
	}
	return t.generateVideoWan(ctx, userID, convID, args, duration)
}

// extractLastFrame extracts the last frame of a completed video and saves it as a JPEG.
// Returns the URL to the extracted frame image.
func (t *VideoTool) extractLastFrame(ctx context.Context, args videoArgs) (string, error) {
	if args.TaskID == "" {
		return "", fmt.Errorf("task_id (video record ID or task ID) is required")
	}
	frameURL, err := t.getLastFrameURL(args.TaskID)
	if err != nil {
		return "", err
	}
	return toJSON(map[string]interface{}{
		"action":    "extract_last_frame",
		"frame_url": frameURL,
		"message":   "已提取视频最后一帧。可将此 URL 作为下一场景的 img_url 使用 wan2.6-i2v 模型实现场景衔接。",
	}), nil
}

// getLastFrameURL extracts the last frame from a video record and returns its accessible URL.
func (t *VideoTool) getLastFrameURL(recordOrTaskID string) (string, error) {
	var rec model.VideoRecord
	if err := t.db.Where("id = ? OR task_id = ?", recordOrTaskID, recordOrTaskID).First(&rec).Error; err != nil {
		return "", fmt.Errorf("video record not found: %s", recordOrTaskID)
	}
	if rec.Status != "succeeded" || rec.VideoURL == "" {
		return "", fmt.Errorf("video not ready (status=%s)", rec.Status)
	}

	// Resolve video to local path
	tmpDir, tmpErr := os.MkdirTemp("", "lastframe-*")
	if tmpErr != nil {
		return "", tmpErr
	}
	defer os.RemoveAll(tmpDir)
	videoPath, err := ResolveClipToLocal(rec.VideoURL, filepath.Join(tmpDir, "input.mp4"))
	if err != nil {
		return "", fmt.Errorf("failed to resolve video: %v", err)
	}

	// Extract last frame using ffmpeg (seek to near-end)
	dur := ProbeDuration(videoPath)
	if dur <= 0 {
		dur = 5.0
	}
	seekTime := dur - 0.1
	if seekTime < 0 {
		seekTime = 0
	}

	frameDir := "/app/images"
	os.MkdirAll(frameDir, 0755)
	frameFile := fmt.Sprintf("lastframe_%s.jpg", rec.ID)
	framePath := filepath.Join(frameDir, frameFile)

	extractCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(extractCtx, "ffmpeg", "-y",
		"-ss", fmt.Sprintf("%.2f", seekTime),
		"-i", videoPath,
		"-frames:v", "1", "-q:v", "2", framePath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("ffmpeg last frame extraction failed: %v\n%s", err, string(out))
	}

	if _, err := os.Stat(framePath); os.IsNotExist(err) {
		return "", fmt.Errorf("extracted frame file not found")
	}

	frameURL := fmt.Sprintf("/v1/images/%s", frameFile)
	log.Printf("[VideoTool] Extracted last frame from %s → %s", rec.ID, frameURL)
	return frameURL, nil
}

// ── DashScope Wan Provider ──

func (t *VideoTool) generateVideoWan(ctx context.Context, userID, convID string, args videoArgs, duration int) (string, error) {
	apiKey, baseHost := GetDashScopeAPIKey(t.db, userID)
	if apiKey == "" {
		return "", fmt.Errorf("no DashScope API key found. Please configure a qwen model first")
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

	record := model.VideoRecord{
		UserID: userID, ConversationID: convID, TaskID: taskID,
		Model: args.Model, Prompt: args.Prompt, ImgURL: args.ImgURL,
		Size: args.Size, Duration: duration, Scene: args.Scene, Status: "running",
	}
	t.db.Create(&record)
	if args.Scene != "" {
		t.ensureVideoWorkflow(userID, convID)
	}

	go func() {
		videoURL, _, err := t.pollDashScopeTask(context.Background(), apiKey, baseHost, taskID, 10*time.Minute)
		if err != nil {
			log.Printf("[VideoTool] Task %s failed: %v", taskID, err)
			t.db.Model(&model.VideoRecord{}).Where("task_id = ?", taskID).Updates(map[string]interface{}{"status": "failed"})
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
		"message": fmt.Sprintf("视频任务已提交！任务ID: %s，模型: %s。片段完成后会自动合成。", taskID, args.Model),
	}), nil
}

// ── fal.ai Provider ──

func (t *VideoTool) generateVideoFal(ctx context.Context, userID, convID string, args videoArgs, duration int) (string, error) {
	apiKey := GetFalAPIKey(t.db, userID)
	if apiKey == "" {
		return "", fmt.Errorf("no fal.ai API key found. Please configure a fal provider first")
	}
	endpoint, ok := falVideoEndpoints[args.Model]
	if !ok {
		return "", fmt.Errorf("unknown fal.ai video model: %s", args.Model)
	}

	body := map[string]interface{}{
		"prompt":   args.Prompt,
		"duration": fmt.Sprintf("%d", duration),
	}
	if args.ImgURL != "" {
		body["image_url"] = args.ImgURL
	}
	sizeParts := strings.Split(args.Size, "*")
	if len(sizeParts) == 2 {
		if w, err := strconv.Atoi(sizeParts[0]); err == nil {
			body["width"] = w
		}
		if h, err := strconv.Atoi(sizeParts[1]); err == nil {
			body["height"] = h
		}
	}

	requestID, err := SubmitToFal(apiKey, endpoint, body)
	if err != nil {
		return "", fmt.Errorf("fal.ai submit failed: %v", err)
	}

	log.Printf("[VideoTool] fal.ai submitted: %s (model=%s, scene=%s)", requestID, args.Model, args.Scene)

	record := model.VideoRecord{
		UserID: userID, ConversationID: convID, TaskID: requestID,
		Model: args.Model, Prompt: args.Prompt, ImgURL: args.ImgURL,
		Size: args.Size, Duration: duration, Scene: args.Scene, Status: "running",
	}
	t.db.Create(&record)
	if args.Scene != "" {
		t.ensureVideoWorkflow(userID, convID)
	}

	go func() {
		result, err := PollFalStatus(apiKey, endpoint, requestID, 10*time.Minute)
		if err != nil {
			log.Printf("[VideoTool] fal.ai %s failed: %v", requestID, err)
			t.db.Model(&model.VideoRecord{}).Where("task_id = ?", requestID).Updates(map[string]interface{}{"status": "failed"})
			return
		}
		videoURL := extractFalVideoURL(result)
		if videoURL == "" {
			t.db.Model(&model.VideoRecord{}).Where("task_id = ?", requestID).Updates(map[string]interface{}{"status": "failed"})
			return
		}
		// Download to local storage
		outputDir := "/app/merged_videos"
		os.MkdirAll(outputDir, 0755)
		localFile := fmt.Sprintf("fal_%s.mp4", uuid.New().String()[:8])
		localPath := filepath.Join(outputDir, localFile)
		savedURL := videoURL
		if err := DownloadFile(videoURL, localPath); err == nil {
			savedURL = fmt.Sprintf("/v1/videos/merged/%s", localFile)
		}
		t.db.Model(&model.VideoRecord{}).Where("task_id = ?", requestID).Updates(map[string]interface{}{
			"video_url": savedURL, "status": "succeeded",
		})
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
		"message": fmt.Sprintf("视频任务已提交！请求ID: %s，模型: %s (fal.ai)。", requestID, args.Model),
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
	return ""
}

// ── Check Status ──

func (t *VideoTool) checkStatus(ctx context.Context, args videoArgs) (string, error) {
	if args.TaskID == "" {
		return "", fmt.Errorf("task_id is required")
	}
	// Check DB first
	var rec model.VideoRecord
	if err := t.db.Where("task_id = ? OR id = ?", args.TaskID, args.TaskID).First(&rec).Error; err == nil {
		if rec.Status == "succeeded" {
			return toJSON(map[string]interface{}{
				"action": "check_status", "task_id": rec.TaskID,
				"task_status": "SUCCEEDED", "video_url": rec.VideoURL,
				"message": "视频已就绪！",
			}), nil
		}
		if rec.Status == "failed" {
			return toJSON(map[string]interface{}{
				"action": "check_status", "task_id": rec.TaskID,
				"task_status": "FAILED", "message": "视频生成失败",
			}), nil
		}
	}

	// Poll DashScope
	userID := ""
	if uid, ok := ctx.Value(CtxKeyUserID).(string); ok {
		userID = uid
	}
	apiKey, baseHost := GetDashScopeAPIKey(t.db, userID)
	if apiKey != "" {
		deadline := time.Now().Add(3 * time.Minute)
		for time.Now().Before(deadline) {
			status, videoURL, err := t.getDashScopeTaskStatus(ctx, apiKey, baseHost, args.TaskID)
			if err != nil {
				break
			}
			if status == "SUCCEEDED" || videoURL != "" {
				t.db.Model(&model.VideoRecord{}).Where("task_id = ?", args.TaskID).Updates(map[string]interface{}{
					"video_url": videoURL, "status": "succeeded",
				})
				return toJSON(map[string]interface{}{
					"action": "check_status", "task_id": args.TaskID,
					"task_status": "SUCCEEDED", "video_url": videoURL, "message": "视频已就绪！",
				}), nil
			}
			if status == "FAILED" {
				return toJSON(map[string]interface{}{
					"action": "check_status", "task_id": args.TaskID,
					"task_status": "FAILED", "message": "视频生成失败",
				}), nil
			}
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(5 * time.Second):
			}
		}
	}

	return toJSON(map[string]interface{}{
		"action": "check_status", "task_id": args.TaskID,
		"task_status": "RUNNING", "message": "视频仍在生成中，请稍后再查。",
	}), nil
}

// ── List Models ──

func (t *VideoTool) listModels() (string, error) {
	models := []map[string]string{
		{"name": "wan2.6-t2v", "type": "text-to-video", "provider": "dashscope", "description": "阿里云万相文生视频，最高10秒"},
		{"name": "wan2.6-i2v", "type": "image-to-video", "provider": "dashscope", "description": "阿里云万相图生视频，需要img_url"},
		{"name": "veo3", "type": "text-to-video", "provider": "fal.ai", "description": "Google Veo 3, 电影级画质"},
		{"name": "sora2", "type": "text-to-video", "provider": "fal.ai", "description": "OpenAI Sora 2"},
		{"name": "kling-v2", "type": "text-to-video", "provider": "fal.ai", "description": "快手可灵 v2"},
		{"name": "minimax-video", "type": "text-to-video", "provider": "fal.ai", "description": "MiniMax 视频生成"},
		{"name": "luma", "type": "text-to-video", "provider": "fal.ai", "description": "Luma Dream Machine"},
	}
	return toJSON(map[string]interface{}{
		"action": "list_models", "models": models,
		"tips": "wan系列需要qwen的API Key，其他模型需要fal.ai的API Key。超过10秒请生成多场景再合并。",
	}), nil
}

// ── Resolution helpers ──

// probeResolution returns width, height of a video file using ffprobe.
func probeResolution(path string) (int, int, error) {
	out, err := exec.Command("ffprobe", "-v", "error", "-select_streams", "v:0",
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
		cmd := exec.CommandContext(ctx, "ffmpeg", "-y", "-i", clipPaths[0], "-c", "copy", outputPath)
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
			cmd := exec.CommandContext(ctx, "ffmpeg", "-y", "-i", p,
				"-c:v", "libx264", "-c:a", "aac", "-preset", "fast", "-r", "30",
				"-pix_fmt", "yuv420p", normPath)
			if out, err := cmd.CombinedOutput(); err != nil {
				log.Printf("[VideoMerge] re-encode clip %d failed: %s, trying copy", i, string(out))
				exec.CommandContext(ctx, "cp", p, normPath).Run()
			}
		} else {
			filter := fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2:color=black",
				targetRes.w, targetRes.h, targetRes.w, targetRes.h)
			cmd := exec.CommandContext(ctx, "ffmpeg", "-y", "-i", p,
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

	cmd := exec.CommandContext(ctx, "ffmpeg", ffmpegArgs...)
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
		cmd2 := exec.CommandContext(ctx, "ffmpeg", args2...)
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

	cmd := exec.CommandContext(ctx, "ffmpeg", "-y", "-f", "concat", "-safe", "0",
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
		t.db.Where("user_id = ? AND task_id IN ? AND status = ?", userID, ids, "succeeded").Find(&records)
		idOrder := make(map[string]int)
		for i, id := range ids {
			idOrder[id] = i
		}
		sort.Slice(records, func(i, j int) bool {
			return idOrder[records[i].TaskID] < idOrder[records[j].TaskID]
		})
	} else if convID != "" {
		t.db.Where("user_id = ? AND conversation_id = ? AND (type = 'clip' OR type = '') AND status = ?", userID, convID, "succeeded").
			Order("scene ASC, created_at ASC").Find(&records)
	} else {
		return "", fmt.Errorf("no conversation context and no task_ids provided")
	}

	if len(records) == 0 {
		return "", fmt.Errorf("no completed videos found to merge")
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
	outputDir := "/app/merged_videos"
	os.MkdirAll(outputDir, 0755)
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
				os.Remove(filepath.Join("/app/merged_videos", filepath.Base(om.VideoURL)))
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
			oldFile := filepath.Join("/app/merged_videos", filepath.Base(existingMerge.VideoURL))
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
	outputDir := "/app/merged_videos"
	os.MkdirAll(outputDir, 0755)
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

// RetryNarration is kept as stub for backward compatibility (narration moved to dubbing tool)
func (t *VideoTool) RetryNarration(userID string) {
	// No-op: narration is now handled by the dubbing tool
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
	url := fmt.Sprintf("https://%s/api/v1/tasks/%s", baseHost, taskID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
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
