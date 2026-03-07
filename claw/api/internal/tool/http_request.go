package tool

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// HTTPRequestTool makes HTTP requests to external APIs
type HTTPRequestTool struct {
	client *http.Client
}

func NewHTTPRequestTool() *HTTPRequestTool {
	return &HTTPRequestTool{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (t *HTTPRequestTool) Name() string {
	return "http_request"
}

func (t *HTTPRequestTool) Description() string {
	return "发送 HTTP 请求。支持 GET 和 POST 方法，可用于调用 API 接口或获取网页数据。"
}

func (t *HTTPRequestTool) Parameters() interface{} {
	return JSONSchema{
		Type: "object",
		Properties: map[string]Property{
			"url": {
				Type:        "string",
				Description: "The URL to send the request to",
			},
			"method": {
				Type:        "string",
				Description: "HTTP method (GET or POST)",
				Enum:        []string{"GET", "POST"},
			},
			"body": {
				Type:        "string",
				Description: "Request body (for POST requests)",
			},
			"headers": {
				Type:        "string",
				Description: "JSON object of request headers",
			},
		},
		Required: []string{"url"},
	}
}

type httpRequestArgs struct {
	URL     string `json:"url"`
	Method  string `json:"method"`
	Body    string `json:"body"`
	Headers string `json:"headers"`
}

func (t *HTTPRequestTool) Execute(ctx context.Context, args string) (string, error) {
	parsed, err := ParseArgs[httpRequestArgs](args)
	if err != nil {
		return "", err
	}

	if parsed.URL == "" {
		return "", fmt.Errorf("url is required")
	}

	method := strings.ToUpper(parsed.Method)
	if method == "" {
		method = "GET"
	}
	if method != "GET" && method != "POST" {
		return "", fmt.Errorf("only GET and POST methods are supported")
	}

	var bodyReader io.Reader
	if parsed.Body != "" {
		bodyReader = strings.NewReader(parsed.Body)
	}

	req, err := http.NewRequestWithContext(ctx, method, parsed.URL, bodyReader)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	if parsed.Body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("User-Agent", "StarClaw/1.0")

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Sprintf("Request failed: %v", err), nil
	}
	defer resp.Body.Close()

	// Limit response size to 50KB
	body, err := io.ReadAll(io.LimitReader(resp.Body, 50*1024))
	if err != nil {
		return fmt.Sprintf("Failed to read response: %v", err), nil
	}

	return fmt.Sprintf("Status: %d\nBody:\n%s", resp.StatusCode, string(body)), nil
}
