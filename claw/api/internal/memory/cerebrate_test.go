package memory

import (
	"testing"

	"github.com/yinhe/starclaw/internal/model"
	"github.com/yinhe/starclaw/internal/provider"
)

func TestExtractKeywords(t *testing.T) {
	tests := []struct {
		query string
		min   int // minimum expected keywords
	}{
		{"帮我写一个 Python 爬虫", 2},       // "帮" and "我" are stopwords
		{"FFmpeg 视频合并 crossfade", 3},   // technical terms
		{"Hello world program in Go", 3}, // English
		{"", 0},                          // empty
		{"的了是", 0},                       // all stopwords
	}

	for _, tt := range tests {
		kw := extractKeywords(tt.query)
		if len(kw) < tt.min {
			t.Errorf("extractKeywords(%q): got %d keywords %v, want at least %d", tt.query, len(kw), kw, tt.min)
		}
	}
}

func TestExtractKeywords_MaxLimit(t *testing.T) {
	long := "a b c d e f g h i j k l m n o p q r s t u v w x y z"
	kw := extractKeywords(long)
	if len(kw) > 8 {
		t.Errorf("extractKeywords should limit to 8, got %d", len(kw))
	}
}

func TestBuildPromptInjection_Empty(t *testing.T) {
	result := BuildPromptInjection(nil)
	if result != "" {
		t.Errorf("expected empty string for nil memories, got %q", result)
	}

	result = BuildPromptInjection([]model.Memory{})
	if result != "" {
		t.Errorf("expected empty string for empty memories, got %q", result)
	}
}

func TestBuildPromptInjection_WithMemories(t *testing.T) {
	memories := []model.Memory{
		{Category: model.MemCatInstruct, Content: "请用中文回答"},
		{Category: model.MemCatFact, Content: "用户是后端工程师"},
		{Category: model.MemCatPreference, Content: "喜欢简洁的代码风格"},
		{Category: model.MemCatSkill, Content: "FFmpeg xfade 可以实现视频转场"},
	}

	result := BuildPromptInjection(memories)

	if result == "" {
		t.Fatal("expected non-empty injection")
	}

	// Should contain cerebrate tags
	if !contains(result, "<cerebrate_memory>") {
		t.Error("missing <cerebrate_memory> tag")
	}
	if !contains(result, "</cerebrate_memory>") {
		t.Error("missing </cerebrate_memory> tag")
	}

	// Should contain all memory contents
	for _, m := range memories {
		if !contains(result, m.Content) {
			t.Errorf("missing memory content: %s", m.Content)
		}
	}

	// Instruct should appear before other categories
	instructIdx := indexOf(result, "用户指令")
	factIdx := indexOf(result, "用户信息")
	if instructIdx > factIdx {
		t.Error("instruct should appear before fact")
	}
}

func TestBuildConversationText(t *testing.T) {
	messages := []provider.ChatMessage{
		{Role: "system", Content: "You are helpful."},
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there!"},
		{Role: "user", Content: "How are you?"},
	}

	text := buildConversationText(messages, 3000)

	// Should skip system messages
	if contains(text, "You are helpful") {
		t.Error("should not include system messages")
	}

	if !contains(text, "Hello") || !contains(text, "Hi there") {
		t.Error("should include user and assistant messages")
	}
}

func TestBuildConversationText_MaxChars(t *testing.T) {
	messages := []provider.ChatMessage{
		{Role: "user", Content: "Short message"},
		{Role: "assistant", Content: "This is a very long response that should be truncated when we set a very small max chars limit"},
	}

	text := buildConversationText(messages, 50)

	// Should be truncated
	if len(text) > 100 { // some overhead for role labels
		t.Errorf("text too long: %d chars", len(text))
	}
}

func TestTruncate(t *testing.T) {
	if truncate("hello", 10) != "hello" {
		t.Error("short string should not be truncated")
	}
	result := truncate("hello world", 5)
	if result != "hello..." {
		t.Errorf("got %q, want %q", result, "hello...")
	}
}

func TestTruncate_Unicode(t *testing.T) {
	result := truncate("你好世界测试", 3)
	if result != "你好世..." {
		t.Errorf("got %q, want %q", result, "你好世...")
	}
}

// helpers
func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && indexOfStr(s, substr) >= 0
}

func indexOf(s, substr string) int {
	return indexOfStr(s, substr)
}

func indexOfStr(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
