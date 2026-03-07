package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type AnthropicProvider struct {
	apiKey  string
	baseURL string
	models  []string
	client  *http.Client
}

type AnthropicConfig struct {
	APIKey  string
	BaseURL string
}

func NewAnthropicProvider(cfg AnthropicConfig) *AnthropicProvider {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	return &AnthropicProvider{
		apiKey:  cfg.APIKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		models:  []string{"claude-sonnet-4-20250514", "claude-3-5-sonnet-20241022", "claude-3-5-haiku-20241022", "claude-3-opus-20240229"},
		client:  &http.Client{},
	}
}

func (p *AnthropicProvider) Name() string {
	return "anthropic"
}

func (p *AnthropicProvider) Models() []string {
	return p.models
}

func (p *AnthropicProvider) ChatSync(ctx context.Context, req *ChatRequest) (*ChatChunk, error) {
	req.Stream = false
	body, err := p.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	var resp anthropicResponse
	if err := json.NewDecoder(body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	chunk := &ChatChunk{
		ID:   resp.ID,
		Role: resp.Role,
		Done: true,
	}

	for _, block := range resp.Content {
		if block.Type == "text" {
			chunk.Content += block.Text
		}
		if block.Type == "tool_use" {
			argsJSON, _ := json.Marshal(block.Input)
			chunk.Tool = &ToolCall{
				ID:   block.ID,
				Type: "function",
				Function: FunctionCall{
					Name:      block.Name,
					Arguments: string(argsJSON),
				},
			}
		}
	}

	if resp.Usage != nil {
		chunk.Usage = &TokenUsage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		}
	}

	return chunk, nil
}

func (p *AnthropicProvider) Chat(ctx context.Context, req *ChatRequest) (<-chan *ChatChunk, error) {
	req.Stream = true
	body, err := p.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	ch := make(chan *ChatChunk, 32)

	go func() {
		defer close(ch)
		defer body.Close()

		scanner := bufio.NewScanner(body)
		var currentToolCall *ToolCall

		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")

			var event anthropicStreamEvent
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}

			switch event.Type {
			case "content_block_start":
				if event.ContentBlock != nil && event.ContentBlock.Type == "tool_use" {
					currentToolCall = &ToolCall{
						ID:   event.ContentBlock.ID,
						Type: "function",
						Function: FunctionCall{
							Name: event.ContentBlock.Name,
						},
					}
				}

			case "content_block_delta":
				if event.Delta != nil {
					if event.Delta.Type == "text_delta" {
						select {
						case ch <- &ChatChunk{Content: event.Delta.Text}:
						case <-ctx.Done():
							return
						}
					}
					if event.Delta.Type == "input_json_delta" && currentToolCall != nil {
						currentToolCall.Function.Arguments += event.Delta.PartialJSON
					}
				}

			case "content_block_stop":
				if currentToolCall != nil {
					select {
					case ch <- &ChatChunk{Tool: currentToolCall}:
					case <-ctx.Done():
						return
					}
					currentToolCall = nil
				}

			case "message_delta":
				// May contain usage info at the end

			case "message_stop":
				var usage *TokenUsage
				if event.Usage != nil {
					usage = &TokenUsage{
						PromptTokens:     event.Usage.InputTokens,
						CompletionTokens: event.Usage.OutputTokens,
						TotalTokens:      event.Usage.InputTokens + event.Usage.OutputTokens,
					}
				}
				select {
				case ch <- &ChatChunk{Done: true, Usage: usage}:
				case <-ctx.Done():
				}
				return
			}
		}
	}()

	return ch, nil
}

func (p *AnthropicProvider) doRequest(ctx context.Context, req *ChatRequest) (io.ReadCloser, error) {
	// Convert messages: extract system prompt, convert tool messages
	systemPrompt, messages := toAnthropicMessages(req.Messages)

	payload := anthropicRequest{
		Model:     req.Model,
		Messages:  messages,
		MaxTokens: req.MaxTokens,
		Stream:    req.Stream,
	}

	if systemPrompt != "" {
		payload.System = systemPrompt
	}

	if req.Temperature > 0 {
		payload.Temperature = &req.Temperature
	}

	if payload.MaxTokens == 0 {
		payload.MaxTokens = 4096
	}

	// Convert tools to Anthropic format
	if len(req.Tools) > 0 {
		for _, t := range req.Tools {
			payload.Tools = append(payload.Tools, anthropicTool{
				Name:        t.Function.Name,
				Description: t.Function.Description,
				InputSchema: t.Function.Parameters,
			})
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Anthropic API error (status %d): %s", resp.StatusCode, string(errBody))
	}

	return resp.Body, nil
}

func toAnthropicMessages(msgs []ChatMessage) (string, []anthropicMessage) {
	var systemPrompt string
	var messages []anthropicMessage

	for _, m := range msgs {
		if m.Role == "system" {
			systemPrompt = m.Content
			continue
		}

		role := m.Role
		if role == "tool" {
			// Anthropic uses "user" role with tool_result content
			messages = append(messages, anthropicMessage{
				Role: "user",
				Content: []anthropicContent{{
					Type:      "tool_result",
					ToolUseID: m.ToolCallID,
					Content:   m.Content,
				}},
			})
			continue
		}

		if len(m.ToolCalls) > 0 {
			// Assistant message with tool calls
			var content []anthropicContent
			if m.Content != "" {
				content = append(content, anthropicContent{Type: "text", Text: m.Content})
			}
			for _, tc := range m.ToolCalls {
				var input interface{}
				json.Unmarshal([]byte(tc.Function.Arguments), &input)
				content = append(content, anthropicContent{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Function.Name,
					Input: input,
				})
			}
			messages = append(messages, anthropicMessage{Role: "assistant", Content: content})
			continue
		}

		messages = append(messages, anthropicMessage{
			Role:    role,
			Content: []anthropicContent{{Type: "text", Text: m.Content}},
		})
	}

	return systemPrompt, messages
}

// --- Anthropic API types ---

type anthropicRequest struct {
	Model       string             `json:"model"`
	Messages    []anthropicMessage `json:"messages"`
	System      string             `json:"system,omitempty"`
	MaxTokens   int                `json:"max_tokens"`
	Temperature *float64           `json:"temperature,omitempty"`
	Stream      bool               `json:"stream"`
	Tools       []anthropicTool    `json:"tools,omitempty"`
}

type anthropicMessage struct {
	Role    string             `json:"role"`
	Content []anthropicContent `json:"content"`
}

type anthropicContent struct {
	Type        string      `json:"type"`
	Text        string      `json:"text,omitempty"`
	ID          string      `json:"id,omitempty"`
	Name        string      `json:"name,omitempty"`
	Input       interface{} `json:"input,omitempty"`
	ToolUseID   string      `json:"tool_use_id,omitempty"`
	Content     string      `json:"content,omitempty"`
}

type anthropicTool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"input_schema"`
}

type anthropicResponse struct {
	ID      string             `json:"id"`
	Role    string             `json:"role"`
	Content []anthropicContent `json:"content"`
	Usage   *anthropicUsage    `json:"usage"`
}

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type anthropicStreamEvent struct {
	Type         string           `json:"type"`
	Delta        *anthropicDelta  `json:"delta,omitempty"`
	ContentBlock *anthropicContent `json:"content_block,omitempty"`
	Usage        *anthropicUsage  `json:"usage,omitempty"`
}

type anthropicDelta struct {
	Type        string `json:"type"`
	Text        string `json:"text,omitempty"`
	PartialJSON string `json:"partial_json,omitempty"`
}
