package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw/internal/agent"
	"github.com/yinhe/starclaw/internal/model"
	"github.com/yinhe/starclaw/internal/provider"
	"github.com/yinhe/starclaw/internal/sandbox"
	"github.com/yinhe/starclaw/internal/tool"
	"gorm.io/gorm"
)

type CodingHandler struct {
	db           *gorm.DB
	sandbox      *sandbox.Manager
	providerReg  *provider.Registry
	toolRegistry *tool.Registry
	runningMu    sync.Mutex
	runningCtx   map[string]context.CancelFunc // key: userID
}

func NewCodingHandler(db *gorm.DB, sb *sandbox.Manager, pr *provider.Registry, tr *tool.Registry) *CodingHandler {
	return &CodingHandler{
		db:           db,
		sandbox:      sb,
		providerReg:  pr,
		toolRegistry: tr,
		runningCtx:   make(map[string]context.CancelFunc),
	}
}

func (h *CodingHandler) getProvider(cfg model.ModelConfig) provider.ModelProvider {
	if p, ok := h.providerReg.Get(cfg.Provider); ok {
		return p
	}
	switch cfg.Provider {
	case "anthropic":
		return provider.NewAnthropicProvider(provider.AnthropicConfig{APIKey: cfg.APIKey, BaseURL: cfg.BaseURL})
	case "deepseek":
		return provider.NewDeepSeekProvider(provider.DeepSeekConfig{APIKey: cfg.APIKey, BaseURL: cfg.BaseURL})
	case "ollama":
		return provider.NewOllamaProvider(provider.OllamaConfig{BaseURL: cfg.BaseURL})
	case "openrouter":
		return provider.NewOpenRouterProvider(provider.OpenRouterConfig{APIKey: cfg.APIKey})
	case "qwen":
		return provider.NewQwenProvider(provider.QwenConfig{APIKey: cfg.APIKey, BaseURL: cfg.BaseURL})
	default:
		return provider.NewOpenAIProvider(provider.OpenAIConfig{APIKey: cfg.APIKey, BaseURL: cfg.BaseURL})
	}
}

type CodingRequest struct {
	WorkspaceID string `json:"workspace_id"`
	Message     string `json:"message"`
	ModelID     string `json:"model_id"`
}

// Run handles a coding agent request with SSE streaming
func (h *CodingHandler) Run(c *gin.Context) {
	var req CodingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Message == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "message is required"})
		return
	}
	if req.WorkspaceID == "" {
		req.WorkspaceID = "default"
	}

	// Get model config
	var modelConfig model.ModelConfig
	if req.ModelID != "" {
		h.db.First(&modelConfig, "id = ?", req.ModelID)
	}
	if modelConfig.ID == "" {
		h.db.First(&modelConfig)
	}

	if modelConfig.ID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no model configured"})
		return
	}

	modelProvider := h.getProvider(modelConfig)

	// Build coding-specific tool list
	codingTools := []string{"code"}

	systemPrompt := fmt.Sprintf(`You are an expert autonomous coding agent. You have access to a sandboxed workspace (ID: %s) where you can read, write, and execute code.

## Your Capabilities
- Read and write files in the workspace
- Execute code in Python, JavaScript, Bash, or Go
- Run shell commands (ls, pip install, npm install, etc.)
- Search files by name or content

## Guidelines
1. **Plan first**: Before coding, briefly outline your approach
2. **Read before write**: Always read existing files before modifying them
3. **Write complete files**: When using write_file, write the entire file content
4. **Test your code**: After writing code, execute it to verify it works
5. **Fix errors**: If execution fails, read the error, fix the code, and retry
6. **Use workspace_id**: Always use workspace_id "%s" for all file operations

## Workflow
1. Understand the task
2. List existing files if relevant
3. Plan the implementation
4. Write the code files
5. Execute and test
6. Fix any errors
7. Report the final result

When done, summarize what you created and how to use it.`, req.WorkspaceID, req.WorkspaceID)

	rt := agent.NewRuntime(modelProvider, h.toolRegistry)

	runReq := &agent.RunRequest{
		Messages: []provider.ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: req.Message},
		},
		Model:       modelConfig.ModelName,
		Tools:       codingTools,
		Temperature: modelConfig.Temperature,
		MaxTokens:   modelConfig.MaxTokens,
	}

	ch, err := rt.StreamRun(c.Request.Context(), runReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	c.Stream(func(w io.Writer) bool {
		chunk, ok := <-ch
		if !ok {
			return false
		}

		if chunk.Error != "" {
			data, _ := json.Marshal(gin.H{"error": chunk.Error})
			fmt.Fprintf(w, "data: %s\n\n", data)
			return false
		}

		if chunk.ToolCall != "" {
			data, _ := json.Marshal(gin.H{"tool_call": chunk.ToolCall})
			fmt.Fprintf(w, "data: %s\n\n", data)
			return true
		}

		if chunk.ToolResult != "" {
			data, _ := json.Marshal(gin.H{"tool_result": chunk.ToolResult, "tool_name": chunk.ToolName})
			fmt.Fprintf(w, "data: %s\n\n", data)
			return true
		}

		if chunk.Done {
			data, _ := json.Marshal(gin.H{"done": true, "workspace_id": req.WorkspaceID})
			fmt.Fprintf(w, "data: %s\n\n", data)
			return false
		}

		data, _ := json.Marshal(gin.H{"content": chunk.Content})
		fmt.Fprintf(w, "data: %s\n\n", data)
		return true
	})
}

// ExecuteCode runs code directly (for Artifacts feature)
func (h *CodingHandler) ExecuteCode(c *gin.Context) {
	var req struct {
		Language string `json:"language"`
		Code     string `json:"code"`
		Timeout  int    `json:"timeout"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "code is required"})
		return
	}
	if req.Language == "" {
		req.Language = "python"
	}
	if req.Timeout <= 0 || req.Timeout > 30 {
		req.Timeout = 15
	}

	result, err := h.sandbox.Execute(c.Request.Context(), "artifacts", req.Language, req.Code, req.Timeout)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"result": result})
}

// langFromExt maps file extension to language name for sandbox execution
func langFromExt(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".py":
		return "python"
	case ".js":
		return "javascript"
	case ".ts":
		return "typescript"
	case ".go":
		return "go"
	case ".sh":
		return "bash"
	case ".rb":
		return "ruby"
	case ".php":
		return "php"
	case ".java":
		return "java"
	case ".rs":
		return "rust"
	case ".c":
		return "c"
	case ".cpp", ".cxx", ".cc":
		return "cpp"
	case ".pl":
		return "perl"
	case ".lua":
		return "lua"
	default:
		return ""
	}
}

// RunFile executes a file by path in the user's workspace
func (h *CodingHandler) RunFile(c *gin.Context) {
	userID := c.GetString("user_id")
	var req struct {
		WorkspaceID    string `json:"workspace_id"`
		ConversationID string `json:"conversation_id"`
		FilePath       string `json:"file_path" binding:"required"`
		Timeout        int    `json:"timeout"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.WorkspaceID == "" {
		req.WorkspaceID = userID
	}
	// Files are stored under {workspace_id}/{conversation_id}/
	effectiveWS := req.WorkspaceID
	if req.ConversationID != "" {
		effectiveWS = filepath.Join(req.WorkspaceID, req.ConversationID)
	}
	if req.Timeout <= 0 || req.Timeout > 60 {
		req.Timeout = 30
	}

	// Detect language from extension
	lang := langFromExt(req.FilePath)
	if lang == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported file type: " + filepath.Ext(req.FilePath)})
		return
	}

	// Read the file content
	content, err := h.sandbox.ReadFile(effectiveWS, req.FilePath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found: " + err.Error()})
		return
	}

	// Create cancellable context
	ctx, cancel := context.WithCancel(c.Request.Context())
	h.runningMu.Lock()
	// Cancel any previous run for this user
	if prev, ok := h.runningCtx[userID]; ok {
		prev()
	}
	h.runningCtx[userID] = cancel
	h.runningMu.Unlock()

	defer func() {
		h.runningMu.Lock()
		delete(h.runningCtx, userID)
		h.runningMu.Unlock()
		cancel()
	}()

	result, err := h.sandbox.Execute(ctx, effectiveWS, lang, content, req.Timeout)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"result":   result,
		"language": lang,
		"file":     req.FilePath,
	})
}

// RunCommand runs a shell command in the user's workspace
func (h *CodingHandler) RunCommand(c *gin.Context) {
	userID := c.GetString("user_id")
	var req struct {
		WorkspaceID    string `json:"workspace_id"`
		ConversationID string `json:"conversation_id"`
		Command        string `json:"command" binding:"required"`
		Timeout        int    `json:"timeout"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.WorkspaceID == "" {
		req.WorkspaceID = userID
	}
	// Files are stored under {workspace_id}/{conversation_id}/
	effectiveWS := req.WorkspaceID
	if req.ConversationID != "" {
		effectiveWS = filepath.Join(req.WorkspaceID, req.ConversationID)
	}
	if req.Timeout <= 0 || req.Timeout > 60 {
		req.Timeout = 30
	}

	ctx, cancel := context.WithCancel(c.Request.Context())
	h.runningMu.Lock()
	if prev, ok := h.runningCtx[userID]; ok {
		prev()
	}
	h.runningCtx[userID] = cancel
	h.runningMu.Unlock()

	defer func() {
		h.runningMu.Lock()
		delete(h.runningCtx, userID)
		h.runningMu.Unlock()
		cancel()
	}()

	result, err := h.sandbox.RunCommand(ctx, effectiveWS, req.Command, req.Timeout)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"result":  result,
		"command": req.Command,
	})
}

// StopExecution stops the currently running file execution for this user
func (h *CodingHandler) StopExecution(c *gin.Context) {
	userID := c.GetString("user_id")
	h.runningMu.Lock()
	if cancel, ok := h.runningCtx[userID]; ok {
		cancel()
		delete(h.runningCtx, userID)
		h.runningMu.Unlock()
		c.JSON(http.StatusOK, gin.H{"status": "stopped"})
		return
	}
	h.runningMu.Unlock()
	c.JSON(http.StatusOK, gin.H{"status": "no_running_process"})
}

// ListWorkspaceFiles lists files in a workspace
func (h *CodingHandler) ListWorkspaceFiles(c *gin.Context) {
	workspaceID := c.Param("workspace_id")
	dirPath := c.DefaultQuery("path", ".")

	files, err := h.sandbox.ListFiles(workspaceID, dirPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"files": files, "workspace_id": workspaceID})
}

// ReadWorkspaceFile reads a file from a workspace
func (h *CodingHandler) ReadWorkspaceFile(c *gin.Context) {
	workspaceID := c.Param("workspace_id")
	filePath := c.Query("path")
	if filePath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path is required"})
		return
	}

	content, err := h.sandbox.ReadFile(workspaceID, filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"content": content, "path": filePath, "workspace_id": workspaceID})
}
