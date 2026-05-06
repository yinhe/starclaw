package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/yinhe/starclaw/internal/model"
	"github.com/yinhe/starclaw/internal/provider"
	"github.com/yinhe/starclaw/internal/rag"
	"github.com/yinhe/starclaw/internal/security"
	"gorm.io/gorm"
)

// Cerebrate is the cross-session memory engine.
// It extracts memories from conversations and retrieves relevant ones for injection.
type Cerebrate struct {
	db               *gorm.DB
	providerRegistry *provider.Registry
	// O4: callback to send toast notifications to user after memory extraction.
	// Set via SetNotifyFunc to avoid import cycle with ws package.
	notifyFunc func(userID string, event string, data interface{})
	// P4: embedding provider for vector semantic recall
	embedder rag.EmbeddingProvider
}

// NewCerebrate creates a new Cerebrate memory engine.
func NewCerebrate(db *gorm.DB, pr *provider.Registry) *Cerebrate {
	return &Cerebrate{db: db, providerRegistry: pr}
}

// SetNotifyFunc sets the callback used to push real-time notifications to the user.
// Typically wired to ws.GetHub().SendToUser.
func (c *Cerebrate) SetNotifyFunc(fn func(userID string, event string, data interface{})) {
	c.notifyFunc = fn
}

// SetEmbedder sets the embedding provider for P4 vector semantic recall.
func (c *Cerebrate) SetEmbedder(e rag.EmbeddingProvider) {
	c.embedder = e
}

func defaultPalaceFields(agentID, category, key, conversationID, scope string) (string, string, string) {
	token := normalizePalaceToken(key)
	if token == "" {
		token = "memory"
	}

	room := model.MemRoomTask
	anchor := "task/" + token

	switch category {
	case model.MemCatSkill:
		room = model.MemRoomSkill
		anchor = "skill/" + token
	case model.MemCatSummary:
		room = model.MemRoomTask
		if conversationID != "" {
			anchor = "task/conversation_" + conversationID[:min(8, len(conversationID))]
		}
	case model.MemCatContext:
		room = model.MemRoomTask
		if conversationID != "" {
			anchor = "task/conversation_" + conversationID[:min(8, len(conversationID))]
		}
	case model.MemCatInstruct, model.MemCatPreference, model.MemCatFact:
		switch {
		case strings.Contains(token, "project"):
			room = model.MemRoomProject
			anchor = "project/" + token
		case strings.Contains(token, "task") || strings.Contains(token, "todo") || strings.Contains(token, "goal") || strings.Contains(token, "deploy") || strings.Contains(token, "release"):
			room = model.MemRoomTask
			anchor = "task/" + token
		default:
			room = model.MemRoomUser
			anchor = "user/" + token
		}
	}

	pathRoot := "user/default"
	if scope == model.MemScopeAgent && agentID != "" {
		pathRoot = "agent/" + normalizePalaceToken(agentID)
	}

	return room, anchor, pathRoot + " > " + anchor
}

func normalizePalaceToken(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return ""
	}
	replacer := strings.NewReplacer(
		" ", "_",
		"-", "_",
		"/", "_",
		"\\", "_",
		".", "_",
		":", "_",
		"，", "_",
		"。", "_",
		"（", "_",
		"）", "_",
		"(", "_",
		")", "_",
	)
	s = replacer.Replace(s)
	for strings.Contains(s, "__") {
		s = strings.ReplaceAll(s, "__", "_")
	}
	return strings.Trim(s, "_")
}

func palaceTermsFromQuery(query string) []string {
	seen := map[string]bool{}
	var terms []string
	add := func(term string) {
		term = strings.TrimSpace(strings.ToLower(term))
		if len(term) < 2 || seen[term] {
			return
		}
		seen[term] = true
		terms = append(terms, term)
	}

	normalized := normalizePalaceToken(query)
	add(normalized)
	for _, part := range strings.Split(normalized, "_") {
		add(part)
	}
	for _, kw := range extractKeywords(query) {
		add(kw)
		add(normalizePalaceToken(kw))
	}
	return terms
}

func scorePalaceMemory(mem model.Memory, query string, terms []string) float64 {
	queryToken := normalizePalaceToken(query)
	derivedRoom, derivedAnchor, derivedPath := palaceFieldsForMemory(mem)
	room := strings.ToLower(derivedRoom)
	anchor := strings.ToLower(derivedAnchor)
	path := strings.ToLower(derivedPath)
	key := strings.ToLower(mem.Key)
	content := strings.ToLower(mem.Content)
	category := strings.ToLower(mem.Category)

	score := mem.Importance * 0.4
	for _, term := range terms {
		if term == "" {
			continue
		}
		if anchor == term || strings.HasSuffix(anchor, "/"+term) {
			score += 6
		}
		if strings.Contains(anchor, term) {
			score += 3.5
		}
		if strings.Contains(path, term) {
			score += 2.5
		}
		if strings.Contains(room, term) || strings.Contains(category, term) {
			score += 1.25
		}
		if strings.Contains(key, term) {
			score += 1.75
		}
		if strings.Contains(content, term) {
			score += 1.1
		}
	}
	if queryToken != "" {
		if anchor == queryToken || strings.HasSuffix(anchor, "/"+queryToken) {
			score += 6
		}
		if strings.Contains(path, queryToken) {
			score += 2
		}
	}
	if mem.AccessCount > 0 {
		score += float64(mem.AccessCount) * 0.05
	}
	return score
}

func (c *Cerebrate) palaceRecall(userID, agentID, query string, topK int) []model.Memory {
	if topK <= 0 || strings.TrimSpace(query) == "" {
		return nil
	}

	terms := palaceTermsFromQuery(query)
	if len(terms) == 0 {
		return nil
	}

	var candidates []model.Memory
	c.db.Where("user_id = ? AND category != ? AND (agent_id = ? OR scope = ?)",
		userID, model.MemCatInstruct, agentID, model.MemScopeGlobal).
		Order("updated_at DESC").
		Limit(200).
		Find(&candidates)

	type scored struct {
		mem   model.Memory
		score float64
	}
	var ranked []scored
	for _, mem := range candidates {
		score := scorePalaceMemory(mem, query, terms)
		if score < 1.5 {
			continue
		}
		ranked = append(ranked, scored{mem: mem, score: score})
	}

	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].score == ranked[j].score {
			if ranked[i].mem.Importance == ranked[j].mem.Importance {
				return ranked[i].mem.UpdatedAt.After(ranked[j].mem.UpdatedAt)
			}
			return ranked[i].mem.Importance > ranked[j].mem.Importance
		}
		return ranked[i].score > ranked[j].score
	})

	if len(ranked) > topK {
		ranked = ranked[:topK]
	}
	out := make([]model.Memory, 0, len(ranked))
	for _, item := range ranked {
		out = append(out, item.mem)
	}
	return out
}

func palaceFieldsForMemory(mem model.Memory) (string, string, string) {
	room := mem.Room
	anchor := mem.Anchor
	path := mem.Path
	if room != "" && anchor != "" && path != "" {
		return room, anchor, path
	}
	return defaultPalaceFields(mem.AgentID, mem.Category, mem.Key, mem.ConversationID, mem.Scope)
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

	// 2. Memory Palace recall (same room / anchor / path gets priority)
	remaining := maxResults - len(result)
	if remaining <= 0 {
		remaining = 3
	}
	palaceMems := c.palaceRecall(userID, agentID, query, remaining)
	addUnique(palaceMems)

	// 3. P4: Vector semantic recall (if embedder available)
	remaining = maxResults - len(result)
	if remaining <= 0 {
		remaining = 3
	}

	if c.embedder != nil && len(query) >= 5 {
		semanticMems := c.vectorRecall(userID, agentID, query, remaining)
		addUnique(semanticMems)
	}

	// 4. Global non-instruct memories (cross-agent knowledge)
	remaining = maxResults - len(result)
	if remaining > 0 {
		var globals []model.Memory
		c.db.Where("user_id = ? AND scope = ? AND category != ?",
			userID, model.MemScopeGlobal, model.MemCatInstruct).
			Order("importance DESC, updated_at DESC").Limit(remaining).Find(&globals)
		addUnique(globals)
	}

	// 5. Fallback: Agent-specific keyword-matched memories
	remaining = maxResults - len(result)
	if remaining > 0 {
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
				conditions = append(conditions, "(`key` LIKE ? OR content LIKE ? OR room LIKE ? OR anchor LIKE ? OR path LIKE ?)")
				args = append(args, "%"+kw+"%", "%"+kw+"%", "%"+kw+"%", "%"+kw+"%", "%"+kw+"%")
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
	}

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

// ─── P4: Vector Semantic Recall ───

// vectorRecall finds memories semantically similar to the query using embeddings.
func (c *Cerebrate) vectorRecall(userID, agentID, query string, topK int) []model.Memory {
	if c.embedder == nil {
		return nil
	}

	// Embed the query
	embeddings, err := c.embedder.Embed(context.Background(), []string{query})
	if err != nil || len(embeddings) == 0 || len(embeddings[0]) == 0 {
		return nil
	}
	queryVec := embeddings[0]

	// Load all memories with embeddings for this user+agent
	var candidates []model.Memory
	c.db.Where("user_id = ? AND (agent_id = ? OR scope = ?) AND embedding IS NOT NULL AND LENGTH(embedding) > 0",
		userID, agentID, model.MemScopeGlobal).
		Find(&candidates)

	if len(candidates) == 0 {
		return nil
	}

	// Score by cosine similarity
	type scored struct {
		mem   model.Memory
		score float32
	}
	var results []scored

	for _, m := range candidates {
		vec := rag.DeserializeVector(m.Embedding)
		if len(vec) == 0 {
			continue
		}
		sim := rag.CosineSimilarity(queryVec, vec)
		if sim > 0.35 { // similarity threshold
			results = append(results, scored{mem: m, score: sim})
		}
	}

	// Sort by similarity descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	if len(results) > topK {
		results = results[:topK]
	}

	var out []model.Memory
	for _, r := range results {
		out = append(out, r.mem)
	}
	return out
}

// embedMemory generates and stores an embedding for a memory's content.
func (c *Cerebrate) embedMemory(mem *model.Memory) {
	if c.embedder == nil || mem == nil || len(mem.Content) < 5 {
		return
	}

	text := mem.Key + ": " + mem.Content
	embeddings, err := c.embedder.Embed(context.Background(), []string{text})
	if err != nil || len(embeddings) == 0 || len(embeddings[0]) == 0 {
		return
	}

	mem.Embedding = rag.SerializeVector(embeddings[0])
	c.db.Model(mem).Update("embedding", mem.Embedding)
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

// ─── O3: Instant Memory ("记住这个" / "remember this") ───

// instantMemoryPatterns are phrases that trigger immediate memory storage.
var instantMemoryPatterns = []string{
	"记住这个", "记住这一点", "请记住", "帮我记住", "记下来",
	"remember this", "remember that", "keep this in mind", "note this",
}

// CheckInstantMemory checks if the user message contains an instant memory trigger.
// If so, it immediately stores the content as a high-importance memory.
// Returns true if a memory was stored.
func (c *Cerebrate) CheckInstantMemory(userID, agentID, message string) bool {
	lower := strings.ToLower(message)
	triggered := false
	for _, pat := range instantMemoryPatterns {
		if strings.Contains(lower, pat) {
			triggered = true
			break
		}
	}
	if !triggered {
		return false
	}

	// Extract the actual content to remember (remove the trigger phrase)
	content := message
	for _, pat := range instantMemoryPatterns {
		content = strings.ReplaceAll(content, pat, "")
	}
	content = strings.TrimSpace(content)
	content = strings.Trim(content, "，。：:！!、,.")
	content = strings.TrimSpace(content)
	if len(content) < 3 {
		return false
	}

	// Generate a key from content (first 30 chars, snake_case-ish)
	key := fmt.Sprintf("instant_%d", time.Now().UnixMilli()%100000)
	room, anchor, path := defaultPalaceFields(agentID, model.MemCatFact, key, "", model.MemScopeGlobal)

	mem := model.Memory{
		UserID:     userID,
		AgentID:    agentID,
		Key:        key,
		Content:    content,
		Category:   model.MemCatFact,
		Source:     "instant",
		Scope:      model.MemScopeGlobal,
		Room:       room,
		Anchor:     anchor,
		Path:       path,
		Importance: 0.9,
	}
	c.db.Create(&mem)
	log.Printf("[cerebrate] instant memory stored: %s", truncate(content, 60))
	return true
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
		log.Printf("[cerebrate] skipping extraction: only %d messages (need >=2), user=%s agent=%s", len(messages), userID, agentID)
		return
	}

	// Get a model provider for extraction (prefer lightweight models — O1)
	p, extractModel := c.getExtractionProvider(userID)
	if p == nil {
		log.Printf("[cerebrate] skipping memory extraction: no provider available (user=%s)", userID)
		return
	}
	// Fallback: if extractModel is empty, use provider's first available model
	if extractModel == "" {
		if models := p.Models(); len(models) > 0 {
			extractModel = models[0]
			log.Printf("[cerebrate] using provider default model %q for extraction", extractModel)
		}
	}

	// Build conversation summary for extraction (last N messages, max 3000 chars)
	convText := buildConversationText(messages, 3000)
	if len(convText) < 50 {
		log.Printf("[cerebrate] skipping extraction: convText too short (%d bytes), user=%s agent=%s", len(convText), userID, agentID)
		return
	}
	log.Printf("[cerebrate] starting extraction: %d messages, %d bytes convText, model=%s, user=%s agent=%s", len(messages), len(convText), extractModel, userID, agentID)

	extractMessages := []provider.ChatMessage{
		{Role: "system", Content: extractionPrompt},
		{Role: "user", Content: "对话内容：\n\n" + convText},
	}

	// O2: Retry extraction up to 2 times with 30s delay on failure.
	// Use streaming Chat instead of ChatSync — the Ed25519 registry provider
	// (used when all API keys are corrupted) only works with streaming.
	var extractedContent string
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		attemptCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		chatReq := &provider.ChatRequest{
			Model:       extractModel,
			Messages:    extractMessages,
			Temperature: 0.1,
			MaxTokens:   1000,
			Stream:      true,
		}
		ch, chatErr := p.Chat(attemptCtx, chatReq)
		if chatErr != nil {
			cancel()
			err = chatErr
			log.Printf("[cerebrate] extraction LLM call failed (attempt %d/3): %v", attempt+1, err)
			if attempt < 2 {
				select {
				case <-ctx.Done():
					return
				case <-time.After(30 * time.Second):
				}
			}
			continue
		}
		// Collect streaming response
		var sb strings.Builder
		for chunk := range ch {
			if chunk.Error != "" {
				err = fmt.Errorf("%s", chunk.Error)
				break
			}
			sb.WriteString(chunk.Content)
		}
		cancel()
		if err == nil {
			extractedContent = sb.String()
			break
		}
		log.Printf("[cerebrate] extraction LLM stream error (attempt %d/3): %v", attempt+1, err)
		if attempt < 2 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(30 * time.Second):
			}
		}
	}
	if err != nil {
		log.Printf("[cerebrate] extraction LLM call failed after 3 attempts: %v", err)
		return
	}

	// Parse extracted memories
	var extracted []struct {
		Key        string  `json:"key"`
		Content    string  `json:"content"`
		Category   string  `json:"category"`
		Importance float64 `json:"importance"`
	}

	content := strings.TrimSpace(extractedContent)
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
		room, anchor, path := defaultPalaceFields(agentID, e.Category, e.Key, convID, scope)

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
				"room":       room,
				"anchor":     anchor,
				"path":       path,
			}
			if convID != "" {
				updates["conversation_id"] = convID
			}
			c.db.Model(&existing).Updates(updates)
			// P4: re-embed on content change
			existing.Content = e.Content
			existing.Key = e.Key
			go c.embedMemory(&existing)
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
				Room:           room,
				Anchor:         anchor,
				Path:           path,
				ConversationID: convID,
				Importance:     e.Importance,
				Tags:           "[]",
				LastAccessAt:   time.Now(),
			}
			c.db.Create(&mem)
			// P4: embed new memory
			go c.embedMemory(&mem)
			log.Printf("[cerebrate] new memory: [%s/%s] %s = %s", e.Category, scope, e.Key, truncate(e.Content, 60))
		}
		stored++
	}

	if stored > 0 {
		log.Printf("[cerebrate] extracted %d memories from conversation (user=%s)", stored, userID)
		// O4: Push toast notification via WebSocket
		if c.notifyFunc != nil {
			c.notifyFunc(userID, "memory_extracted", map[string]interface{}{
				"count":   stored,
				"message": fmt.Sprintf("🧠 已从对话中提取 %d 条新记忆", stored),
			})
		}
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

	p, summaryModel := c.getExtractionProvider(userID)
	if p == nil {
		return
	}

	convText := buildConversationText(messages, 2000)
	if len(convText) < 50 {
		return
	}

	summaryCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	ch, err := p.Chat(summaryCtx, &provider.ChatRequest{
		Model: summaryModel,
		Messages: []provider.ChatMessage{
			{Role: "system", Content: summaryPrompt},
			{Role: "user", Content: convText},
		},
		Temperature: 0.2,
		MaxTokens:   200,
		Stream:      true,
	})
	if err != nil {
		log.Printf("[cerebrate] summary generation failed: %v", err)
		return
	}
	var sb strings.Builder
	for chunk := range ch {
		sb.WriteString(chunk.Content)
	}

	summary := strings.TrimSpace(sb.String())
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
	room, anchor, path := defaultPalaceFields(agentID, model.MemCatSummary, "conv_summary_"+conversationID[:min(8, len(conversationID))], conversationID, model.MemScopeAgent)

	mem := model.Memory{
		UserID:         userID,
		AgentID:        agentID,
		Key:            "conv_summary_" + conversationID[:min(8, len(conversationID))],
		Content:        summary,
		Category:       model.MemCatSummary,
		Source:         "auto_extract",
		Scope:          model.MemScopeAgent,
		Room:           room,
		Anchor:         anchor,
		Path:           path,
		ConversationID: conversationID,
		Importance:     0.4,
	}
	c.db.Create(&mem)
	log.Printf("[cerebrate] saved summary for conv %s: %s", conversationID[:min(16, len(conversationID))], truncate(summary, 60))
}

// ─── Helpers ───

func (c *Cerebrate) getExtractionProvider(userID string) (provider.ModelProvider, string) {
	var cfg model.ModelConfig
	// O1: Prefer lightweight models for extraction to save compute.
	// Priority: star-ai → ollama (local, free) → cheapest user model → platform model.
	// Returns (provider, modelOverride) — modelOverride selects a cheap model when available.

	// 1. Prefer star-ai with a usable API key.
	//    Skip configs whose encrypted key decrypts to empty (corrupted/rotated master key).
	//    Pass nil registry to avoid Ed25519 signed transport which hangs on non-streaming ChatSync.
	var starConfigs []model.ModelConfig
	if err := c.db.Where("user_id = ? AND provider = ? AND is_enabled = ? AND api_key != ''",
		userID, "star-ai", true).Find(&starConfigs).Error; err == nil {
		for _, sc := range starConfigs {
			decKey := security.DecryptAPIKey(sc.APIKey)
			if decKey != "" {
				sc.APIKey = decKey // pre-decrypted so CreateFromConfig won't double-decrypt
				return provider.CreateFromConfig(nil, sc), extractionModelOverride("star-ai")
			}
		}
	}
	// 2. User model with API key configured (use lightweight model override)
	//    This covers openai, qwen, deepseek, etc. — reliable cloud providers.
	if err := c.db.Where("user_id = ? AND is_enabled = ? AND api_key != '' AND provider NOT IN ('star-ai','ollama','fal','volcengine-tts')",
		userID, true).Order("created_at ASC").First(&cfg).Error; err == nil {
		log.Printf("[cerebrate] using %s for extraction, user %s", cfg.Provider, userID)
		return provider.CreateFromConfig(c.providerRegistry, cfg), extractionModelOverride(cfg.Provider)
	}
	// 3. Registry star-ai provider (Ed25519 signed transport — streaming only).
	//    Used when all API keys are corrupted/missing but registry is available.
	if c.providerRegistry != nil {
		if rp, ok := c.providerRegistry.Get("star-ai"); ok {
			log.Printf("[cerebrate] using registry star-ai provider (Ed25519) for extraction, user %s", userID)
			return rp, extractionModelOverride("star-ai")
		}
	}
	// 4. Local ollama (free, no API cost — only works when ollama is reachable)
	if err := c.db.Where("user_id = ? AND provider = ? AND is_enabled = ?",
		userID, "ollama", true).First(&cfg).Error; err == nil {
		log.Printf("[cerebrate] using ollama (free) for extraction, user %s", userID)
		return provider.CreateFromConfig(c.providerRegistry, cfg), ""
	}
	// 5. Any user model (e.g. lm-studio, no key needed)
	if err := c.db.Where("user_id = ? AND is_enabled = ? AND provider NOT IN ('star-ai','fal','volcengine-tts')",
		userID, true).Order("created_at ASC").First(&cfg).Error; err == nil {
		return provider.CreateFromConfig(c.providerRegistry, cfg), extractionModelOverride(cfg.Provider)
	}
	// 6. Platform model
	if err := c.db.Where("is_platform = ? AND is_enabled = ?", true, true).
		Order("created_at ASC").First(&cfg).Error; err == nil {
		log.Printf("[cerebrate] using platform model (%s) for user %s", cfg.Provider, userID)
		return provider.CreateFromConfig(c.providerRegistry, cfg), extractionModelOverride(cfg.Provider)
	}
	log.Printf("[cerebrate] no extraction provider found for user %s", userID)
	return nil, ""
}

// extractionModelOverride returns a lighter model name for extraction tasks.
// Falls back to empty string (provider default) if no lightweight option is available.
func extractionModelOverride(providerName string) string {
	switch providerName {
	case "openai":
		return "gpt-4o-mini"
	case "openrouter":
		return "openai/gpt-4o-mini"
	case "anthropic":
		return "claude-3-haiku-20240307"
	case "star-ai", "starai":
		return "qwen3.5-flash"
	case "qwen":
		return "qwen-plus"
	case "deepseek":
		return "deepseek-chat"
	default:
		return "" // use provider default
	}
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
	scope := model.MemScopeAgent
	if category == model.MemCatFact || category == model.MemCatPreference || category == model.MemCatInstruct {
		scope = model.MemScopeGlobal
	}
	room, anchor, path := defaultPalaceFields(agentID, category, key, "", scope)

	mem := &model.Memory{
		UserID:     userID,
		AgentID:    agentID,
		Key:        key,
		Content:    content,
		Category:   category,
		Source:     "user_explicit",
		Scope:      scope,
		Room:       room,
		Anchor:     anchor,
		Path:       path,
		Importance: 0.8,
	}
	if err := c.db.Create(mem).Error; err != nil {
		return nil, err
	}
	return mem, nil
}
