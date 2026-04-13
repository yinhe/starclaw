package agent

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/yinhe/starclaw/internal/provider"
)

// ════════════════════════════════════════════════════════════════
// Context Compression — Hermes-inspired
//
// When conversations grow too long for the context window, compress
// older messages into a concise summary while preserving recent context.
// Uses the LLM itself to produce high-fidelity summaries.
// ════════════════════════════════════════════════════════════════

const compressionPrompt = `你是一个上下文压缩助手。将以下对话历史压缩为简洁的摘要。

保留规则：
- 关键决策和结论
- 修改过的文件路径和代码片段
- 错误信息和解决方案
- 用户偏好和指令
- 任务进度和待办事项

丢弃规则：
- 冗长的工具输出
- 重复的问答
- 思考过程和中间推理

输出一段简洁的摘要（不超过500字），用事实性语言，不要叙事。`

// CompressContext compresses conversation history when it exceeds token limits.
// keepRecent: number of recent messages to preserve uncompressed.
// Returns compressed messages and a stats string.
func CompressContext(ctx context.Context, p provider.ModelProvider, model string, messages []provider.ChatMessage, keepRecent int) ([]provider.ChatMessage, string, error) {
	if keepRecent <= 0 {
		keepRecent = 6
	}
	if len(messages) <= keepRecent+2 {
		return messages, "", nil
	}

	// Split: find system prompt, old messages, recent messages
	var systemMsg *provider.ChatMessage
	var conversationMsgs []provider.ChatMessage

	for i, m := range messages {
		if i == 0 && m.Role == "system" {
			systemMsg = &messages[i]
		} else {
			conversationMsgs = append(conversationMsgs, m)
		}
	}

	if len(conversationMsgs) <= keepRecent {
		return messages, "", nil
	}

	oldMsgs := conversationMsgs[:len(conversationMsgs)-keepRecent]
	recentMsgs := conversationMsgs[len(conversationMsgs)-keepRecent:]

	// Build text representation of old messages
	var sb strings.Builder
	for _, m := range oldMsgs {
		content := m.Content
		if m.Role == "tool" {
			if len(content) > 200 {
				content = content[:200] + "..."
			}
			sb.WriteString(fmt.Sprintf("[工具结果]: %s\n", content))
		} else {
			if len(content) > 500 {
				content = content[:500] + "..."
			}
			role := "用户"
			if m.Role == "assistant" {
				role = "助手"
			}
			sb.WriteString(fmt.Sprintf("[%s]: %s\n", role, content))
		}
	}

	// Ask LLM to compress
	result, err := p.ChatSync(ctx, &provider.ChatRequest{
		Model: model,
		Messages: []provider.ChatMessage{
			{Role: "system", Content: compressionPrompt},
			{Role: "user", Content: sb.String()},
		},
		MaxTokens:   1024,
		Temperature: 0.2,
	})
	if err != nil {
		// Fallback: simple truncation
		log.Printf("[compress] LLM compression failed, using truncation: %v", err)
		return truncateMessages(messages, systemMsg, recentMsgs, keepRecent), "(truncation fallback)", nil
	}

	summary := strings.TrimSpace(result.Content)
	if len(summary) < 20 {
		return truncateMessages(messages, systemMsg, recentMsgs, keepRecent), "(truncation fallback)", nil
	}

	// Build compressed message list
	var compressed []provider.ChatMessage
	if systemMsg != nil {
		compressed = append(compressed, *systemMsg)
	}
	compressed = append(compressed,
		provider.ChatMessage{
			Role:    "user",
			Content: fmt.Sprintf("[之前的对话摘要]\n%s", summary),
		},
		provider.ChatMessage{
			Role:    "assistant",
			Content: "好的，我已了解之前的对话上下文。请继续。",
		},
	)
	compressed = append(compressed, recentMsgs...)

	oldTokens := estimateTokenCount(oldMsgs)
	newTokens := estimateTokenCount(compressed[:len(compressed)-len(recentMsgs)])
	stats := fmt.Sprintf("压缩 %d 条消息 → 摘要 + %d 条最近消息 (节省 ~%d tokens)",
		len(oldMsgs), keepRecent, oldTokens-newTokens)

	log.Printf("[compress] %s", stats)
	return compressed, stats, nil
}

// truncateMessages is a simple fallback when LLM compression fails.
func truncateMessages(messages []provider.ChatMessage, systemMsg *provider.ChatMessage, recentMsgs []provider.ChatMessage, keepRecent int) []provider.ChatMessage {
	truncated := len(messages) - keepRecent
	var result []provider.ChatMessage
	if systemMsg != nil {
		result = append(result, *systemMsg)
	}
	result = append(result,
		provider.ChatMessage{
			Role:    "user",
			Content: fmt.Sprintf("[注意: 之前 %d 条消息已被截断以节省上下文空间]", truncated),
		},
		provider.ChatMessage{
			Role:    "assistant",
			Content: "好的，我将继续处理当前的上下文。",
		},
	)
	result = append(result, recentMsgs...)
	return result
}

// estimateTokenCount estimates token count (rough: 1 token ≈ 2 CJK chars or 4 ASCII chars)
func estimateTokenCount(msgs []provider.ChatMessage) int {
	total := 0
	for _, m := range msgs {
		// CJK-aware: count runes, assume ~2 chars per token for mixed content
		total += len([]rune(m.Content))/2 + 4
	}
	return total
}

// ShouldCompress checks if a conversation is approaching context limits.
// maxTokens is the model's context window size.
func ShouldCompress(messages []provider.ChatMessage, maxTokens int) bool {
	if maxTokens <= 0 {
		maxTokens = 32000
	}
	return estimateTokenCount(messages) > maxTokens*3/4
}
