package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"gorm.io/gorm"

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
- veo3.1: Google Veo 3.1 文生视频 (fal.ai)，最新最强，支持4-8秒，原生音频
- sora2: OpenAI Sora 2 Pro 文生视频 (fal.ai)，支持5-20秒，原生音频
- kling-v3: 快手可灵 v3 文生视频 (fal.ai)，支持3-15秒，原生音频
- minimax-video: MiniMax 视频生成 (fal.ai)
- luma: Luma Dream Machine Ray-2 (fal.ai)

操作：generate_video、check_status、merge_videos、list_models、extract_last_frame、list_videos
制作流程：1) 编写脚本 2) 逐场景调用 generate_video（用 ref_video_id 衔接上一场景尾帧，用 style_prefix 统一风格） 3) 所有场景完成后自动合成最终视频（带 crossfade 转场效果）。

list_videos 可以查看当前会话或全局已生成的视频，避免重复生成。`
}

func (t *VideoTool) Parameters() interface{} {
	return &JSONSchema{
		Type: "object",
		Properties: map[string]Property{
			"action":       {Type: "string", Description: "Action: generate_video, check_status, merge_videos, list_models, extract_last_frame, list_videos"},
			"prompt":       {Type: "string", Description: "Text prompt describing the video scene. Be detailed about motion, camera angle, style."},
			"model":        {Type: "string", Description: "Model: wan2.6-t2v (default), wan2.6-i2v (requires img_url), veo3, veo3.1, sora2, kling-v3, minimax-video, luma"},
			"img_url":      {Type: "string", Description: "Image URL for image-to-video models (wan2.6-i2v). Tip: use extract_last_frame to get the last frame of the previous scene for continuity."},
			"size":         {Type: "string", Description: "Video resolution: 1280*720 (landscape), 720*1280 (portrait), 960*960 (square). Default: 1280*720"},
			"duration":     {Type: "string", Description: "Video duration in seconds: 5 or 10. Default: 5"},
			"task_id":      {Type: "string", Description: "Task ID or record ID for check_status / extract_last_frame"},
			"scene":        {Type: "string", Description: "Scene label for multi-scene projects (e.g. 'scene_1')"},
			"task_ids":     {Type: "string", Description: "For merge_videos: comma-separated task_ids to merge in order. If empty, merges all in current conversation."},
			"style_prefix": {Type: "string", Description: "Shared style prefix prepended to all scene prompts for visual consistency (e.g. 'cinematic film style, warm color grading, shallow depth of field'). Stored with the record."},
			"ref_video_id": {Type: "string", Description: "Previous scene's video record ID. Auto-extracts its last frame as img_url for i2v, ensuring visual continuity between scenes."},
			"category":     {Type: "string", Description: "Video category: general (default), ad, short_drama, short_film, mv, tutorial. Used for organization and filtering."},
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
	Category    string `json:"category"`
}

// fal.ai video model endpoints
var falVideoEndpoints = map[string]string{
	"veo3":          "fal-ai/veo3",
	"veo3.1":        "fal-ai/veo3.1",
	"sora2":         "fal-ai/sora-2/text-to-video/pro",
	"kling-v3":      "fal-ai/kling-video/v3/pro/text-to-video",
	"minimax-video": "fal-ai/minimax-video/video-01-live",
	"luma":          "fal-ai/luma-dream-machine/ray-2",
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
	case "list_videos":
		return t.listVideos(ctx, args)
	default:
		return "", fmt.Errorf("unknown action: %s. Use: generate_video, check_status, merge_videos, list_models, extract_last_frame, list_videos", args.Action)
	}
}

func isFalVideoModel(m string) bool {
	_, ok := falVideoEndpoints[m]
	return ok
}

// videoFallbackChain defines auto-fallback when a model times out or fails.
// Key = original model, Value = fallback model to retry with.
var videoFallbackChain = map[string]string{
	"veo3.1":        "kling-v3",
	"sora2":         "kling-v3",
	"kling-v3":      "wan2.6-t2v",
	"luma":          "wan2.6-t2v",
	"minimax-video": "wan2.6-t2v",
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
	// Auto-redirect deprecated/invalid model names
	switch args.Model {
	case "kling-v2", "kling-v1", "kling":
		log.Printf("[VideoTool] Model %q is deprecated/invalid, auto-redirecting to kling-v3", args.Model)
		args.Model = "kling-v3"
	case "veo3":
		log.Printf("[VideoTool] Model veo3 → veo3.1 (latest)")
		args.Model = "veo3.1"
	case "":
		args.Model = "wan2.6-t2v"
	}
	if args.Size == "" {
		args.Size = "1280*720"
	}
	duration := 5
	if d, err := strconv.Atoi(args.Duration); err == nil && d > 0 {
		duration = d
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
func (t *VideoTool) extractLastFrame(_ context.Context, args videoArgs) (string, error) {
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

	frameDir := ImagesDir()
	os.MkdirAll(frameDir, 0755)
	frameFile := fmt.Sprintf("lastframe_%s.jpg", rec.ID)
	framePath := filepath.Join(frameDir, frameFile)

	extractCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := hiddenCmdCtx(extractCtx, "ffmpeg", "-y",
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

// ── Check Status ──

func (t *VideoTool) checkStatus(ctx context.Context, args videoArgs) (string, error) {
	if args.TaskID == "" {
		return "", fmt.Errorf("task_id is required")
	}
	// Check DB first
	var rec model.VideoRecord
	dbFound := t.db.Where("task_id = ? OR id = ?", args.TaskID, args.TaskID).First(&rec).Error == nil
	if dbFound {
		if rec.Status == "succeeded" {
			return toJSON(map[string]interface{}{
				"action": "check_status", "task_id": rec.TaskID,
				"record_id": rec.ID, "scene": rec.Scene,
				"task_status": "SUCCEEDED", "video_url": rec.VideoURL,
				"message": fmt.Sprintf("视频已就绪！record_id=%s 可直接用于 compose_pro 的 video_id", rec.ID),
			}), nil
		}
		if rec.Status == "failed" {
			return toJSON(map[string]interface{}{
				"action": "check_status", "task_id": rec.TaskID,
				"record_id": rec.ID, "scene": rec.Scene,
				"task_status": "FAILED", "message": "视频生成失败",
			}), nil
		}
		// Record exists with running/pending status — wait for background goroutine
		// For fal.ai models, background goroutine is already polling; just re-check DB.
		// For DashScope models, we also re-check DB (background goroutine polls too).
		deadline := time.Now().Add(3 * time.Minute)
		for time.Now().Before(deadline) {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(5 * time.Second):
			}
			var updated model.VideoRecord
			if err := t.db.Where("task_id = ? OR id = ?", args.TaskID, args.TaskID).First(&updated).Error; err == nil {
				if updated.Status == "succeeded" {
					return toJSON(map[string]interface{}{
						"action": "check_status", "task_id": updated.TaskID,
						"record_id": updated.ID, "scene": updated.Scene,
						"task_status": "SUCCEEDED", "video_url": updated.VideoURL,
						"message": fmt.Sprintf("视频已就绪！record_id=%s 可直接用于 compose_pro 的 video_id", updated.ID),
					}), nil
				}
				if updated.Status == "failed" {
					return toJSON(map[string]interface{}{
						"action": "check_status", "task_id": updated.TaskID,
						"record_id": updated.ID, "scene": updated.Scene,
						"task_status": "FAILED", "message": "视频生成失败",
					}), nil
				}
			}
		}
		return toJSON(map[string]interface{}{
			"action": "check_status", "task_id": rec.TaskID,
			"record_id": rec.ID, "scene": rec.Scene,
			"task_status": "RUNNING", "message": "视频仍在生成中（后台轮询中），请稍后再查。",
		}), nil
	}

	// No DB record — try DashScope polling as fallback (old-style tasks without DB record)
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
				var updated model.VideoRecord
				t.db.Where("task_id = ?", args.TaskID).First(&updated)
				return toJSON(map[string]interface{}{
					"action": "check_status", "task_id": args.TaskID,
					"record_id": updated.ID, "scene": updated.Scene,
					"task_status": "SUCCEEDED", "video_url": videoURL,
					"message": fmt.Sprintf("视频已就绪！record_id=%s 可直接用于 compose_pro 的 video_id", updated.ID),
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
		"task_status": "NOT_FOUND", "message": fmt.Sprintf("未找到任务 %s，请检查 task_id 是否正确。可能已过期或从未创建。", args.TaskID),
	}), nil
}

// ── List Models ──

func (t *VideoTool) listModels() (string, error) {
	models := []map[string]interface{}{
		{"name": "wan2.6-t2v", "type": "text-to-video", "provider": "dashscope", "durations": "5s, 10s", "resolutions": "1280*720, 720*1280, 960*960", "quality": "good", "speed": "fast", "description": "阿里云万相文生视频。通用首选，速度快，3种画幅", "best_for": "通用场景、第一个镜头、快速迭代"},
		{"name": "wan2.6-i2v", "type": "image-to-video", "provider": "dashscope", "durations": "5s", "resolutions": "1280*720, 720*1280, 960*960", "quality": "good", "speed": "fast", "description": "阿里云万相图生视频，需要img_url。用于尾帧衔接保持场景连续", "best_for": "场景衔接（上一场景尾帧→下一场景起始帧）"},
		{"name": "veo3", "type": "text-to-video", "provider": "fal.ai", "durations": "~8s (模型自动)", "resolutions": "最高1080p", "quality": "cinematic", "speed": "slow", "description": "Google Veo 3，电影级画质", "best_for": "远景建立镜头、电影级MV、风景空镜"},
		{"name": "veo3.1", "type": "text-to-video", "provider": "fal.ai", "durations": "4s, 6s, 8s", "resolutions": "720p/1080p", "quality": "cinematic+", "speed": "medium", "description": "Google Veo 3.1，最新最强视频模型，支持原生音频、图生视频", "best_for": "电影级画质+音频、高质量i2v场景衔接"},
		{"name": "sora2", "type": "text-to-video", "provider": "fal.ai", "durations": "5s, 10s, 15s, 20s", "resolutions": "最高1080p", "quality": "very high", "speed": "medium", "description": "OpenAI Sora 2 Pro，强运动理解，支持长视频，原生音频", "best_for": "复杂动作、长镜头、20秒连续画面"},
		{"name": "kling-v3", "type": "text-to-video", "provider": "fal.ai", "durations": "3-15s", "resolutions": "16:9, 9:16, 1:1", "quality": "cinematic", "speed": "medium", "description": "快手可灵 v3 Pro，电影级画质，原生音频生成，支持3-15秒", "best_for": "人物特写、动态场景、角色动作、带声音视频"},
		{"name": "minimax-video", "type": "text-to-video", "provider": "fal.ai", "durations": "~5s", "resolutions": "1280*720", "quality": "good", "speed": "fast", "description": "MiniMax Video-01-Live，快速生成", "best_for": "快速出片、动画风格"},
		{"name": "luma", "type": "text-to-video", "provider": "fal.ai", "durations": "~5s", "resolutions": "最高1080p", "quality": "artistic", "speed": "medium", "description": "Luma Dream Machine，梦幻艺术风格", "best_for": "艺术风格、梦幻场景、概念视觉"},
	}
	return toJSON(map[string]interface{}{
		"action": "list_models", "models": models,
		"tips": "wan系列通过 StarAI/DashScope API Key 调用，其他模型通过 fal.ai API Key 调用。MV制作推荐：veo3(电影级远景) + kling-v3(人物特写+原生音频) + wan(快速补充镜头)。",
	}), nil
}
