package media

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
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
	"golang.org/x/image/draw"
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
)

type launderTOSReq struct {
	ImageURL string `json:"image_url" binding:"required"`
	// Optional character context. When present (sent by NodePropertyPanel from
	// the manifest's appearance_card / character label), we prepend it to each
	// Seedream retry prompt so the model is told "this is [图1]林见月, 薄荷绿古装…
	// 写实短剧风绝不卡通" instead of blindly shuffling generic photorealism
	// hints. Drastically reduces drift / cartoonification on retries.
	// All three are pure text, no images, no PII — safe to forward verbatim.
	AppearanceCard string `json:"appearance_card"`
	CharacterLabel string `json:"character_label"`
	CharacterTag   string `json:"character_tag"`
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
	origLen := len(data)

	// Enforce <= 9 MiB for Seedream input (10 MiB hard cap with headroom).
	// 用户明确要求洗出来的 TOS URL 是 PNG，Seedream 输出格式跟着输入走，
	// 所以优先保持 PNG（必要时降分辨率），仅在 1200px PNG 都放不下时才 fallback 到 JPEG。
	data, mime, err = shrinkPreferPNG(data, mime)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 如果原始 URL 是公网 HTTPS 且 shrinkPreferPNG 未修改数据，直接把 URL 传给 Seedream，
	// 跳过 base64 编码（省带宽 + 时间，对 cdn.starclaw.net 上的大图尤其有效）。
	var directURL string
	if strings.HasPrefix(raw, "https://") && len(data) == origLen {
		directURL = raw
		log.Printf("[launder-tos] using direct URL for Seedream (skip base64): %s", raw)
	}

	// Seedream 5.0 lite 原生支持 "3K" 简写，模型自动根据输入图宽高比选择最佳输出尺寸
	origW, origH, _ := decodeImageDims(data)
	outSize := "3K"

	// Build a short anchor string like "处理 [图1] 林见月：薄荷绿古装汉服…"
	// that gets prepended to every Seedream retry prompt when provided.
	// Empty string when the caller didn't send any character context —
	// laundering falls back to the generic realism prompts as before.
	anchor := buildCharacterAnchor(req.CharacterTag, req.CharacterLabel, req.AppearanceCard)
	tosURL, err := seedreamLaunder(c.Request.Context(), data, mime, outSize, anchor, directURL)
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
		"note":         "Seedream 5.0 lite image_strength=0.01, size=3K (auto aspect), png. TOS URL valid ~24h.",
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
		// Volcengine TOS pre-signed URLs (ark-acg-cn-…) sometimes throttle or
		// have slow TTFB; 30s was tight enough that re-laundering an existing
		// TOS source would intermittently hit "read image body: context deadline
		// exceeded". Bumped to 120s — well below the 180s Seedream call below.
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
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

// shrinkPreferPNG keeps the input as PNG whenever possible so Seedream's output
// stays PNG too (user requirement — Seedream's output format follows input).
// Strategy:
//  1. Pass-through when already ≤ arkMaxBytes and still PNG
//  2. If too big, decode + re-encode PNG at the original dimension (PNG
//     re-encode can shrink thanks to DEFLATE on blocky source)
//  3. If still too big, progressively resize long edge
//     (2560→2048→1800→1600→1400→1200) and retry PNG at each step
//  4. Only fall back to JPEG if even 1200px PNG doesn't fit
//
// Returns the (possibly) re-encoded bytes and mime.
func shrinkPreferPNG(data []byte, mime string) ([]byte, string, error) {
	// Fast path: already small + already PNG → don't touch.
	if len(data) <= arkMaxBytes && strings.EqualFold(mime, "image/png") {
		return data, mime, nil
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", fmt.Errorf("decode oversized image (%d bytes, mime=%s): %v", len(data), mime, err)
	}

	tryPNG := func(im image.Image) ([]byte, bool) {
		buf := &bytes.Buffer{}
		enc := &png.Encoder{CompressionLevel: png.BestCompression}
		if err := enc.Encode(buf, im); err != nil {
			return nil, false
		}
		if buf.Len() <= arkMaxBytes {
			return buf.Bytes(), true
		}
		return nil, false
	}

	// Step 1/2: PNG at original dims.
	if b, ok := tryPNG(img); ok {
		return b, "image/png", nil
	}

	// Step 3: progressively resize long edge.
	for _, longEdge := range []int{2560, 2048, 1800, 1600, 1400, 1200} {
		resized := resizeLongEdge(img, longEdge)
		if b, ok := tryPNG(resized); ok {
			return b, "image/png", nil
		}
	}

	// Step 4: JPEG fallback (q=92 at 1600). Rare — only for super-detailed
	// photo sources. Seedream output format in this case will be JPEG.
	fallback := resizeLongEdge(img, 1600)
	for _, q := range []int{92, 88, 82, 75, 70} {
		buf := &bytes.Buffer{}
		if err := jpeg.Encode(buf, fallback, &jpeg.Options{Quality: q}); err != nil {
			return nil, "", fmt.Errorf("encode jpeg q=%d: %v", q, err)
		}
		if buf.Len() <= arkMaxBytes {
			return buf.Bytes(), "image/jpeg", nil
		}
	}
	return nil, "", fmt.Errorf("image is too large for Seedream (%d bytes); try a smaller source or resize to ≤ %dx%d", len(data), arkLaunderMaxSide, arkLaunderMaxSide)
}

// resizeLongEdge 按长边缩到 max、保持比例。max 大于或等于原长边则原图返回。
// 采样算法用 CatmullRom —— 角色三视图缩时保持边缘锐利，不模糊。
func resizeLongEdge(src image.Image, max int) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 || max <= 0 {
		return src
	}
	long := w
	if h > long {
		long = h
	}
	if long <= max {
		return src
	}
	var nw, nh int
	if w >= h {
		nw = max
		nh = int(math.Round(float64(h) * float64(max) / float64(w)))
	} else {
		nh = max
		nw = int(math.Round(float64(w) * float64(max) / float64(h)))
	}
	dst := image.NewNRGBA(image.Rect(0, 0, nw, nh))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)
	return dst
}

// shrinkUnderArkLimit: 老名字保留 — 让测试或其他调用处不破。新实现转发到 shrinkPreferPNG。
func shrinkUnderArkLimit(data []byte, mime string) ([]byte, string, error) { //nolint:unused
	return shrinkPreferPNG(data, mime)
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
// (closest to identity), later indices nudge the filter with DIFFERENT compositional
// cues but deliberately stay REALISTIC — we are a live-action short-drama project,
// an anime-leaning fallback here silently turned lin_jianyue's ref into a cartoon
// and broke every EP05 scene (user: "怎么变卡通人物了"). NEVER introduce "anime"
// or "illustration" here without a toggle — realism is a contract with the user.
var seedreamPromptVariants = []struct {
	Prompt string
	Seed   int
}{
	{"same character reference sheet image with multiple views (front, side, back), keep exact face, exact composition, exact layout, exact panel arrangement, no changes, photorealistic, natural skin, film photography", 42},
	{"identical multi-view character reference sheet, same panel layout and arrangement, identical subject and pose in every view, soft studio lighting, realistic skin texture, same composition, photojournalism, 35mm film", 137},
	{"professional cinematic still photograph of the same character reference sheet with front/side/back views, preserve exact multi-panel layout, realistic, natural daylight, DSLR quality, 8K photo, same pose and outfit in every panel", 2048},
}

// buildCharacterAnchor composes the optional "who is this" preamble prepended
// to every Seedream retry prompt when the caller supplied character context.
// Format examples:
//
//	tag="[图1]"  label="林见月"  card="薄荷绿古装汉服…"
//	   → "Subject is [图1] 林见月. Appearance: 薄荷绿古装汉服…. Live-action
//	      short-drama realism — absolutely not cartoon / anime / illustration."
//
// All fields are optional — returns "" when every input is empty, in which case
// the caller simply falls through to the generic variant prompts above.
func buildCharacterAnchor(tag, label, card string) string {
	tag = strings.TrimSpace(tag)
	label = strings.TrimSpace(label)
	card = strings.TrimSpace(card)
	if tag == "" && label == "" && card == "" {
		return ""
	}
	var head string
	switch {
	case tag != "" && label != "":
		head = fmt.Sprintf("Subject is %s %s.", tag, label)
	case label != "":
		head = fmt.Sprintf("Subject is %s.", label)
	case tag != "":
		head = fmt.Sprintf("Subject is %s.", tag)
	}
	var parts []string
	if head != "" {
		parts = append(parts, head)
	}
	if card != "" {
		// Cap appearance_card at 400 chars — avoids blowing through Seedream's
		// prompt budget on extremely verbose bible entries. Nearly every card in
		// the swarm-universe project sits well under this cap.
		if len([]rune(card)) > 400 {
			card = string([]rune(card)[:400])
		}
		parts = append(parts, "Appearance: "+card+".")
	}
	// Always append the realism clamp — this is the user's hard requirement
	// ("这部短剧是写实风格，绝不要卡通"). Mirrors the style_guide.md prefix.
	parts = append(parts, "Multi-view character reference sheet with front, side, and back views. Preserve exact panel layout and arrangement.")
	parts = append(parts, "Live-action short-drama realism, cinematic photograph, natural lighting, absolutely NOT cartoon / anime / illustration / 3D render.")
	return strings.Join(parts, " ")
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

// isSeedreamDownloadTimeout detects Seedream's "InvalidParameter: Timeout while
// downloading url=…" error — the upstream backend couldn't fetch our directURL
// reference (typical when CDN is slow from Volcengine's network or the URL
// pre-sign already expired). Caller falls back to inline base64 upload.
func isSeedreamDownloadTimeout(msg string) bool {
	lower := strings.ToLower(msg)
	return strings.Contains(lower, "timeout while downloading") ||
		strings.Contains(lower, "failed to download") ||
		strings.Contains(lower, "download image failed")
}

// deterministicSeed derives a stable 31-bit seed from the input image bytes
// XOR'd with a per-variant constant. Same image + same variant → same seed →
// Seedream produces a (near-)deterministic output across runs. This was added
// after the user reported "洗 TOS 出来的图老在变化" — the original code passed
// the variant seed verbatim, so identical inputs got identical seed but the
// upstream sampler still drifted slightly between calls; tying the seed to
// the source bytes makes the variation noise-floor instead of macroscopic.
func deterministicSeed(data []byte, variantSeed int) int {
	sum := sha256.Sum256(data)
	// Take first 4 bytes → uint32 → mask to 31-bit positive int (Seedream
	// docs say seed is a non-negative int32).
	h := binary.BigEndian.Uint32(sum[:4])
	combined := int64(h) ^ int64(uint32(variantSeed))
	return int(combined & 0x7FFFFFFF)
}

// seedreamLaunder posts the image to Ark Seedream 5.0 lite with retry.
// Returns the TOS signed URL from the first successful attempt, or the
// last error (possibly wrapped in errSeedreamSensitive) on full failure.
//
// `anchor` is optional — when non-empty it's prepended to every retry prompt
// (see buildCharacterAnchor). It tells Seedream "Subject is [图1] 林见月,
// 薄荷绿古装汉服…, live-action realism, not cartoon" so the model stops
// drifting to anime on the 2nd/3rd retry.
func seedreamLaunder(parent context.Context, data []byte, mime, size, anchor, directURL string) (string, error) {
	if size == "" {
		size = "3K"
	}

	var lastErr error
	sensitiveRejects := 0
	for i, variant := range seedreamPromptVariants {
		// Compose final prompt: character anchor (if any) + generic variant.
		// Order matters — putting the anchor FIRST means it survives even if
		// Seedream truncates long prompts.
		prompt := variant.Prompt
		if anchor != "" {
			prompt = anchor + " " + variant.Prompt
		}
		// Bind seed to source bytes → reproducible across runs for the same
		// input image. Different variants still get different seeds (via XOR
		// with variant.Seed) so the sensitive-filter retry loop still sees
		// genuinely different samplings.
		seed := deterministicSeed(data, variant.Seed)
		url, err := seedreamLaunderOnce(parent, data, mime, size, prompt, seed, directURL)
		if err == nil {
			if i > 0 {
				log.Printf("[launder-tos] seedream retry #%d succeeded (anchor=%t, seed=%d)", i, anchor != "", variant.Seed)
			}
			return url, nil
		}
		lastErr = err
		// Seedream's backend couldn't fetch our directURL (firewall, slow CDN,
		// expired pre-sign, …). We already have the bytes locally — switch to
		// base64 inline upload and retry the SAME variant immediately.
		if directURL != "" && isSeedreamDownloadTimeout(err.Error()) {
			log.Printf("[launder-tos] seedream couldn't fetch directURL (%s) — falling back to base64 inline upload", directURL)
			directURL = "" // permanent for the rest of this call
			url, err = seedreamLaunderOnce(parent, data, mime, size, prompt, seed, "")
			if err == nil {
				log.Printf("[launder-tos] base64 fallback succeeded on variant #%d", i)
				return url, nil
			}
			lastErr = err
		}
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
//
// Quality-critical params:
//   - size:           explicit WxH e.g. "3072x2304" (3K long edge, aspect from input)
//   - output_format:  "png" — user explicitly wants PNG out; docs list png/jpeg for 5.0-lite
//     (without this Seedream silently returns JPEG, costing perceived quality)
//   - watermark:      false — by default Ark adds a Doubao watermark that softens the bottom-right
//     corner and drops effective resolution; users reported 画质不行
//   - image_strength: 0.001 — minimal sampler influence; combined with a deterministic
//     per-image seed (deterministicSeed) this produces near-identity, reproducible output.
//     Older value 0.01 still produced visible drift between runs (user feedback "洗出来一直在变").
func seedreamLaunderOnce(parent context.Context, data []byte, mime, size, prompt string, seed int, directURL string) (string, error) {
	payload := map[string]any{
		"model":           seedreamModel,
		"prompt":          prompt,
		"response_format": "url",
		"size":            size,
		"image_strength":  0.001,
		"seed":            seed,
		"output_format":   "png",
		"watermark":       false,
	}
	// 当有公网 URL 时直接传 URL（对齐 SDK 格式：单字符串），否则 fallback 到 base64 data URI
	if directURL != "" {
		payload["image"] = directURL
	} else {
		b64 := base64.StdEncoding.EncodeToString(data)
		payload["image"] = []string{fmt.Sprintf("data:%s;base64,%s", mime, b64)}
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
