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

// OllamaProvider connects to a local Ollama instance
type OllamaProvider struct {
	baseURL string
	models  []string
	client  *http.Client
}

type OllamaConfig struct {
	BaseURL string
}

func NewOllamaProvider(cfg OllamaConfig) *OllamaProvider {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	return &OllamaProvider{
		baseURL: strings.TrimRight(baseURL, "/"),
		models:  []string{},
		client:  &http.Client{},
	}
}

func (p *OllamaProvider) Name() string {
	return "ollama"
}

func (p *OllamaProvider) Models() []string {
	// Dynamically fetch available models from Ollama
	models, err := p.fetchModels()
	if err != nil {
		return p.models
	}
	return models
}

func (p *OllamaProvider) fetchModels() ([]string, error) {
	resp, err := p.client.Get(p.baseURL + "/api/tags")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	names := make([]string, len(result.Models))
	for i, m := range result.Models {
		names[i] = m.Name
	}
	return names, nil
}

func (p *OllamaProvider) ChatSync(ctx context.Context, req *ChatRequest) (*ChatChunk, error) {
	noThink := false
	payload := ollamaChatRequest{
		Model:    req.Model,
		Messages: toOllamaMessages(req.Messages),
		Stream:   false,
		Think:    &noThink,
		Options: ollamaOptions{
			Temperature: req.Temperature,
			NumPredict:  req.MaxTokens,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ollama request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama error (status %d): %s", resp.StatusCode, string(errBody))
	}

	var ollamaResp ollamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, err
	}

	content := ollamaResp.Message.Content
	if content == "" && ollamaResp.Message.Thinking != "" {
		content = ollamaResp.Message.Thinking
	}
	chunk := &ChatChunk{
		Content: content,
		Role:    ollamaResp.Message.Role,
		Done:    true,
	}

	if ollamaResp.PromptEvalCount > 0 || ollamaResp.EvalCount > 0 {
		chunk.Usage = &TokenUsage{
			PromptTokens:     ollamaResp.PromptEvalCount,
			CompletionTokens: ollamaResp.EvalCount,
			TotalTokens:      ollamaResp.PromptEvalCount + ollamaResp.EvalCount,
		}
	}

	return chunk, nil
}

func (p *OllamaProvider) Chat(ctx context.Context, req *ChatRequest) (<-chan *ChatChunk, error) {
	payload := ollamaChatRequest{
		Model:    req.Model,
		Messages: toOllamaMessages(req.Messages),
		Stream:   true,
		Options: ollamaOptions{
			Temperature: req.Temperature,
			NumPredict:  req.MaxTokens,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ollama request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama error (status %d): %s", resp.StatusCode, string(errBody))
	}

	ch := make(chan *ChatChunk, 32)

	go func() {
		defer close(ch)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}

			var streamResp ollamaChatResponse
			if err := json.Unmarshal([]byte(line), &streamResp); err != nil {
				continue
			}

			chunk := &ChatChunk{
				Content: streamResp.Message.Content,
			}

			if streamResp.Done {
				chunk.Done = true
				if streamResp.PromptEvalCount > 0 || streamResp.EvalCount > 0 {
					chunk.Usage = &TokenUsage{
						PromptTokens:     streamResp.PromptEvalCount,
						CompletionTokens: streamResp.EvalCount,
						TotalTokens:      streamResp.PromptEvalCount + streamResp.EvalCount,
					}
				}
			}

			select {
			case ch <- chunk:
			case <-ctx.Done():
				return
			}

			if streamResp.Done {
				return
			}
		}
	}()

	return ch, nil
}

func toOllamaMessages(msgs []ChatMessage) []ollamaMessage {
	out := make([]ollamaMessage, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, ollamaMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}
	return out
}

// --- Ollama API types ---

type ollamaChatRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Think    *bool           `json:"think,omitempty"`
	Options  ollamaOptions   `json:"options,omitempty"`
}

type ollamaOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
	NumPredict  int     `json:"num_predict,omitempty"`
}

type ollamaMessage struct {
	Role     string `json:"role"`
	Content  string `json:"content"`
	Thinking string `json:"thinking,omitempty"`
}

type ollamaChatResponse struct {
	Message         ollamaMessage `json:"message"`
	Done            bool          `json:"done"`
	PromptEvalCount int           `json:"prompt_eval_count"`
	EvalCount       int           `json:"eval_count"`
}
