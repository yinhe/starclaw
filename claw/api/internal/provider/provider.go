package provider

import "context"

// ContentPart represents a part of multimodal content
type ContentPart struct {
	Type     string    `json:"type"` // "text" or "image_url"
	Text     string    `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
}

// ImageURL represents an image URL in multimodal content
type ImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"` // "auto", "low", "high"
}

// ChatMessage represents a message in a conversation
type ChatMessage struct {
	Role         string        `json:"role"`
	Content      string        `json:"content"`
	MultiContent []ContentPart `json:"-"` // multimodal content parts (images + text)
	ToolCalls    []ToolCall    `json:"tool_calls,omitempty"`
	ToolCallID   string        `json:"tool_call_id,omitempty"`
}

// ToolCall represents a function call made by the model
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

// FunctionCall represents the function name and arguments
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ToolDefinition defines a tool that can be used by the model
type ToolDefinition struct {
	Type     string         `json:"type"` // "function"
	Function FunctionSchema `json:"function"`
}

// FunctionSchema defines the schema for a function tool
type FunctionSchema struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"`
}

// ChatRequest represents a chat completion request
type ChatRequest struct {
	Model       string           `json:"model"`
	Messages    []ChatMessage    `json:"messages"`
	Tools       []ToolDefinition `json:"tools,omitempty"`
	Temperature float64          `json:"temperature,omitempty"`
	MaxTokens   int              `json:"max_tokens,omitempty"`
	Stream      bool             `json:"stream"`
}

// ChatChunk represents a streamed response chunk
type ChatChunk struct {
	ID      string      `json:"id"`
	Content string      `json:"content"`
	Role    string      `json:"role,omitempty"`
	Tool    *ToolCall   `json:"tool_call,omitempty"`
	Done    bool        `json:"done"`
	Usage   *TokenUsage `json:"usage,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// TokenUsage represents token usage information
type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ModelProvider is the unified interface for all LLM providers
type ModelProvider interface {
	// Chat sends a chat completion request and returns a channel of streamed chunks
	Chat(ctx context.Context, req *ChatRequest) (<-chan *ChatChunk, error)

	// ChatSync sends a non-streaming chat completion request
	ChatSync(ctx context.Context, req *ChatRequest) (*ChatChunk, error)

	// Name returns the provider name
	Name() string

	// Models returns available model names for this provider
	Models() []string
}

// Registry manages all registered model providers
type Registry struct {
	providers map[string]ModelProvider
}

// NewRegistry creates a new provider registry
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]ModelProvider),
	}
}

// Register adds a provider to the registry
func (r *Registry) Register(name string, provider ModelProvider) {
	r.providers[name] = provider
}

// Get returns a provider by name
func (r *Registry) Get(name string) (ModelProvider, bool) {
	p, ok := r.providers[name]
	return p, ok
}

// List returns all registered provider names
func (r *Registry) List() []string {
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}
