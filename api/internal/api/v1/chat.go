package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	agentpkg "github.com/yinhe/starclaw/internal/agent"
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
}

func NewChatHandler(db *gorm.DB, pr *provider.Registry, tr *tool.Registry, emb rag.EmbeddingProvider) *ChatHandler {
	return &ChatHandler{db: db, providerRegistry: pr, toolRegistry: tr, embedder: emb}
}

type ChatCompletionRequest struct {
	AgentID        string   `json:"agent_id" binding:"required"`
	ConversationID string   `json:"conversation_id"`
	Message        string   `json:"message" binding:"required"`
	Images         []string `json:"images,omitempty"` // base64 data URLs for vision
	Stream         bool     `json:"stream"`
}

func (h *ChatHandler) Chat(c *gin.Context) {
	userID := c.GetString("user_id")

	var req ChatCompletionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get agent
	var agent model.Agent
	if err := h.db.Where("id = ? AND (user_id = ? OR is_public = ?)", req.AgentID, userID, true).First(&agent).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
		return
	}

	// Get model config  system built-in agents have no model_id, fallback to user's or platform model
	var modelCfg model.ModelConfig
	if agent.ModelID != "" {
		if err := h.db.Where("id = ?", agent.ModelID).First(&modelCfg).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "model not configured for this agent"})
			return
		}
	} else {
		// Fallback: try user's own model, then platform model
		if err := h.db.Where("user_id = ? AND is_enabled = ?", userID, true).Order("created_at ASC").First(&modelCfg).Error; err != nil {
			if err2 := h.db.Where("user_id = 'platform' AND is_enabled = ?", true).Order("created_at ASC").First(&modelCfg).Error; err2 != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "请先在「模型管理」中配置至少一个模型"})
				return
			}
		}
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

	// Save user message
	userMsg := model.Message{
		ConversationID: conversation.ID,
		Role:           "user",
		Content:        req.Message,
	}
	h.db.Create(&userMsg)

	// Build message history
	var history []model.Message
	h.db.Where("conversation_id = ?", conversation.ID).Order("created_at ASC").Find(&history)

	// RAG: retrieve relevant context if agent has a knowledge base
	systemPrompt := agent.SystemPrompt
	if agent.KnowledgeBaseID != "" && h.embedder != nil {
		retriever := rag.NewRetriever(h.db, h.embedder)
		results, err := retriever.Search(c.Request.Context(), agent.KnowledgeBaseID, req.Message, 5)
		if err != nil {
			log.Printf("[RAG] search failed: %v", err)
		} else if len(results) > 0 {
			context := rag.BuildContext(results, 4000)
			systemPrompt = systemPrompt + "\n\n以下是与用户问题相关的参考资料，请基于这些信息回答：\n\n" + context
			log.Printf("[RAG] injected %d chunks into prompt for KB %s", len(results), agent.KnowledgeBaseID)
		}
	}

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

	// Inject user_id and conversation_id into context so tools can access them
	ctx := context.WithValue(c.Request.Context(), tool.CtxKeyUserID, userID)
	ctx = context.WithValue(ctx, tool.CtxKeyConversationID, conversation.ID)
	c.Request = c.Request.WithContext(ctx)

	// Store platform key flag for billing
	c.Set("is_platform_key", modelCfg.IsPlatform)

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
			data, _ := json.Marshal(gin.H{"content": chunk.Content, "conversation_id": convID})
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

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
