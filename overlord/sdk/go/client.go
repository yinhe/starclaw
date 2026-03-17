package starclaw

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is the StarClaw Go SDK client.
//
// Usage:
//
//	client := starclaw.NewClient("sk-xxx", starclaw.WithEndpoint("https://overlord.company.com"))
//	resp, err := client.Chat(ctx, starclaw.ChatCompletionRequest{
//	    Model:    "deepseek-chat",
//	    Messages: []starclaw.ChatMessage{{Role: "user", Content: "Hello"}},
//	})
type Client struct {
	endpoint   string
	apiKey     string
	timeoutSec int
	httpClient *http.Client
}

// NewClient creates a new StarClaw client with the given API key and options.
func NewClient(apiKey string, opts ...Option) *Client {
	c := &Client{
		endpoint:   "https://overlord.company.com",
		apiKey:     apiKey,
		timeoutSec: 30,
	}
	for _, opt := range opts {
		opt(c)
	}
	c.httpClient = &http.Client{
		Timeout: time.Duration(c.timeoutSec) * time.Second,
	}
	return c
}

func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.endpoint+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	return c.httpClient.Do(req)
}

// Chat creates a non-streaming chat completion.
func (c *Client) Chat(ctx context.Context, req ChatCompletionRequest) (*ChatCompletionResponse, error) {
	req.Stream = false

	resp, err := c.doRequest(ctx, "POST", "/v1/chat/completions", req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var result ChatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}

// ListModels returns the available models.
func (c *Client) ListModels(ctx context.Context) ([]Model, error) {
	resp, err := c.doRequest(ctx, "GET", "/v1/models", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var result ModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return result.Data, nil
}

// Health checks the API health endpoint.
func (c *Client) Health(ctx context.Context) error {
	resp, err := c.doRequest(ctx, "GET", "/health", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed: %d", resp.StatusCode)
	}
	return nil
}
