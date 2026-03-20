package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	agentpkg "github.com/yinhe/starclaw/internal/agent"
	"github.com/yinhe/starclaw/internal/memory"
	"github.com/yinhe/starclaw/internal/model"
	"github.com/yinhe/starclaw/internal/provider"
	"github.com/yinhe/starclaw/internal/rag"
	"github.com/yinhe/starclaw/internal/tool"
	"gorm.io/gorm"
)

type ChatHandler struct {
	db               *gorm.DB
	providerRegistry *provider.Registry
	toolRegistry     *tool.Registry
	embedder         rag.EmbeddingProvider
	cerebrate        *memory.Cerebrate
}

func NewChatHandler(db *gorm.DB, pr *provider.Registry, tr *tool.Registry, emb rag.EmbeddingProvider) *ChatHandler {
	return &ChatHandler{
		db: db, providerRegistry: pr, toolRegistry: tr, embedder: emb,
		cerebrate: memory.NewCerebrate(db, pr),
	}
}

type ChatCompletionRequest struct {
	AgentID          string           `json:"agent_id" binding:"required"`
	ConversationID   string           `json:"conversation_id"`
	Message          string           `json:"message" binding:"required"`
	Images           []string         `json:"images,omitempty"`             // base64 data URLs for vision
	Files            []FileAttachment `json:"files,omitempty"`              // uploaded file attachments
	KnowledgeBaseIDs []string         `json:"knowledge_base_ids,omitempty"` // user-selected KBs for RAG
	Stream           bool             `json:"stream"`
}

type FileAttachment struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	URL      string `json:"url"`
	Size     int64  `json:"size"`
	Mime     string `json:"mime"`
	Category string `json:"category"`
	Stored   string `json:"stored,omitempty"`
}

func (h *ChatHandler) Chat(c *gin.Context) {
	userID := c.GetString("user_id")

	var req ChatCompletionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Handle /model command — switch or list models
	if strings.HasPrefix(req.Message, "/model") {
		h.handleModelCommand(c, userID, req)
		return
	}

	// Get agent
	var agent model.Agent
	if err := h.db.Where("id = ? AND (user_id = ? OR is_public = ?)", req.AgentID, userID, true).First(&agent).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
		return
	}

	// Get or create conversation
	var conversation model.Conversation
	if req.ConversationID != "" {
		if err := h.db.Where("id = ? AND user_id = ?", req.ConversationID, userID).First(&conversation).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "conversation not found"})
			return
		}
	} else {
		conversation = model.Conversation{
			UserID:  userID,
			AgentID: agent.ID,
			Title:   truncate(req.Message, 100),
		}
		h.db.Create(&conversation)
	}

	// Resolve model config: conversation override > agent setting > user default > platform
	modelCfg, err := h.resolveModelConfig(userID, conversation.ModelID, agent.ModelID)

	// Save user message first (even if no model)
	// Save user message (with file attachments if any)
	userMsg := model.Message{
		ConversationID: conversation.ID,
		Role:           "user",
		Content:        req.Message,
	}
	if len(req.Files) > 0 {
		filesJSON, _ := json.Marshal(req.Files)
		userMsg.Attachments = string(filesJSON)
	}
	h.db.Create(&userMsg)

	// No model configured → friendly assistant prompt
	if err != nil {
		guide := "👋 你好！我还不能回答你的问题，因为还没有配置 AI 模型。\n\n" +
			"**请按以下步骤操作：**\n" +
			"1. 点击左侧菜单的「模型管理」\n" +
			"2. 添加一个模型供应商（如 Qwen / OpenAI / DeepSeek / MiniMax）\n" +
			"3. 填入对应的 API Key 并启用\n\n" +
			"配置完成后回到这里，我就能为你服务了！\n\n" +
			"💡 **提示：** 配置好后可以用 `/model` 命令查看和切换模型。"
		assistantMsg := model.Message{
			ConversationID: conversation.ID,
			Role:           "assistant",
			Content:        guide,
		}
		h.db.Create(&assistantMsg)
		// Return as SSE so frontend displays it properly
		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")
		c.Writer.WriteHeader(http.StatusOK)
		sseData := fmt.Sprintf(`{"conversation_id":"%s","content":"%s"}`,
			conversation.ID, strings.ReplaceAll(strings.ReplaceAll(guide, `"`, `\"`), "\n", `\n`))
		fmt.Fprintf(c.Writer, "data: %s\n\n", sseData)
		fmt.Fprintf(c.Writer, "data: [DONE]\n\n")
		c.Writer.Flush()
		return
	}

	// Build message history
	var history []model.Message
	h.db.Where("conversation_id = ?", conversation.ID).Order("created_at ASC").Find(&history)

	// RAG: retrieve relevant context from knowledge bases
	systemPrompt := agent.SystemPrompt
	// Collect all KB IDs: agent's default KB + user-selected KBs
	kbIDs := make(map[string]bool)
	if agent.KnowledgeBaseID != "" {
		kbIDs[agent.KnowledgeBaseID] = true
	}
	for _, id := range req.KnowledgeBaseIDs {
		if id != "" {
			kbIDs[id] = true
		}
	}
	if len(kbIDs) > 0 && h.embedder != nil {
		retriever := rag.NewRetriever(h.db, h.embedder)
		var allResults []rag.SearchResult
		for kbID := range kbIDs {
			// Verify user owns this KB
			var kb model.KnowledgeBase
			if err := h.db.Where("id = ? AND user_id = ?", kbID, userID).First(&kb).Error; err != nil {
				continue
			}
			results, err := retriever.Search(c.Request.Context(), kbID, req.Message, 5)
			if err != nil {
				log.Printf("[RAG] search KB %s failed: %v", kbID, err)
				continue
			}
			allResults = append(allResults, results...)
		}
		if len(allResults) > 0 {
			// Sort by score descending and take top 8
			rag.SortResults(allResults)
			if len(allResults) > 8 {
				allResults = allResults[:8]
			}
			context := rag.BuildContext(allResults, 6000)
			systemPrompt = systemPrompt + "\n\n以下是从知识库中检索到的与用户问题相关的参考资料，请基于这些信息回答：\n\n" + context
			log.Printf("[RAG] injected %d chunks from %d KBs into prompt", len(allResults), len(kbIDs))
		}
	}

	// Inject file context into prompt if files are attached
	if len(req.Files) > 0 {
		fileContext := buildFileContext(req.Files)
		if fileContext != "" {
			systemPrompt += "\n\n" + fileContext
		}

		// Extract vision from file attachments (videos: extract frames; images: convert to base64)
		// Skip image files if base64 URLs were already provided via req.Images (avoid duplicates)
		filesToExtract := req.Files
		if len(req.Images) > 0 {
			var nonImageFiles []FileAttachment
			for _, f := range req.Files {
				if !strings.HasPrefix(strings.ToLower(f.Mime), "image/") {
					nonImageFiles = append(nonImageFiles, f)
				}
			}
			filesToExtract = nonImageFiles
		}
		visionURLs := extractVisionFromFiles(filesToExtract, 4)
		if len(visionURLs) > 0 {
			req.Images = append(req.Images, visionURLs...)
			log.Printf("[vision] injected %d image/video-frame URLs from %d file attachments", len(visionURLs), len(filesToExtract))
		}
	}

	// Cerebrate: inject cross-session memories into system prompt
	if h.cerebrate != nil {
		memories, err := h.cerebrate.Retrieve(userID, agent.ID, req.Message, 10)
		if err == nil && len(memories) > 0 {
			systemPrompt += memory.BuildPromptInjection(memories)
			log.Printf("[cerebrate] injected %d memories for user %s", len(memories), userID)
		}
	}

	// Inject resource directory info so AI knows where media files are stored
	systemPrompt += "\n\n" + tool.DataDirSummary()

	messages := buildProviderMessages(systemPrompt, history, req.Images)

	// Get or create provider dynamically
	p := h.getProvider(modelCfg)

	// Parse agent tool config
	var enabledTools []string
	if agent.Tools != "" {
		json.Unmarshal([]byte(agent.Tools), &enabledTools)
	}

	// Use Agent Runtime for tool calling support
	rt := agentpkg.NewRuntime(p, h.toolRegistry)

	// Use agent's own model name if set, otherwise fall back to provider config default
	chatModel := modelCfg.ModelName
	if agent.ModelName != "" {
		chatModel = agent.ModelName
	}
	// Per-conversation model override (e.g. /model deepseek-chat within star-ai)
	if conversation.ModelOverride != "" {
		chatModel = conversation.ModelOverride
	}
	// If model_name is empty or "default", auto-select the provider's first model
	if chatModel == "" || chatModel == "default" {
		if models := p.Models(); len(models) > 0 {
			chatModel = models[0]
			log.Printf("[Chat] Auto-selected model %q for provider %s", chatModel, modelCfg.Provider)
		}
	}

	// Auto-switch to a vision model when images are present
	if len(req.Images) > 0 && !isVisionModel(chatModel) {
		if vm := pickVisionModel(p.Models(), modelCfg.Provider); vm != "" {
			log.Printf("[Chat] Images detected — switching from %q to vision model %q", chatModel, vm)
			chatModel = vm
		}
	}

	maxTok := modelCfg.MaxTokens
	if maxTok < 16384 {
		maxTok = 16384
	}
	runReq := &agentpkg.RunRequest{
		Model:       chatModel,
		Messages:    messages,
		Tools:       enabledTools,
		Temperature: modelCfg.Temperature,
		MaxTokens:   maxTok,
	}

	// Inject user_id, conversation_id, and provider into context so tools can access them
	ctx := context.WithValue(c.Request.Context(), tool.CtxKeyUserID, userID)
	ctx = context.WithValue(ctx, tool.CtxKeyConversationID, conversation.ID)
	ctx = context.WithValue(ctx, tool.CtxKeyProvider, modelCfg.Provider)
	c.Request = c.Request.WithContext(ctx)

	// Store platform key flag for billing
	c.Set("is_platform_key", modelCfg.IsPlatform)
	c.Set("agent_id_for_cerebrate", agent.ID)

	if req.Stream {
		h.handleStreamWithTools(c, rt, runReq, conversation.ID)
	} else {
		h.handleSyncWithTools(c, rt, runReq, conversation.ID)
	}
}

func (h *ChatHandler) getProvider(cfg model.ModelConfig) provider.ModelProvider {
	return provider.CreateFromConfig(h.providerRegistry, cfg)
}

func (h *ChatHandler) handleSyncWithTools(c *gin.Context, rt *agentpkg.Runtime, req *agentpkg.RunRequest, convID string) {
	result, err := rt.Run(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	assistantMsg := model.Message{
		ConversationID: convID,
		Role:           "assistant",
		Content:        result.Content,
	}
	if result.Usage != nil {
		assistantMsg.TokensUsed = result.Usage.TotalTokens
		go recordUsage(h.db, c.GetString("user_id"), "tokens", int64(result.Usage.TotalTokens), c.GetBool("is_platform_key"))
	}
	h.db.Create(&assistantMsg)

	// Cerebrate: async extract memories + summary from this conversation turn
	if h.cerebrate != nil {
		uid, aid := c.GetString("user_id"), c.GetString("agent_id_for_cerebrate")
		go h.cerebrate.ExtractAndStore(context.Background(), uid, aid, result.Messages, convID)
		go h.cerebrate.GenerateSummary(context.Background(), uid, aid, convID, result.Messages)
	}

	c.JSON(http.StatusOK, gin.H{
		"conversation_id": convID,
		"message":         assistantMsg,
	})
}

// toolDisplayName returns a user-friendly name for a tool
func toolDisplayName(name string) string {
	m := map[string]string{
		"video_generation": "视频生成", "music_generation": "音乐生成",
		"image_generation": "图片生成", "mv_production": "MV合成",
		"comic_production": "漫剧制作", "dubbing": "配音字幕",
		"code": "代码执行", "web_search": "网页搜索",
		"browser": "浏览器", "http_request": "HTTP请求",
		"system": "系统操作",
	}
	if v, ok := m[name]; ok {
		return v
	}
	return name
}

func (h *ChatHandler) handleStreamWithTools(c *gin.Context, rt *agentpkg.Runtime, req *agentpkg.RunRequest, convID string) {
	ch, err := rt.StreamRun(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	userID := c.GetString("user_id")
	startTime := time.Now()
	var fullContent string
	saved := false

	// Track tool calls for persistence
	type toolRecord struct {
		ToolCall   string `json:"tool_call"`
		ToolResult string `json:"tool_result,omitempty"`
		ToolName   string `json:"tool_name,omitempty"`
		Reasoning  string `json:"reasoning,omitempty"`
	}
	var toolRecords []toolRecord

	// Ensure partial content is saved even if client disconnects
	defer func() {
		if !saved && (fullContent != "" || len(toolRecords) > 0) {
			assistantMsg := model.Message{
				ConversationID: convID,
				Role:           "assistant",
				Content:        fullContent,
			}
			if len(toolRecords) > 0 {
				tcJSON, _ := json.Marshal(toolRecords)
				assistantMsg.ToolCalls = string(tcJSON)
			}
			h.db.Create(&assistantMsg)
			log.Printf("[Chat] Saved partial assistant message (%d chars, %d tools) for conv %s", len(fullContent), len(toolRecords), convID)
		}
	}()

	// Custom streaming loop with heartbeat to keep SSE alive during long tool executions
	w := c.Writer
	flusher, _ := w.(http.Flusher)
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	clientGone := c.Request.Context().Done()

	for {
		select {
		case <-clientGone:
			log.Printf("[Chat] Client disconnected for conv %s", convID)
			return
		case <-heartbeat.C:
			fmt.Fprintf(w, ": heartbeat\n\n")
			if flusher != nil {
				flusher.Flush()
			}
		case chunk, ok := <-ch:
			if !ok {
				return
			}

			if chunk.Error != "" {
				data, _ := json.Marshal(gin.H{"error": chunk.Error})
				fmt.Fprintf(w, "data: %s\n\n", data)
				if flusher != nil {
					flusher.Flush()
				}
				return
			}

			if chunk.AgentStep != "" {
				data, _ := json.Marshal(gin.H{
					"agent_step":        chunk.AgentStep,
					"agent_step_detail": chunk.AgentStepDetail,
					"agent_step_index":  chunk.AgentStepIndex,
					"conversation_id":   convID,
				})
				fmt.Fprintf(w, "data: %s\n\n", data)
				if flusher != nil {
					flusher.Flush()
				}
				continue
			}

			if chunk.ToolCall != "" {
				toolRecords = append(toolRecords, toolRecord{ToolCall: chunk.ToolCall, Reasoning: chunk.Reasoning})
				sseFields := gin.H{"tool_call": chunk.ToolCall, "conversation_id": convID}
				if chunk.Reasoning != "" {
					sseFields["reasoning"] = chunk.Reasoning
				}
				data, _ := json.Marshal(sseFields)
				fmt.Fprintf(w, "data: %s\n\n", data)
				if flusher != nil {
					flusher.Flush()
				}
				continue
			}

			if chunk.ToolResult != "" {
				// Attach result to the last tool record
				if len(toolRecords) > 0 && toolRecords[len(toolRecords)-1].ToolResult == "" {
					toolRecords[len(toolRecords)-1].ToolResult = chunk.ToolResult
					toolRecords[len(toolRecords)-1].ToolName = chunk.ToolName
				}
				// Send tool completion notification
				go func(uid, cid, tName, tResult string) {
					display := toolDisplayName(tName)
					resultPreview := tResult
					if len(resultPreview) > 200 {
						resultPreview = resultPreview[:200] + "..."
					}
					h.db.Create(&model.Notification{
						UserID:  uid,
						Type:    model.NotifySuccess,
						Title:   fmt.Sprintf("%s 执行完成", display),
						Content: resultPreview,
					})
				}(userID, convID, chunk.ToolName, chunk.ToolResult)
				// Extract screenshot URL from browser tool results
				sseFields := gin.H{"tool_result": chunk.ToolResult, "tool_name": chunk.ToolName, "conversation_id": convID}
				var parsed map[string]interface{}
				if err := json.Unmarshal([]byte(chunk.ToolResult), &parsed); err == nil {
					if screenshotURL, ok := parsed["screenshot"].(string); ok && screenshotURL != "" {
						sseFields["screenshot_url"] = screenshotURL
					}
				}
				data, _ := json.Marshal(sseFields)
				fmt.Fprintf(w, "data: %s\n\n", data)
				if flusher != nil {
					flusher.Flush()
				}
				continue
			}

			if chunk.Done {
				assistantMsg := model.Message{
					ConversationID: convID,
					Role:           "assistant",
					Content:        fullContent,
				}
				if chunk.Usage != nil {
					assistantMsg.TokensUsed = chunk.Usage.TotalTokens
					go recordUsage(h.db, c.GetString("user_id"), "tokens", int64(chunk.Usage.TotalTokens), c.GetBool("is_platform_key"))
				}
				if len(toolRecords) > 0 {
					tcJSON, _ := json.Marshal(toolRecords)
					assistantMsg.ToolCalls = string(tcJSON)
				}
				h.db.Create(&assistantMsg)
				saved = true

				// Cerebrate: async extract memories + summary from this conversation
				if h.cerebrate != nil {
					aid := c.GetString("agent_id_for_cerebrate")
					go h.cerebrate.ExtractAndStore(context.Background(), userID, aid, req.Messages, convID)
					go h.cerebrate.GenerateSummary(context.Background(), userID, aid, convID, req.Messages)
				}

				// Long-running conversation notification (>30s)
				if elapsed := time.Since(startTime); elapsed > 30*time.Second {
					go func(uid, cid string, dur time.Duration, content string) {
						preview := content
						if len(preview) > 150 {
							preview = preview[:150] + "..."
						}
						h.db.Create(&model.Notification{
							UserID:  uid,
							Type:    model.NotifyInfo,
							Title:   fmt.Sprintf("对话已完成（耗时 %ds）", int(dur.Seconds())),
							Content: preview,
						})
					}(userID, convID, elapsed, fullContent)
				}

				data, _ := json.Marshal(gin.H{"done": true, "conversation_id": convID})
				fmt.Fprintf(w, "data: %s\n\n", data)
				if flusher != nil {
					flusher.Flush()
				}
				return
			}

			fullContent += chunk.Content
			sseFields := gin.H{"content": chunk.Content, "conversation_id": convID}
			if chunk.Meta != nil {
				sseFields["meta"] = chunk.Meta
			}
			data, _ := json.Marshal(sseFields)
			fmt.Fprintf(w, "data: %s\n\n", data)
			if flusher != nil {
				flusher.Flush()
			}
		}
	}
}

func (h *ChatHandler) ListConversations(c *gin.Context) {
	userID := c.GetString("user_id")

	var conversations []model.Conversation
	h.db.Where("user_id = ?", userID).Order("updated_at DESC").Find(&conversations)

	c.JSON(http.StatusOK, gin.H{"conversations": conversations})
}

func (h *ChatHandler) GetMessages(c *gin.Context) {
	userID := c.GetString("user_id")
	convID := c.Param("id")

	var conv model.Conversation
	if err := h.db.Where("id = ? AND user_id = ?", convID, userID).First(&conv).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "conversation not found"})
		return
	}

	var messages []model.Message
	h.db.Where("conversation_id = ?", convID).Order("created_at ASC").Find(&messages)

	c.JSON(http.StatusOK, gin.H{"messages": messages})
}

func (h *ChatHandler) RenameConversation(c *gin.Context) {
	userID := c.GetString("user_id")
	convID := c.Param("id")

	var req struct {
		Title string `json:"title" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result := h.db.Model(&model.Conversation{}).Where("id = ? AND user_id = ?", convID, userID).Update("title", req.Title)
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "conversation not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "renamed"})
}

func (h *ChatHandler) DeleteConversation(c *gin.Context) {
	userID := c.GetString("user_id")
	convID := c.Param("id")

	result := h.db.Where("id = ? AND user_id = ?", convID, userID).Delete(&model.Conversation{})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "conversation not found"})
		return
	}
	// Clean up messages
	h.db.Where("conversation_id = ?", convID).Delete(&model.Message{})
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// TruncateMessages deletes a message and all subsequent messages in a conversation (for resend)
func (h *ChatHandler) TruncateMessages(c *gin.Context) {
	userID := c.GetString("user_id")
	convID := c.Param("id")
	msgID := c.Param("msg_id")

	// Verify conversation ownership
	var conv model.Conversation
	if err := h.db.Where("id = ? AND user_id = ?", convID, userID).First(&conv).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "conversation not found"})
		return
	}

	// Find the target message to get its created_at
	var msg model.Message
	if err := h.db.Where("id = ? AND conversation_id = ?", msgID, convID).First(&msg).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "message not found"})
		return
	}

	// Delete this message and all messages created at or after it
	result := h.db.Where("conversation_id = ? AND created_at >= ?", convID, msg.CreatedAt).Delete(&model.Message{})
	c.JSON(http.StatusOK, gin.H{"deleted": result.RowsAffected})
}

func (h *ChatHandler) BatchDeleteConversations(c *gin.Context) {
	userID := c.GetString("user_id")
	var req struct {
		IDs []string `json:"ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result := h.db.Where("id IN ? AND user_id = ?", req.IDs, userID).Delete(&model.Conversation{})
	h.db.Where("conversation_id IN ?", req.IDs).Delete(&model.Message{})

	c.JSON(http.StatusOK, gin.H{"deleted": result.RowsAffected})
}

func (h *ChatHandler) PinConversation(c *gin.Context) {
	userID := c.GetString("user_id")
	convID := c.Param("id")

	var conv model.Conversation
	if err := h.db.Where("id = ? AND user_id = ?", convID, userID).First(&conv).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "conversation not found"})
		return
	}
	h.db.Model(&conv).Update("is_pinned", !conv.IsPinned)
	c.JSON(http.StatusOK, gin.H{"is_pinned": !conv.IsPinned})
}

func (h *ChatHandler) FeedbackMessage(c *gin.Context) {
	convID := c.Param("id")
	msgID := c.Param("msg_id")
	userID := c.GetString("user_id")

	var req struct {
		Feedback int `json:"feedback"` // 1=up, -1=down, 0=clear
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify conversation ownership
	var conv model.Conversation
	if err := h.db.Where("id = ? AND user_id = ?", convID, userID).First(&conv).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "conversation not found"})
		return
	}

	result := h.db.Model(&model.Message{}).Where("id = ? AND conversation_id = ?", msgID, convID).Update("feedback", req.Feedback)
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "message not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "feedback saved"})
}

func (h *ChatHandler) ExportConversation(c *gin.Context) {
	userID := c.GetString("user_id")
	convID := c.Param("id")

	var conv model.Conversation
	if err := h.db.Where("id = ? AND user_id = ?", convID, userID).First(&conv).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "conversation not found"})
		return
	}

	var messages []model.Message
	h.db.Where("conversation_id = ?", convID).Order("created_at ASC").Find(&messages)

	// Build markdown export
	export := "# " + conv.Title + "\n\n"
	for _, msg := range messages {
		role := "User"
		if msg.Role == "assistant" {
			role = "Assistant"
		}
		export += "## " + role + "\n\n" + msg.Content + "\n\n---\n\n"
	}

	c.JSON(http.StatusOK, gin.H{
		"title":   conv.Title,
		"content": export,
		"format":  "markdown",
	})
}

func buildProviderMessages(systemPrompt string, history []model.Message, images []string) []provider.ChatMessage {
	var messages []provider.ChatMessage

	if systemPrompt != "" {
		messages = append(messages, provider.ChatMessage{
			Role:    "system",
			Content: systemPrompt,
		})
	}

	for i, msg := range history {
		pm := provider.ChatMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
		// Attach images to the last user message
		if i == len(history)-1 && msg.Role == "user" && len(images) > 0 {
			parts := []provider.ContentPart{
				{Type: "text", Text: msg.Content},
			}
			for _, imgURL := range images {
				parts = append(parts, provider.ContentPart{
					Type:     "image_url",
					ImageURL: &provider.ImageURL{URL: imgURL, Detail: "auto"},
				})
			}
			pm.MultiContent = parts
			pm.Content = msg.Content
		}
		messages = append(messages, pm)
	}

	return messages
}

// buildFileContext creates a text summary of attached files for the LLM to understand
func buildFileContext(files []FileAttachment) string {
	if len(files) == 0 {
		return ""
	}

	var parts []string
	parts = append(parts, "用户附带了以下文件：")
	for i, f := range files {
		sizeStr := formatFileSize(f.Size)
		mime := strings.ToLower(f.Mime)

		if strings.HasPrefix(mime, "image/") {
			parts = append(parts, fmt.Sprintf("%d. 📷 %s (%s, %s) — 图片已注入到视觉消息中，你可以直接看到并分析图片内容", i+1, f.Filename, f.Mime, sizeStr))
			continue
		}
		if strings.HasPrefix(mime, "audio/") {
			parts = append(parts, fmt.Sprintf("%d. 🎵 %s (%s, %s) — 音频文件路径: %s", i+1, f.Filename, f.Mime, sizeStr, f.URL))
			parts = append(parts, fmt.Sprintf("   可直接用于 audio_analysis（file_url: \"%s\"）和 mv_production.compose_pro（audio_url: \"%s\"）", f.URL, f.URL))
			continue
		}
		if strings.HasPrefix(mime, "video/") {
			parts = append(parts, fmt.Sprintf("%d. 🎬 %s (%s, %s, 路径: %s) — 已从视频中提取关键帧注入到视觉消息中，你可以看到视频画面并分析内容", i+1, f.Filename, f.Mime, sizeStr, f.URL))
			continue
		}

		parts = append(parts, fmt.Sprintf("%d. %s (%s, %s, %s)", i+1, f.Filename, f.Category, f.Mime, sizeStr))

		// For text-readable files, try to read and include content
		if isTextReadable(f.Mime, f.Filename) && f.Stored != "" {
			content, err := readUploadedFileContent("/app/uploads/" + f.Stored)
			if err == nil && content != "" {
				// Limit to 10000 chars to avoid prompt overflow
				if len(content) > 10000 {
					content = content[:10000] + "\n... (文件内容已截断，共 " + fmt.Sprintf("%d", len(content)) + " 字符)"
				}
				parts = append(parts, "```\n"+content+"\n```")
			}
		}
	}
	return strings.Join(parts, "\n")
}

func isTextReadable(mime, filename string) bool {
	textPrefixes := []string{"text/", "application/json", "application/xml"}
	for _, p := range textPrefixes {
		if strings.HasPrefix(mime, p) {
			return true
		}
	}
	textExts := []string{".md", ".csv", ".yaml", ".yml", ".toml", ".ini", ".log", ".sql", ".sh", ".py", ".js", ".ts", ".go", ".java", ".c", ".cpp", ".rs", ".rb", ".php", ".html", ".xml", ".json", ".txt", ".rtf"}
	ext := strings.ToLower(filepath.Ext(filename))
	for _, e := range textExts {
		if ext == e {
			return true
		}
	}
	return false
}

func readUploadedFileContent(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func formatFileSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	} else if size < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(size)/1024)
	} else if size < 1024*1024*1024 {
		return fmt.Sprintf("%.1f MB", float64(size)/(1024*1024))
	}
	return fmt.Sprintf("%.1f GB", float64(size)/(1024*1024*1024))
}

// isVisionModel returns true if the model name indicates vision capability
func isVisionModel(model string) bool {
	m := strings.ToLower(model)
	// Models with explicit vision naming
	if strings.Contains(m, "-vl") || strings.Contains(m, "vl-") || strings.Contains(m, "vision") {
		return true
	}
	// Models known to support vision natively
	visionModels := []string{
		"gpt-4o", "gpt-4o-mini", "gpt-4-turbo", "gpt-4.1", "gpt-4.1-mini", "gpt-4.1-nano",
		"chatgpt-4o-latest",
		"claude-sonnet-4", "claude-3-7-sonnet", "claude-3-5-sonnet", "claude-3-opus",
		"gemini-2.5-pro", "gemini-2.5-flash", "gemini-2.0-flash", "gemini-1.5-pro", "gemini-1.5-flash",
		"o3", "o3-mini", "o4-mini", "o1", "o1-mini",
	}
	for _, vm := range visionModels {
		if strings.HasPrefix(m, vm) {
			return true
		}
	}
	return false
}

// pickVisionModel selects the best vision model from the provider's available models
func pickVisionModel(available []string, providerName string) string {
	// Provider-specific preferred vision models (best first)
	preferred := map[string][]string{
		"star-ai":   {"qwen-vl-max", "qwen-vl-plus", "gpt-4o", "gemini-2.0-flash"},
		"qwen":      {"qwen-vl-max", "qwen-vl-plus", "qwen3-vl-plus", "qwen3-vl-flash"},
		"openai":    {"gpt-4o", "gpt-4o-mini", "gpt-4-turbo"},
		"google":    {"gemini-2.5-flash", "gemini-2.0-flash", "gemini-1.5-flash"},
		"anthropic": {"claude-sonnet-4-20250514", "claude-3-7-sonnet-20250219", "claude-3-5-sonnet-20241022"},
	}

	// Try preferred models first
	if prefs, ok := preferred[providerName]; ok {
		for _, pref := range prefs {
			for _, avail := range available {
				if strings.HasPrefix(strings.ToLower(avail), strings.ToLower(pref)) {
					return avail
				}
			}
		}
	}

	// Fallback: any vision model from available list
	for _, avail := range available {
		if isVisionModel(avail) {
			return avail
		}
	}
	return ""
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

// resolveModelConfig picks the best model config by priority:
// 1. conversationModelID (per-conversation override via /model command)
// 2. agentModelID (agent setting)
// 3. user's first enabled model
// 4. platform model
func (h *ChatHandler) resolveModelConfig(userID, conversationModelID, agentModelID string) (model.ModelConfig, error) {
	var cfg model.ModelConfig
	// 1. Conversation override
	if conversationModelID != "" {
		if err := h.db.Where("id = ?", conversationModelID).First(&cfg).Error; err == nil {
			return cfg, nil
		}
	}
	// 2. Agent setting
	if agentModelID != "" {
		if err := h.db.Where("id = ?", agentModelID).First(&cfg).Error; err == nil {
			return cfg, nil
		}
	}
	// 3. User's first enabled model
	if err := h.db.Where("user_id = ? AND is_enabled = ?", userID, true).Order("created_at ASC").First(&cfg).Error; err == nil {
		return cfg, nil
	}
	// 4. Platform model
	if err := h.db.Where("user_id = 'platform' AND is_enabled = ?", true).Order("created_at ASC").First(&cfg).Error; err == nil {
		return cfg, nil
	}
	return cfg, fmt.Errorf("请先在「模型管理」中配置至少一个模型")
}

// handleModelCommand processes /model commands:
//   - /model              → list available models and current selection
//   - /model <provider>   → switch current conversation to that model provider
//   - /model <model-name> → switch model within star-ai super router (e.g. /model deepseek-chat)
func (h *ChatHandler) handleModelCommand(c *gin.Context, userID string, req ChatCompletionRequest) {
	arg := strings.TrimSpace(strings.TrimPrefix(req.Message, "/model"))

	// List all user's model configs
	var models []model.ModelConfig
	h.db.Where("(user_id = ? OR user_id = 'platform') AND is_enabled = ?", userID, true).Order("created_at ASC").Find(&models)

	if len(models) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"message": "暂无可用模型，请先在「模型管理」中添加。",
			"command": true,
		})
		return
	}

	// Find the star-ai provider config (if any) for super-router model switching
	var starAICfg *model.ModelConfig
	for i := range models {
		if models[i].Provider == "star-ai" || models[i].Provider == "starai" {
			starAICfg = &models[i]
			break
		}
	}

	// /model (no arg) → list models
	if arg == "" {
		// Get current conversation state
		currentProvider := ""
		currentOverride := ""
		if req.ConversationID != "" {
			var conv model.Conversation
			if err := h.db.Where("id = ? AND user_id = ?", req.ConversationID, userID).First(&conv).Error; err == nil {
				currentOverride = conv.ModelOverride
				if conv.ModelID != "" {
					var current model.ModelConfig
					if h.db.Where("id = ?", conv.ModelID).First(&current).Error == nil {
						currentProvider = current.Provider
					}
				}
			}
		}

		var sb strings.Builder
		sb.WriteString("**可用模型供应商：**\n")
		for _, m := range models {
			marker := "  "
			if m.Provider == currentProvider {
				marker = "▶ "
			}
			name := m.DisplayName
			if name == "" {
				name = m.Provider + "/" + m.ModelName
			}
			sb.WriteString(fmt.Sprintf("%s`%s` — %s\n", marker, m.Provider, name))
		}

		// Show star-ai available models if star-ai is configured
		if starAICfg != nil {
			p := h.getProvider(*starAICfg)
			starModels := p.Models()
			if len(starModels) > 0 {
				sb.WriteString("\n**Star AI 可用模型（超级路由）：**\n")
				for _, m := range starModels {
					marker := "  "
					if m == currentOverride {
						marker = "▶ "
					}
					sb.WriteString(fmt.Sprintf("%s`%s`\n", marker, m))
				}
			}
		}

		sb.WriteString("\n**切换命令：**\n")
		sb.WriteString("- `/model <provider>` — 切换供应商，如 `/model deepseek`\n")
		if starAICfg != nil {
			sb.WriteString("- `/model <模型名>` — Star AI 内切换模型，如 `/model deepseek-chat`\n")
		}

		c.JSON(http.StatusOK, gin.H{
			"message": sb.String(),
			"command": true,
		})
		return
	}

	// /model <name> → switch model or provider
	arg = strings.ToLower(arg)

	// Handle provider/model format (e.g. "starai/qwen3-max" or "star-ai/deepseek-chat")
	if parts := strings.SplitN(arg, "/", 2); len(parts) == 2 && parts[1] != "" {
		provPart := parts[0]
		modelPart := parts[1]
		provNorm := strings.NewReplacer("-", "", "_", "", " ", "").Replace(provPart)
		// Find matching provider
		for i := range models {
			pLower := strings.ToLower(models[i].Provider)
			pNorm := strings.NewReplacer("-", "", "_", "", " ", "").Replace(pLower)
			if pLower == provPart || pNorm == provNorm {
				// Provider found — switch to it with model override
				h.switchStarAIModel(c, userID, req, &models[i], modelPart)
				return
			}
		}
	}

	// Normalize: strip hyphens/underscores for fuzzy matching (e.g. "starai" matches "star-ai")
	argNorm := strings.NewReplacer("-", "", "_", "", " ", "").Replace(arg)

	// First: try matching a provider config
	var target *model.ModelConfig
	for i := range models {
		provLower := strings.ToLower(models[i].Provider)
		provNorm := strings.NewReplacer("-", "", "_", "", " ", "").Replace(provLower)
		if provLower == arg || provNorm == argNorm ||
			strings.Contains(strings.ToLower(models[i].DisplayName), arg) ||
			strings.Contains(strings.ToLower(models[i].ModelName), arg) {
			target = &models[i]
			break
		}
	}

	// Second: if no provider match but star-ai exists, check star-ai's model list
	if target == nil && starAICfg != nil {
		p := h.getProvider(*starAICfg)
		for _, m := range p.Models() {
			mLower := strings.ToLower(m)
			mNorm := strings.NewReplacer("-", "", "_", "", " ", "").Replace(mLower)
			if mLower == arg || mNorm == argNorm || strings.Contains(mLower, arg) {
				// Match! Switch to star-ai provider + set model override
				h.switchStarAIModel(c, userID, req, starAICfg, m)
				return
			}
		}
	}

	if target == nil {
		// Build helpful message
		providers := make([]string, len(models))
		for i, m := range models {
			providers[i] = m.Provider
		}
		msg := fmt.Sprintf("未找到模型「%s」。\n\n可用供应商：%s", arg, strings.Join(providers, ", "))
		if starAICfg != nil {
			msg += "\n\n💡 Star AI 支持所有模型，试试 `/model deepseek-chat` 或 `/model qwen-max`"
		}
		c.JSON(http.StatusOK, gin.H{
			"message": msg,
			"command": true,
		})
		return
	}

	// Switching provider — clear any model override
	if req.ConversationID == "" {
		c.JSON(http.StatusOK, gin.H{
			"message":  fmt.Sprintf("已选择 **%s** (%s)，发送消息后生效。", target.Provider, target.ModelName),
			"command":  true,
			"model_id": target.ID,
		})
		return
	}

	// Update conversation: set provider, clear model override
	h.db.Model(&model.Conversation{}).Where("id = ? AND user_id = ?", req.ConversationID, userID).
		Updates(map[string]interface{}{"model_id": target.ID, "model_override": ""})

	name := target.DisplayName
	if name == "" {
		name = target.Provider + "/" + target.ModelName
	}
	c.JSON(http.StatusOK, gin.H{
		"message":  fmt.Sprintf("✅ 已切换到 **%s** (%s)", name, target.Provider),
		"command":  true,
		"model_id": target.ID,
	})
}

// switchStarAIModel handles switching to a specific model within the star-ai super router.
func (h *ChatHandler) switchStarAIModel(c *gin.Context, userID string, req ChatCompletionRequest, starAICfg *model.ModelConfig, modelName string) {
	if req.ConversationID == "" {
		c.JSON(http.StatusOK, gin.H{
			"message":  fmt.Sprintf("已选择 Star AI → **%s**，发送消息后生效。", modelName),
			"command":  true,
			"model_id": starAICfg.ID,
		})
		return
	}

	// Update conversation: set star-ai provider + model override
	h.db.Model(&model.Conversation{}).Where("id = ? AND user_id = ?", req.ConversationID, userID).
		Updates(map[string]interface{}{"model_id": starAICfg.ID, "model_override": modelName})

	c.JSON(http.StatusOK, gin.H{
		"message":  fmt.Sprintf("✅ 已切换到 Star AI → **%s**", modelName),
		"command":  true,
		"model_id": starAICfg.ID,
	})
}
