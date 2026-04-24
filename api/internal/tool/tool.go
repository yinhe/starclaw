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
const CtxKeyTaskExecution contextKey = "tool_task_execution"

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

// ExecuteHook is a function that wraps tool execution (e.g. billing gateway).
// It receives the tool, name, and args, and is responsible for calling t.Execute().
type ExecuteHook func(ctx context.Context, t Tool, name, args string) (string, error)

// Registry manages all available tools
type Registry struct {
	tools map[string]Tool
	hook  ExecuteHook
}

// NewRegistry creates a new tool registry
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

// SetExecuteHook sets a hook that wraps every tool execution (e.g. billing gateway).
func (r *Registry) SetExecuteHook(hook ExecuteHook) {
	r.hook = hook
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

// Execute runs a tool by name with the given JSON arguments.
// If an ExecuteHook is set (e.g. billing gateway), the hook wraps the execution.
// Supports dotted names like "video_generation.list_videos" — splits into tool
// "video_generation" and injects action "list_videos" into args.
func (r *Registry) Execute(ctx context.Context, name string, args string) (string, error) {
	t, ok := r.tools[name]
	if !ok {
		// Handle dotted tool names: "tool_name.action_name" → tool "tool_name" + action "action_name"
		if dot := strings.IndexByte(name, '.'); dot > 0 {
			baseName := name[:dot]
			actionName := name[dot+1:]
			if bt, bok := r.tools[baseName]; bok {
				// Inject the action into args JSON
				var argsMap map[string]interface{}
				if err := json.Unmarshal([]byte(args), &argsMap); err != nil || argsMap == nil {
					argsMap = make(map[string]interface{})
				}
				if _, hasAction := argsMap["action"]; !hasAction {
					argsMap["action"] = actionName
				}
				merged, _ := json.Marshal(argsMap)
				args = string(merged)
				t = bt
				name = baseName
				ok = true
			}
		}
		if !ok {
			return "", fmt.Errorf("tool not found: %s", name)
		}
	}
	if r.hook != nil {
		return r.hook(ctx, t, name, args)
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

// ScopeFor returns a scoped registry that only exposes the given tool names.
// The scoped registry shares the same underlying tools and hook.
func (r *Registry) ScopeFor(allowedTools []string) *Registry {
	scoped := &Registry{
		tools: make(map[string]Tool, len(allowedTools)),
		hook:  r.hook,
	}
	for _, name := range allowedTools {
		if t, ok := r.tools[name]; ok {
			scoped.tools[name] = t
		}
	}
	// Always include MCP tools in scope
	for name, t := range r.tools {
		if strings.HasPrefix(name, "mcp_") {
			scoped.tools[name] = t
		}
	}
	return scoped
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
