package agent

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/yinhe/starclaw/internal/provider"
	"github.com/yinhe/starclaw/internal/tool"
)

// AgentNode represents a single agent in a multi-agent collaboration
type AgentNode struct {
	ID           string
	Name         string
	SystemPrompt string
	Model        string
	Provider     provider.ModelProvider
	Tools        []string
	MaxTokens    int
	Temperature  float64
}

// MultiAgentConfig defines the collaboration setup
type MultiAgentConfig struct {
	Agents       []AgentNode
	Orchestrator *AgentNode // optional: a manager agent that delegates
	Mode         string     // "sequential", "parallel", "orchestrated"
	MaxRounds    int
}

// MultiAgentResult holds the final output
type MultiAgentResult struct {
	Output       string
	AgentOutputs map[string]string // agentID -> output
	TotalUsage   *provider.TokenUsage
}

// RunMultiAgent executes a multi-agent collaboration
func RunMultiAgent(ctx context.Context, cfg *MultiAgentConfig, toolRegistry *tool.Registry, input string) (*MultiAgentResult, error) {
	if cfg.MaxRounds <= 0 {
		cfg.MaxRounds = 5
	}

	switch cfg.Mode {
	case "sequential":
		return runSequential(ctx, cfg, toolRegistry, input)
	case "parallel":
		return runParallel(ctx, cfg, toolRegistry, input)
	case "orchestrated":
		return runOrchestrated(ctx, cfg, toolRegistry, input)
	default:
		return runSequential(ctx, cfg, toolRegistry, input)
	}
}

// runSequential passes output of each agent as input to the next
func runSequential(ctx context.Context, cfg *MultiAgentConfig, toolRegistry *tool.Registry, input string) (*MultiAgentResult, error) {
	result := &MultiAgentResult{
		AgentOutputs: make(map[string]string),
		TotalUsage:   &provider.TokenUsage{},
	}

	currentInput := input

	for _, ag := range cfg.Agents {
		log.Printf("[MultiAgent] Sequential: running agent %s (%s)", ag.Name, ag.ID)

		rt := NewRuntime(ag.Provider, toolRegistry)
		messages := []provider.ChatMessage{
			{Role: "system", Content: ag.SystemPrompt},
			{Role: "user", Content: currentInput},
		}

		runResult, err := rt.Run(ctx, &RunRequest{
			Model:       ag.Model,
			Messages:    messages,
			Tools:       ag.Tools,
			Temperature: ag.Temperature,
			MaxTokens:   ag.MaxTokens,
		})
		if err != nil {
			return nil, fmt.Errorf("agent %s failed: %w", ag.Name, err)
		}

		result.AgentOutputs[ag.ID] = runResult.Content
		currentInput = runResult.Content

		if runResult.Usage != nil {
			result.TotalUsage.PromptTokens += runResult.Usage.PromptTokens
			result.TotalUsage.CompletionTokens += runResult.Usage.CompletionTokens
			result.TotalUsage.TotalTokens += runResult.Usage.TotalTokens
		}
	}

	result.Output = currentInput
	return result, nil
}

// runParallel runs all agents in parallel and combines outputs
func runParallel(ctx context.Context, cfg *MultiAgentConfig, toolRegistry *tool.Registry, input string) (*MultiAgentResult, error) {
	result := &MultiAgentResult{
		AgentOutputs: make(map[string]string),
		TotalUsage:   &provider.TokenUsage{},
	}

	type agentResult struct {
		id      string
		name    string
		content string
		usage   *provider.TokenUsage
		err     error
	}

	ch := make(chan agentResult, len(cfg.Agents))

	for _, ag := range cfg.Agents {
		go func(a AgentNode) {
			log.Printf("[MultiAgent] Parallel: running agent %s (%s)", a.Name, a.ID)

			rt := NewRuntime(a.Provider, toolRegistry)
			messages := []provider.ChatMessage{
				{Role: "system", Content: a.SystemPrompt},
				{Role: "user", Content: input},
			}

			runResult, err := rt.Run(ctx, &RunRequest{
				Model:       a.Model,
				Messages:    messages,
				Tools:       a.Tools,
				Temperature: a.Temperature,
				MaxTokens:   a.MaxTokens,
			})

			if err != nil {
				ch <- agentResult{id: a.ID, name: a.Name, err: err}
				return
			}
			ch <- agentResult{id: a.ID, name: a.Name, content: runResult.Content, usage: runResult.Usage}
		}(ag)
	}

	var outputs []string
	for range cfg.Agents {
		ar := <-ch
		if ar.err != nil {
			result.AgentOutputs[ar.id] = fmt.Sprintf("Error: %v", ar.err)
		} else {
			result.AgentOutputs[ar.id] = ar.content
			outputs = append(outputs, fmt.Sprintf("## %s\n%s", ar.name, ar.content))
		}
		if ar.usage != nil {
			result.TotalUsage.PromptTokens += ar.usage.PromptTokens
			result.TotalUsage.CompletionTokens += ar.usage.CompletionTokens
			result.TotalUsage.TotalTokens += ar.usage.TotalTokens
		}
	}

	result.Output = strings.Join(outputs, "\n\n---\n\n")
	return result, nil
}

// runOrchestrated uses a manager agent to delegate tasks to sub-agents
func runOrchestrated(ctx context.Context, cfg *MultiAgentConfig, toolRegistry *tool.Registry, input string) (*MultiAgentResult, error) {
	if cfg.Orchestrator == nil {
		return nil, fmt.Errorf("orchestrated mode requires an orchestrator agent")
	}

	result := &MultiAgentResult{
		AgentOutputs: make(map[string]string),
		TotalUsage:   &provider.TokenUsage{},
	}

	// Build agent descriptions for the orchestrator
	var agentDescs []string
	agentMap := make(map[string]AgentNode)
	for _, ag := range cfg.Agents {
		agentDescs = append(agentDescs, fmt.Sprintf("- %s (ID: %s): %s", ag.Name, ag.ID, ag.SystemPrompt))
		agentMap[ag.ID] = ag
	}

	orchestratorPrompt := cfg.Orchestrator.SystemPrompt + "\n\n你可以委托以一Agent 来完成子任务：\n" +
		strings.Join(agentDescs, "\n") +
		"\n\n请分析用户的需求，决定委托哪些 Agent 处理，并综合它们的结果给出最终回答。" +
		"\n当你需要委托时，使用格式：[DELEGATE:agent_id]任务描述[/DELEGATE]" +
		"\n如果不需要委托，直接回答即可。"

	// Run orchestrator
	ort := NewRuntime(cfg.Orchestrator.Provider, toolRegistry)
	messages := []provider.ChatMessage{
		{Role: "system", Content: orchestratorPrompt},
		{Role: "user", Content: input},
	}

	for round := 0; round < cfg.MaxRounds; round++ {
		log.Printf("[MultiAgent] Orchestrated round %d", round+1)

		ortResult, err := ort.Run(ctx, &RunRequest{
			Model:       cfg.Orchestrator.Model,
			Messages:    messages,
			Temperature: cfg.Orchestrator.Temperature,
			MaxTokens:   cfg.Orchestrator.MaxTokens,
		})
		if err != nil {
			return nil, fmt.Errorf("orchestrator failed: %w", err)
		}

		if ortResult.Usage != nil {
			result.TotalUsage.PromptTokens += ortResult.Usage.PromptTokens
			result.TotalUsage.CompletionTokens += ortResult.Usage.CompletionTokens
			result.TotalUsage.TotalTokens += ortResult.Usage.TotalTokens
		}

		content := ortResult.Content

		// Check for delegation markers
		if !strings.Contains(content, "[DELEGATE:") {
			// No delegation, final answer
			result.Output = content
			return result, nil
		}

		// Parse and execute delegations
		delegationResults := parseDelegations(ctx, content, agentMap, toolRegistry, result)

		// Feed results back to orchestrator
		messages = append(messages,
			provider.ChatMessage{Role: "assistant", Content: content},
			provider.ChatMessage{Role: "user", Content: "各Agent 的执行结果：\n\n" + delegationResults + "\n\n请综合以上结果，给出最终回答。"},
		)
	}

	result.Output = "编排超过最大轮数限制"
	return result, nil
}

func parseDelegations(ctx context.Context, content string, agentMap map[string]AgentNode, toolRegistry *tool.Registry, result *MultiAgentResult) string {
	var outputs []string

	// Simple parser for [DELEGATE:agent_id]task[/DELEGATE]
	remaining := content
	for {
		start := strings.Index(remaining, "[DELEGATE:")
		if start == -1 {
			break
		}
		endTag := strings.Index(remaining[start:], "]")
		if endTag == -1 {
			break
		}
		agentID := remaining[start+10 : start+endTag]

		closeTag := strings.Index(remaining[start:], "[/DELEGATE]")
		if closeTag == -1 {
			break
		}

		task := remaining[start+endTag+1 : start+closeTag]
		remaining = remaining[start+closeTag+11:]

		ag, ok := agentMap[agentID]
		if !ok {
			outputs = append(outputs, fmt.Sprintf("### %s\nAgent not found", agentID))
			continue
		}

		log.Printf("[MultiAgent] Delegating to %s: %.100s", ag.Name, task)

		rt := NewRuntime(ag.Provider, toolRegistry)
		runResult, err := rt.Run(ctx, &RunRequest{
			Model:       ag.Model,
			Messages:    []provider.ChatMessage{{Role: "system", Content: ag.SystemPrompt}, {Role: "user", Content: task}},
			Tools:       ag.Tools,
			Temperature: ag.Temperature,
			MaxTokens:   ag.MaxTokens,
		})

		if err != nil {
			outputs = append(outputs, fmt.Sprintf("### %s\n执行失败: %v", ag.Name, err))
			result.AgentOutputs[agentID] = fmt.Sprintf("Error: %v", err)
		} else {
			outputs = append(outputs, fmt.Sprintf("### %s\n%s", ag.Name, runResult.Content))
			result.AgentOutputs[agentID] = runResult.Content
			if runResult.Usage != nil {
				result.TotalUsage.PromptTokens += runResult.Usage.PromptTokens
				result.TotalUsage.CompletionTokens += runResult.Usage.CompletionTokens
				result.TotalUsage.TotalTokens += runResult.Usage.TotalTokens
			}
		}
	}

	return strings.Join(outputs, "\n\n")
}
