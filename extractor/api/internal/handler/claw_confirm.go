package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"starclaw.net/extractor/api/internal/engine"
)

// ClawConfirmHandler manages the Claw AI secondary confirmation flow.
type ClawConfirmHandler struct {
	DB   *gorm.DB
	Claw *engine.ClawClient
}

// ConfirmCandidatesRequest is the request body for /v1/claw/confirm.
type ConfirmCandidatesRequest struct {
	Candidates []engine.CandidateStock `json:"candidates"`
	MarketEnv  string                  `json:"market_env"`
	Model      string                  `json:"model"`
}

// Confirm sends candidates to Claw AI for secondary analysis.
// POST /v1/claw/confirm
func (h *ClawConfirmHandler) Confirm(c *gin.Context) {
	var req ConfirmCandidatesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if len(req.Candidates) == 0 {
		c.JSON(http.StatusOK, gin.H{"confirmed": []engine.CandidateStock{}, "message": "no candidates"})
		return
	}

	if h.Claw == nil {
		log.Println("[claw] no Claw client configured, passing all candidates through")
		c.JSON(http.StatusOK, gin.H{"confirmed": req.Candidates, "message": "claw not configured, all passed"})
		return
	}

	// Build prompt from candidates
	prompt := buildConfirmPrompt(req.Candidates, req.MarketEnv)

	// Call Claw AI
	model := req.Model
	if model == "" {
		model = os.Getenv("EXTRACTOR_CLAW_MODEL")
		if model == "" {
			model = "qwen-max"
		}
	}

	responseText, err := h.Claw.RequestConfirmation(prompt, model)
	if err != nil {
		log.Printf("[claw] AI confirmation failed: %v, passing all candidates through", err)
		for i := range req.Candidates {
			req.Candidates[i].ClawAction = "confirm"
			req.Candidates[i].ClawConfidence = 0.5
			req.Candidates[i].RiskFlags = []string{"AI确认失败，降级放行: " + err.Error()}
		}
		c.JSON(http.StatusOK, gin.H{
			"confirmed":   req.Candidates,
			"message":     "claw error, degraded pass-through",
			"ai_response": "",
		})
		return
	}

	// Parse Claw response
	confirmed := parseClawResponse(responseText, req.Candidates)

	c.JSON(http.StatusOK, gin.H{
		"confirmed":   confirmed,
		"rejected":    len(req.Candidates) - len(confirmed),
		"total":       len(req.Candidates),
		"ai_response": responseText,
	})
}

// Status checks Claw connectivity.
// GET /v1/claw/status
func (h *ClawConfirmHandler) Status(c *gin.Context) {
	if h.Claw == nil {
		c.JSON(http.StatusOK, gin.H{"connected": false, "message": "claw client not configured"})
		return
	}
	err := h.Claw.Ping()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"connected": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"connected": true, "url": h.Claw.BaseURL})
}

func buildConfirmPrompt(candidates []engine.CandidateStock, marketEnv string) string {
	envZH := map[string]string{
		"bull": "牛市(进攻)", "sideways": "震荡市(中性)",
		"bear": "熊市(防守)", "extreme_bear": "极端熊市(极度防守)",
	}
	envStr := envZH[marketEnv]
	if envStr == "" {
		envStr = "震荡市"
	}

	lines := ""
	for i, c := range candidates {
		trendIcon := "⚠️非多头"
		if c.TrendOK {
			trendIcon = "✅多头"
		}
		lines += fmt.Sprintf("%d. %s | 综合分=%.2f | 趋势=%s | 涨幅=%.1f%% | 量比=%.1f | 理由: %s\n",
			i+1, c.Code, c.Score, trendIcon, c.TodayChange*100, c.VolumeRatio, c.Reason)
	}

	return fmt.Sprintf(`你是 StarClaw 量化交易系统的 AI 风控分析师。

当前市场环境: %s

量化策略(主升浪趋势)筛选出以下候选股票，请你进行二次确认分析：

%s
请对每只股票进行以下分析：
1. 基本面快检: 是否有近期财报预警、重大利空公告、ST风险
2. 消息面扫描: 近3日是否有政策利空、行业利空、高管减持等负面消息
3. 技术面验证: 量化给出的多头排列+放量突破是否与你了解的走势一致
4. 板块共振: 所属行业/概念板块当前是否处于活跃状态

请用JSON格式回复，每只股票一个对象：
[{"code":"600519.SH","action":"confirm/reject/reduce","confidence":0.0-1.0,"risk_flags":["风险1"],"suggestion":"建议"}]`, envStr, lines)
}

func parseClawResponse(responseText string, candidates []engine.CandidateStock) []engine.CandidateStock {
	var aiResults []struct {
		Code       string   `json:"code"`
		Action     string   `json:"action"`
		Confidence float64  `json:"confidence"`
		RiskFlags  []string `json:"risk_flags"`
		Suggestion string   `json:"suggestion"`
	}

	// Try to extract JSON from response (may be wrapped in ```json ... ```)
	text := responseText
	if idx := findJSONArray(text); idx >= 0 {
		text = text[idx:]
		if end := findJSONArrayEnd(text); end > 0 {
			text = text[:end+1]
		}
	}

	if err := json.Unmarshal([]byte(text), &aiResults); err != nil {
		log.Printf("[claw] parse AI response failed: %v, passing all through", err)
		for i := range candidates {
			candidates[i].ClawAction = "confirm"
			candidates[i].ClawConfidence = 0.5
			candidates[i].RiskFlags = []string{"AI回复解析失败，降级放行"}
		}
		return candidates
	}

	aiMap := make(map[string]int)
	for i, r := range aiResults {
		aiMap[r.Code] = i
	}

	var confirmed []engine.CandidateStock
	for _, c := range candidates {
		if idx, ok := aiMap[c.Code]; ok {
			ai := aiResults[idx]
			c.ClawAction = ai.Action
			c.ClawConfidence = ai.Confidence
			c.RiskFlags = ai.RiskFlags
			c.Suggestion = ai.Suggestion
		} else {
			c.ClawAction = "confirm"
			c.ClawConfidence = 0.5
		}

		if c.ClawAction == "confirm" {
			confirmed = append(confirmed, c)
		} else if c.ClawAction == "reduce" {
			c.ReducePosition = true
			confirmed = append(confirmed, c)
		} else {
			log.Printf("[claw] REJECT %s: %v", c.Code, c.RiskFlags)
		}
	}
	return confirmed
}

func findJSONArray(s string) int {
	for i, ch := range s {
		if ch == '[' {
			return i
		}
	}
	return -1
}

func findJSONArrayEnd(s string) int {
	depth := 0
	for i, ch := range s {
		if ch == '[' {
			depth++
		} else if ch == ']' {
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}
