package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/yinhe/starclaw/internal/provider"
	"github.com/yinhe/starclaw/internal/tool"
)

const maxReActSteps = 25

// ReActRuntime implements the ReAct (Reasoning + Acting) pattern
// The agent explicitly reasons about each step before taking action
type ReActRuntime struct {
	modelProvider provider.ModelProvider
	toolRegistry  *tool.Registry
}

func NewReActRuntime(p provider.ModelProvider, tr *tool.Registry) *ReActRuntime {
	return &ReActRuntime{
		modelProvider: p,
		toolRegistry:  tr,
	}
}

// ReActStep represents one step in the ReAct loop
type ReActStep struct {
	Step        int    `json:"step"`
	Thought     string `json:"thought"`
	Action      string `json:"action,omitempty"`
	ActionInput string `json:"action_input,omitempty"`
	Observation string `json:"observation,omitempty"`
}

// ReActResult contains the full result of a ReAct execution
type ReActResult struct {
	Steps    []ReActStep `json:"steps"`
	Answer   string      `json:"answer"`
	Messages []provider.ChatMessage
	Usage    *provider.TokenUsage
}

// Run executes the ReAct loop: Think ↀAct ↀObserve ↀThink ↀ... ↀAnswer
func (r *ReActRuntime) Run(ctx context.Context, req *RunRequest) (*ReActResult, error) {
	// Build the ReAct system prompt
	toolNames := r.getToolNames(req.Tools)
	systemPrompt := buildReActSystemPrompt(toolNames)

	messages := make([]provider.ChatMessage, 0, len(req.Messages)+10)

	// Prepend ReAct system message
	messages = append(messages, provider.ChatMessage{
		Role:    "system",
		Content: systemPrompt,
	})

	// Add user messages (skip existing system messages)
	for _, m := range req.Messages {
		if m.Role != "system" {
			messages = append(messages, m)
		} else {
			// Merge existing system prompt as context
			messages[0].Content += "\n\nAdditional context:\n" + m.Content
		}
	}

	var toolDefs []provider.ToolDefinition
	if r.toolRegistry != nil {
		toolDefs = r.toolRegistry.GetDefinitions(req.Tools)
	}

	var steps []ReActStep
	var totalUsage provider.TokenUsage

	for step := 0; step < maxReActSteps; step++ {
		log.Printf("[ReAct] Step %d - requesting LLM", step+1)

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
			return nil, fmt.Errorf("ReAct step %d failed: %w", step+1, err)
		}

		if result.Usage != nil {
			totalUsage.PromptTokens += result.Usage.PromptTokens
			totalUsage.CompletionTokens += result.Usage.CompletionTokens
			totalUsage.TotalTokens += result.Usage.TotalTokens
		}

		// Check if LLM made a tool call (Action)
		if result.Tool != nil {
			messages = append(messages, provider.ChatMessage{
				Role:    "assistant",
				Content: result.Content,
				ToolCalls: []provider.ToolCall{
					{
						ID:   result.Tool.ID,
						Type: result.Tool.Type,
						Function: provider.FunctionCall{
							Name:      result.Tool.Function.Name,
							Arguments: result.Tool.Function.Arguments,
						},
					},
				},
			})

			log.Printf("[ReAct] Action: %s(%s)", result.Tool.Function.Name, result.Tool.Function.Arguments)

			sanitizedArgs := sanitizeJSON(result.Tool.Function.Arguments)
			toolResult, execErr := r.toolRegistry.Execute(ctx, result.Tool.Function.Name, sanitizedArgs)
			if execErr != nil {
				toolResult = fmt.Sprintf("Error: %v", execErr)
			}

			// Truncate long results
			if len(toolResult) > 2000 {
				toolResult = toolResult[:2000] + "...(truncated)"
			}

			steps = append(steps, ReActStep{
				Step:        step + 1,
				Thought:     result.Content,
				Action:      result.Tool.Function.Name,
				ActionInput: result.Tool.Function.Arguments,
				Observation: toolResult,
			})

			messages = append(messages, provider.ChatMessage{
				Role:       "tool",
				Content:    toolResult,
				ToolCallID: result.Tool.ID,
			})
			continue
		}

		// No tool call = final answer
		steps = append(steps, ReActStep{
			Step:    step + 1,
			Thought: result.Content,
		})

		// Check if response contains FINAL ANSWER marker
		answer := result.Content
		if idx := strings.Index(strings.ToUpper(answer), "FINAL ANSWER:"); idx >= 0 {
			answer = strings.TrimSpace(answer[idx+13:])
		}

		return &ReActResult{
			Steps:    steps,
			Answer:   answer,
			Messages: messages,
			Usage:    &totalUsage,
		}, nil
	}

	// Max steps reached
	return &ReActResult{
		Steps:    steps,
		Answer:   "I reached the maximum number of reasoning steps. Here's what I found so far based on my analysis.",
		Messages: messages,
		Usage:    &totalUsage,
	}, nil
}

func (r *ReActRuntime) getToolNames(requested []string) []string {
	if requested != nil {
		return requested
	}
	return r.toolRegistry.List()
}

func buildReActSystemPrompt(toolNames []string) string {
	tools := strings.Join(toolNames, ", ")
	return fmt.Sprintf(`You are a reasoning agent that follows the ReAct (Reasoning + Acting) framework.

For each user request, you should:
1. **Think**: Analyze the problem and decide what to do next
2. **Act**: Use a tool if needed to gather information
3. **Observe**: Review the tool result
4. **Repeat** steps 1-3 as needed
5. **Answer**: When you have enough information, provide the final answer

Available tools: %s

Guidelines:
- Always reason step by step before acting
- Use tools when you need external information
- After each observation, reflect on whether you have enough info to answer
- When ready, provide your final answer clearly
- If a tool fails, try an alternative approach
- Be concise but thorough in your reasoning`, tools)
}

// ToJSON serializes ReAct steps to a readable JSON string
func (r *ReActResult) ToJSON() string {
	data, _ := json.MarshalIndent(r.Steps, "", "  ")
	return string(data)
}
