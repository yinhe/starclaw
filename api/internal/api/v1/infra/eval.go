package infra

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw/internal/agent"
	"github.com/yinhe/starclaw/internal/model"
	"github.com/yinhe/starclaw/internal/provider"
	"github.com/yinhe/starclaw/internal/tool"
	"gorm.io/gorm"
)

type EvalHandler struct {
	db               *gorm.DB
	providerRegistry *provider.Registry
	toolRegistry     *tool.Registry
}

func NewEvalHandler(db *gorm.DB, pr *provider.Registry, tr *tool.Registry) *EvalHandler {
	return &EvalHandler{db: db, providerRegistry: pr, toolRegistry: tr}
}

// ListTestCases returns test cases for a specific agent
func (h *EvalHandler) ListTestCases(c *gin.Context) {
	userID := c.GetString("user_id")
	agentID := c.Query("agent_id")

	q := h.db.Where("user_id = ?", userID)
	if agentID != "" {
		q = q.Where("agent_id = ?", agentID)
	}
	var cases []model.AgentTestCase
	q.Order("created_at DESC").Find(&cases)
	c.JSON(http.StatusOK, gin.H{"test_cases": cases})
}

// CreateTestCase creates a new test case
func (h *EvalHandler) CreateTestCase(c *gin.Context) {
	userID := c.GetString("user_id")
	var req struct {
		AgentID        string `json:"agent_id" binding:"required"`
		Name           string `json:"name" binding:"required"`
		Input          string `json:"input" binding:"required"`
		ExpectedOutput string `json:"expected_output"`
		Tags           string `json:"tags"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tc := model.AgentTestCase{
		UserID:         userID,
		AgentID:        req.AgentID,
		Name:           req.Name,
		Input:          req.Input,
		ExpectedOutput: req.ExpectedOutput,
		Tags:           req.Tags,
	}
	if err := h.db.Create(&tc).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create test case"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"test_case": tc})
}

// DeleteTestCase removes a test case
func (h *EvalHandler) DeleteTestCase(c *gin.Context) {
	userID := c.GetString("user_id")
	id := c.Param("id")
	result := h.db.Where("id = ? AND user_id = ?", id, userID).Delete(&model.AgentTestCase{})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "test case not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// RunTestCase executes a single test case against the agent
func (h *EvalHandler) RunTestCase(c *gin.Context) {
	userID := c.GetString("user_id")
	id := c.Param("id")

	var tc model.AgentTestCase
	if err := h.db.Where("id = ? AND user_id = ?", id, userID).First(&tc).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "test case not found"})
		return
	}

	var ag model.Agent
	if err := h.db.Where("id = ?", tc.AgentID).First(&ag).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
		return
	}

	// Resolve model provider
	var modelCfg model.ModelConfig
	if err := h.db.Where("id = ?", ag.ModelID).First(&modelCfg).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "agent model not configured"})
		return
	}

	p := h.getProvider(modelCfg)

	// Build messages
	messages := []provider.ChatMessage{
		{Role: "system", Content: ag.SystemPrompt},
		{Role: "user", Content: tc.Input},
	}

	var toolNames []string
	if ag.Tools != "" {
		toolNames = strings.Split(ag.Tools, ",")
	}

	runtime := agent.NewRuntime(p, h.toolRegistry)
	start := time.Now()

	result, err := runtime.Run(c.Request.Context(), &agent.RunRequest{
		Model:       modelCfg.ModelName,
		Messages:    messages,
		Tools:       toolNames,
		Temperature: 0.3,
		MaxTokens:   1024,
	})

	duration := time.Since(start).Milliseconds()

	run := model.AgentTestRun{
		TestCaseID: tc.ID,
		AgentID:    tc.AgentID,
		UserID:     userID,
		DurationMs: duration,
	}

	if err != nil {
		run.Status = "error"
		run.ErrorMsg = err.Error()
	} else {
		run.ActualOutput = result.Content
		if result.Usage != nil {
			run.TokensUsed = result.Usage.TotalTokens
		}

		// Simple scoring: check if expected output keywords appear in actual output
		if tc.ExpectedOutput != "" {
			run.Score = computeScore(tc.ExpectedOutput, result.Content)
			if run.Score >= 0.5 {
				run.Status = "passed"
			} else {
				run.Status = "failed"
			}
		} else {
			run.Status = "passed"
			run.Score = 1.0
		}
	}

	h.db.Create(&run)
	c.JSON(http.StatusOK, gin.H{"run": run})
}

// ListTestRuns returns test runs for a specific agent
func (h *EvalHandler) ListTestRuns(c *gin.Context) {
	userID := c.GetString("user_id")
	agentID := c.Query("agent_id")

	q := h.db.Where("user_id = ?", userID)
	if agentID != "" {
		q = q.Where("agent_id = ?", agentID)
	}
	var runs []model.AgentTestRun
	q.Order("created_at DESC").Limit(100).Find(&runs)
	c.JSON(http.StatusOK, gin.H{"runs": runs})
}

func (h *EvalHandler) getProvider(cfg model.ModelConfig) provider.ModelProvider {
	if p, ok := h.providerRegistry.Get(cfg.Provider); ok {
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

// computeScore calculates a simple keyword-overlap score between expected and actual
func computeScore(expected, actual string) float64 {
	expectedLower := strings.ToLower(expected)
	actualLower := strings.ToLower(actual)

	// Split expected into keywords
	words := strings.Fields(expectedLower)
	if len(words) == 0 {
		return 1.0
	}

	matches := 0
	for _, w := range words {
		if len(w) < 3 {
			continue // skip short words
		}
		if strings.Contains(actualLower, w) {
			matches++
		}
	}

	significantWords := 0
	for _, w := range words {
		if len(w) >= 3 {
			significantWords++
		}
	}
	if significantWords == 0 {
		return 1.0
	}

	return float64(matches) / float64(significantWords)
}
