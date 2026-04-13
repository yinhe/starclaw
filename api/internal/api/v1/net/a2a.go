package network

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/yinhe/starclaw/internal/model"
	"github.com/yinhe/starclaw/internal/molt"
	"github.com/yinhe/starclaw/internal/provider"
	"github.com/yinhe/starclaw/internal/tool"
	"gorm.io/gorm"
)

// A2AHandler implements the Google A2A (Agent-to-Agent) protocol
// Spec: https://google.github.io/A2A/
type A2AHandler struct {
	db               *gorm.DB
	providerRegistry *provider.Registry
	toolRegistry     *tool.Registry
}

func NewA2AHandler(db *gorm.DB, pr *provider.Registry, tr *tool.Registry) *A2AHandler {
	return &A2AHandler{db: db, providerRegistry: pr, toolRegistry: tr}
}

// ---- Agent Card (/.well-known/agent.json) ----

type AgentCard struct {
	Name               string          `json:"name"`
	Description        string          `json:"description"`
	URL                string          `json:"url"`
	Version            string          `json:"version"`
	Capabilities       A2ACapabilities `json:"capabilities"`
	Skills             []A2ASkill      `json:"skills"`
	DefaultInputModes  []string        `json:"defaultInputModes"`
	DefaultOutputModes []string        `json:"defaultOutputModes"`
}

type A2ACapabilities struct {
	Streaming         bool `json:"streaming"`
	PushNotifications bool `json:"pushNotifications"`
}

type A2ASkill struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tags        []string `json:"tags,omitempty"`
}

func (h *A2AHandler) AgentCardHandler(c *gin.Context) {
	baseURL := fmt.Sprintf("%s://%s", scheme(c), c.Request.Host)

	// Build skills from available agents
	skills := []A2ASkill{
		{ID: "chat", Name: "General Chat", Description: "Multi-model AI conversation with tool use", Tags: []string{"chat", "reasoning"}},
		{ID: "coding", Name: "Coding Agent", Description: "Autonomous coding in 14 languages with sandbox execution", Tags: []string{"code", "programming"}},
		{ID: "video", Name: "Video Generation", Description: "AI video generation, dubbing, MV production", Tags: []string{"video", "multimedia"}},
		{ID: "research", Name: "Research & Analysis", Description: "Web search, browsing, data collection and analysis", Tags: []string{"research", "search"}},
		{ID: "workflow", Name: "Visual Workflow", Description: "Visual workflow automation with conditional logic", Tags: []string{"workflow", "automation"}},
		{ID: "rag", Name: "Knowledge Base", Description: "RAG-powered document Q&A", Tags: []string{"rag", "knowledge"}},
	}

	card := AgentCard{
		Name:        "StarClaw",
		Description: "Open-Source AI Agent Orchestration Platform — Multi-Model, Visual Workflow, RAG, MCP Compatible",
		URL:         baseURL + "/a2a",
		Version:     molt.Version,
		Capabilities: A2ACapabilities{
			Streaming:         true,
			PushNotifications: false,
		},
		Skills:             skills,
		DefaultInputModes:  []string{"text"},
		DefaultOutputModes: []string{"text"},
	}

	c.JSON(http.StatusOK, card)
}

// ---- A2A JSON-RPC Endpoint ----

type A2ARequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type A2AResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *A2AError   `json:"error,omitempty"`
}

type A2AError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type A2AMessage struct {
	Role  string           `json:"role"`
	Parts []A2AMessagePart `json:"parts"`
}

type A2AMessagePart struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type A2ATask struct {
	ID        string        `json:"id"`
	Status    A2ATaskStatus `json:"status"`
	History   []A2AMessage  `json:"history,omitempty"`
	Artifacts []A2AArtifact `json:"artifacts,omitempty"`
}

type A2ATaskStatus struct {
	State   string      `json:"state"` // submitted, working, completed, failed, input-required
	Message *A2AMessage `json:"message,omitempty"`
}

type A2AArtifact struct {
	Name  string           `json:"name,omitempty"`
	Parts []A2AMessagePart `json:"parts"`
}

type TaskSendParams struct {
	ID      string     `json:"id"`
	Message A2AMessage `json:"message"`
}

type TaskGetParams struct {
	ID string `json:"id"`
}

// HandleRPC is the main JSON-RPC endpoint for A2A protocol
func (h *A2AHandler) HandleRPC(c *gin.Context) {
	var req A2ARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, A2AResponse{
			JSONRPC: "2.0",
			Error:   &A2AError{Code: -32700, Message: "Parse error: " + err.Error()},
		})
		return
	}

	if req.JSONRPC != "2.0" {
		req.JSONRPC = "2.0"
	}

	switch req.Method {
	case "tasks/send":
		h.handleTaskSend(c, req)
	case "tasks/get":
		h.handleTaskGet(c, req)
	case "tasks/cancel":
		h.handleTaskCancel(c, req)
	case "tasks/sendSubscribe":
		h.handleTaskSendSubscribe(c, req)
	default:
		c.JSON(http.StatusOK, A2AResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &A2AError{Code: -32601, Message: "Method not found: " + req.Method},
		})
	}
}

func (h *A2AHandler) handleTaskSend(c *gin.Context, req A2ARequest) {
	var params TaskSendParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		c.JSON(http.StatusOK, A2AResponse{
			JSONRPC: "2.0", ID: req.ID,
			Error: &A2AError{Code: -32602, Message: "Invalid params"},
		})
		return
	}

	taskID := params.ID
	if taskID == "" {
		taskID = uuid.New().String()
	}

	// Extract text from message parts
	userText := extractText(params.Message)
	if userText == "" {
		c.JSON(http.StatusOK, A2AResponse{
			JSONRPC: "2.0", ID: req.ID,
			Error: &A2AError{Code: -32602, Message: "No text content in message"},
		})
		return
	}

	// Run agent synchronously
	result, err := h.runAgent(c, userText)
	if err != nil {
		c.JSON(http.StatusOK, A2AResponse{
			JSONRPC: "2.0", ID: req.ID,
			Result: A2ATask{
				ID: taskID,
				Status: A2ATaskStatus{
					State:   "failed",
					Message: &A2AMessage{Role: "agent", Parts: []A2AMessagePart{{Type: "text", Text: err.Error()}}},
				},
				History: []A2AMessage{params.Message},
			},
		})
		return
	}

	agentMsg := A2AMessage{
		Role:  "agent",
		Parts: []A2AMessagePart{{Type: "text", Text: result}},
	}

	c.JSON(http.StatusOK, A2AResponse{
		JSONRPC: "2.0", ID: req.ID,
		Result: A2ATask{
			ID: taskID,
			Status: A2ATaskStatus{
				State:   "completed",
				Message: &agentMsg,
			},
			History: []A2AMessage{params.Message, agentMsg},
			Artifacts: []A2AArtifact{
				{Name: "response", Parts: agentMsg.Parts},
			},
		},
	})
}

func (h *A2AHandler) handleTaskGet(c *gin.Context, req A2ARequest) {
	// For stateless mode, we don't persist A2A tasks
	c.JSON(http.StatusOK, A2AResponse{
		JSONRPC: "2.0", ID: req.ID,
		Error: &A2AError{Code: -32001, Message: "Task not found (stateless mode)"},
	})
}

func (h *A2AHandler) handleTaskCancel(c *gin.Context, req A2ARequest) {
	c.JSON(http.StatusOK, A2AResponse{
		JSONRPC: "2.0", ID: req.ID,
		Error: &A2AError{Code: -32001, Message: "Task not found (stateless mode)"},
	})
}

func (h *A2AHandler) handleTaskSendSubscribe(c *gin.Context, req A2ARequest) {
	var params TaskSendParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		c.JSON(http.StatusOK, A2AResponse{
			JSONRPC: "2.0", ID: req.ID,
			Error: &A2AError{Code: -32602, Message: "Invalid params"},
		})
		return
	}

	taskID := params.ID
	if taskID == "" {
		taskID = uuid.New().String()
	}

	userText := extractText(params.Message)
	if userText == "" {
		c.JSON(http.StatusOK, A2AResponse{
			JSONRPC: "2.0", ID: req.ID,
			Error: &A2AError{Code: -32602, Message: "No text content in message"},
		})
		return
	}

	// SSE streaming response
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming not supported"})
		return
	}

	// Send "working" status
	sendSSE(c.Writer, flusher, req.ID, A2ATask{
		ID:     taskID,
		Status: A2ATaskStatus{State: "working"},
	})

	// Stream the agent response
	result, err := h.runAgentStreaming(c, userText, func(chunk string) {
		sendSSE(c.Writer, flusher, req.ID, A2ATask{
			ID: taskID,
			Status: A2ATaskStatus{
				State:   "working",
				Message: &A2AMessage{Role: "agent", Parts: []A2AMessagePart{{Type: "text", Text: chunk}}},
			},
		})
	})

	if err != nil {
		sendSSE(c.Writer, flusher, req.ID, A2ATask{
			ID: taskID,
			Status: A2ATaskStatus{
				State:   "failed",
				Message: &A2AMessage{Role: "agent", Parts: []A2AMessagePart{{Type: "text", Text: err.Error()}}},
			},
		})
		return
	}

	// Send final completed status
	agentMsg := A2AMessage{Role: "agent", Parts: []A2AMessagePart{{Type: "text", Text: result}}}
	sendSSE(c.Writer, flusher, req.ID, A2ATask{
		ID: taskID,
		Status: A2ATaskStatus{
			State:   "completed",
			Message: &agentMsg,
		},
		Artifacts: []A2AArtifact{
			{Name: "response", Parts: agentMsg.Parts},
		},
	})
}

// ---- Internal helpers ----

func (h *A2AHandler) runAgent(c *gin.Context, userText string) (string, error) {
	// Find the first available model config
	var modelCfg model.ModelConfig
	if err := h.db.Where("is_enabled = ?", true).First(&modelCfg).Error; err != nil {
		return "", fmt.Errorf("no enabled model configured")
	}

	p := provider.CreateFromConfig(h.providerRegistry, modelCfg)

	messages := []provider.ChatMessage{
		{Role: "system", Content: "You are StarClaw, a helpful AI assistant. Answer concisely and accurately."},
		{Role: "user", Content: userText},
	}

	result, err := p.ChatSync(c.Request.Context(), &provider.ChatRequest{
		Model:    modelCfg.ModelName,
		Messages: messages,
	})
	if err != nil {
		return "", err
	}

	return result.Content, nil
}

func (h *A2AHandler) runAgentStreaming(c *gin.Context, userText string, onChunk func(string)) (string, error) {
	var modelCfg model.ModelConfig
	if err := h.db.Where("is_enabled = ?", true).First(&modelCfg).Error; err != nil {
		return "", fmt.Errorf("no enabled model configured")
	}

	p := provider.CreateFromConfig(h.providerRegistry, modelCfg)

	messages := []provider.ChatMessage{
		{Role: "system", Content: "You are StarClaw, a helpful AI assistant. Answer concisely and accurately."},
		{Role: "user", Content: userText},
	}

	ch, err := p.Chat(c.Request.Context(), &provider.ChatRequest{
		Model:    modelCfg.ModelName,
		Messages: messages,
		Stream:   true,
	})
	if err != nil {
		return "", err
	}

	var full strings.Builder
	for chunk := range ch {
		if chunk.Error != "" {
			return full.String(), fmt.Errorf("%s", chunk.Error)
		}
		if chunk.Content != "" {
			full.WriteString(chunk.Content)
			onChunk(chunk.Content)
		}
		if chunk.Done {
			break
		}
	}

	return full.String(), nil
}

func extractText(msg A2AMessage) string {
	var parts []string
	for _, p := range msg.Parts {
		if p.Type == "text" && p.Text != "" {
			parts = append(parts, p.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func sendSSE(w http.ResponseWriter, flusher http.Flusher, reqID interface{}, task A2ATask) {
	resp := A2AResponse{
		JSONRPC: "2.0",
		ID:      reqID,
		Result:  task,
	}
	data, _ := json.Marshal(resp)
	fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()
}

func scheme(c *gin.Context) string {
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		return "https"
	}
	return "http"
}
