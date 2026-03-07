package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/yinhe/starclaw/internal/provider"
	"github.com/yinhe/starclaw/internal/tool"
)

// ProviderResolver resolves a model name to a ModelProvider
type ProviderResolver func(modelName string) provider.ModelProvider

// Engine executes a workflow definition
type Engine struct {
	resolveProvider ProviderResolver
	toolRegistry    *tool.Registry
}

// NewEngine creates a new workflow execution engine
func NewEngine(resolver ProviderResolver, tr *tool.Registry) *Engine {
	return &Engine{
		resolveProvider: resolver,
		toolRegistry:    tr,
	}
}

// Definition represents the JSON workflow structure from the frontend
type Definition struct {
	Nodes []NodeDef `json:"nodes"`
	Edges []EdgeDef `json:"edges"`
}

// NodeDef represents a node in the workflow graph
type NodeDef struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Position map[string]float64     `json:"position"`
	Data     map[string]interface{} `json:"data"`
}

// EdgeDef represents an edge connecting two nodes
type EdgeDef struct {
	ID           string `json:"id"`
	Source       string `json:"source"`
	Target       string `json:"target"`
	SourceHandle string `json:"sourceHandle,omitempty"`
	TargetHandle string `json:"targetHandle,omitempty"`
}

// Execute runs the workflow and returns the final output
func (e *Engine) Execute(ctx context.Context, definitionJSON string, input string) (string, error) {
	var def Definition
	if err := json.Unmarshal([]byte(definitionJSON), &def); err != nil {
		return "", fmt.Errorf("invalid workflow definition: %w", err)
	}

	// Build adjacency map: nodeID -> outgoing edges
	outEdges := map[string][]EdgeDef{}
	for _, edge := range def.Edges {
		outEdges[edge.Source] = append(outEdges[edge.Source], edge)
	}

	// Build node lookup
	nodeMap := map[string]NodeDef{}
	for _, n := range def.Nodes {
		nodeMap[n.ID] = n
	}

	// Find the start node
	var startID string
	for _, n := range def.Nodes {
		if n.Type == "start" {
			startID = n.ID
			break
		}
	}
	if startID == "" {
		return "", fmt.Errorf("workflow has no start node")
	}

	// Execute nodes in topological order following edges
	currentValue := input
	currentNodeID := startID
	visited := map[string]bool{}
	maxSteps := 50

	for step := 0; step < maxSteps; step++ {
		if visited[currentNodeID] {
			return "", fmt.Errorf("cycle detected at node %s", currentNodeID)
		}
		visited[currentNodeID] = true

		node, ok := nodeMap[currentNodeID]
		if !ok {
			return "", fmt.Errorf("node not found: %s", currentNodeID)
		}

		log.Printf("[Workflow] Step %d: executing node %s (type=%s)", step+1, node.ID, node.Type)

		var err error
		var nextHandle string

		switch node.Type {
		case "start":
			// Pass through, input is already set

		case "end":
			// Terminal node  apply output mapping if present
			if mapping, ok := node.Data["outputMapping"].(string); ok && mapping != "" {
				currentValue = templateReplace(mapping, currentValue)
			}
			log.Printf("[Workflow] Completed. Output: %.200s", currentValue)
			return currentValue, nil

		case "llm":
			currentValue, err = e.executeLLM(ctx, node, currentValue)
			if err != nil {
				return "", fmt.Errorf("LLM node %s failed: %w", node.ID, err)
			}

		case "tool":
			currentValue, err = e.executeTool(ctx, node, currentValue)
			if err != nil {
				return "", fmt.Errorf("tool node %s failed: %w", node.ID, err)
			}

		case "condition":
			var match bool
			match, err = e.evaluateCondition(node, currentValue)
			if err != nil {
				return "", fmt.Errorf("condition node %s failed: %w", node.ID, err)
			}
			if match {
				nextHandle = "true"
			} else {
				nextHandle = "false"
			}

		default:
			return "", fmt.Errorf("unknown node type: %s", node.Type)
		}

		// Find next node via edges
		edges := outEdges[currentNodeID]
		if len(edges) == 0 {
			// No outgoing edges  this is effectively the end
			return currentValue, nil
		}

		var nextEdge *EdgeDef
		if nextHandle != "" {
			// Condition node: pick edge by handle
			for i := range edges {
				if edges[i].SourceHandle == nextHandle {
					nextEdge = &edges[i]
					break
				}
			}
		} else {
			nextEdge = &edges[0]
		}

		if nextEdge == nil {
			return currentValue, nil
		}

		currentNodeID = nextEdge.Target
	}

	return "", fmt.Errorf("workflow exceeded maximum steps (%d)", maxSteps)
}

func (e *Engine) executeLLM(ctx context.Context, node NodeDef, input string) (string, error) {
	modelName, _ := node.Data["model"].(string)
	prompt, _ := node.Data["prompt"].(string)
	temperature, _ := node.Data["temperature"].(float64)
	maxTokensFloat, _ := node.Data["maxTokens"].(float64)
	maxTokens := int(maxTokensFloat)

	if modelName == "" {
		return "", fmt.Errorf("LLM node has no model configured")
	}

	p := e.resolveProvider(modelName)
	if p == nil {
		return "", fmt.Errorf("no provider found for model: %s", modelName)
	}

	// Build prompt with template substitution
	systemPrompt := templateReplace(prompt, input)

	messages := []provider.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: input},
	}

	if maxTokens == 0 {
		maxTokens = 4096
	}

	req := &provider.ChatRequest{
		Model:       modelName,
		Messages:    messages,
		Temperature: temperature,
		MaxTokens:   maxTokens,
		Stream:      false,
	}

	result, err := p.ChatSync(ctx, req)
	if err != nil {
		return "", err
	}

	return result.Content, nil
}

func (e *Engine) executeTool(ctx context.Context, node NodeDef, input string) (string, error) {
	toolName, _ := node.Data["toolName"].(string)
	argsTemplate, _ := node.Data["argsTemplate"].(string)

	if toolName == "" {
		return "", fmt.Errorf("tool node has no tool configured")
	}

	// Build args from template
	args := argsTemplate
	if args == "" {
		// Default: pass input as query
		args = fmt.Sprintf(`{"query": %q}`, input)
	} else {
		args = templateReplace(args, input)
	}

	result, err := e.toolRegistry.Execute(ctx, toolName, args)
	if err != nil {
		return "", err
	}

	return result, nil
}

func (e *Engine) evaluateCondition(node NodeDef, input string) (bool, error) {
	expression, _ := node.Data["expression"].(string)
	if expression == "" {
		return true, nil
	}

	// Simple expression evaluation:
	// - "input.contains(X)" ↀstrings.Contains
	// - "input.length > N" ↀlen check
	// - "input == X" ↀequality
	// For MVP, support basic contains check
	expr := strings.TrimSpace(expression)

	if strings.HasPrefix(expr, "input.contains(") {
		inner := strings.TrimPrefix(expr, "input.contains(")
		inner = strings.TrimSuffix(inner, ")")
		inner = strings.Trim(inner, `"'`)
		return strings.Contains(input, inner), nil
	}

	if strings.HasPrefix(expr, "input.length >") {
		var n int
		fmt.Sscanf(expr, "input.length > %d", &n)
		return len(input) > n, nil
	}

	if strings.Contains(expr, "==") {
		parts := strings.SplitN(expr, "==", 2)
		if len(parts) == 2 {
			right := strings.TrimSpace(parts[1])
			right = strings.Trim(right, `"'`)
			return strings.TrimSpace(input) == right, nil
		}
	}

	// Default: non-empty input ↀtrue
	return input != "", nil
}

// templateReplace replaces {{input}} with the actual input value
func templateReplace(template string, input string) string {
	result := strings.ReplaceAll(template, "{{input}}", input)
	result = strings.ReplaceAll(result, "{{last_node_output}}", input)
	return result
}
