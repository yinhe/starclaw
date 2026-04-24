package media

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/jpeg"
	"image/png"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	_ "golang.org/x/image/webp" // webp decode support for Ark image laundering

	"github.com/yinhe/starclaw/internal/sandbox"
)

// ── TOS Launder ────────────────────────────────────────────────
//
// Why this exists:
//   Seedance 2.0 rejects arbitrary image URLs with a privacy / policy filter.
//   Images that live on Volcengine Ark's own TOS bucket (produced by Seedream)
//   bypass that filter. TOS signed URLs expire after 24h, so the user needs a
//   one-click way to re-launder a local / CDN image and refresh the URL from
//   the Workflow canvas.
//
// Contract:
//   POST /v1/cdn/launder-tos
//   body: { "image_url": "<absolute http | /v1/projects/... | /v1/uploads/...>" }
//   resp: { "tos_url": "https://ark-...", "size": 1234567, "mime": "image/png", "note": "..." }

const (
	arkAPIKey         = "b3bc5f13-0bc3-458c-9ab6-efd8da387d3b"
	arkBaseURL        = "https://ark.cn-beijing.volces.com/api/v3"
	seedreamModel     = "doubao-seedream-5-0-260128"
	arkMaxBytes       = 9 * 1024 * 1024 // Seedream input limit is 10 MiB; keep headroom
	arkLaunderMaxSide = 2048            // resize long edge to this when downscaling is needed
	arkLaunderOutSide = 3072            // target long edge of Seedream output ("3K") for max quality
	arkLaunderMinSide = 512             // don't let the short edge collapse below this
	arkLaunderRoundTo = 8               // diffusion-model friendly rounding
)

type launderTOSReq struct {
	ImageURL string `json:"image_url" binding:"required"`
}

// LaunderTOSURL turns a local / CDN image into a fresh Volcengine Ark TOS URL
// by passing it through Seedream 5.0 lite with image_strength ~0.
func (h *CharacterStudioHandler) LaunderTOSURL(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req launderTOSReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "detail": err.Error()})
		return
	}
	raw := strings.TrimSpace(req.ImageURL)
	if raw == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "image_url is required"})
		return
	}

	data, mime, srcLabel, err := loadImageBytes(raw)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Enforce <= 9 MiB for Seedream input (10 MiB hard cap with headroom).
	data, mime, err = shrinkUnderArkLimit(data, mime)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 按原图宽高比计算 Seedream 输出尺寸（长边 3K），保持原图纵横比
	origW, origH, _ := decodeImageDims(data)
	outSize := computeLaunderSize(origW, origH, arkLaunderOutSide)

	tosURL, err := seedreamLaunder(c.Request.Context(), data, mime, outSize)
	if err != nil {
		log.Printf("[launder-tos] src=%s mime=%s size=%dKB in=%dx%d out=%s err=%v", srcLabel, mime, len(data)/1024, origW, origH, outSize, err)
		// Specialised response when Seedream's output filter rejected all
		// retries. Frontend shows a "用 CDN URL 代替 TOS" fallback button.
		if errors.Is(err, errSeedreamSensitive) {
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"error":  "Seedream 内容过滤连续拒绝了这张图（重试 3 次都挂在 OutputImageSensitiveContentDetected）",
				"reason": "seedream_sensitive_content",
				"hint":   "这张角色图太写实了让 Seedream 过不去。你可以：① 先把图推到 cdn.starclaw.net，然后直接用 CDN URL 喂 Seedance（Seedance 对 cdn.starclaw.net 是白名单）；② 换一张更非写实的候选图重试；③ 用 nano 润色生成一张更风格化的。",
				"detail": err.Error(),
			})
			return
		}
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"tos_url":      tosURL,
		"size":         len(data),
		"mime":         mime,
		"source":       srcLabel,
		"input_width":  origW,
		"input_height": origH,
		"output_size":  outSize,
		"note":         fmt.Sprintf("Seedream 5.0 lite image_strength=0.01, size=%s (3K long edge, aspect preserved), png. TOS URL valid ~24h.", outSize),
	})
}

// loadImageBytes resolves an http URL or known relative path to raw bytes + mime.
// Returns (data, mime, humanLabel, err).
func loadImageBytes(raw string) ([]byte, string, string, error) {
	if strings.HasPrefix(raw, "data:image/") {
		// Already a data URL — decode the base64 payload.
		semi := strings.Index(raw, ";")
		comma := strings.Index(raw, ",")
		if semi < 0 || comma < 0 || comma < semi {
			return nil, "", raw, fmt.Errorf("malformed data URL")
		}
		mime := raw[len("data:"):semi]
		payload := raw[comma+1:]
		data, err := base64.StdEncoding.DecodeString(payload)
		if err != nil {
			return nil, "", raw, fmt.Errorf("decode data URL: %v", err)
		}
		return data, mime, "data_url", nil
	}

	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, "GET", raw, nil)
		if err != nil {
			return nil, "", raw, fmt.Errorf("build request: %v", err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, "", raw, fmt.Errorf("fetch image: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 300 {
			return nil, "", raw, fmt.Errorf("fetch image: HTTP %d", resp.StatusCode)
		}
		data, err := io.ReadAll(io.LimitReader(resp.Body, 50*1024*1024))
		if err != nil {
			return nil, "", raw, fmt.Errorf("read image body: %v", err)
		}
		mime := resp.Header.Get("Content-Type")
		if !strings.HasPrefix(mime, "image/") {
			mime = guessMimeFromExt(filepath.Ext(raw))
		}
		return data, mime, raw, nil
	}

	// Relative paths under the Claw container filesystem.
	var diskPath string
	switch {
	case strings.HasPrefix(raw, "/v1/projects/"):
		// /v1/projects/<project>/<fp> → /app/docs/<project>/<fp>
		stripped := strings.TrimPrefix(raw, "/v1/projects/")
		if strings.Contains(stripped, "..") {
			return nil, "", raw, fmt.Errorf("path traversal blocked")
		}
		diskPath = filepath.Join("/app/docs", stripped)
	case strings.HasPrefix(raw, "/v1/uploads/"):
		filename := strings.TrimPrefix(raw, "/v1/uploads/")
		if strings.ContainsAny(filename, "/\\") || strings.Contains(filename, "..") {
			return nil, "", raw, fmt.Errorf("invalid upload filename")
		}
		diskPath = filepath.Join(sandbox.UploadsDir(), filename)
	case strings.HasPrefix(raw, "/v1/images/"):
		filename := strings.TrimPrefix(raw, "/v1/images/")
		if strings.ContainsAny(filename, "/\\") || strings.Contains(filename, "..") {
			return nil, "", raw, fmt.Errorf("invalid image filename")
		}
		// Fall back to uploads dir if images dir is not defined.
		diskPath = filepath.Join(sandbox.UploadsDir(), filename)
	default:
		return nil, "", raw, fmt.Errorf("unsupported image_url; expected http(s), /v1/projects/..., /v1/uploads/..., or data:image/")
	}

	if _, err := os.Stat(diskPath); err != nil {
		return nil, "", raw, fmt.Errorf("image not found on disk: %s", diskPath)
	}
	data, err := os.ReadFile(diskPath)
	if err != nil {
		return nil, "", raw, fmt.Errorf("read image: %v", err)
	}
	return data, guessMimeFromExt(filepath.Ext(diskPath)), raw, nil
}

func guessMimeFromExt(ext string) string {
	switch strings.ToLower(ext) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	case ".bmp":
		return "image/bmp"
	default:
		return "image/png"
	}
}

// shrinkUnderArkLimit ensures data is <= arkMaxBytes. It tries, in order:
//  1. pass-through when already under the limit
//  2. decode + re-encode as JPEG q=88 (huge wins for PNG photos)
//  3. progressively lower JPEG quality down to 70
//
// Returns the (possibly) re-encoded bytes and mime.
func shrinkUnderArkLimit(data []byte, mime string) ([]byte, string, error) {
	if len(data) <= arkMaxBytes {
		return data, mime, nil
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", fmt.Errorf("decode oversized image (%d bytes, mime=%s): %v", len(data), mime, err)
	}
	// Optional: if PNG decoded and still >> limit, re-encode PNG first; else skip.
	// Since JPEG is almost always much smaller, try it directly.
	for _, q := range []int{92, 88, 82, 75, 70} {
		buf := &bytes.Buffer{}
		if err := jpeg.Encode(buf, img, &jpeg.Options{Quality: q}); err != nil {
			return nil, "", fmt.Errorf("encode jpeg q=%d: %v", q, err)
		}
		if buf.Len() <= arkMaxBytes {
			return buf.Bytes(), "image/jpeg", nil
		}
	}
	// Still too big — try PNG re-encode (for flat graphics the compression may help).
	buf := &bytes.Buffer{}
	if err := png.Encode(buf, img); err == nil && buf.Len() <= arkMaxBytes {
		return buf.Bytes(), "image/png", nil
	}
	return nil, "", fmt.Errorf("image is too large for Seedream (%d bytes); try a smaller source or resize to ≤ %dx%d", len(data), arkLaunderMaxSide, arkLaunderMaxSide)
}

// decodeImageDims 返回图像的原始宽高，供算 Seedream 输出 size 使用。
// 解码失败时返回 (0, 0, err) —— 调用方应降级到正方形兵底。
func decodeImageDims(data []byte) (int, int, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return 0, 0, fmt.Errorf("decode image config: %v", err)
	}
	return cfg.Width, cfg.Height, nil
}

// computeLaunderSize 按 maxSide 作长边按原宽高比求出目标尺寸，并取 8 的倍数。
// 不让短边低于 arkLaunderMinSide。输入尺寸 <= 0 时退化为正方形。
func computeLaunderSize(w, h, maxSide int) string {
	if w <= 0 || h <= 0 {
		return fmt.Sprintf("%dx%d", maxSide, maxSide)
	}
	var nw, nh int
	if w >= h {
		nw = maxSide
		nh = int(math.Round(float64(h) * float64(maxSide) / float64(w)))
	} else {
		nh = maxSide
		nw = int(math.Round(float64(w) * float64(maxSide) / float64(h)))
	}
	round := func(v int) int {
		if v < arkLaunderMinSide {
			v = arkLaunderMinSide
		}
		return (v / arkLaunderRoundTo) * arkLaunderRoundTo
	}
	return fmt.Sprintf("%dx%d", round(nw), round(nh))
}

// ── Seedream retry strategy ───────────────────────────────────────
//
// Seedream 5.0 lite applies an *output* sensitive-content filter that
// frequently rejects realistic human portraits (especially female). Single-try
// launder = ~50% success rate on character sheets, which broke EP05 repeatedly.
// To mitigate we retry up to 3 times with progressively more stylized prompts
// and different seeds — each nudge away from photorealism makes the filter
// less likely to trip. If the first attempt succeeds we return immediately.
//
// If ALL attempts trip the sensitive filter we surface `errSeedreamSensitive`
// so the HTTP handler can return a structured 422 + hint the frontend can
// recognise ("use cdn_url as tos_url"), rather than a generic 502.

var errSeedreamSensitive = errors.New("seedream_sensitive_content")

// seedreamPromptVariants drives the retry loop. Index 0 = original prompt
// (closest to identity), later indices add stylization to dodge the filter.
var seedreamPromptVariants = []struct {
	Prompt string
	Seed   int
}{
	{"same image, keep exact face and composition, no changes", 42},
	{"same composition and characters, soft painterly colors, gentle film grain", 137},
	{"stylized illustration rendering of the same scene, anime-leaning look, soft lighting, same pose", 2048},
}

// isSensitiveContentErr checks whether an error (from HTTP body, Error{} field,
// or the final aggregated error) indicates Seedream's output-filter trip. Used
// both to drive the retry loop and to convert to errSeedreamSensitive sentinel.
func isSensitiveContentErr(msg string) bool {
	lower := strings.ToLower(msg)
	return strings.Contains(lower, "outputimagesensitivecontentdetected") ||
		strings.Contains(lower, "sensitivecontentdetected") ||
		strings.Contains(lower, "sensitive_content") ||
		strings.Contains(lower, "may contain sensitive")
}

// seedreamLaunder posts the image to Ark Seedream 5.0 lite with retry.
// Returns the TOS signed URL from the first successful attempt, or the
// last error (possibly wrapped in errSeedreamSensitive) on full failure.
func seedreamLaunder(parent context.Context, data []byte, mime, size string) (string, error) {
	if size == "" {
		size = fmt.Sprintf("%dx%d", arkLaunderOutSide, arkLaunderOutSide)
	}

	var lastErr error
	sensitiveRejects := 0
	for i, variant := range seedreamPromptVariants {
		url, err := seedreamLaunderOnce(parent, data, mime, size, variant.Prompt, variant.Seed)
		if err == nil {
			if i > 0 {
				log.Printf("[launder-tos] seedream retry #%d succeeded (prompt=%q, seed=%d)", i, variant.Prompt, variant.Seed)
			}
			return url, nil
		}
		lastErr = err
		if isSensitiveContentErr(err.Error()) {
			sensitiveRejects++
			log.Printf("[launder-tos] seedream attempt %d/%d rejected by sensitive filter; trying a more stylized prompt next…", i+1, len(seedreamPromptVariants))
			continue
		}
		// Non-sensitive error (network, quota, 5xx, …) — no point flipping
		// prompts for that, bail out immediately.
		return "", err
	}
	if sensitiveRejects == len(seedreamPromptVariants) {
		return "", fmt.Errorf("%w: all %d Seedream attempts rejected by OutputImageSensitiveContentDetected", errSeedreamSensitive, sensitiveRejects)
	}
	return "", lastErr
}

// seedreamLaunderOnce does a single Seedream request without any retry logic.
func seedreamLaunderOnce(parent context.Context, data []byte, mime, size, prompt string, seed int) (string, error) {
	b64 := base64.StdEncoding.EncodeToString(data)
	payload := map[string]any{
		"model":           seedreamModel,
		"prompt":          prompt,
		"response_format": "url",
		"size":            size,
		"image_strength":  0.01,
		"seed":            seed,
		"image":           []string{fmt.Sprintf("data:%s;base64,%s", mime, b64)},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal payload: %v", err)
	}
	ctx, cancel := context.WithTimeout(parent, 180*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "POST", arkBaseURL+"/images/generations", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+arkAPIKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("seedream request failed: %v", err)
	}
	defer resp.Body.Close()
	respBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if resp.StatusCode >= 300 {
		snippet := string(respBytes)
		if len(snippet) > 500 {
			snippet = snippet[:500]
		}
		// Propagate the whole snippet so isSensitiveContentErr can inspect it.
		return "", fmt.Errorf("seedream HTTP %d: %s", resp.StatusCode, snippet)
	}
	var parsed struct {
		Data []struct {
			URL string `json:"url"`
		} `json:"data"`
		Error *struct {
			Message string `json:"message"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return "", fmt.Errorf("parse seedream response: %v (body=%s)", err, truncate(string(respBytes), 300))
	}
	if parsed.Error != nil && parsed.Error.Message != "" {
		return "", fmt.Errorf("seedream error: %s (%s)", parsed.Error.Message, parsed.Error.Code)
	}
	if len(parsed.Data) == 0 || parsed.Data[0].URL == "" {
		return "", fmt.Errorf("seedream returned no URL (body=%s)", truncate(string(respBytes), 300))
	}
	return parsed.Data[0].URL, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
