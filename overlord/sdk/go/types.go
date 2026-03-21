package starclaw

// ClientConfig holds configuration for the StarClaw client.
type ClientConfig struct {
	Endpoint string // API endpoint, e.g. "https://overlord.company.com"
	APIKey   string // API key (sk-xxx)
}

// Option configures the client.
type Option func(*Client)

// WithEndpoint sets a custom API endpoint.
func WithEndpoint(endpoint string) Option {
	return func(c *Client) { c.endpoint = endpoint }
}

// WithTimeout sets the HTTP timeout in seconds.
func WithTimeout(seconds int) Option {
	return func(c *Client) { c.timeoutSec = seconds }
}

// ChatMessage represents a single message in a conversation.
type ChatMessage struct {
	Role    string `json:"role"` // system, user, assistant
	Content string `json:"content"`
}

// ChatCompletionRequest is the request body for /v1/chat/completions.
type ChatCompletionRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Stream      bool          `json:"stream,omitempty"`
	Temperature *float64      `json:"temperature,omitempty"`
	MaxTokens   *int          `json:"max_tokens,omitempty"`
	TopP        *float64      `json:"top_p,omitempty"`
	Stop        []string      `json:"stop,omitempty"`
}

// ChatChoice is a single completion choice.
type ChatChoice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason *string     `json:"finish_reason"`
}

// ChatUsage reports token usage.
type ChatUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ChatCompletionResponse is the response from /v1/chat/completions (non-streaming).
type ChatCompletionResponse struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Created int64        `json:"created"`
	Model   string       `json:"model"`
	Choices []ChatChoice `json:"choices"`
	Usage   ChatUsage    `json:"usage"`
}

// ChatCompletionChunkDelta holds partial content in streaming.
type ChatCompletionChunkDelta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

// ChatCompletionChunkChoice is a streaming choice.
type ChatCompletionChunkChoice struct {
	Index        int                      `json:"index"`
	Delta        ChatCompletionChunkDelta `json:"delta"`
	FinishReason *string                  `json:"finish_reason"`
}

// ChatCompletionChunk is a single SSE chunk from streaming completions.
type ChatCompletionChunk struct {
	ID      string                      `json:"id"`
	Object  string                      `json:"object"`
	Created int64                       `json:"created"`
	Model   string                      `json:"model"`
	Choices []ChatCompletionChunkChoice `json:"choices"`
}

// Model represents an available model.
type Model struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	OwnedBy string `json:"owned_by"`
}

// ModelsResponse is the response from /v1/models.
type ModelsResponse struct {
	Data []Model `json:"data"`
}

// ── Team Agent ──

// TeamAgentTemplate is a reusable team blueprint.
type TeamAgentTemplate struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Roles       string `json:"roles"`
	IsOfficial  bool   `json:"is_official"`
	Version     string `json:"version"`
}

// TeamInstance is a running team agent.
type TeamInstance struct {
	ID           string  `json:"id"`
	TemplateID   string  `json:"template_id"`
	TemplateName string  `json:"template_name"`
	Name         string  `json:"name"`
	Goal         string  `json:"goal"`
	Status       string  `json:"status"`
	EnergyBudget int     `json:"energy_budget"`
	EnergyUsed   int     `json:"energy_used"`
	MissionCount int     `json:"mission_count"`
	AvgScore     float64 `json:"avg_score"`
	CreatedAt    string  `json:"created_at"`
}

// TeamMission is a task assigned to a team.
type TeamMission struct {
	ID          string  `json:"id"`
	InstanceID  string  `json:"instance_id"`
	Title       string  `json:"title"`
	Goal        string  `json:"goal"`
	Status      string  `json:"status"`
	TotalSteps  int     `json:"total_steps"`
	DoneSteps   int     `json:"done_steps"`
	ReviewScore float64 `json:"review_score"`
	EnergyUsed  int     `json:"energy_used"`
	PreviewURL  string  `json:"preview_url"`
	CreatedAt   string  `json:"created_at"`
	CompletedAt *string `json:"completed_at"`
}

// CreateTeamInstanceRequest creates a new team instance.
type CreateTeamInstanceRequest struct {
	TemplateID   string `json:"template_id"`
	ClawNodeID   string `json:"claw_node_id"`
	Name         string `json:"name"`
	Goal         string `json:"goal,omitempty"`
	EnergyBudget int    `json:"energy_budget,omitempty"`
}

// CreateTeamMissionRequest creates a new mission for a team.
type CreateTeamMissionRequest struct {
	Goal        string `json:"goal"`
	AutoConfirm bool   `json:"auto_confirm,omitempty"`
}
