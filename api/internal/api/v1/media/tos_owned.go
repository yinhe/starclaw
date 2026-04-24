package media

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/yinhe/starclaw/internal/sandbox"
)

type promoteOwnedTOSReq struct {
	SourceURL   string `json:"source_url" binding:"required"`
	Class       string `json:"class"`
	TenantID    string `json:"tenant_id"`
	WorkspaceID string `json:"workspace_id"`
	ProjectID   string `json:"project_id"`
	AssetKind   string `json:"asset_kind"`
	AssetID     string `json:"asset_id"`
	Variant     string `json:"variant"`
	FileName    string `json:"file_name"`
	ExpiresSec  int64  `json:"expires_sec,omitempty"`
}

func (h *CharacterStudioHandler) PromoteToOwnedTOS(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	if h.cfg == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server config unavailable"})
		return
	}

	var req promoteOwnedTOSReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "detail": err.Error()})
		return
	}

	tosCfg := h.cfg.Storage.TOS
	if !tosCfg.Enabled {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "storage.tos is disabled"})
		return
	}
	ak := strings.TrimSpace(tosCfg.AccessKey)
	skRaw := strings.TrimSpace(tosCfg.SecretKey)
	bucket := strings.TrimSpace(tosCfg.OwnedBucket)
	endpointHost, err := normalizeTOSEndpointHost(tosCfg.Endpoint)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid storage.tos.endpoint", "detail": err.Error()})
		return
	}
	if ak == "" || skRaw == "" || bucket == "" || endpointHost == "" {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "storage.tos access_key / secret_key / owned_bucket / endpoint not fully configured"})
		return
	}

	className, err := normalizeOwnedTOSClass(req.Class)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	data, mimeType, srcLabel, srcName, err := loadPromoteSourceBytes(strings.TrimSpace(req.SourceURL))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(data) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "source produced empty body"})
		return
	}

	ext := inferPromoteExtension(strings.TrimSpace(req.FileName), srcName, mimeType)
	hash := sha256.Sum256(data)
	shaHex := fmt.Sprintf("%x", hash[:])
	objectKey := buildOwnedTOSObjectKey(userID, req, className, shaHex, ext, time.Now().UTC())
	objectURL := buildOwnedTOSObjectURL(bucket, endpointHost, objectKey)
	expiresSec := normalizeTOSExpires(req.ExpiresSec, tosCfg.PreSignTTLSeconds)
	skCandidates := buildSKCandidates(skRaw)
	if len(skCandidates) == 0 {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "no usable TOS secret key variants derived from storage.tos.secret_key"})
		return
	}

	region := strings.TrimSpace(tosCfg.Region)
	if region == "" {
		region = inferTOSRegionFromEndpoint(endpointHost)
	}
	if region == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot infer TOS region; set storage.tos.region"})
		return
	}

	if label, err := h.ensureOwnedBucket(c.Request.Context(), ak, region, endpointHost, bucket, skCandidates); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"error":  "owned TOS bucket unreachable and could not be created",
			"detail": err.Error(),
			"bucket": bucket,
			"region": region,
			"hint":   "verify AKSK has HeadBucket/CreateBucket on this region, or pre-create the bucket manually",
		})
		return
	} else if label != "" {
		for i := range skCandidates {
			if skCandidates[i].label == label && i > 0 {
				first := skCandidates[i]
				reordered := append([]skCandidate{first}, skCandidates[:i]...)
				reordered = append(reordered, skCandidates[i+1:]...)
				skCandidates = reordered
				break
			}
		}
	}

	var (
		lastErr     string
		lastStatus  = -1
		probeResult tosProbeResult
		matched     string
		summaries   []gin.H
	)

	for _, cand := range skCandidates {
		putURL, err := presignTOSMethodURL(objectURL, http.MethodPut, ak, cand.sk, 900, time.Now().UTC())
		if err != nil {
			lastErr = err.Error()
			summaries = append(summaries, gin.H{
				"sk_variant": cand.label,
				"upload":     "sign_failed",
				"error":      err.Error(),
			})
			continue
		}

		putStatus, putBody, err := putTOSObject(c.Request.Context(), putURL, mimeType, data)
		if err != nil {
			lastErr = err.Error()
			lastStatus = putStatus
			summaries = append(summaries, gin.H{
				"sk_variant":    cand.label,
				"upload_status": putStatus,
				"upload":        "failed",
				"error":         truncate(strings.TrimSpace(putBody), 300),
			})
			continue
		}

		readURL, err := resignTOSURLImpl(objectURL, ak, cand.sk, expiresSec, time.Now().UTC())
		if err != nil {
			lastErr = err.Error()
			lastStatus = putStatus
			summaries = append(summaries, gin.H{
				"sk_variant":    cand.label,
				"upload_status": putStatus,
				"upload":        "ok",
				"error":         err.Error(),
			})
			continue
		}

		probe := validateTOSByHEAD(c.Request.Context(), readURL)
		probeResult = probe
		summaries = append(summaries, gin.H{
			"sk_variant":    cand.label,
			"upload_status": putStatus,
			"head_status":   probe.StatusCode,
			"code":          probe.Code,
			"error":         probe.Err,
			"request_id":    probe.RequestID,
		})
		if probe.StatusCode >= 200 && probe.StatusCode < 400 {
			matched = cand.label
			c.JSON(http.StatusOK, gin.H{
				"tos_url":       readURL,
				"bucket":        bucket,
				"object_key":    objectKey,
				"object_url":    objectURL,
				"expires_sec":   expiresSec,
				"source":        srcLabel,
				"size":          len(data),
				"mime":          mimeType,
				"sha256":        shaHex,
				"sk_variant":    matched,
				"put_status":    putStatus,
				"head_status":   probe.StatusCode,
				"logical_claw":  tosCfg.LogicalClaw,
				"storage_class": tosCfg.DefaultStorageClass,
			})
			return
		}
		lastErr = probe.Err
		lastStatus = probe.StatusCode
	}

	c.JSON(http.StatusBadGateway, gin.H{
		"error":       "promote to owned TOS failed",
		"detail":      lastErr,
		"status":      lastStatus,
		"bucket":      bucket,
		"object_key":  objectKey,
		"object_url":  objectURL,
		"source":      srcLabel,
		"mime":        mimeType,
		"size":        len(data),
		"sha256":      shaHex,
		"variants":    summaries,
		"head_remote": probeResult.toGin(),
		"hint":        "verify storage.tos credentials, bucket policy, and owned bucket endpoint/region alignment",
	})
}

func normalizeTOSEndpointHost(endpoint string) (string, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return "", nil
	}
	if strings.Contains(endpoint, "://") {
		u, err := url.Parse(endpoint)
		if err != nil {
			return "", err
		}
		if u.Host == "" {
			return "", fmt.Errorf("missing host")
		}
		if u.Path != "" && u.Path != "/" {
			return "", fmt.Errorf("endpoint must not contain path")
		}
		return u.Host, nil
	}
	endpoint = strings.TrimSuffix(endpoint, "/")
	if strings.Contains(endpoint, "/") {
		return "", fmt.Errorf("endpoint must be host-only when scheme is omitted")
	}
	return endpoint, nil
}

func normalizeOwnedTOSClass(className string) (string, error) {
	className = strings.TrimSpace(strings.ToLower(className))
	if className == "" {
		return "raw", nil
	}
	switch className {
	case "raw", "derived", "quarantine", "tmp", "exports":
		return className, nil
	default:
		return "", fmt.Errorf("invalid class: %s", className)
	}
}

func normalizeTOSExpires(requested, fallback int64) int64 {
	expires := requested
	if expires <= 0 {
		expires = fallback
	}
	if expires <= 0 {
		expires = tosDefaultExpiresSec
	}
	if expires > tosMaxExpiresSec {
		expires = tosMaxExpiresSec
	}
	return expires
}

func loadPromoteSourceBytes(raw string) ([]byte, string, string, string, error) {
	if strings.HasPrefix(raw, "data:") {
		comma := strings.Index(raw, ",")
		semi := strings.Index(raw, ";")
		if comma <= 0 || semi <= 0 || semi > comma {
			return nil, "", raw, "", fmt.Errorf("malformed data URL")
		}
		meta := raw[len("data:"):comma]
		if !strings.Contains(meta, ";base64") {
			return nil, "", raw, "", fmt.Errorf("only base64 data URLs are supported")
		}
		mimeType := strings.TrimSuffix(meta, ";base64")
		data, err := base64.StdEncoding.DecodeString(raw[comma+1:])
		if err != nil {
			return nil, "", raw, "", fmt.Errorf("decode data URL: %v", err)
		}
		if len(data) > maxUploadSize {
			return nil, "", raw, "", fmt.Errorf("source too large, max %dMB", maxUploadSize/(1024*1024))
		}
		return data, mimeType, "data_url", "inline" + guessExtFromMimeType(mimeType), nil
	}

	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
		if err != nil {
			return nil, "", raw, "", fmt.Errorf("build request: %v", err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, "", raw, "", fmt.Errorf("fetch source: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 300 {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			return nil, "", raw, "", fmt.Errorf("fetch source: HTTP %d %s", resp.StatusCode, truncate(strings.TrimSpace(string(body)), 200))
		}
		data, err := io.ReadAll(io.LimitReader(resp.Body, maxUploadSize+1))
		if err != nil {
			return nil, "", raw, "", fmt.Errorf("read source body: %v", err)
		}
		if len(data) > maxUploadSize {
			return nil, "", raw, "", fmt.Errorf("source too large, max %dMB", maxUploadSize/(1024*1024))
		}
		u, _ := url.Parse(raw)
		fileName := path.Base(u.Path)
		mimeType := normalizeResponseMime(resp.Header.Get("Content-Type"))
		if mimeType == "" || mimeType == "application/octet-stream" {
			mimeType = guessMediaMimeFromExt(filepath.Ext(fileName))
		}
		return data, mimeType, raw, fileName, nil
	}

	var diskPath string
	switch {
	case strings.HasPrefix(raw, "/v1/projects/"):
		stripped := strings.TrimPrefix(raw, "/v1/projects/")
		if strings.Contains(stripped, "..") {
			return nil, "", raw, "", fmt.Errorf("path traversal blocked")
		}
		diskPath = filepath.Join("/app/docs", stripped)
	case strings.HasPrefix(raw, "/v1/uploads/"):
		filename := strings.TrimPrefix(raw, "/v1/uploads/")
		if strings.ContainsAny(filename, "/\\") || strings.Contains(filename, "..") {
			return nil, "", raw, "", fmt.Errorf("invalid upload filename")
		}
		diskPath = filepath.Join(sandbox.UploadsDir(), filename)
	case strings.HasPrefix(raw, "/v1/images/"):
		filename := strings.TrimPrefix(raw, "/v1/images/")
		if strings.ContainsAny(filename, "/\\") || strings.Contains(filename, "..") {
			return nil, "", raw, "", fmt.Errorf("invalid image filename")
		}
		diskPath = filepath.Join(sandbox.UploadsDir(), filename)
	default:
		return nil, "", raw, "", fmt.Errorf("unsupported source_url; expected http(s), /v1/projects/..., /v1/uploads/..., /v1/images/..., or data:...")
	}

	if _, err := os.Stat(diskPath); err != nil {
		return nil, "", raw, "", fmt.Errorf("source not found on disk: %s", diskPath)
	}
	data, err := os.ReadFile(diskPath)
	if err != nil {
		return nil, "", raw, "", fmt.Errorf("read source: %v", err)
	}
	if len(data) > maxUploadSize {
		return nil, "", raw, "", fmt.Errorf("source too large, max %dMB", maxUploadSize/(1024*1024))
	}
	fileName := filepath.Base(diskPath)
	return data, guessMediaMimeFromExt(filepath.Ext(fileName)), raw, fileName, nil
}

func normalizeResponseMime(contentType string) string {
	contentType = strings.TrimSpace(contentType)
	if contentType == "" {
		return ""
	}
	if i := strings.Index(contentType, ";"); i >= 0 {
		contentType = strings.TrimSpace(contentType[:i])
	}
	return contentType
}

func guessMediaMimeFromExt(ext string) string {
	ext = strings.ToLower(strings.TrimSpace(ext))
	if ext == "" {
		return "application/octet-stream"
	}
	if mimeType, ok := allowedExtensions[ext]; ok {
		return mimeType
	}
	if mimeType := mime.TypeByExtension(ext); mimeType != "" {
		return normalizeResponseMime(mimeType)
	}
	return "application/octet-stream"
}

func guessExtFromMimeType(mimeType string) string {
	mimeType = normalizeResponseMime(mimeType)
	switch mimeType {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	case "video/mp4":
		return ".mp4"
	case "audio/mpeg":
		return ".mp3"
	}
	if exts, err := mime.ExtensionsByType(mimeType); err == nil && len(exts) > 0 {
		return exts[0]
	}
	return ".bin"
}

func inferPromoteExtension(preferredName, sourceName, mimeType string) string {
	for _, name := range []string{preferredName, sourceName} {
		ext := strings.ToLower(filepath.Ext(strings.TrimSpace(name)))
		if ext != "" {
			return ext
		}
	}
	return guessExtFromMimeType(mimeType)
}

func sanitizeTOSKeySegment(s string, fallback string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return fallback
	}
	var b strings.Builder
	lastUnderscore := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastUnderscore = false
		case r == '-', r == '_', r == '.':
			b.WriteRune(r)
			lastUnderscore = false
		case r >= 0x4e00 && r <= 0x9fff:
			b.WriteRune(r)
			lastUnderscore = false
		default:
			if !lastUnderscore {
				b.WriteByte('_')
				lastUnderscore = true
			}
		}
	}
	out := strings.Trim(b.String(), "._-")
	if out == "" {
		return fallback
	}
	return out
}

func buildOwnedTOSObjectKey(userID string, req promoteOwnedTOSReq, className string, shaHex string, ext string, now time.Time) string {
	tenantID := sanitizeTOSKeySegment(req.TenantID, sanitizeTOSKeySegment(userID, "default"))
	workspaceID := sanitizeTOSKeySegment(req.WorkspaceID, "default")
	projectID := sanitizeTOSKeySegment(req.ProjectID, "default")
	assetKind := sanitizeTOSKeySegment(req.AssetKind, "imports")
	assetID := sanitizeTOSKeySegment(req.AssetID, sanitizeTOSKeySegment(uuid.New().String(), "asset"))
	variant := sanitizeTOSKeySegment(req.Variant, "orig")
	ext = strings.ToLower(strings.TrimSpace(ext))
	if ext == "" || !strings.HasPrefix(ext, ".") {
		ext = ".bin"
	}
	return strings.Join([]string{
		className,
		tenantID,
		workspaceID,
		projectID,
		assetKind,
		now.Format("2006"),
		now.Format("01"),
		now.Format("02"),
		assetID,
		shaHex + "_" + variant + ext,
	}, "/")
}

func buildOwnedTOSObjectURL(bucket string, endpointHost string, objectKey string) string {
	parts := strings.Split(objectKey, "/")
	escaped := make([]string, 0, len(parts))
	for _, part := range parts {
		escaped = append(escaped, rfc3986Escape(part))
	}
	return "https://" + bucket + "." + endpointHost + "/" + strings.Join(escaped, "/")
}

func presignTOSMethodURL(rawURL string, method string, ak string, sk []byte, expiresSec int64, now time.Time) (string, error) {
	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "" {
		method = http.MethodGet
	}
	if expiresSec <= 0 {
		expiresSec = 900
	}
	if expiresSec > tosMaxExpiresSec {
		expiresSec = tosMaxExpiresSec
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("parse url: %v", err)
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return "", fmt.Errorf("unsupported scheme: %s", u.Scheme)
	}
	if ak == "" || len(sk) == 0 {
		return "", fmt.Errorf("missing credentials")
	}

	host := u.Host
	dot := strings.Index(host, ".")
	if dot < 0 {
		return "", fmt.Errorf("host missing bucket prefix: %s", host)
	}
	endpoint := host[dot+1:]
	firstSeg := endpoint
	if i := strings.Index(endpoint, "."); i > 0 {
		firstSeg = endpoint[:i]
	}
	if !strings.HasPrefix(firstSeg, "tos-") {
		return "", fmt.Errorf("endpoint not recognized as TOS: %s", endpoint)
	}
	region := strings.TrimPrefix(firstSeg, "tos-")

	isoDate := now.Format("20060102T150405Z")
	dateOnly := now.Format("20060102")
	credentialScope := fmt.Sprintf("%s/%s/tos/request", dateOnly, region)
	credential := ak + "/" + credentialScope

	q := url.Values{}
	for k, vv := range u.Query() {
		if strings.HasPrefix(strings.ToUpper(k), "X-TOS-") {
			continue
		}
		q[k] = vv
	}
	q.Set("X-Tos-Algorithm", "TOS4-HMAC-SHA256")
	q.Set("X-Tos-Credential", credential)
	q.Set("X-Tos-Date", isoDate)
	q.Set("X-Tos-Expires", fmt.Sprintf("%d", expiresSec))
	q.Set("X-Tos-SignedHeaders", "host")

	canonicalQuery := buildCanonicalQuery(q)
	canonicalURI := u.EscapedPath()
	if canonicalURI == "" {
		canonicalURI = "/"
	}
	canonicalHeaders := "host:" + host + "\n"
	signedHeaders := "host"
	hashedPayload := "UNSIGNED-PAYLOAD"
	canonicalRequest := strings.Join([]string{
		method,
		canonicalURI,
		canonicalQuery,
		canonicalHeaders,
		signedHeaders,
		hashedPayload,
	}, "\n")
	crHash := sha256.Sum256([]byte(canonicalRequest))
	stringToSign := strings.Join([]string{
		"TOS4-HMAC-SHA256",
		isoDate,
		credentialScope,
		fmt.Sprintf("%x", crHash[:]),
	}, "\n")
	kDate := hmacSHA256(sk, []byte(dateOnly))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte("tos"))
	kSigning := hmacSHA256(kService, []byte("request"))
	signature := fmt.Sprintf("%x", hmacSHA256(kSigning, []byte(stringToSign)))
	q.Set("X-Tos-Signature", signature)
	u.RawQuery = buildCanonicalQuery(q)
	return u.String(), nil
}

func putTOSObject(parent context.Context, putURL string, mimeType string, data []byte) (int, string, error) {
	ctx, cancel := context.WithTimeout(parent, 120*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, putURL, bytes.NewReader(data))
	if err != nil {
		return -1, "", err
	}
	if mimeType != "" {
		req.Header.Set("Content-Type", mimeType)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return -1, "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	bodyStr := strings.TrimSpace(string(body))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, bodyStr, fmt.Errorf("PUT %d: %s", resp.StatusCode, truncate(bodyStr, 300))
	}
	return resp.StatusCode, bodyStr, nil
}

// inferTOSRegionFromEndpoint derives "cn-beijing" from "tos-cn-beijing.volces.com".
// Returns empty string if the endpoint doesn't follow the expected shape.
func inferTOSRegionFromEndpoint(endpointHost string) string {
	endpointHost = strings.TrimSpace(endpointHost)
	if endpointHost == "" {
		return ""
	}
	firstSeg := endpointHost
	if i := strings.Index(endpointHost, "."); i > 0 {
		firstSeg = endpointHost[:i]
	}
	if !strings.HasPrefix(firstSeg, "tos-") {
		return ""
	}
	return strings.TrimPrefix(firstSeg, "tos-")
}

// signTOSRequestV4 adds a Volcengine TOS V4 Authorization header (and the
// required x-tos-date / x-tos-content-sha256 headers) to req. Uses the
// AWS-style canonical request / string-to-sign / derived signing key chain.
// Intended for administrative operations (HeadBucket / CreateBucket) where
// query-string presigning is not appropriate.
func signTOSRequestV4(req *http.Request, ak string, sk []byte, region string, payload []byte) error {
	if req == nil || req.URL == nil {
		return fmt.Errorf("nil request or url")
	}
	if region == "" {
		return fmt.Errorf("missing region")
	}
	host := req.Host
	if host == "" {
		host = req.URL.Host
	}
	now := time.Now().UTC()
	isoDate := now.Format("20060102T150405Z")
	dateOnly := now.Format("20060102")

	payloadHash := sha256.Sum256(payload)
	payloadHashHex := fmt.Sprintf("%x", payloadHash[:])

	req.Host = host
	req.Header.Set("X-Tos-Date", isoDate)
	req.Header.Set("X-Tos-Content-Sha256", payloadHashHex)

	canonicalHeaders := "host:" + host + "\n" +
		"x-tos-content-sha256:" + payloadHashHex + "\n" +
		"x-tos-date:" + isoDate + "\n"
	signedHeaders := "host;x-tos-content-sha256;x-tos-date"

	canonicalURI := req.URL.EscapedPath()
	if canonicalURI == "" {
		canonicalURI = "/"
	}
	canonicalQuery := ""
	if req.URL.RawQuery != "" {
		canonicalQuery = buildCanonicalQuery(req.URL.Query())
	}
	canonicalRequest := strings.Join([]string{
		req.Method,
		canonicalURI,
		canonicalQuery,
		canonicalHeaders,
		signedHeaders,
		payloadHashHex,
	}, "\n")
	crHash := sha256.Sum256([]byte(canonicalRequest))
	credentialScope := fmt.Sprintf("%s/%s/tos/request", dateOnly, region)
	stringToSign := strings.Join([]string{
		"TOS4-HMAC-SHA256",
		isoDate,
		credentialScope,
		fmt.Sprintf("%x", crHash[:]),
	}, "\n")

	kDate := hmacSHA256(sk, []byte(dateOnly))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte("tos"))
	kSigning := hmacSHA256(kService, []byte("request"))
	signature := fmt.Sprintf("%x", hmacSHA256(kSigning, []byte(stringToSign)))

	req.Header.Set("Authorization", fmt.Sprintf(
		"TOS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		ak, credentialScope, signedHeaders, signature,
	))
	return nil
}

// ensureOwnedBucket serialises HEAD/PUT bucket attempts and caches success.
// After the first successful HEAD or CreateBucket the handler skips further
// checks for the lifetime of the process. On failure we keep the cache
// untouched so the next promote request can retry.
func (h *CharacterStudioHandler) ensureOwnedBucket(ctx context.Context, ak, region, endpointHost, bucket string, skCandidates []skCandidate) (string, error) {
	h.ensureBucketMu.Lock()
	defer h.ensureBucketMu.Unlock()
	if h.ensureBucketOK {
		return h.ensureBucketLabel, nil
	}
	label, err := ensureOwnedBucketImpl(ctx, bucket, endpointHost, region, ak, skCandidates)
	if err != nil {
		return "", err
	}
	h.ensureBucketOK = true
	h.ensureBucketLabel = label
	return label, nil
}

// ensureOwnedBucketImpl does one-pass HEAD-then-PUT across every SK variant.
//   - HEAD 2xx: bucket exists and this SK variant has access → success.
//   - HEAD 404: bucket missing → PUT to create.
//   - HEAD 403: likely wrong SK variant OR bucket belongs to someone else;
//     we advance to the next candidate but surface the error if none succeed.
//   - PUT 2xx: bucket created with this SK variant → success.
func ensureOwnedBucketImpl(ctx context.Context, bucket, endpointHost, region, ak string, skCandidates []skCandidate) (string, error) {
	bucketURL := "https://" + bucket + "." + endpointHost + "/"
	var lastErr string
	for _, cand := range skCandidates {
		headCtx, headCancel := context.WithTimeout(ctx, 10*time.Second)
		headReq, err := http.NewRequestWithContext(headCtx, http.MethodHead, bucketURL, nil)
		if err != nil {
			headCancel()
			lastErr = err.Error()
			continue
		}
		if err := signTOSRequestV4(headReq, ak, cand.sk, region, nil); err != nil {
			headCancel()
			lastErr = err.Error()
			continue
		}
		headResp, err := http.DefaultClient.Do(headReq)
		headCancel()
		if err != nil {
			lastErr = err.Error()
			continue
		}
		headStatus := headResp.StatusCode
		headReqID := headResp.Header.Get("X-Tos-Request-Id")
		headResp.Body.Close()
		if headStatus >= 200 && headStatus < 300 {
			return cand.label, nil
		}
		if headStatus != 404 {
			lastErr = fmt.Sprintf("HEAD bucket %d sk_variant=%s request_id=%s", headStatus, cand.label, headReqID)
			continue
		}

		putCtx, putCancel := context.WithTimeout(ctx, 30*time.Second)
		putReq, err := http.NewRequestWithContext(putCtx, http.MethodPut, bucketURL, nil)
		if err != nil {
			putCancel()
			lastErr = err.Error()
			continue
		}
		putReq.ContentLength = 0
		if err := signTOSRequestV4(putReq, ak, cand.sk, region, nil); err != nil {
			putCancel()
			lastErr = err.Error()
			continue
		}
		putResp, err := http.DefaultClient.Do(putReq)
		putCancel()
		if err != nil {
			lastErr = err.Error()
			continue
		}
		body, _ := io.ReadAll(io.LimitReader(putResp.Body, 4096))
		putStatus := putResp.StatusCode
		putReqID := putResp.Header.Get("X-Tos-Request-Id")
		putResp.Body.Close()
		if putStatus >= 200 && putStatus < 300 {
			return cand.label, nil
		}
		lastErr = fmt.Sprintf("PUT bucket %d sk_variant=%s request_id=%s body=%s",
			putStatus, cand.label, putReqID, truncate(strings.TrimSpace(string(body)), 200))
	}
	if lastErr == "" {
		lastErr = "no SK variants tried"
	}
	return "", fmt.Errorf("ensure owned TOS bucket %s failed: %s", bucket, lastErr)
}
