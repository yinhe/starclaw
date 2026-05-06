package media

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw/internal/tool"
	"gorm.io/gorm"
)

// ── Nano Banana (starai provider) generate endpoint ──────────────────────────
//
// POST /v1/nano/generate
//
//	body: {
//	  project, entity_kind ("characters"|"props"|"scenes"),
//	  entity_key ("lin_jianyue" | "coin" | ...),
//	  source_url (optional; triggers nano-banana-2/edit),
//	  prompt, model?, aspect_ratio?, resolution?, safety_tolerance?
//	}
//	resp: { local_path, local_url, sidecar_path, request_id, model, source_url?, generated_url }
//
// This wraps tool.SubmitToFal + tool.PollFalStatus (which already route through
// StarAI and deduct 星能), then downloads the result PNG into the v2 entities
// tree:
//
//	docs/<project>/entities/<kind>/<key>/nano/<ISO-ts>_<slug>.png
//
// A .json sidecar with the same basename records prompt / source / request_id
// so "回填本地" is fully auditable and reversible. The frontend NodePropertyPanel
// candidate bar auto-rescans after a successful generate, so the new image
// shows up as a picker tile under the "nano" tab.

// NanoHandler is the HTTP surface for StarAI-proxied nano-banana generation.
type NanoHandler struct {
	db *gorm.DB
}

func NewNanoHandler(db *gorm.DB) *NanoHandler { return &NanoHandler{db: db} }

// sanitize regex: strict ASCII for the slug portion of the filename to keep
// Seedance / Volcengine / fal.ai happy on any codepath.
var nanoSlugNonWord = regexp.MustCompile(`[^a-z0-9]+`)

// sanitizeSlug turns an arbitrary prompt into a short, lowercase, hyphenated
// filename slug: "make her smile 温暖一点" -> "make-her-smile". Empty → "gen".
func sanitizeSlug(prompt string) string {
	s := strings.ToLower(strings.TrimSpace(prompt))
	s = nanoSlugNonWord.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 32 {
		s = s[:32]
		s = strings.Trim(s, "-")
	}
	if s == "" {
		return "gen"
	}
	return s
}

type nanoGenReq struct {
	Project    string `json:"project" binding:"required"`
	EntityKind string `json:"entity_kind"` // characters | props | scenes
	EntityKey  string `json:"entity_key" binding:"required"`
	SourceURL  string `json:"source_url"` // empty = text→image; non-empty = image→image edit
	Prompt     string `json:"prompt" binding:"required"`
	Model      string `json:"model"` // default: nano-banana-2 or nano-banana-2/edit
	Size       string `json:"size"`  // passthrough to tool.buildNanoBananaBody
	// Optional: the caller can override aspect_ratio / resolution, but for v1
	// we just pass `size` and let the provider defaults stand.
}

// Generate handles POST /v1/nano/generate.
func (h *NanoHandler) Generate(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req nanoGenReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body", "detail": err.Error()})
		return
	}
	// Path-component sanitization: no traversal, no weird chars.
	if !archiveValidEpRe.MatchString(req.Project) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project id"})
		return
	}
	if req.EntityKind == "" {
		req.EntityKind = "characters"
	}
	if !archiveValidEpRe.MatchString(req.EntityKind) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid entity_kind"})
		return
	}
	if !archiveValidEpRe.MatchString(req.EntityKey) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid entity_key"})
		return
	}
	switch req.EntityKind {
	case "characters", "props", "scenes":
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "entity_kind must be characters|props|scenes"})
		return
	}

	// Pick model: editing an image → nano-banana-2/edit; pure text → nano-banana-2.
	// Caller can override with model=gpt-image-2 / gpt-image-2/edit (alpha).
	model := strings.TrimSpace(req.Model)
	if model == "" {
		if strings.TrimSpace(req.SourceURL) != "" {
			model = "nano-banana-2/edit"
		} else {
			model = "nano-banana-2"
		}
	} else if isGPTImage2Model(model) && strings.TrimSpace(req.SourceURL) != "" && model == "gpt-image-2" {
		// 用户选了 gpt-image-2 文生图但传了 source_url，自动升级到 /edit
		model = "gpt-image-2/edit"
	}

	// We route through StarAI regardless — that's the only path that gives us
	// a working fal.ai key + 星能 metering.
	apiKey := tool.GetFalAPIKeyCtx(c.Request.Context(), h.db, userID)
	if apiKey == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "no fal/starai API key available for this user"})
		return
	}

	endpoint := nanoEndpointForModel(model)
	body, err := nanoBuildBody(req.Prompt, req.SourceURL, req.Size, model)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot build nano request", "detail": err.Error()})
		return
	}

	// Submit + poll. We mirror tool/image_tool.go's 3-minute cap so the HTTP
	// caller doesn't hang forever; nano-banana-2 usually resolves in ~15-30s.
	requestID, statusEndpoint, err := tool.SubmitToFal(apiKey, endpoint, body)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "submit failed", "detail": err.Error()})
		return
	}
	result, err := tool.PollFalStatus(apiKey, statusEndpoint, requestID, 3*time.Minute)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "generation failed", "request_id": requestID, "detail": err.Error()})
		return
	}

	// fal.ai images[0].url is the remote PNG. Download + write next to entities/.
	remoteURL := nanoExtractImageURL(result)
	if remoteURL == "" {
		c.JSON(http.StatusBadGateway, gin.H{"error": "generation returned no image", "request_id": requestID, "result": result})
		return
	}

	docsDir := os.Getenv("DOCS_DIR")
	if docsDir == "" {
		docsDir = "/app/docs"
	}
	nanoDir := filepath.Join(docsDir, req.Project, "entities", req.EntityKind, req.EntityKey, "nano")
	if err := os.MkdirAll(nanoDir, 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot mkdir nano", "detail": err.Error()})
		return
	}

	// Filename: 2026-04-24T1015Z_smile-warmer_<short-hash>.png
	ts := time.Now().UTC().Format("2006-01-02T1504Z")
	slug := sanitizeSlug(req.Prompt)
	hashSrc := fmt.Sprintf("%s|%s|%s|%s", req.Prompt, req.SourceURL, model, requestID)
	sum := sha256.Sum256([]byte(hashSrc))
	short := hex.EncodeToString(sum[:])[:6]
	fileName := fmt.Sprintf("%s_%s_%s.png", ts, slug, short)
	localFSPath := filepath.Join(nanoDir, fileName)

	if err := nanoDownloadTo(c.Request.Context(), remoteURL, localFSPath); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "download back to local failed", "detail": err.Error(), "remote_url": remoteURL})
		return
	}
	fi, _ := os.Stat(localFSPath)
	sizeBytes := int64(0)
	if fi != nil {
		sizeBytes = fi.Size()
	}

	// Relative path for manifest + UI (same convention as suggestRef output).
	relSlash := fmt.Sprintf("/entities/%s/%s/nano/%s", req.EntityKind, req.EntityKey, fileName)
	localURL := "/v1/projects/" + req.Project + relSlash

	// Sidecar JSON with audit metadata.
	sidecarPath := strings.TrimSuffix(localFSPath, ".png") + ".json"
	sidecar := map[string]interface{}{
		"request_id":    requestID,
		"model":         model,
		"prompt":        req.Prompt,
		"source_url":    req.SourceURL,
		"generated_at":  time.Now().UTC().Format(time.RFC3339),
		"generated_url": remoteURL,
		"local_url":     localURL,
		"size_bytes":    sizeBytes,
	}
	if sb, err := json.MarshalIndent(sidecar, "", "  "); err == nil {
		_ = os.WriteFile(sidecarPath, sb, 0o644)
	}

	c.JSON(http.StatusOK, gin.H{
		"local_path":    relSlash,
		"local_url":     localURL,
		"sidecar_path":  strings.TrimSuffix(relSlash, ".png") + ".json",
		"request_id":    requestID,
		"model":         model,
		"source_url":    req.SourceURL,
		"generated_url": remoteURL,
		"size_bytes":    sizeBytes,
	})
}

// ── Helpers (mirror of tool/image_tool internals, duplicated here to avoid
//    exporting private provider builders) ─────────────────────────────────

func nanoEndpointForModel(model string) string {
	switch strings.ToLower(strings.TrimSpace(model)) {
	case "nano-banana-2":
		return "fal-ai/nano-banana-2"
	case "nano-banana-2/edit":
		return "fal-ai/nano-banana-2/edit"
	case "gpt-image-2":
		return "openai/gpt-image-2"
	case "gpt-image-2/edit":
		return "openai/gpt-image-2/edit"
	default:
		return "fal-ai/nano-banana-2"
	}
}

func isGPTImage2Model(model string) bool {
	switch strings.ToLower(strings.TrimSpace(model)) {
	case "gpt-image-2", "gpt-image-2/edit":
		return true
	}
	return false
}

func nanoBuildBody(prompt, imageURL, size, model string) (map[string]interface{}, error) {
	// OpenAI GPT Image 2 使用 image_size/quality 联合参数，不接受 aspect_ratio/resolution/safety_tolerance。
	if isGPTImage2Model(model) {
		imgSize := "auto"
		switch strings.TrimSpace(size) {
		case "square_hd", "square", "portrait_4_3", "portrait_16_9", "landscape_4_3", "landscape_16_9":
			imgSize = strings.TrimSpace(size)
		}
		body := map[string]interface{}{
			"prompt":        prompt,
			"num_images":    1,
			"image_size":    imgSize,
			"quality":       "high",
			"output_format": "png",
		}
		if strings.EqualFold(strings.TrimSpace(model), "gpt-image-2/edit") {
			urls := nanoSplitURLs(imageURL)
			if len(urls) == 0 {
				return nil, fmt.Errorf("source_url is required for gpt-image-2/edit")
			}
			body["image_urls"] = urls
		}
		return body, nil
	}

	aspect := nanoAspect(size)
	resolution := nanoResolution(size)
	body := map[string]interface{}{
		"prompt":            prompt,
		"num_images":        1,
		"aspect_ratio":      aspect,
		"output_format":     "png",
		"safety_tolerance":  "4",
		"resolution":        resolution,
		"limit_generations": true,
	}
	if strings.EqualFold(strings.TrimSpace(model), "nano-banana-2/edit") {
		urls := nanoSplitURLs(imageURL)
		if len(urls) == 0 {
			return nil, fmt.Errorf("source_url is required for nano-banana-2/edit")
		}
		body["image_urls"] = urls
	}
	return body, nil
}

func nanoAspect(size string) string {
	switch strings.TrimSpace(size) {
	case "", "square_hd", "square":
		return "1:1"
	case "portrait_4_3":
		return "3:4"
	case "portrait_16_9":
		return "9:16"
	case "landscape_4_3":
		return "4:3"
	case "landscape_16_9":
		return "16:9"
	}
	return "1:1"
}

func nanoResolution(size string) string {
	// 4K for nano-banana character sheets — user requires max-res PNG output.
	switch strings.TrimSpace(size) {
	case "", "square_hd", "square", "portrait_4_3", "portrait_16_9", "landscape_4_3", "landscape_16_9":
		return "4K"
	}
	return "4K"
}

// nanoSplitURLs parses a comma-separated source_url into a []string list.
// Most UIs pass a single URL, but fal.ai edit models accept up to 4.
func nanoSplitURLs(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// nanoExtractImageURL mirrors tool.extractFalImageURL but lives here to keep
// this package self-contained.
func nanoExtractImageURL(result map[string]interface{}) string {
	if images, ok := result["images"].([]interface{}); ok && len(images) > 0 {
		if img, ok := images[0].(map[string]interface{}); ok {
			if u, _ := img["url"].(string); u != "" {
				return u
			}
		}
	}
	// Some models nest under data.image.url.
	if data, ok := result["data"].(map[string]interface{}); ok {
		if img, ok := data["image"].(map[string]interface{}); ok {
			if u, _ := img["url"].(string); u != "" {
				return u
			}
		}
	}
	return ""
}

// nanoDownloadTo streams a remote PNG into a local file, honoring the
// caller's context so a client disconnect cancels the download.
func nanoDownloadTo(ctx context.Context, remoteURL, destPath string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, remoteURL, nil)
	if err != nil {
		return err
	}
	cli := &http.Client{Timeout: 90 * time.Second}
	resp, err := cli.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("remote returned %d", resp.StatusCode)
	}

	f, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer f.Close()

	// 20 MiB safety cap — nano-banana 1K PNGs sit around ~1-3 MiB, if we're
	// over 20 something went wrong upstream.
	if _, err := io.CopyN(f, resp.Body, 20*1024*1024); err != nil && err != io.EOF {
		// EOF on CopyN means we finished within the cap — that's fine.
		if err == io.ErrUnexpectedEOF {
			// Actually we want the tail; keep going with unbounded copy for
			// the rest of the (smaller-than-cap) body.
			if _, err2 := io.Copy(f, resp.Body); err2 != nil {
				return err2
			}
			return nil
		}
		return err
	}
	return nil
}
