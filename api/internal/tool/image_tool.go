package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/yinhe/starclaw/internal/model"
)

// ImageTool allows agents to generate images via fal.ai platform (Flux models)
type ImageTool struct {
	db *gorm.DB
}

func NewImageTool(db *gorm.DB) *ImageTool {
	return &ImageTool{db: db}
}

func (t *ImageTool) Name() string { return "image_generation" }

func (t *ImageTool) Description() string {
	return `AI 图片生成工具，通过 fal.ai 平台调用 Flux 系列图像模型。

## FLUX.1 模型
- flux-schnell：快速生成（默认），高质量，约10秒
- flux-dev：开发版，更高质量
- flux-pro：Pro，最高质量
- flux-realism：写实风格
- stable-diffusion-v35-large：SD 3.5 Large

## FLUX.2 模型（新一代，增强真实感与文字生成）
**文生图：**
- flux-2：FLUX.2 [dev]（推荐，增强真实感）
- flux-2-pro：FLUX.2 [pro]（最高质量，超级写实）
- flux-2-max：FLUX.2 [max]（SOTA，极致精度）
- flux-2-flex：FLUX.2 [flex]（可调推理步数，增强排版）
- flux-2-turbo：FLUX.2 Turbo（极速）
- flux-2-flash：FLUX.2 Flash（闪电级速度）
**图片编辑（需提供 image_url）：**
- flux-2-edit：自然语言精确修改图片
- flux-2-pro-edit：高质量图片编辑
- flux-2-max-edit：SOTA 图片编辑
- flux-2-flex-edit：多参考图编辑
- flux-2-turbo-edit：极速编辑
- flux-2-flash-edit：闪电编辑
**LoRA 自定义风格：**
- flux-2-lora：LoRA 文生图
- flux-2-lora-edit：LoRA 图片编辑
**Klein 轻量模型：**
- flux-2-klein-4b / flux-2-klein-9b：轻量高速
- flux-2-klein-realtime：实时生成
**LoRA Gallery 预训练风格：**
- flux-2-gallery-realism / flux-2-gallery-comic / flux-2-gallery-sketch
- flux-2-gallery-vintage / flux-2-gallery-hdr / flux-2-gallery-satellite
- flux-2-gallery-tryon / flux-2-gallery-background / flux-2-gallery-portrait / flux-2-gallery-angles / flux-2-gallery-staging

操作：generate_image（生成单张图片）、batch_generate（批量生成多张图片，一次提交所有分镜）、check_status（检查状态）、list_images（列出已生成图片）。
漫剧制作：用一致的 style 和 prompt 风格前缀保持角色和画面一致性。用 scene 字段标注分镜序号。`
}

func (t *ImageTool) Parameters() interface{} {
	return &JSONSchema{
		Type: "object",
		Properties: map[string]Property{
			"action":          {Type: "string", Description: "Action: generate_image, batch_generate, check_status, list_images"},
			"prompt":          {Type: "string", Description: "Image description. Be detailed: subject, action, composition, lighting, art style. For comics: include 'comic book style' or 'manga style' etc."},
			"negative_prompt": {Type: "string", Description: "What to avoid. Example: 'blurry, low quality, text, watermark, deformed'"},
			"model":           {Type: "string", Description: "Model: flux-schnell (default), flux-dev, flux-pro, flux-realism, stable-diffusion-v35-large, flux-2 (recommended), flux-2-pro, flux-2-max, flux-2-flex, flux-2-turbo, flux-2-flash, flux-2-edit, flux-2-pro-edit, flux-2-max-edit, flux-2-flex-edit, flux-2-turbo-edit, flux-2-flash-edit, flux-2-lora, flux-2-lora-edit, flux-2-klein-4b, flux-2-klein-9b, flux-2-klein-realtime, flux-2-gallery-realism, flux-2-gallery-comic, flux-2-gallery-sketch, flux-2-gallery-vintage, flux-2-gallery-hdr, flux-2-gallery-satellite, flux-2-gallery-tryon, flux-2-gallery-background, flux-2-gallery-portrait, flux-2-gallery-angles, flux-2-gallery-staging"},
			"size":            {Type: "string", Description: "Image size: square_hd (1024x1024, default), portrait_4_3 (768x1024), portrait_16_9 (576x1024), landscape_4_3 (1024x768), landscape_16_9 (1024x576). Or WxH like 720x1280."},
			"style":           {Type: "string", Description: "Style tag stored with record. Example: 'comic', 'manga', 'realistic', 'anime', 'watercolor'"},
			"scene":           {Type: "string", Description: "Panel/scene label for ordering (e.g. 'panel_1', 'panel_2'). Used by compose_comic."},
			"n":               {Type: "string", Description: "Number of images (1-4). Default: 1"},
			"task_id":         {Type: "string", Description: "For check_status: the image record ID or request ID."},
			"prompts":         {Type: "string", Description: "For batch_generate: JSON array of prompts. Each: {\"prompt\":\"...\", \"scene\":\"panel_1\"}. Shared negative_prompt/model/size/style apply to all. All images submitted in parallel."},
		},
		Required: []string{"action"},
	}
}

type imageArgs struct {
	Action         string `json:"action"`
	Prompt         string `json:"prompt"`
	NegativePrompt string `json:"negative_prompt"`
	Model          string `json:"model"`
	Size           string `json:"size"`
	Style          string `json:"style"`
	Scene          string `json:"scene"`
	N              string `json:"n"`
	TaskID         string `json:"task_id"`
	Prompts        string `json:"prompts"`
}

func (t *ImageTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	var args imageArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %v", err)
	}

	switch args.Action {
	case "generate_image":
		return t.generateImage(ctx, args)
	case "batch_generate":
		return t.batchGenerate(ctx, args)
	case "check_status":
		return t.checkStatus(ctx, args)
	case "list_images":
		return t.listImages(ctx)
	default:
		return "", fmt.Errorf("unknown action: %s. Use: generate_image, batch_generate, check_status, list_images", args.Action)
	}
}

// getFalAPIKeyCtx checks StarAI provider first, then falls back to user config
func (t *ImageTool) getFalAPIKeyCtx(ctx context.Context, userID string) string {
	return GetFalAPIKeyCtx(ctx, t.db, userID)
}

// modelToEndpoint maps user-facing model name to fal.ai endpoint
func modelToEndpoint(m string) string {
	switch m {
	// ── FLUX.1 ──
	case "flux-schnell":
		return "fal-ai/flux/schnell"
	case "flux-dev":
		return "fal-ai/flux/dev"
	case "flux-pro":
		return "fal-ai/flux-pro/v1.1"
	case "flux-realism":
		return "fal-ai/flux-realism"
	case "stable-diffusion-v35-large":
		return "fal-ai/stable-diffusion-v35-large"
	// ── FLUX.2 text-to-image ──
	case "flux-2":
		return "fal-ai/flux-2"
	case "flux-2-pro":
		return "fal-ai/flux-2-pro"
	case "flux-2-max":
		return "fal-ai/flux-2-max"
	case "flux-2-flex":
		return "fal-ai/flux-2-flex"
	case "flux-2-turbo":
		return "fal-ai/flux-2/turbo"
	case "flux-2-flash":
		return "fal-ai/flux-2/flash"
	// ── FLUX.2 image editing ──
	case "flux-2-edit":
		return "fal-ai/flux-2/edit"
	case "flux-2-pro-edit":
		return "fal-ai/flux-2-pro/edit"
	case "flux-2-max-edit":
		return "fal-ai/flux-2-max/edit"
	case "flux-2-flex-edit":
		return "fal-ai/flux-2-flex/edit"
	case "flux-2-turbo-edit":
		return "fal-ai/flux-2/turbo/edit"
	case "flux-2-flash-edit":
		return "fal-ai/flux-2/flash/edit"
	// ── FLUX.2 LoRA ──
	case "flux-2-lora":
		return "fal-ai/flux-2/lora"
	case "flux-2-lora-edit":
		return "fal-ai/flux-2/lora/edit"
	// ── FLUX.2 Klein ──
	case "flux-2-klein-4b":
		return "fal-ai/flux-2/klein/4b"
	case "flux-2-klein-4b-base":
		return "fal-ai/flux-2/klein/4b/base"
	case "flux-2-klein-4b-edit":
		return "fal-ai/flux-2/klein/4b/edit"
	case "flux-2-klein-4b-lora":
		return "fal-ai/flux-2/klein/4b/lora"
	case "flux-2-klein-4b-base-edit":
		return "fal-ai/flux-2/klein/4b/base/edit"
	case "flux-2-klein-4b-base-lora":
		return "fal-ai/flux-2/klein/4b/base/lora"
	case "flux-2-klein-4b-base-edit-lora":
		return "fal-ai/flux-2/klein/4b/base/edit/lora"
	case "flux-2-klein-4b-edit-lora":
		return "fal-ai/flux-2/klein/4b/edit/lora"
	case "flux-2-klein-9b":
		return "fal-ai/flux-2/klein/9b"
	case "flux-2-klein-9b-base":
		return "fal-ai/flux-2/klein/9b/base"
	case "flux-2-klein-9b-edit":
		return "fal-ai/flux-2/klein/9b/edit"
	case "flux-2-klein-9b-lora":
		return "fal-ai/flux-2/klein/9b/lora"
	case "flux-2-klein-9b-base-edit":
		return "fal-ai/flux-2/klein/9b/base/edit"
	case "flux-2-klein-9b-base-lora":
		return "fal-ai/flux-2/klein/9b/base/lora"
	case "flux-2-klein-9b-base-edit-lora":
		return "fal-ai/flux-2/klein/9b/base/edit/lora"
	case "flux-2-klein-9b-edit-lora":
		return "fal-ai/flux-2/klein/9b/edit/lora"
	case "flux-2-klein-realtime":
		return "fal-ai/flux-2/klein/realtime"
	// ── FLUX.2 LoRA Gallery ──
	case "flux-2-gallery-realism":
		return "fal-ai/flux-2-lora-gallery/realism"
	case "flux-2-gallery-angles":
		return "fal-ai/flux-2-lora-gallery/multiple-angles"
	case "flux-2-gallery-staging":
		return "fal-ai/flux-2-lora-gallery/apartment-staging"
	case "flux-2-gallery-tryon":
		return "fal-ai/flux-2-lora-gallery/virtual-tryon"
	case "flux-2-gallery-portrait":
		return "fal-ai/flux-2-lora-gallery/face-to-full-portrait"
	case "flux-2-gallery-background":
		return "fal-ai/flux-2-lora-gallery/add-background"
	case "flux-2-gallery-comic":
		return "fal-ai/flux-2-lora-gallery/digital-comic-art"
	case "flux-2-gallery-sketch":
		return "fal-ai/flux-2-lora-gallery/ballpoint-pen-sketch"
	case "flux-2-gallery-vintage":
		return "fal-ai/flux-2-lora-gallery/sepia-vintage"
	case "flux-2-gallery-satellite":
		return "fal-ai/flux-2-lora-gallery/satellite-view-style"
	case "flux-2-gallery-hdr":
		return "fal-ai/flux-2-lora-gallery/hdr-style"
	default:
		return "fal-ai/flux/schnell"
	}
}

// buildImageSize converts size string to fal.ai image_size parameter
func buildImageSize(size string) interface{} {
	// Named presets
	switch size {
	case "square_hd", "square", "portrait_4_3", "portrait_16_9", "landscape_4_3", "landscape_16_9":
		return size
	}
	// Custom WxH format (e.g. "720x1280" or "720*1280")
	size = strings.ReplaceAll(size, "*", "x")
	parts := strings.SplitN(size, "x", 2)
	if len(parts) == 2 {
		w, e1 := strconv.Atoi(strings.TrimSpace(parts[0]))
		h, e2 := strconv.Atoi(strings.TrimSpace(parts[1]))
		if e1 == nil && e2 == nil && w > 0 && h > 0 {
			return map[string]int{"width": w, "height": h}
		}
	}
	return "square_hd" // default
}

func (t *ImageTool) generateImage(ctx context.Context, args imageArgs) (string, error) {
	if args.Prompt == "" {
		return "", fmt.Errorf("prompt is required for image generation")
	}

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

	// Defaults
	if args.Model == "" {
		args.Model = "flux-schnell"
	}
	if args.Size == "" {
		args.Size = "square_hd"
	}
	n := 1
	if args.N != "" {
		if nn, err := strconv.Atoi(args.N); err == nil && nn >= 1 && nn <= 4 {
			n = nn
		}
	}

	endpoint := modelToEndpoint(args.Model)

	// Build fal.ai request body
	body := map[string]interface{}{
		"prompt":     args.Prompt,
		"image_size": buildImageSize(args.Size),
		"num_images": n,
	}

	// Submit to fal.ai queue (supports StarAI proxy)
	requestID, statusEndpoint, err := SubmitToFal(apiKey, endpoint, body)
	if err != nil {
		return "", fmt.Errorf("failed to submit image generation: %v", err)
	}

	log.Printf("[ImageTool] Submitted: request_id=%s (model=%s, size=%s, n=%d, scene=%s)", requestID, args.Model, args.Size, n, args.Scene)

	// Save record
	record := model.ImageRecord{
		UserID:         userID,
		ConversationID: convID,
		TaskID:         requestID,
		Model:          args.Model,
		Prompt:         args.Prompt,
		NegativePrompt: args.NegativePrompt,
		Size:           args.Size,
		Style:          args.Style,
		Scene:          args.Scene,
		Status:         "running",
	}
	t.db.Create(&record)

	// Audit log for reconciliation
	genLogID := LogGeneration(t.db, apiKey, GenLogOpts{
		UserID: userID, ConversationID: convID,
		Provider: "fal", Model: args.Model, Type: "image",
		TaskID: requestID, RecordID: record.ID,
		Prompt: args.Prompt, Status: "running",
	})

	// Block and wait for result (up to 3 min) to prevent LLM hallucinating fake URLs
	result, err := PollFalStatus(apiKey, statusEndpoint, requestID, 3*time.Minute)
	if err != nil {
		t.db.Model(&model.ImageRecord{}).Where("id = ?", record.ID).Updates(map[string]interface{}{"status": "failed"})
		UpdateGenLog(t.db, genLogID, "failed", "", err.Error())
		return "", fmt.Errorf("图片生成失败: %v", err)
	}

	// Extract image URL from result
	imageURL := extractFalImageURL(result)
	if imageURL == "" {
		t.db.Model(&model.ImageRecord{}).Where("id = ?", record.ID).Updates(map[string]interface{}{"status": "failed"})
		UpdateGenLog(t.db, genLogID, "failed", "", "no image URL in result")
		return toJSON(map[string]interface{}{
			"action":  "generate_image",
			"status":  "failed",
			"message": "图片生成完成但无法提取图片 URL",
		}), nil
	}

	localURL := t.downloadImage(imageURL, record.ID)
	if localURL == "" {
		t.db.Model(&model.ImageRecord{}).Where("id = ?", record.ID).Updates(map[string]interface{}{"status": "failed"})
		UpdateGenLog(t.db, genLogID, "failed", imageURL, "download to local failed")
		return toJSON(map[string]interface{}{
			"action":  "generate_image",
			"status":  "failed",
			"message": "图片已生成，但同步到本地图片服务失败，请重试。",
		}), nil
	}
	updates := map[string]interface{}{"status": "succeeded", "image_url": imageURL}
	if localURL != "" {
		updates["local_url"] = localURL
	}
	t.db.Model(&model.ImageRecord{}).Where("id = ?", record.ID).Updates(updates)
	UpdateGenLog(t.db, genLogID, "succeeded", localURL, "")

	displayURL := localURL
	log.Printf("[ImageTool] Image %s completed: %s", record.ID, displayURL)

	return toJSON(map[string]interface{}{
		"action":    "generate_image",
		"status":    "succeeded",
		"image_id":  record.ID,
		"image_url": displayURL,
		"model":     args.Model,
		"scene":     args.Scene,
		"size":      args.Size,
		"message":   "图片已生成完成，结果已保存到本地图片服务。",
	}), nil
}

// batchPrompt is a single prompt entry in the batch_generate prompts array
type batchPrompt struct {
	Prompt string `json:"prompt"`
	Scene  string `json:"scene"`
}

// batchGenerate submits multiple image generation tasks at once via fal.ai.
// This drastically reduces the number of LLM round-trips needed for multi-panel comics.
func (t *ImageTool) batchGenerate(ctx context.Context, args imageArgs) (string, error) {
	if args.Prompts == "" {
		return "", fmt.Errorf("prompts JSON array is required for batch_generate")
	}

	var prompts []batchPrompt
	if err := json.Unmarshal([]byte(args.Prompts), &prompts); err != nil {
		return "", fmt.Errorf("invalid prompts JSON: %v", err)
	}
	if len(prompts) == 0 {
		return "", fmt.Errorf("prompts array is empty")
	}
	if len(prompts) > 20 {
		return "", fmt.Errorf("max 20 images per batch")
	}

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
		args.Model = "flux-schnell"
	}
	if args.Size == "" {
		args.Size = "square_hd"
	}

	endpoint := modelToEndpoint(args.Model)
	imageSize := buildImageSize(args.Size)

	var results []map[string]interface{}

	for i, bp := range prompts {
		// Rate limit: small delay between submissions
		if i > 0 {
			time.Sleep(200 * time.Millisecond)
		}
		prompt := bp.Prompt
		if prompt == "" {
			continue
		}
		scene := bp.Scene
		if scene == "" {
			scene = fmt.Sprintf("panel_%d", i+1)
		}

		body := map[string]interface{}{
			"prompt":     prompt,
			"image_size": imageSize,
			"num_images": 1,
		}

		requestID, _, err := SubmitToFal(apiKey, endpoint, body)
		if err != nil {
			log.Printf("[ImageTool] batch: failed to submit %s: %v", scene, err)
			continue
		}

		// Save record
		record := model.ImageRecord{
			UserID:         userID,
			ConversationID: convID,
			TaskID:         requestID,
			Model:          args.Model,
			Prompt:         prompt,
			NegativePrompt: args.NegativePrompt,
			Size:           args.Size,
			Style:          args.Style,
			Scene:          scene,
			Status:         "running",
		}
		t.db.Create(&record)

		// Audit log for reconciliation
		genLogID := LogGeneration(t.db, apiKey, GenLogOpts{
			UserID: userID, ConversationID: convID,
			Provider: "fal", Model: args.Model, Type: "image",
			TaskID: requestID, RecordID: record.ID,
			Prompt: prompt, Status: "running",
		})

		// Poll in background
		go t.pollAndDownloadWithLog(apiKey, endpoint, requestID, record.ID, genLogID) // batch: background poll is OK

		results = append(results, map[string]interface{}{
			"image_id": record.ID,
			"scene":    scene,
		})

		log.Printf("[ImageTool] batch: submitted %s ↀ%s", scene, requestID)
	}

	return toJSON(map[string]interface{}{
		"action":  "batch_generate",
		"status":  "submitted",
		"count":   len(results),
		"model":   args.Model,
		"size":    args.Size,
		"tasks":   results,
		"message": fmt.Sprintf("已批量提交 %d 张图片生成任务！flux-schnell约10秒完成，用 list_images 查看进度。", len(results)),
	}), nil
}

// extractFalImageURL extracts the image URL from a fal.ai completed result.
func extractFalImageURL(result map[string]interface{}) string {
	// Flux models: result.images[0].url
	if images, ok := result["images"].([]interface{}); ok && len(images) > 0 {
		if img, ok := images[0].(map[string]interface{}); ok {
			if u, ok := img["url"].(string); ok && u != "" {
				return u
			}
		}
	}
	// SD models: result.image.url
	if img, ok := result["image"].(map[string]interface{}); ok {
		if u, ok := img["url"].(string); ok && u != "" {
			return u
		}
	}
	return ""
}

// pollAndDownload polls fal.ai queue status and downloads the result image (used by batch_generate)
// pollAndDownloadWithLog wraps polling with GenerationLog status updates
func (t *ImageTool) pollAndDownloadWithLog(apiKey, endpoint, requestID, recordID, genLogID string) {
	result, err := PollFalStatus(apiKey, endpoint, requestID, 5*time.Minute)
	if err != nil {
		log.Printf("[ImageTool] Poll failed for %s: %v", requestID, err)
		t.db.Model(&model.ImageRecord{}).Where("id = ?", recordID).Updates(map[string]interface{}{"status": "failed"})
		UpdateGenLog(t.db, genLogID, "failed", "", err.Error())
		return
	}

	imageURL := extractFalImageURL(result)
	if imageURL == "" {
		log.Printf("[ImageTool] No image URL in result for %s", requestID)
		t.db.Model(&model.ImageRecord{}).Where("id = ?", recordID).Updates(map[string]interface{}{"status": "failed"})
		UpdateGenLog(t.db, genLogID, "failed", "", "no image URL in result")
		return
	}

	localURL := t.downloadImage(imageURL, recordID)
	if localURL == "" {
		log.Printf("[ImageTool] Local sync failed for %s", requestID)
		t.db.Model(&model.ImageRecord{}).Where("id = ?", recordID).Updates(map[string]interface{}{"status": "failed"})
		UpdateGenLog(t.db, genLogID, "failed", imageURL, "download to local failed")
		return
	}
	updates := map[string]interface{}{"status": "succeeded", "image_url": imageURL}
	if localURL != "" {
		updates["local_url"] = localURL
	}
	t.db.Model(&model.ImageRecord{}).Where("id = ?", recordID).Updates(updates)
	UpdateGenLog(t.db, genLogID, "succeeded", localURL, "")
	log.Printf("[ImageTool] Image %s completed: %s", recordID, localURL)
}

// downloadImage downloads a remote image to /app/images/ and returns the local serve URL
func (t *ImageTool) downloadImage(remoteURL, recordID string) string {
	imgDir := ImagesDir()

	ext := ".png"
	lower := strings.ToLower(remoteURL)
	if strings.Contains(lower, ".jpg") || strings.Contains(lower, ".jpeg") {
		ext = ".jpg"
	} else if strings.Contains(lower, ".webp") {
		ext = ".webp"
	}

	filename := recordID + ext
	localPath := filepath.Join(imgDir, filename)

	if err := DownloadFile(remoteURL, localPath); err != nil {
		log.Printf("[ImageTool] Failed to download image %s: %v", recordID, err)
		return ""
	}

	localURL := "/v1/images/" + filename
	log.Printf("[ImageTool] Downloaded: %s ↀ%s", remoteURL, localURL)
	return localURL
}

func (t *ImageTool) checkStatus(ctx context.Context, args imageArgs) (string, error) {
	if args.TaskID == "" {
		return "", fmt.Errorf("task_id (image record ID) is required for check_status")
	}

	// Block and poll DB until terminal status or timeout (3 min)
	deadline := time.Now().Add(3 * time.Minute)
	var rec model.ImageRecord
	for time.Now().Before(deadline) {
		// Try by record ID first, then by task_id
		if err := t.db.Where("id = ?", args.TaskID).First(&rec).Error; err != nil {
			if err2 := t.db.Where("task_id = ?", args.TaskID).First(&rec).Error; err2 != nil {
				return "", fmt.Errorf("image record not found: %s", args.TaskID)
			}
		}
		if rec.Status == "succeeded" || rec.Status == "failed" {
			break
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(3 * time.Second):
		}
	}

	result := map[string]interface{}{
		"image_id": rec.ID,
		"status":   rec.Status,
		"model":    rec.Model,
		"scene":    rec.Scene,
	}

	switch rec.Status {
	case "succeeded":
		result["local_url"] = rec.LocalURL
		result["message"] = "图片已生成完成！"
	case "failed":
		result["message"] = "图片生成失败，请重试。"
	default:
		result["message"] = "图片正在生成中，请稍候..."
	}

	return toJSON(result), nil
}

func (t *ImageTool) listImages(ctx context.Context) (string, error) {
	userID := ""
	if uid, ok := ctx.Value(CtxKeyUserID).(string); ok {
		userID = uid
	}
	convID := ""
	if cid, ok := ctx.Value(CtxKeyConversationID).(string); ok {
		convID = cid
	}

	var records []model.ImageRecord
	query := t.db.Where("user_id = ?", userID)
	if convID != "" {
		query = query.Where("conversation_id = ?", convID)
	}
	query.Order("scene ASC, created_at ASC").Limit(50).Find(&records)

	var items []map[string]interface{}
	for _, r := range records {
		promptPreview := r.Prompt
		runes := []rune(promptPreview)
		if len(runes) > 60 {
			promptPreview = string(runes[:60]) + "..."
		}
		item := map[string]interface{}{
			"id":      r.ID,
			"task_id": r.TaskID,
			"status":  r.Status,
			"model":   r.Model,
			"scene":   r.Scene,
			"style":   r.Style,
			"size":    r.Size,
			"prompt":  promptPreview,
		}
		if r.LocalURL != "" {
			item["local_url"] = r.LocalURL
		}
		items = append(items, item)
	}

	return toJSON(map[string]interface{}{
		"action": "list_images",
		"count":  len(items),
		"images": items,
	}), nil
}
