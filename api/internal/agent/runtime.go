package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/yinhe/starclaw/internal/provider"
	"github.com/yinhe/starclaw/internal/tool"
)

const maxToolIterations = 50

// Runtime orchestrates the Agent execution loop with Tool Calling
type Runtime struct {
	modelProvider provider.ModelProvider
	toolRegistry  *tool.Registry
}

// NewRuntime creates a new Agent Runtime
func NewRuntime(p provider.ModelProvider, tr *tool.Registry) *Runtime {
	return &Runtime{
		modelProvider: p,
		toolRegistry:  tr,
	}
}

// RunRequest contains all parameters for an agent run
type RunRequest struct {
	Model       string
	Messages    []provider.ChatMessage
	Tools       []string // tool names to enable, nil = all
	Temperature float64
	MaxTokens   int
}

// RunResult contains the final result of an agent run
type RunResult struct {
	Content  string
	Messages []provider.ChatMessage // full message history including tool calls
	Usage    *provider.TokenUsage
}

// Run executes the agent loop: LLM call -> tool call -> LLM call -> ... until done
func (r *Runtime) Run(ctx context.Context, req *RunRequest) (*RunResult, error) {
	messages := make([]provider.ChatMessage, len(req.Messages))
	copy(messages, req.Messages)

	// Get tool definitions
	var toolDefs []provider.ToolDefinition
	if r.toolRegistry != nil {
		toolDefs = r.toolRegistry.GetDefinitions(req.Tools)
	}

	var totalUsage provider.TokenUsage

	for i := 0; i < maxToolIterations; i++ {
		chatReq := &provider.ChatRequest{
			Model:       req.Model,
			Messages:    messages,
			Tools:       toolDefs,
			Temperature: req.Temperature,
			MaxTokens:   req.MaxTokens,
			Stream:      false,
		}

		result, err := r.modelProvider.ChatSync(ctx, chatReq)
		if err != nil {
			return nil, fmt.Errorf("LLM call failed (iteration %d): %w", i+1, err)
		}

		if result.Usage != nil {
			totalUsage.PromptTokens += result.Usage.PromptTokens
			totalUsage.CompletionTokens += result.Usage.CompletionTokens
			totalUsage.TotalTokens += result.Usage.TotalTokens
		}

		// No tool call  we're done
		if result.Tool == nil {
			messages = append(messages, provider.ChatMessage{
				Role:    "assistant",
				Content: result.Content,
			})

			return &RunResult{
				Content:  result.Content,
				Messages: messages,
				Usage:    &totalUsage,
			}, nil
		}

		// Has tool call  execute it
		log.Printf("[Agent] Tool call: %s(%s)", result.Tool.Function.Name, result.Tool.Function.Arguments)

		// Sanitize arguments to ensure valid JSON (some models return malformed args)
		sanitizedArgs := sanitizeJSON(result.Tool.Function.Arguments)
		ensureToolCall(result.Tool)

		// Add assistant message with tool call (use sanitized args so subsequent API calls don't fail)
		messages = append(messages, provider.ChatMessage{
			Role:    "assistant",
			Content: result.Content,
			ToolCalls: []provider.ToolCall{
				{
					ID:   result.Tool.ID,
					Type: result.Tool.Type,
					Function: provider.FunctionCall{
						Name:      result.Tool.Function.Name,
						Arguments: sanitizedArgs,
					},
				},
			},
		})

		// Execute the tool
		toolResult, err := r.toolRegistry.Execute(ctx, result.Tool.Function.Name, sanitizedArgs)
		if err != nil {
			toolResult = fmt.Sprintf("Tool execution error: %v", err)
		}

		log.Printf("[Agent] Tool result: %s (%.100s...)", result.Tool.Function.Name, toolResult)

		// Add tool result message
		messages = append(messages, provider.ChatMessage{
			Role:       "tool",
			Content:    toolResult,
			ToolCallID: result.Tool.ID,
		})
	}

	return nil, fmt.Errorf("agent exceeded maximum tool iterations (%d)", maxToolIterations)
}

// StreamRun executes the agent loop with streaming for the final response
// Tool calls are executed synchronously, only the final LLM response is streamed
func (r *Runtime) StreamRun(ctx context.Context, req *RunRequest) (<-chan *StreamChunk, error) {
	ch := make(chan *StreamChunk, 32)

	go func() {
		defer close(ch)

		messages := make([]provider.ChatMessage, len(req.Messages))
		copy(messages, req.Messages)

		var toolDefs []provider.ToolDefinition
		if r.toolRegistry != nil {
			toolDefs = r.toolRegistry.GetDefinitions(req.Tools)
		}

		var totalUsage provider.TokenUsage
		stepIndex := 0

		for i := 0; i < maxToolIterations; i++ {
			chatReq := &provider.ChatRequest{
				Model:       req.Model,
				Messages:    messages,
				Tools:       toolDefs,
				Temperature: req.Temperature,
				MaxTokens:   req.MaxTokens,
				Stream:      false, // Use sync for tool-calling rounds
			}

			if i == 0 {
				toolNames := make([]string, len(toolDefs))
				for j, td := range toolDefs {
					toolNames[j] = td.Function.Name
				}
				log.Printf("[Agent/Stream] Model=%s, Tools=%v, Messages=%d", req.Model, toolNames, len(messages))
			}

			// Emit step event: thinking
			stepIndex++
			if i == 0 {
				ch <- &StreamChunk{AgentStep: "thinking", AgentStepDetail: "正在分析任务...", AgentStepIndex: stepIndex}
			} else {
				ch <- &StreamChunk{AgentStep: "thinking", AgentStepDetail: "正在整合结果，规划下一步...", AgentStepIndex: stepIndex}
			}

			// First, try a sync call to check for tool calls
			result, err := r.modelProvider.ChatSync(ctx, chatReq)
			if err != nil {
				ch <- &StreamChunk{Error: fmt.Sprintf("LLM call failed: %v", err)}
				return
			}
			log.Printf("[Agent/Stream] Iteration %d: Tool=%v, ContentLen=%d", i+1, result.Tool != nil, len(result.Content))

			if result.Usage != nil {
				totalUsage.PromptTokens += result.Usage.PromptTokens
				totalUsage.CompletionTokens += result.Usage.CompletionTokens
				totalUsage.TotalTokens += result.Usage.TotalTokens
			}

			// No tool call → stream the final response
			if result.Tool == nil {
				stepIndex++
				if i > 0 {
					ch <- &StreamChunk{AgentStep: "summarizing", AgentStepDetail: "正在生成最终回复...", AgentStepIndex: stepIndex}
				}
				// Re-do with streaming for the final response
				chatReq.Stream = true
				streamCh, err := r.modelProvider.Chat(ctx, chatReq)
				if err != nil {
					// Fallback to sync result
					ch <- &StreamChunk{Content: result.Content}
					ch <- &StreamChunk{Done: true, Usage: &totalUsage}
					return
				}

				for chunk := range streamCh {
					if chunk.Done {
						ch <- &StreamChunk{Done: true, Usage: &totalUsage}
						return
					}
					if chunk.Content != "" {
						ch <- &StreamChunk{Content: chunk.Content}
					}
				}
				// Stream closed without explicit Done — ensure Done is sent
				// (required for cerebrate memory extraction in chat handler)
				ch <- &StreamChunk{Done: true, Usage: &totalUsage}
				return
			}

			// Has tool call → notify client and execute
			log.Printf("[Agent/Stream] Tool call: %s(%s)", result.Tool.Function.Name, result.Tool.Function.Arguments)

			// Sanitize arguments and normalize tool call fields
			sanitizedArgs := sanitizeJSON(result.Tool.Function.Arguments)
			ensureToolCall(result.Tool)

			// Send sanitized tool call to client
			sanitizedTool := *result.Tool
			sanitizedTool.Function.Arguments = sanitizedArgs
			toolCallJSON, _ := json.Marshal(&sanitizedTool)
			ch <- &StreamChunk{
				ToolCall:  string(toolCallJSON),
				Reasoning: result.Content,
			}

			messages = append(messages, provider.ChatMessage{
				Role:    "assistant",
				Content: result.Content,
				ToolCalls: []provider.ToolCall{
					{
						ID:   result.Tool.ID,
						Type: result.Tool.Type,
						Function: provider.FunctionCall{
							Name:      result.Tool.Function.Name,
							Arguments: sanitizedArgs,
						},
					},
				},
			})

			toolResult, err := r.toolRegistry.Execute(ctx, result.Tool.Function.Name, sanitizedArgs)
			if err != nil {
				toolResult = fmt.Sprintf("Tool execution error: %v", err)
			}
			log.Printf("[Agent/Stream] Tool result from %s: %d bytes", result.Tool.Function.Name, len(toolResult))

			ch <- &StreamChunk{
				ToolResult: toolResult,
				ToolName:   result.Tool.Function.Name,
			}

			// Strip large data (e.g. browser screenshots) before feeding back to LLM
			llmResult := stripLargeData(toolResult)
			messages = append(messages, provider.ChatMessage{
				Role:       "tool",
				Content:    llmResult,
				ToolCallID: result.Tool.ID,
			})
		}

		ch <- &StreamChunk{Error: "Agent exceeded maximum tool iterations"}
	}()

	return ch, nil
}

// sanitizeJSON extracts the first valid JSON object from potentially malformed LLM output.
// Some LLMs generate concatenated JSON like `{...}{...}` which causes parse errors.
func sanitizeJSON(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "{}"
	}

	// Try parsing as-is first
	if json.Valid([]byte(s)) {
		return s
	}

	// If doesn't start with '{', it's not JSON at all  wrap it
	if s[0] != '{' {
		log.Printf("[Agent] Non-JSON args detected, wrapping: %.100s", s)
		wrapped, _ := json.Marshal(map[string]string{"raw_input": s})
		return string(wrapped)
	}

	// Find the end of the first complete JSON object by tracking brace depth
	depth := 0
	inString := false
	escaped := false
	for i, c := range s {
		if escaped {
			escaped = false
			continue
		}
		if c == '\\' && inString {
			escaped = true
			continue
		}
		if c == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		if c == '{' {
			depth++
		} else if c == '}' {
			depth--
			if depth == 0 {
				candidate := s[:i+1]
				if json.Valid([]byte(candidate)) {
					log.Printf("[Agent] Sanitized malformed JSON args: trimmed %d extra bytes", len(s)-len(candidate))
					return candidate
				}
			}
		}
	}

	// Last resort: couldn't extract valid JSON, wrap as raw
	log.Printf("[Agent] Could not extract valid JSON, wrapping: %.100s", s)
	wrapped, _ := json.Marshal(map[string]string{"raw_input": s})
	return string(wrapped)
}

// ensureToolCall normalizes a ToolCall: sets type to "function" if empty, generates ID if missing
func ensureToolCall(tc *provider.ToolCall) {
	if tc.Type == "" {
		tc.Type = "function"
	}
	if tc.ID == "" {
		tc.ID = fmt.Sprintf("call_%d", time.Now().UnixNano())
	}
}

// stripLargeData removes base64 image data from tool results to save LLM tokens
func stripLargeData(result string) string {
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		return result
	}
	if _, ok := parsed["screenshot"]; ok {
		parsed["screenshot"] = "[screenshot captured]"
	}
	out, _ := json.Marshal(parsed)
	return string(out)
}

// StreamChunk represents a chunk of the streaming agent response
type StreamChunk struct {
	Content         string               `json:"content,omitempty"`
	ToolCall        string               `json:"tool_call,omitempty"`
	ToolResult      string               `json:"tool_result,omitempty"`
	ToolName        string               `json:"tool_name,omitempty"`
	Reasoning       string               `json:"reasoning,omitempty"`
	Done            bool                 `json:"done,omitempty"`
	Usage           *provider.TokenUsage `json:"usage,omitempty"`
	Error           string               `json:"error,omitempty"`
	AgentStep       string               `json:"agent_step,omitempty"`        // e.g. "thinking", "tool_calling", "summarizing"
	AgentStepDetail string               `json:"agent_step_detail,omitempty"` // human-readable description
	AgentStepIndex  int                  `json:"agent_step_index,omitempty"`  // 1-based step counter
}
