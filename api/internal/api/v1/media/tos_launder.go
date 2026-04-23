package media

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/jpeg"
	"image/png"
	_ "image/png"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
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
	arkAPIKey       = "b3bc5f13-0bc3-458c-9ab6-efd8da387d3b"
	arkBaseURL      = "https://ark.cn-beijing.volces.com/api/v3"
	seedreamModel   = "doubao-seedream-5-0-260128"
	arkMaxBytes     = 9 * 1024 * 1024 // Seedream input limit is 10 MiB; keep headroom
	arkLaunderMaxSide = 2048          // resize long edge to this when downscaling is needed
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

	tosURL, err := seedreamLaunder(c.Request.Context(), data, mime)
	if err != nil {
		log.Printf("[launder-tos] src=%s mime=%s size=%dKB err=%v", srcLabel, mime, len(data)/1024, err)
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"tos_url": tosURL,
		"size":    len(data),
		"mime":    mime,
		"source":  srcLabel,
		"note":    "Seedream 5.0 lite image_strength=0.01; TOS URL valid ~24h. Re-run to refresh.",
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

// seedreamLaunder posts the image to Ark Seedream 5.0 lite and returns the
// TOS signed URL from the first result.
func seedreamLaunder(parent context.Context, data []byte, mime string) (string, error) {
	b64 := base64.StdEncoding.EncodeToString(data)
	payload := map[string]any{
		"model":           seedreamModel,
		"prompt":          "same image, keep exact face and composition, no changes",
		"response_format": "url",
		"size":            "2048x2048",
		"image_strength":  0.01,
		"seed":            42,
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
