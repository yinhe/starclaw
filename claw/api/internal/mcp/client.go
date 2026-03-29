package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/yinhe/starclaw/internal/tool"
)

// Client connects to an MCP-compatible tool server
type Client struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

// ServerConfig holds MCP server connection info
type ServerConfig struct {
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key"`
	Name    string `json:"name"`
}

// NewClient creates a new MCP client
func NewClient(cfg ServerConfig) *Client {
	return &Client{
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
		apiKey:  cfg.APIKey,
		client:  &http.Client{Timeout: 120 * time.Second},
	}
}

// NewClientWithTimeout creates a new MCP client with a custom timeout (for long-running ops like docker build)
func NewClientWithTimeout(cfg ServerConfig, timeout time.Duration) *Client {
	return &Client{
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
		apiKey:  cfg.APIKey,
		client:  &http.Client{Timeout: timeout},
	}
}

// ToolInfo represents a tool exposed by an MCP server
type ToolInfo struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"inputSchema"`
}

// ListTools fetches available tools from the MCP server
func (c *Client) ListTools(ctx context.Context) ([]ToolInfo, error) {
	resp, err := c.jsonRPC(ctx, "tools/list", nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Tools []ToolInfo `json:"tools"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse tools/list response: %w", err)
	}
	return result.Tools, nil
}

// CallTool invokes a tool on the MCP server
func (c *Client) CallTool(ctx context.Context, name string, args string) (string, error) {
	var arguments interface{}
	if args != "" {
		json.Unmarshal([]byte(args), &arguments)
	}

	params := map[string]interface{}{
		"name":      name,
		"arguments": arguments,
	}

	resp, err := c.jsonRPC(ctx, "tools/call", params)
	if err != nil {
		return "", err
	}

	var result struct {
		Content []struct {
			Type     string `json:"type"`
			Text     string `json:"text,omitempty"`
			Data     string `json:"data,omitempty"`
			MimeType string `json:"mimeType,omitempty"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return string(resp), nil
	}

	if result.IsError {
		for _, c := range result.Content {
			if c.Type == "text" {
				return "", fmt.Errorf("MCP tool error: %s", c.Text)
			}
		}
		return "", fmt.Errorf("MCP tool returned error")
	}

	var texts []string
	for _, c := range result.Content {
		switch c.Type {
		case "text":
			texts = append(texts, c.Text)
		case "image":
			// Return base64 image data as a JSON object so the agent runtime can handle it
			// (e.g. pass to vision model or save to file)
			imgJSON := fmt.Sprintf(`{"screenshot":"data:%s;base64,%s"}`, c.MimeType, c.Data)
			texts = append(texts, imgJSON)
		}
	}
	return strings.Join(texts, "\n"), nil
}

// jsonRPC sends a JSON-RPC 2.0 request to the MCP server
func (c *Client) jsonRPC(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
	}
	if params != nil {
		reqBody["params"] = params
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("MCP request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MCP server error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var rpcResp struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return nil, fmt.Errorf("failed to parse JSON-RPC response: %w", err)
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("MCP RPC error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	return rpcResp.Result, nil
}

// MCPTool wraps a remote MCP tool as a local tool.Tool
type MCPTool struct {
	client     *Client
	info       ToolInfo
	serverName string
}

// NewMCPTool creates a tool wrapper for a remote MCP tool
func NewMCPTool(client *Client, info ToolInfo, serverName string) *MCPTool {
	return &MCPTool{client: client, info: info, serverName: serverName}
}

func (t *MCPTool) Name() string {
	return fmt.Sprintf("mcp_%s_%s", t.serverName, t.info.Name)
}

func (t *MCPTool) Description() string {
	return fmt.Sprintf("[MCP:%s] %s", t.serverName, t.info.Description)
}

func (t *MCPTool) Parameters() interface{} {
	return t.info.InputSchema
}

func (t *MCPTool) Execute(ctx context.Context, args string) (string, error) {
	return t.client.CallTool(ctx, t.info.Name, args)
}

// RegisterMCPTools discovers tools from an MCP server and registers them into the tool registry
func RegisterMCPTools(ctx context.Context, registry *tool.Registry, cfg ServerConfig) error {
	client := NewClient(cfg)
	tools, err := client.ListTools(ctx)
	if err != nil {
		return fmt.Errorf("failed to list MCP tools from %s: %w", cfg.Name, err)
	}

	for _, info := range tools {
		mcpTool := NewMCPTool(client, info, cfg.Name)
		registry.Register(mcpTool)
	}

	return nil
}
