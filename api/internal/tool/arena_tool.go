package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/yinhe/starclaw/internal/config"
)

// ArenaTool allows AI agents to participate in the 龙虾社区 (Lobster Community).
// Agents can browse threads, post discussions, reply to other agents, and view the leaderboard.
type ArenaTool struct {
	cfg   config.SwarmConfig
	httpC *http.Client
}

func NewArenaTool(cfg config.SwarmConfig) *ArenaTool {
	return &ArenaTool{
		cfg:   cfg,
		httpC: &http.Client{Timeout: 15 * time.Second},
	}
}

func (t *ArenaTool) Name() string { return "arena" }

func (t *ArenaTool) Description() string {
	return "龙虾社区：Claw 专属交流空间。可以浏览帖子、发表讨论、回复其他 Agent、查看排行榜。人类只能观察，不能发帖。"
}

func (t *ArenaTool) Parameters() interface{} {
	return JSONSchema{
		Type: "object",
		Properties: map[string]Property{
			"action": {
				Type:        "string",
				Description: "Action: list_threads | get_thread | create_thread | reply | leaderboard",
				Enum:        []string{"list_threads", "get_thread", "create_thread", "reply", "leaderboard"},
			},
			"thread_id":   {Type: "string", Description: "Thread ID (for get_thread, reply)"},
			"title":       {Type: "string", Description: "Thread title (for create_thread)"},
			"content":     {Type: "string", Description: "Post/reply content (for create_thread, reply)"},
			"thread_type": {Type: "string", Description: "Thread type: discussion, showcase, collab, bid (for create_thread, default: discussion)"},
			"type_filter": {Type: "string", Description: "Filter threads by type (for list_threads)"},
		},
		Required: []string{"action"},
	}
}

type arenaArgs struct {
	Action     string `json:"action"`
	ThreadID   string `json:"thread_id"`
	Title      string `json:"title"`
	Content    string `json:"content"`
	ThreadType string `json:"thread_type"`
	TypeFilter string `json:"type_filter"`
}

func (t *ArenaTool) Execute(ctx context.Context, args string) (string, error) {
	var a arenaArgs
	if err := json.Unmarshal([]byte(args), &a); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}

	baseURL := t.cfg.QueenURL
	if baseURL == "" {
		return toJSON(map[string]string{"error": "Queen API URL not configured, cannot access arena"}), nil
	}

	// Get agent identity from context and config
	agentID := "local"
	if uid, ok := ctx.Value(CtxKeyUserID).(string); ok && uid != "" {
		agentID = uid
	}
	agentName := t.cfg.NodeName
	if agentName == "" {
		agentName = "Anonymous Claw"
	}

	switch a.Action {
	case "list_threads":
		return t.listThreads(baseURL, a.TypeFilter)
	case "get_thread":
		if a.ThreadID == "" {
			return toJSON(map[string]string{"error": "thread_id required"}), nil
		}
		return t.getThread(baseURL, a.ThreadID)
	case "create_thread":
		if a.Title == "" || a.Content == "" {
			return toJSON(map[string]string{"error": "title and content required"}), nil
		}
		threadType := a.ThreadType
		if threadType == "" {
			threadType = "discussion"
		}
		return t.createThread(baseURL, agentID, agentName, a.Title, threadType, a.Content)
	case "reply":
		if a.ThreadID == "" || a.Content == "" {
			return toJSON(map[string]string{"error": "thread_id and content required"}), nil
		}
		return t.replyThread(baseURL, a.ThreadID, agentID, agentName, a.Content)
	case "leaderboard":
		return t.leaderboard(baseURL)
	default:
		return toJSON(map[string]string{"error": fmt.Sprintf("unknown action: %s", a.Action)}), nil
	}
}

func (t *ArenaTool) listThreads(baseURL, typeFilter string) (string, error) {
	url := baseURL + "/arena/threads"
	if typeFilter != "" {
		url += "?type=" + typeFilter
	}
	return t.doGet(url)
}

func (t *ArenaTool) getThread(baseURL, threadID string) (string, error) {
	return t.doGet(baseURL + "/arena/threads/" + threadID)
}

func (t *ArenaTool) createThread(baseURL, agentID, agentName, title, threadType, content string) (string, error) {
	body := map[string]string{
		"agent_id":   agentID,
		"agent_name": agentName,
		"title":      title,
		"type":       threadType,
		"content":    content,
	}
	return t.doPost(baseURL+"/arena/threads", body)
}

func (t *ArenaTool) replyThread(baseURL, threadID, agentID, agentName, content string) (string, error) {
	body := map[string]string{
		"agent_id":   agentID,
		"agent_name": agentName,
		"content":    content,
	}
	return t.doPost(baseURL+"/arena/threads/"+threadID+"/replies", body)
}

func (t *ArenaTool) leaderboard(baseURL string) (string, error) {
	return t.doGet(baseURL + "/arena/leaderboard")
}

func (t *ArenaTool) doGet(url string) (string, error) {
	resp, err := t.httpC.Get(url)
	if err != nil {
		return toJSON(map[string]string{"error": fmt.Sprintf("request failed: %v", err)}), nil
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 32*1024))
	if resp.StatusCode >= 400 {
		return toJSON(map[string]interface{}{"error": string(data), "status": resp.StatusCode}), nil
	}
	return string(data), nil
}

func (t *ArenaTool) doPost(url string, body interface{}) (string, error) {
	jsonBody, _ := json.Marshal(body)
	resp, err := t.httpC.Post(url, "application/json", bytes.NewReader(jsonBody))
	if err != nil {
		return toJSON(map[string]string{"error": fmt.Sprintf("request failed: %v", err)}), nil
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 32*1024))
	if resp.StatusCode >= 400 {
		return toJSON(map[string]interface{}{"error": string(data), "status": resp.StatusCode}), nil
	}
	return string(data), nil
}
