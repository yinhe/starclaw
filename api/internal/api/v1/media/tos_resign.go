package media

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// ── TOS Resign ────────────────────────────────────────────────────
//
// Why:
//   Volcengine Ark's /images/generations returns TOS pre-signed URLs with
//   X-Tos-Expires=86400 (24h), and the API does not expose an `expires` knob.
//   Re-laundering through Seedream every 24h costs $$$ and is slow.
//   If we have the *user's* own Volcengine AKSK and that AKSK has GetObject
//   on the underlying bucket (ark-acg-cn-beijing), we can sign a fresh V4
//   pre-signed URL with up to 7 days (604800s) validity — zero Seedream cost.
//
//   V4 signing max is 604800s by Volcengine spec — not 30d.
//
// Env:
//   VOLC_TOS_AK       access key id (e.g. "AKLT...")
//   VOLC_TOS_SK_B64   secret key, base64-encoded (Volcengine console default format);
//                     raw plaintext SK also accepted — we fall through if base64 decode fails.

const (
	tosMaxExpiresSec     = 604800 // 7 days (Volcengine V4 hard cap)
	tosDefaultExpiresSec = 604800
	tosHeadTimeoutSec    = 10
)

type resignTOSReq struct {
	TOSUrl     string `json:"tos_url" binding:"required"`
	ExpiresSec int64  `json:"expires_sec,omitempty"`
	SkipHEAD   bool   `json:"skip_head,omitempty"` // opt-out of HEAD validation
}

// ResignTOSURL takes a stale Volcengine TOS pre-signed URL, re-signs it with
// the Claw-held VOLC_TOS_AK/SK for up to 7 days, then HEAD-validates it.
func (h *CharacterStudioHandler) ResignTOSURL(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req resignTOSReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body", "detail": err.Error()})
		return
	}
	ak := strings.TrimSpace(os.Getenv("VOLC_TOS_AK"))
	skRaw := strings.TrimSpace(os.Getenv("VOLC_TOS_SK_B64"))
	if ak == "" || skRaw == "" {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "VOLC_TOS_AK / VOLC_TOS_SK_B64 not set on backend — cannot self-sign"})
		return
	}

	// Build candidate SK interpretations. Volcengine IAM console may show SK
	// in any of these forms depending on which "copy" button the user used.
	// We try each and HEAD-validate; whichever returns 2xx is the real SK.
	skCandidates := buildSKCandidates(skRaw)

	expires := req.ExpiresSec
	if expires <= 0 {
		expires = tosDefaultExpiresSec
	}
	if expires > tosMaxExpiresSec {
		expires = tosMaxExpiresSec
	}

	now := time.Now().UTC()
	var (
		lastErr            string
		lastURL            string
		lastStatus         = -1
		lastRemote         gin.H
		accessDeniedErr    string
		accessDeniedURL    string
		accessDeniedStatus = -1
		accessDeniedRemote gin.H
		accessDeniedLabel  string
		probeSummaries     []gin.H
		triedLabels        []string
		matchedLabel       string
		debugFirst         struct{ CanonicalRequest, StringToSign string }
		skLensSummary      []string
	)
	for i, cand := range skCandidates {
		triedLabels = append(triedLabels, cand.label)
		skLensSummary = append(skLensSummary, fmt.Sprintf("%s=%db", cand.label, len(cand.sk)))
		var dbg *struct{ CanonicalRequest, StringToSign string }
		if i == 0 {
			dbg = &debugFirst // capture first variant's debug for diagnostics
		}
		newURL, err := resignTOSURLImplDebug(req.TOSUrl, ak, cand.sk, expires, now, dbg)
		if err != nil {
			lastErr = err.Error()
			continue
		}
		if req.SkipHEAD {
			c.JSON(http.StatusOK, gin.H{
				"tos_url":     newURL,
				"expires_sec": expires,
				"source":      "resign",
				"sk_variant":  cand.label,
				"head_status": -1,
				"note":        "HEAD skipped — URL not validated",
			})
			return
		}
		probe := validateTOSByHEAD(c.Request.Context(), newURL)
		probeSummaries = append(probeSummaries, gin.H{
			"sk_variant": cand.label,
			"status":     probe.StatusCode,
			"code":       probe.Code,
			"error":      probe.Err,
			"request_id": probe.RequestID,
		})
		log.Printf("[resign-tos] sk_variant=%s status=%d err=%q request_id=%q code=%q", cand.label, probe.StatusCode, probe.Err, probe.RequestID, probe.Code)
		if probe.StatusCode >= 200 && probe.StatusCode < 400 {
			matchedLabel = cand.label
			c.JSON(http.StatusOK, gin.H{
				"tos_url":     newURL,
				"expires_sec": expires,
				"source":      "resign",
				"sk_variant":  matchedLabel,
				"head_status": probe.StatusCode,
				"tried":       triedLabels,
			})
			return
		}
		lastErr = probe.Err
		lastURL = newURL
		lastStatus = probe.StatusCode
		lastRemote = probe.toGin()
		if probe.Code == "AccessDenied" && accessDeniedRemote == nil {
			accessDeniedErr = probe.Err
			accessDeniedURL = newURL
			accessDeniedStatus = probe.StatusCode
			accessDeniedRemote = probe.toGin()
			accessDeniedLabel = cand.label
			break
		}
	}

	log.Printf("[resign-tos] all %d SK variants failed (tried=%v last_status=%d)", len(skCandidates), triedLabels, lastStatus)
	errMsg := fmt.Sprintf("resigned URL validation failed: all %d SK variants returned non-2xx (last HTTP %d)", len(skCandidates), lastStatus)
	detail := lastErr
	attemptedURL := lastURL
	headStatus := lastStatus
	remote := lastRemote
	hint := "AKSK mismatch or no GetObject on this bucket; fall back to /v1/cdn/launder-tos"
	if accessDeniedRemote != nil {
		errMsg = fmt.Sprintf("resigned URL reached TOS but got AccessDenied (sk_variant=%s, HTTP %d)", accessDeniedLabel, accessDeniedStatus)
		detail = accessDeniedErr
		attemptedURL = accessDeniedURL
		headStatus = accessDeniedStatus
		remote = accessDeniedRemote
		hint = "raw-style SK reached TOS but lacks GetObject on the Ark bucket/object; self-signing will not work for this URL, fall back to /v1/cdn/launder-tos"
	}
	c.JSON(http.StatusBadGateway, gin.H{
		"error":             errMsg,
		"detail":            detail,
		"head_status":       headStatus,
		"tos_url_attempted": attemptedURL,
		"tried":             triedLabels,
		"sk_lengths":        skLensSummary,
		"variants":          probeSummaries,
		"remote":            remote,
		// Debug: our computed canonical_request + string_to_sign for the first
		// variant. Compare these byte-for-byte with what Volcengine returns in
		// its SignatureDoesNotMatch error response. If they differ, the bug is
		// in our canonical request construction. If they match, the bug is the
		// SK bytes / algorithm chain.
		"my_canonical_request": debugFirst.CanonicalRequest,
		"my_string_to_sign":    debugFirst.StringToSign,
		"hint":                 hint,
	})
	_ = matchedLabel
}

type skCandidate struct {
	label string
	sk    []byte
}

// buildSKCandidates expands the env var into all plausible HMAC-key shapes
// seen in the wild with Volcengine IAM console copy-paste:
//  1. raw       : the env value as-is (plaintext SK users)
//  2. b64_1     : base64-decode once
//  3. b64_2     : base64-decode twice (handles unpadded inner layer)
//  4. b64_1_hex : b64_1 interpreted as hex-encoded raw bytes
//
// Duplicates are deduped by hex hash of bytes.
func buildSKCandidates(envVal string) []skCandidate {
	out := []skCandidate{{label: "raw", sk: []byte(envVal)}}
	seen := map[string]bool{hex.EncodeToString([]byte(envVal)): true}
	appendIfNew := func(label string, b []byte) {
		if len(b) == 0 {
			return
		}
		key := hex.EncodeToString(b)
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, skCandidate{label: label, sk: b})
	}

	// Layer 1: base64 decode the env (handles Volcengine IAM console copy)
	d1, err := decodeB64Padded(envVal)
	if err == nil {
		appendIfNew("b64_1", d1)

		// Layer 2: if b64_1 is itself printable ASCII, try another layer.
		// Pad with '=' so non-multiple-of-4 strings still decode (very common
		// when the user trimmed trailing '=' or the console didn't include it).
		if isPrintableASCII(d1) {
			if d2, err := decodeB64Padded(string(d1)); err == nil {
				appendIfNew("b64_2", d2)

				// Layer 3: if b64_2 is ASCII AND looks like a hex string
				// (Volcengine IAM console sometimes double-encodes: raw SK
				// bytes → hex → base64 → base64 for copy-paste safety).
				// Hex-decode to get the actual HMAC key bytes.
				if isPrintableASCII(d2) && looksHex(d2) {
					if hx, err := hex.DecodeString(string(d2)); err == nil {
						appendIfNew("b64_2_hex", hx)
					}
				}
			}
			// Alt: if b64_1 looks like a hex string, decode as hex.
			if looksHex(d1) {
				if hx, err := hex.DecodeString(string(d1)); err == nil {
					appendIfNew("b64_1_hex", hx)
				}
			}
		}
	}
	return out
}

// decodeB64Padded wraps base64.StdEncoding.DecodeString but auto-pads with '='
// when the input length is not a multiple of 4. This matches what most tools
// (openssl, most SDKs) do by default.
func decodeB64Padded(s string) ([]byte, error) {
	if m := len(s) % 4; m != 0 {
		s = s + strings.Repeat("=", 4-m)
	}
	return base64.StdEncoding.DecodeString(s)
}

// looksHex reports whether every byte is in [0-9a-fA-F] and length is even.
func looksHex(b []byte) bool {
	if len(b) == 0 || len(b)%2 != 0 {
		return false
	}
	for _, x := range b {
		switch {
		case x >= '0' && x <= '9', x >= 'a' && x <= 'f', x >= 'A' && x <= 'F':
		default:
			return false
		}
	}
	return true
}

// resignTOSURLImpl generates a fresh TOS4-HMAC-SHA256 pre-signed URL for the
// same object path, with a caller-supplied expiration and signing clock.
// debugOut, when non-nil, is populated with the canonical_request and
// string_to_sign for cross-checking against the Volcengine server's own
// computed values (exposed in SignatureDoesNotMatch responses).
func resignTOSURLImplDebug(oldURL, ak string, sk []byte, expiresSec int64, now time.Time, debugOut *struct{ CanonicalRequest, StringToSign string }) (string, error) {
	u, err := url.Parse(oldURL)
	if err != nil {
		return "", fmt.Errorf("parse url: %v", err)
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return "", fmt.Errorf("unsupported scheme: %s", u.Scheme)
	}
	host := u.Host
	dot := strings.Index(host, ".")
	if dot < 0 {
		return "", fmt.Errorf("host missing bucket prefix: %s", host)
	}
	// bucket := host[:dot]  // not needed for signing; host is signed as a whole
	endpoint := host[dot+1:] // e.g. "tos-cn-beijing.volces.com"
	firstSeg := endpoint
	if i := strings.Index(endpoint, "."); i > 0 {
		firstSeg = endpoint[:i]
	}
	if !strings.HasPrefix(firstSeg, "tos-") {
		return "", fmt.Errorf("endpoint not recognized as TOS: %s", endpoint)
	}
	region := strings.TrimPrefix(firstSeg, "tos-") // "cn-beijing"

	isoDate := now.Format("20060102T150405Z")
	dateOnly := now.Format("20060102")
	credentialScope := fmt.Sprintf("%s/%s/tos/request", dateOnly, region)
	credential := ak + "/" + credentialScope

	// Rebuild query: drop old X-Tos-* params, keep any other (rare).
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

	// Canonical query string (RFC3986 encoding, sorted keys).
	canonicalQuery := buildCanonicalQuery(q)

	// Canonical URI is the RFC3986-encoded path (Go's EscapedPath matches).
	canonicalURI := u.EscapedPath()
	if canonicalURI == "" {
		canonicalURI = "/"
	}

	canonicalHeaders := "host:" + host + "\n"
	signedHeaders := "host"
	hashedPayload := "UNSIGNED-PAYLOAD"
	canonicalRequest := strings.Join([]string{
		"GET",
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
		hex.EncodeToString(crHash[:]),
	}, "\n")

	// Volcengine TOS V4 signing chain — unlike AWS S3 it does NOT prefix the
	// SK with "TOS4". Confirmed via official Volcengine ve-tos-golang-sdk
	// sign_v4.go `SigningKey()`:
	//   date = HMAC(sk, dateOnly); region = HMAC(date, region)
	//   service = HMAC(region, "tos"); signing = HMAC(service, "request")
	kDate := hmacSHA256(sk, []byte(dateOnly))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte("tos"))
	kSigning := hmacSHA256(kService, []byte("request"))
	signature := hex.EncodeToString(hmacSHA256(kSigning, []byte(stringToSign)))

	q.Set("X-Tos-Signature", signature)
	// Final URL query must use the same canonical encoding so Volcengine can
	// re-verify the signature bit-for-bit.
	u.RawQuery = buildCanonicalQuery(q)
	if debugOut != nil {
		debugOut.CanonicalRequest = canonicalRequest
		debugOut.StringToSign = stringToSign
	}
	return u.String(), nil
}

// resignTOSURLImpl is the non-debug wrapper kept for call-site compatibility.
func resignTOSURLImpl(oldURL, ak string, sk []byte, expiresSec int64, now time.Time) (string, error) {
	return resignTOSURLImplDebug(oldURL, ak, sk, expiresSec, now, nil)
}

// buildCanonicalQuery produces a sorted, RFC3986-encoded query string.
// Differs from Go's url.Values.Encode in that space becomes %20 (not +),
// matching AWS S3 / Volcengine TOS V4 signing expectations.
func buildCanonicalQuery(q url.Values) string {
	keys := make([]string, 0, len(q))
	for k := range q {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(q))
	for _, k := range keys {
		ek := rfc3986Escape(k)
		for _, v := range q[k] {
			parts = append(parts, ek+"="+rfc3986Escape(v))
		}
	}
	return strings.Join(parts, "&")
}

// rfc3986Escape percent-encodes per RFC3986 unreserved = A-Z a-z 0-9 - _ . ~
// (everything else → %HH, uppercase hex).
func rfc3986Escape(s string) string {
	b := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9',
			c == '-', c == '_', c == '.', c == '~':
			b = append(b, c)
		default:
			b = append(b, '%',
				"0123456789ABCDEF"[c>>4],
				"0123456789ABCDEF"[c&0x0F],
			)
		}
	}
	return string(b)
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

type tosProbeResult struct {
	StatusCode int
	Err        string
	RequestID  string
	ID2        string
	Code       string
	Body       string
}

func (r tosProbeResult) toGin() gin.H {
	if r.StatusCode == 0 && r.Err == "" && r.RequestID == "" && r.ID2 == "" && r.Code == "" && r.Body == "" {
		return nil
	}
	return gin.H{
		"status":     r.StatusCode,
		"error":      r.Err,
		"request_id": r.RequestID,
		"id_2":       r.ID2,
		"code":       r.Code,
		"body":       r.Body,
	}
}

// validateTOSByHEAD issues a HEAD request with a short timeout. Returns the
// HTTP status code (or -1) and a compact error string.
func validateTOSByHEAD(parent context.Context, urlStr string) tosProbeResult {
	ctx, cancel := context.WithTimeout(parent, tosHeadTimeoutSec*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "HEAD", urlStr, nil)
	if err != nil {
		return tosProbeResult{StatusCode: -1, Err: err.Error()}
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return tosProbeResult{StatusCode: -1, Err: err.Error()}
	}
	defer resp.Body.Close()
	result := tosProbeResult{
		StatusCode: resp.StatusCode,
		RequestID:  resp.Header.Get("x-tos-request-id"),
		ID2:        resp.Header.Get("x-tos-id-2"),
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return result
	}
	getReq, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		result.Err = err.Error()
		return result
	}
	getReq.Header.Set("Range", "bytes=0-0")
	getResp, err := http.DefaultClient.Do(getReq)
	if err != nil {
		result.Err = err.Error()
		return result
	}
	defer getResp.Body.Close()
	if result.RequestID == "" {
		result.RequestID = getResp.Header.Get("x-tos-request-id")
	}
	if result.ID2 == "" {
		result.ID2 = getResp.Header.Get("x-tos-id-2")
	}
	body, readErr := io.ReadAll(io.LimitReader(getResp.Body, 4096))
	if readErr != nil {
		result.Err = readErr.Error()
		return result
	}
	bodyStr := strings.TrimSpace(string(body))
	result.Body = bodyStr
	if strings.HasPrefix(bodyStr, "{") {
		var remoteJSON struct {
			Code      string `json:"Code"`
			RequestID string `json:"RequestId"`
		}
		if err := json.Unmarshal(body, &remoteJSON); err == nil {
			result.Code = remoteJSON.Code
			if result.RequestID == "" {
				result.RequestID = remoteJSON.RequestID
			}
		}
	}
	if result.Code == "" && strings.Contains(bodyStr, "<Code>SignatureDoesNotMatch</Code>") {
		result.Code = "SignatureDoesNotMatch"
	} else if result.Code == "" && strings.Contains(bodyStr, "<Code>AccessDenied</Code>") {
		result.Code = "AccessDenied"
	} else if result.Code == "" && bodyStr != "" {
		if i := strings.Index(bodyStr, "<Code>"); i >= 0 {
			if j := strings.Index(bodyStr[i+6:], "</Code>"); j >= 0 {
				result.Code = bodyStr[i+6 : i+6+j]
			}
		}
	}
	if result.Err == "" {
		result.Err = fmt.Sprintf("HEAD %d, GET %d", resp.StatusCode, getResp.StatusCode)
	}
	return result
}

func isPrintableASCII(b []byte) bool {
	if len(b) == 0 {
		return false
	}
	for _, x := range b {
		if x < 0x20 || x > 0x7E {
			return false
		}
	}
	return true
}
