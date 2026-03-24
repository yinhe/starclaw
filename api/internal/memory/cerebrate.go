package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/yinhe/starclaw/internal/model"
	"github.com/yinhe/starclaw/internal/provider"
	"gorm.io/gorm"
)

// Cerebrate is the cross-session memory engine.
// It extracts memories from conversations and retrieves relevant ones for injection.
type Cerebrate struct {
	db               *gorm.DB
	providerRegistry *provider.Registry
}

// NewCerebrate creates a new Cerebrate memory engine.
func NewCerebrate(db *gorm.DB, pr *provider.Registry) *Cerebrate {
	return &Cerebrate{db: db, providerRegistry: pr}
}

// ─── Memory Retrieval (injection before conversation) ───

// Retrieve returns relevant memories for a user+agent, sorted by relevance to the query.
// It fetches: 1) instruct memories (agent + global), 2) global memories, 3) agent keyword-matched memories.
// Returns at most maxResults memories.
func (c *Cerebrate) Retrieve(userID, agentID, query string, maxResults int) ([]model.Memory, error) {
	if maxResults <= 0 {
		maxResults = 10
	}

	seen := map[string]bool{}
	var result []model.Memory

	addUnique := func(mems []model.Memory) {
		for _, m := range mems {
			if !seen[m.ID] {
				seen[m.ID] = true
				result = append(result, m)
			}
		}
	}

	// 1. Instruct memories: agent-specific + global (always injected, highest priority)
	var instructs []model.Memory
	c.db.Where("user_id = ? AND category = ? AND (agent_id = ? OR scope = ?)",
		userID, model.MemCatInstruct, agentID, model.MemScopeGlobal).
		Order("importance DESC, updated_at DESC").Limit(5).Find(&instructs)
	addUnique(instructs)

	// 2. Global non-instruct memories (cross-agent knowledge)
	var globals []model.Memory
	c.db.Where("user_id = ? AND scope = ? AND category != ?",
		userID, model.MemScopeGlobal, model.MemCatInstruct).
		Order("importance DESC, updated_at DESC").Limit(5).Find(&globals)
	addUnique(globals)

	// 3. Agent-specific keyword-matched memories
	remaining := maxResults - len(result)
	if remaining <= 0 {
		remaining = 3
	}

	keywords := extractKeywords(query)
	tx := c.db.Where("user_id = ? AND agent_id = ? AND category NOT IN ?",
		userID, agentID, []string{model.MemCatInstruct})

	if len(keywords) > 0 {
		var conditions []string
		var args []interface{}
		for _, kw := range keywords {
			if len(kw) < 2 {
				continue
			}
			conditions = append(conditions, "(`key` LIKE ? OR content LIKE ?)")
			args = append(args, "%"+kw+"%", "%"+kw+"%")
		}
		if len(conditions) > 0 {
			whereClause := strings.Join(conditions, " OR ")
			tx = tx.Where(whereClause, args...)
		}
	}

	var agentMems []model.Memory
	tx.Order("importance DESC, access_count DESC, updated_at DESC").
		Limit(remaining).Find(&agentMems)
	addUnique(agentMems)

	// Update access count for all retrieved memories
	if len(result) > 0 {
		var ids []string
		for _, m := range result {
			ids = append(ids, m.ID)
		}
		c.db.Model(&model.Memory{}).Where("id IN ?", ids).
			Updates(map[string]interface{}{
				"access_count":   gorm.Expr("access_count + 1"),
				"last_access_at": time.Now(),
			})
	}

	return result, nil
}

// BuildPromptInjection formats retrieved memories into a system prompt supplement.
func BuildPromptInjection(memories []model.Memory) string {
	if len(memories) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n\n<cerebrate_memory>\n")
	sb.WriteString("以下是你对这位用户的长期记忆，请在回答时参考这些信息：\n\n")

	// Group by category
	groups := map[string][]model.Memory{}
	for _, m := range memories {
		groups[m.Category] = append(groups[m.Category], m)
	}

	categoryLabels := map[string]string{
		model.MemCatInstruct:   "📌 用户指令（必须遵守）",
		model.MemCatPreference: "💡 用户偏好",
		model.MemCatFact:       "📋 用户信息",
		model.MemCatContext:    "🔄 近期上下文",
		model.MemCatSkill:      "🛠️ 经验记录",
	}

	// Print in priority order
	for _, cat := range []string{model.MemCatInstruct, model.MemCatFact, model.MemCatPreference, model.MemCatContext, model.MemCatSkill} {
		mems, ok := groups[cat]
		if !ok || len(mems) == 0 {
			continue
		}
		label := categoryLabels[cat]
		if label == "" {
			label = cat
		}
		sb.WriteString(label + ":\n")
		for _, m := range mems {
			sb.WriteString("- " + m.Content + "\n")
		}
		sb.WriteString("\n")
	}

	sb.WriteString("</cerebrate_memory>")
	return sb.String()
}

// ─── Memory Extraction (after conversation) ───

// extractionPrompt is the LLM prompt for extracting memories from conversation.
const extractionPrompt = `你是一个记忆提取助手。分析以下对话，提取值得长期记住的信息。

提取规则：
1. **instruct**: 用户明确要求你以后遵守的指令（如"以后都用中文回答"、"不要用 emoji"、"叫我XX"）
2. **preference**: 用户的偏好和习惯（如喜欢的编程语言、沟通风格、工作方式）
3. **fact**: 用户的事实信息（如姓名、职业、项目名称、技术栈）
4. **skill**: 对话中成功解决问题的方法（如"用 FFmpeg xfade 实现视频转场"）
5. **context**: 用户正在做的事情或近期目标（如"正在开发 StarClaw 项目"）

重要：
- 只提取有长期价值的信息，忽略一次性的问答
- 每条记忆要简洁（一句话）
- 如果对话没有值得记住的内容，返回空数组
- key 字段用英文短语标识（如 "preferred_language", "project_name"）

返回 JSON 数组，格式：
[{"key": "...", "content": "...", "category": "...", "importance": 0.0-1.0}]

只返回 JSON，不要其他文字。如果没有值得记住的内容，返回 []`

// ExtractAndStore extracts memories from a conversation and stores them.
// This should be called asynchronously after a conversation turn.
// conversationID is optional; pass "" if unavailable.
func (c *Cerebrate) ExtractAndStore(ctx context.Context, userID, agentID string, messages []provider.ChatMessage, conversationID ...string) {
	convID := ""
	if len(conversationID) > 0 {
		convID = conversationID[0]
	}
	if len(messages) < 2 {
		return
	}

	// Get a model provider for extraction (use the first available)
	p := c.getExtractionProvider(userID)
	if p == nil {
		log.Printf("[cerebrate] skipping memory extraction: no provider available (user=%s)", userID)
		return
	}

	// Build conversation summary for extraction (last N messages, max 3000 chars)
	convText := buildConversationText(messages, 3000)
	if len(convText) < 50 {
		return // too short to extract anything meaningful
	}

	extractMessages := []provider.ChatMessage{
		{Role: "system", Content: extractionPrompt},
		{Role: "user", Content: "对话内容：\n\n" + convText},
	}

	result, err := p.ChatSync(ctx, &provider.ChatRequest{
		Model:       "", // use provider default
		Messages:    extractMessages,
		Temperature: 0.1,
		MaxTokens:   1000,
	})
	if err != nil {
		log.Printf("[cerebrate] extraction LLM call failed: %v", err)
		return
	}

	// Parse extracted memories
	var extracted []struct {
		Key        string  `json:"key"`
		Content    string  `json:"content"`
		Category   string  `json:"category"`
		Importance float64 `json:"importance"`
	}

	content := strings.TrimSpace(result.Content)
	// Strip markdown code fences if present
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	if err := json.Unmarshal([]byte(content), &extracted); err != nil {
		log.Printf("[cerebrate] failed to parse extraction result: %v (content: %.200s)", err, content)
		return
	}

	if len(extracted) == 0 {
		return
	}

	// Validate categories
	validCats := map[string]bool{
		model.MemCatPreference: true, model.MemCatFact: true,
		model.MemCatContext: true, model.MemCatSkill: true,
		model.MemCatInstruct: true,
	}

	stored := 0
	for _, e := range extracted {
		if e.Key == "" || e.Content == "" {
			continue
		}
		if !validCats[e.Category] {
			e.Category = model.MemCatContext
		}
		if e.Importance <= 0 || e.Importance > 1 {
			e.Importance = 0.5
		}

		// Auto-detect scope: fact/preference/instruct are cross-agent (global), others are agent-specific
		scope := model.MemScopeAgent
		if e.Category == model.MemCatFact || e.Category == model.MemCatPreference || e.Category == model.MemCatInstruct {
			scope = model.MemScopeGlobal
		}

		// Upsert: update if same key+user+agent exists, otherwise create
		var existing model.Memory
		err := c.db.Where("user_id = ? AND agent_id = ? AND `key` = ?", userID, agentID, e.Key).First(&existing).Error
		if err == nil {
			// Update existing memory
			updates := map[string]interface{}{
				"content":    e.Content,
				"category":   e.Category,
				"importance": e.Importance,
				"source":     "auto_extract",
				"scope":      scope,
			}
			if convID != "" {
				updates["conversation_id"] = convID
			}
			c.db.Model(&existing).Updates(updates)
			log.Printf("[cerebrate] updated memory: %s = %s", e.Key, truncate(e.Content, 60))
		} else {
			// Create new memory
			mem := model.Memory{
				UserID:         userID,
				AgentID:        agentID,
				Key:            e.Key,
				Content:        e.Content,
				Category:       e.Category,
				Source:         "auto_extract",
				Scope:          scope,
				ConversationID: convID,
				Importance:     e.Importance,
			}
			c.db.Create(&mem)
			log.Printf("[cerebrate] new memory: [%s/%s] %s = %s", e.Category, scope, e.Key, truncate(e.Content, 60))
		}
		stored++
	}

	if stored > 0 {
		log.Printf("[cerebrate] extracted %d memories from conversation (user=%s)", stored, userID)
	}
}

// ─── Conversation Summary Memory ───

const summaryPrompt = `你是一个会话摘要助手。用1-2句话总结以下对话的核心内容，重点记录：
- 讨论了什么主题
- 达成了什么结论或完成了什么任务
- 有哪些待办事项

要求简洁精炼，中文回复，不超过100字。只返回摘要文本，不要其他内容。`

// GenerateSummary creates a conversation summary memory.
// Should be called after conversations with ≥ 5 user messages.
func (c *Cerebrate) GenerateSummary(ctx context.Context, userID, agentID, conversationID string, messages []provider.ChatMessage) {
	// Count user messages
	userMsgCount := 0
	for _, m := range messages {
		if m.Role == "user" {
			userMsgCount++
		}
	}
	if userMsgCount < 5 {
		return
	}

	p := c.getExtractionProvider(userID)
	if p == nil {
		return
	}

	convText := buildConversationText(messages, 2000)
	if len(convText) < 50 {
		return
	}

	result, err := p.ChatSync(ctx, &provider.ChatRequest{
		Model: "",
		Messages: []provider.ChatMessage{
			{Role: "system", Content: summaryPrompt},
			{Role: "user", Content: convText},
		},
		Temperature: 0.2,
		MaxTokens:   200,
	})
	if err != nil {
		log.Printf("[cerebrate] summary generation failed: %v", err)
		return
	}

	summary := strings.TrimSpace(result.Content)
	if len(summary) < 10 {
		return
	}

	// Enforce max 20 summaries per agent — delete oldest if over
	var count int64
	c.db.Model(&model.Memory{}).Where("user_id = ? AND agent_id = ? AND category = ?",
		userID, agentID, model.MemCatSummary).Count(&count)
	if count >= 20 {
		// Delete the oldest summary
		var oldest model.Memory
		c.db.Where("user_id = ? AND agent_id = ? AND category = ?",
			userID, agentID, model.MemCatSummary).
			Order("created_at ASC").First(&oldest)
		if oldest.ID != "" {
			c.db.Delete(&oldest)
		}
	}

	mem := model.Memory{
		UserID:         userID,
		AgentID:        agentID,
		Key:            "conv_summary_" + conversationID[:min(8, len(conversationID))],
		Content:        summary,
		Category:       model.MemCatSummary,
		Source:         "auto_extract",
		Scope:          model.MemScopeAgent,
		ConversationID: conversationID,
		Importance:     0.4,
	}
	c.db.Create(&mem)
	log.Printf("[cerebrate] saved summary for conv %s: %s", conversationID[:min(16, len(conversationID))], truncate(summary, 60))
}

// ─── Helpers ───

func (c *Cerebrate) getExtractionProvider(userID string) provider.ModelProvider {
	var cfg model.ModelConfig
	// 1. Prefer star-ai (no API key needed, always works in swarm mode)
	if err := c.db.Where("user_id = ? AND provider = ? AND is_enabled = ?",
		userID, "star-ai", true).First(&cfg).Error; err == nil {
		return provider.CreateFromConfig(c.providerRegistry, cfg)
	}
	// 2. User model with API key configured
	if err := c.db.Where("user_id = ? AND is_enabled = ? AND api_key != ''",
		userID, true).Order("created_at ASC").First(&cfg).Error; err == nil {
		return provider.CreateFromConfig(c.providerRegistry, cfg)
	}
	// 3. Any user model (e.g. ollama, no key needed)
	if err := c.db.Where("user_id = ? AND is_enabled = ?", userID, true).
		Order("created_at ASC").First(&cfg).Error; err == nil {
		return provider.CreateFromConfig(c.providerRegistry, cfg)
	}
	// 4. Platform model
	if err := c.db.Where("is_platform = ? AND is_enabled = ?", true, true).
		Order("created_at ASC").First(&cfg).Error; err == nil {
		log.Printf("[cerebrate] using platform model (%s) for user %s", cfg.Provider, userID)
		return provider.CreateFromConfig(c.providerRegistry, cfg)
	}
	log.Printf("[cerebrate] no extraction provider found for user %s", userID)
	return nil
}

func buildConversationText(messages []provider.ChatMessage, maxChars int) string {
	var sb strings.Builder
	// Take last messages, skip system prompt
	for _, m := range messages {
		if m.Role == "system" || m.Role == "tool" {
			continue
		}
		role := "用户"
		if m.Role == "assistant" {
			role = "助手"
		}
		line := fmt.Sprintf("%s: %s\n", role, m.Content)
		if sb.Len()+len(line) > maxChars {
			break
		}
		sb.WriteString(line)
	}
	return sb.String()
}

func extractKeywords(query string) []string {
	// Simple keyword extraction: split by common delimiters, filter short words
	replacer := strings.NewReplacer(
		"，", " ", "。", " ", "？", " ", "！", " ", "、", " ",
		",", " ", ".", " ", "?", " ", "!", " ",
		"的", " ", "了", " ", "是", " ", "在", " ", "我", " ",
		"你", " ", "吗", " ", "呢", " ", "啊", " ", "把", " ",
		"和", " ", "有", " ", "不", " ", "这", " ", "那", " ",
		"什么", " ", "怎么", " ", "如何", " ", "请", " ", "帮", " ",
	)
	cleaned := replacer.Replace(query)
	words := strings.Fields(cleaned)
	var keywords []string
	seen := map[string]bool{}
	for _, w := range words {
		w = strings.ToLower(strings.TrimSpace(w))
		if len(w) >= 2 && !seen[w] {
			keywords = append(keywords, w)
			seen[w] = true
		}
	}
	// Limit to top 8 keywords
	if len(keywords) > 8 {
		keywords = keywords[:8]
	}
	return keywords
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

// ─── CRUD for API ───

// ListMemories returns all memories for a user+agent, optionally filtered by category.
func (c *Cerebrate) ListMemories(userID, agentID, category string, page, pageSize int) ([]model.Memory, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	tx := c.db.Where("user_id = ?", userID)
	if agentID != "" {
		tx = tx.Where("agent_id = ?", agentID)
	}
	if category != "" {
		tx = tx.Where("category = ?", category)
	}

	var total int64
	tx.Model(&model.Memory{}).Count(&total)

	var memories []model.Memory
	tx.Order("importance DESC, updated_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&memories)

	return memories, total, nil
}

// DeleteMemory deletes a single memory by ID (must belong to user).
func (c *Cerebrate) DeleteMemory(userID, memoryID string) error {
	result := c.db.Where("id = ? AND user_id = ?", memoryID, userID).Delete(&model.Memory{})
	if result.RowsAffected == 0 {
		return fmt.Errorf("memory not found")
	}
	return nil
}

// ClearMemories deletes all memories for a user+agent.
func (c *Cerebrate) ClearMemories(userID, agentID string) (int64, error) {
	tx := c.db.Where("user_id = ?", userID)
	if agentID != "" {
		tx = tx.Where("agent_id = ?", agentID)
	}
	result := tx.Delete(&model.Memory{})
	return result.RowsAffected, result.Error
}

// CreateMemory creates a user-explicit memory.
func (c *Cerebrate) CreateMemory(userID, agentID, key, content, category string) (*model.Memory, error) {
	if key == "" || content == "" {
		return nil, fmt.Errorf("key and content are required")
	}
	validCats := map[string]bool{
		model.MemCatPreference: true, model.MemCatFact: true,
		model.MemCatContext: true, model.MemCatSkill: true,
		model.MemCatInstruct: true,
	}
	if !validCats[category] {
		category = model.MemCatFact
	}

	mem := &model.Memory{
		UserID:     userID,
		AgentID:    agentID,
		Key:        key,
		Content:    content,
		Category:   category,
		Source:     "user_explicit",
		Importance: 0.8,
	}
	if err := c.db.Create(mem).Error; err != nil {
		return nil, err
	}
	return mem, nil
}
