package tool

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// VerifyOnlineTool performs post-deployment availability checks.
type VerifyOnlineTool struct{}

func NewVerifyOnlineTool() *VerifyOnlineTool { return &VerifyOnlineTool{} }

func (t *VerifyOnlineTool) Name() string { return "verify_online" }

func (t *VerifyOnlineTool) Description() string {
	return "上线验证：检查 URL 可访问、HTTP 状态、关键词命中情况（支持重试）。"
}

func (t *VerifyOnlineTool) Parameters() interface{} {
	return JSONSchema{
		Type: "object",
		Properties: map[string]Property{
			"url": {
				Type:        "string",
				Description: "Website URL to verify, e.g. https://example.com",
			},
			"expected_keywords": {
				Type:        "string",
				Description: "Expected keywords as comma-separated text or JSON array string",
			},
			"max_wait_sec": {
				Type:        "string",
				Description: "Timeout per request in seconds (default 20)",
			},
			"retry": {
				Type:        "string",
				Description: "Retry times when check fails (default 2)",
			},
			"interval_sec": {
				Type:        "string",
				Description: "Seconds between retries (default 3)",
			},
		},
		Required: []string{"url"},
	}
}

type verifyOnlineArgs struct {
	URL              string `json:"url"`
	ExpectedKeywords string `json:"expected_keywords"`
	MaxWaitSec       string `json:"max_wait_sec"`
	Retry            string `json:"retry"`
	IntervalSec      string `json:"interval_sec"`
}

func (t *VerifyOnlineTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	args, err := ParseArgs[verifyOnlineArgs](argsJSON)
	if err != nil {
		return "", err
	}

	url := strings.TrimSpace(args.URL)
	if url == "" {
		return "", fmt.Errorf("url is required")
	}

	timeout := parseIntWithDefault(args.MaxWaitSec, 20)
	if timeout < 5 {
		timeout = 5
	}
	retry := parseIntWithDefault(args.Retry, 2)
	if retry < 0 {
		retry = 0
	}
	interval := parseIntWithDefault(args.IntervalSec, 3)
	if interval < 1 {
		interval = 1
	}

	keywords := parseKeywords(args.ExpectedKeywords)
	attempts := retry + 1
	var lastCode int
	var lastLatency int64
	var lastErr string
	matched := []string{}

	for i := 1; i <= attempts; i++ {
		ok, code, latencyMs, body, err := probeURLWithBody(ctx, url, time.Duration(timeout)*time.Second)
		lastCode = code
		lastLatency = latencyMs
		if err != nil {
			lastErr = err.Error()
		} else if ok {
			matched = matchKeywords(body, keywords)
			if len(matched) == len(keywords) {
				return toJSON(map[string]interface{}{
					"status":             "success",
					"action":             "verify_online",
					"url":                url,
					"online":             true,
					"http_status":        code,
					"latency_ms":         latencyMs,
					"keywords_expected":  keywords,
					"keywords_matched":   matched,
					"attempt":            i,
					"attempts_total":     attempts,
					"message":            "site is online and verification passed",
				}), nil
			}
			lastErr = fmt.Sprintf("keyword mismatch: matched %d/%d", len(matched), len(keywords))
		}

		if i < attempts {
			time.Sleep(time.Duration(interval) * time.Second)
		}
	}

	return toJSON(map[string]interface{}{
		"status":             "failed",
		"action":             "verify_online",
		"url":                url,
		"online":             lastCode >= 200 && lastCode < 400,
		"http_status":        lastCode,
		"latency_ms":         lastLatency,
		"keywords_expected":  keywords,
		"keywords_matched":   matched,
		"attempts_total":     attempts,
		"error":              lastErr,
		"message":            "verification failed after retries",
	}), nil
}

func probeURL(ctx context.Context, url string, timeout time.Duration) (bool, int, int64, error) {
	ok, code, latencyMs, _, err := probeURLWithBody(ctx, url, timeout)
	return ok, code, latencyMs, err
}

func probeURLWithBody(ctx context.Context, url string, timeout time.Duration) (bool, int, int64, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, 0, 0, "", fmt.Errorf("invalid url: %w", err)
	}

	client := &http.Client{Timeout: timeout}
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return false, 0, 0, "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	latencyMs := time.Since(start).Milliseconds()
	ok := resp.StatusCode >= 200 && resp.StatusCode < 400
	return ok, resp.StatusCode, latencyMs, strings.ToLower(string(body)), nil
}

func parseIntWithDefault(v string, def int) int {
	v = strings.TrimSpace(v)
	if v == "" {
		return def
	}
	var n int
	_, err := fmt.Sscanf(v, "%d", &n)
	if err != nil {
		return def
	}
	return n
}

func parseKeywords(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{}
	}
	// Accept JSON-like array string: ["a","b"]
	raw = strings.TrimPrefix(raw, "[")
	raw = strings.TrimSuffix(raw, "]")
	raw = strings.ReplaceAll(raw, "\"", "")
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.ToLower(strings.TrimSpace(p))
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func matchKeywords(body string, keywords []string) []string {
	if len(keywords) == 0 {
		return []string{}
	}
	matched := make([]string, 0, len(keywords))
	for _, kw := range keywords {
		if strings.Contains(body, kw) {
			matched = append(matched, kw)
		}
	}
	return matched
}
