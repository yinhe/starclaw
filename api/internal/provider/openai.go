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
	"time"
)

type OpenAIProvider struct {
	apiKey     string
	baseURL    string
	authPrefix string
	models     []string
	client     *http.Client
}

type OpenAIConfig struct {
	APIKey     string
	BaseURL    string
	AuthPrefix string // "Bearer" (default) or "Key" for fal.ai
}

func NewOpenAIProvider(cfg OpenAIConfig) *OpenAIProvider {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	authPrefix := cfg.AuthPrefix
	if authPrefix == "" {
		authPrefix = "Bearer"
	}
	return &OpenAIProvider{
		apiKey:     cfg.APIKey,
		baseURL:    strings.TrimRight(baseURL, "/"),
		authPrefix: authPrefix,
		models: []string{
			// ── Reasoning ──
			"o3",
			"o3-mini",
			"o4-mini",
			"o1",
			"o1-mini",
			"o1-pro",

			// ── GPT-4.1 ──
			"gpt-4.1",
			"gpt-4.1-mini",
			"gpt-4.1-nano",

			// ── GPT-4o ──
			"gpt-4o",
			"gpt-4o-mini",
			"gpt-4o-audio-preview",
			"gpt-4o-mini-audio-preview",
			"gpt-4o-search-preview",
			"gpt-4o-mini-search-preview",
			"gpt-4o-transcribe",
			"gpt-4o-mini-transcribe",
			"chatgpt-4o-latest",

			// ── GPT-4 ──
			"gpt-4-turbo",
			"gpt-4",

			// ── GPT-3.5 ──
			"gpt-3.5-turbo",

			// ── Image Generation ──
			"gpt-image-1",
			"dall-e-3",
			"dall-e-2",

			// ── Audio / Speech ──
			"tts-1",
			"tts-1-hd",
			"whisper-1",

			// ── Embeddings ──
			"text-embedding-3-large",
			"text-embedding-3-small",
			"text-embedding-ada-002",

			// ── Moderation ──
			"omni-moderation-latest",
			"text-moderation-latest",

			// ── Codex ──
			"codex-mini-latest",
		},
		client: &http.Client{Timeout: 5 * time.Minute},
	}
}

func (p *OpenAIProvider) Name() string {
	return "openai"
}

func (p *OpenAIProvider) Models() []string {
	return p.models
}

func (p *OpenAIProvider) ChatSync(ctx context.Context, req *ChatRequest) (*ChatChunk, error) {
	req.Stream = false
	resp, err := p.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var apiResp openAIChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(apiResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	choice := apiResp.Choices[0]
	chunk := &ChatChunk{
		ID:      apiResp.ID,
		Content: contentToString(choice.Message.Content),
		Role:    choice.Message.Role,
		Done:    true,
		Meta:    extractStarAIMeta(resp),
	}

	if len(choice.Message.ToolCalls) > 0 {
		chunk.Tool = &ToolCall{
			ID:   choice.Message.ToolCalls[0].ID,
			Type: choice.Message.ToolCalls[0].Type,
			Function: FunctionCall{
				Name:      choice.Message.ToolCalls[0].Function.Name,
				Arguments: choice.Message.ToolCalls[0].Function.Arguments,
			},
		}
	}

	if apiResp.Usage != nil {
		chunk.Usage = &TokenUsage{
			PromptTokens:     apiResp.Usage.PromptTokens,
			CompletionTokens: apiResp.Usage.CompletionTokens,
			TotalTokens:      apiResp.Usage.TotalTokens,
		}
	}

	return chunk, nil
}

func (p *OpenAIProvider) Chat(ctx context.Context, req *ChatRequest) (<-chan *ChatChunk, error) {
	req.Stream = true
	resp, err := p.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	ch := make(chan *ChatChunk, 32)
	meta := extractStarAIMeta(resp)

	go func() {
		defer close(ch)
		defer resp.Body.Close()

		firstChunk := true
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				ch <- &ChatChunk{Done: true}
				return
			}

			var streamResp openAIStreamChunk
			if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
				continue
			}

			if len(streamResp.Choices) == 0 {
				continue
			}

			delta := streamResp.Choices[0].Delta
			chunk := &ChatChunk{
				ID:        streamResp.ID,
				Content:   contentToString(delta.Content),
				Reasoning: delta.ReasoningContent,
				Role:      delta.Role,
			}

			// Attach upstream meta to first content chunk
			if firstChunk && len(meta) > 0 {
				chunk.Meta = meta
				firstChunk = false
			}

			if len(delta.ToolCalls) > 0 {
				tc := delta.ToolCalls[0]
				chunk.Tool = &ToolCall{
					ID:   tc.ID,
					Type: tc.Type,
					Function: FunctionCall{
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					},
				}
			}

			if streamResp.Usage != nil {
				chunk.Usage = &TokenUsage{
					PromptTokens:     streamResp.Usage.PromptTokens,
					CompletionTokens: streamResp.Usage.CompletionTokens,
					TotalTokens:      streamResp.Usage.TotalTokens,
				}
			}

			select {
			case ch <- chunk:
			case <-ctx.Done():
				return
			}
		}
	}()

	return ch, nil
}

// extractStarAIMeta pulls X-StarAI-* headers from the upstream response.
func extractStarAIMeta(resp *http.Response) map[string]string {
	meta := make(map[string]string)
	for key, vals := range resp.Header {
		if strings.HasPrefix(key, "X-Starai-") || strings.HasPrefix(key, "X-StarAI-") {
			if len(vals) > 0 {
				meta[key] = vals[0]
			}
		}
	}
	if len(meta) == 0 {
		return nil
	}
	return meta
}

func (p *OpenAIProvider) doRequest(ctx context.Context, req *ChatRequest) (*http.Response, error) {
	payload := openAIChatRequest{
		Model:       req.Model,
		Messages:    toOpenAIMessages(req.Messages),
		Stream:      req.Stream,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
	}

	if len(req.Tools) > 0 {
		payload.Tools = req.Tools
	}

	if req.Stream {
		payload.StreamOptions = &streamOptions{IncludeUsage: true}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", p.authPrefix+" "+p.apiKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(errBody))
	}

	return resp, nil
}

func toOpenAIMessages(msgs []ChatMessage) []openAIMessage {
	out := make([]openAIMessage, len(msgs))
	for i, m := range msgs {
		msg := openAIMessage{
			Role:       m.Role,
			ToolCalls:  m.ToolCalls,
			ToolCallID: m.ToolCallID,
		}
		if len(m.MultiContent) > 0 {
			msg.Content = m.MultiContent
		} else {
			msg.Content = m.Content
		}
		out[i] = msg
	}
	return out
}

func contentToString(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

// --- OpenAI API types ---

type openAIChatRequest struct {
	Model         string           `json:"model"`
	Messages      []openAIMessage  `json:"messages"`
	Tools         []ToolDefinition `json:"tools,omitempty"`
	Stream        bool             `json:"stream"`
	StreamOptions *streamOptions   `json:"stream_options,omitempty"`
	Temperature   float64          `json:"temperature,omitempty"`
	MaxTokens     int              `json:"max_tokens,omitempty"`
}

type streamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type openAIMessage struct {
	Role             string      `json:"role"`
	Content          interface{} `json:"content"`
	ReasoningContent string      `json:"reasoning_content,omitempty"` // DeepSeek-R1, QwQ, o-series thinking output
	ToolCalls        []ToolCall  `json:"tool_calls,omitempty"`
	ToolCallID       string      `json:"tool_call_id,omitempty"`
}

type openAIChatResponse struct {
	ID      string `json:"id"`
	Choices []struct {
		Message openAIMessage `json:"message"`
	} `json:"choices"`
	Usage *TokenUsage `json:"usage"`
}

type openAIStreamChunk struct {
	ID      string `json:"id"`
	Choices []struct {
		Delta openAIMessage `json:"delta"`
	} `json:"choices"`
	Usage *TokenUsage `json:"usage"`
}
