package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yinhe/starclaw/internal/provider"
)

// Context key for injecting user identity into tool execution
type contextKey string

const CtxKeyUserID contextKey = "tool_user_id"
const CtxKeyConversationID contextKey = "tool_conversation_id"
const CtxKeyProvider contextKey = "tool_provider" // e.g. "star-ai", "qwen", "fal"

// Tool is the interface that all tools must implement
type Tool interface {
	// Name returns the unique tool name
	Name() string

	// Description returns a human-readable description
	Description() string

	// Parameters returns the JSON Schema for the tool's input
	Parameters() interface{}

	// Execute runs the tool with the given arguments and returns the result
	Execute(ctx context.Context, args string) (string, error)
}

// ToProviderDefinition converts a Tool to a provider.ToolDefinition for LLM calling
func ToProviderDefinition(t Tool) provider.ToolDefinition {
	return provider.ToolDefinition{
		Type: "function",
		Function: provider.FunctionSchema{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  t.Parameters(),
		},
	}
}

// Registry manages all available tools
type Registry struct {
	tools map[string]Tool
}

// NewRegistry creates a new tool registry
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

// Register adds a tool to the registry
func (r *Registry) Register(t Tool) {
	r.tools[t.Name()] = t
}

// Get returns a tool by name
func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// GetDefinitions returns provider.ToolDefinition for the given tool names
// If names is nil, returns all tools
func (r *Registry) GetDefinitions(names []string) []provider.ToolDefinition {
	var defs []provider.ToolDefinition

	if names == nil {
		for _, t := range r.tools {
			defs = append(defs, ToProviderDefinition(t))
		}
		return defs
	}

	matched := make(map[string]bool)
	for _, name := range names {
		if t, ok := r.tools[name]; ok {
			defs = append(defs, ToProviderDefinition(t))
			matched[name] = true
		}
	}
	// Auto-include all MCP tools (mcp_*) so agents can use MCP Bridge
	for name, t := range r.tools {
		if strings.HasPrefix(name, "mcp_") && !matched[name] {
			defs = append(defs, ToProviderDefinition(t))
		}
	}
	return defs
}

// Execute runs a tool by name with the given JSON arguments
func (r *Registry) Execute(ctx context.Context, name string, args string) (string, error) {
	t, ok := r.tools[name]
	if !ok {
		return "", fmt.Errorf("tool not found: %s", name)
	}
	return t.Execute(ctx, args)
}

// List returns all registered tool names
func (r *Registry) List() []string {
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

// JSONSchema is a helper to define tool parameter schemas
type JSONSchema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties"`
	Required   []string            `json:"required,omitempty"`
}

type Property struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Enum        []string `json:"enum,omitempty"`
}

// ParseArgs is a helper to unmarshal JSON arguments into a struct
func ParseArgs[T any](args string) (T, error) {
	var result T
	if err := json.Unmarshal([]byte(args), &result); err != nil {
		return result, fmt.Errorf("failed to parse tool arguments: %w", err)
	}
	return result, nil
}
