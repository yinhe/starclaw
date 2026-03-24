package aggregator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DevBridgeClient communicates with the Dev Bridge MCP server for task sync.
type DevBridgeClient struct {
	BaseURL string
	client  *http.Client
}

func NewDevBridgeClient(baseURL string) *DevBridgeClient {
	return &DevBridgeClient{
		BaseURL: baseURL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

// DevBridgeTask represents a task in the Dev Bridge system.
type DevBridgeTask struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Assignee    string `json:"assignee"`
	Branch      string `json:"branch"`
	Service     string `json:"service"`
	CreatedAt   string `json:"created_at"`
}

// callMCP sends a JSON-RPC 2.0 call to Dev Bridge and returns the result.
func (c *DevBridgeClient) callMCP(tool string, args map[string]interface{}) (json.RawMessage, error) {
	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      tool,
			"arguments": args,
		},
	}
	bodyJSON, _ := json.Marshal(reqBody)

	resp, err := c.client.Post(c.BaseURL, "application/json", bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("devbridge %s: %w", tool, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return body, nil // return raw if not parseable
	}
	if result.Error != nil {
		return nil, fmt.Errorf("devbridge %s: %s", tool, result.Error.Message)
	}
	return result.Result, nil
}

// CreateTask creates a task in Dev Bridge from a Forge Issue.
func (c *DevBridgeClient) CreateTask(issueKey, title, description, assignee, service string) error {
	_, err := c.callMCP("task_create", map[string]interface{}{
		"title":       fmt.Sprintf("[%s] %s", issueKey, title),
		"description": description,
		"assignee":    assignee,
		"service":     service,
	})
	return err
}

// CreateBranch creates a feature branch via Dev Bridge.
func (c *DevBridgeClient) CreateBranch(branchName, baseBranch string) error {
	_, err := c.callMCP("git_create_branch", map[string]interface{}{
		"branch": branchName,
		"base":   baseBranch,
	})
	return err
}

// ListBranches returns active branches from Dev Bridge.
func (c *DevBridgeClient) ListBranches() (json.RawMessage, error) {
	return c.callMCP("git_branches", map[string]interface{}{})
}

// UpdateTaskStatus updates a task's status in Dev Bridge.
func (c *DevBridgeClient) UpdateTaskStatus(taskID, status string) error {
	_, err := c.callMCP("task_update", map[string]interface{}{
		"id":     taskID,
		"status": status,
	})
	return err
}
