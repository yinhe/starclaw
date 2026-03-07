package v1

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	agentpkg "github.com/yinhe/starclaw/internal/agent"
	"github.com/yinhe/starclaw/internal/model"
	"github.com/yinhe/starclaw/internal/provider"
	"github.com/yinhe/starclaw/internal/tool"
	"gorm.io/gorm"
)

type MultiAgentHandler struct {
	db               *gorm.DB
	providerRegistry *provider.Registry
	toolRegistry     *tool.Registry
}

func NewMultiAgentHandler(db *gorm.DB, pr *provider.Registry, tr *tool.Registry) *MultiAgentHandler {
	return &MultiAgentHandler{db: db, providerRegistry: pr, toolRegistry: tr}
}

type MultiAgentRequest struct {
	AgentIDs       []string `json:"agent_ids" binding:"required"`
	OrchestratorID string   `json:"orchestrator_id"` // optional, for orchestrated mode
	Mode           string   `json:"mode"`            // sequential, parallel, orchestrated
	Input          string   `json:"input" binding:"required"`
	MaxRounds      int      `json:"max_rounds"`
}

func (h *MultiAgentHandler) Run(c *gin.Context) {
	userID := c.GetString("user_id")

	var req MultiAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Mode == "" {
		req.Mode = "sequential"
	}

	// Load agents from DB
	var agents []model.Agent
	if err := h.db.Where("id IN ? AND (user_id = ? OR is_public = ?)", req.AgentIDs, userID, true).Find(&agents).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to load agents"})
		return
	}

	if len(agents) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no valid agents found"})
		return
	}

	// Build AgentNodes
	var agentNodes []agentpkg.AgentNode
	for _, ag := range agents {
		node, err := h.buildAgentNode(ag)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "agent " + ag.Name + ": " + err.Error()})
			return
		}
		agentNodes = append(agentNodes, *node)
	}

	cfg := &agentpkg.MultiAgentConfig{
		Agents:    agentNodes,
		Mode:      req.Mode,
		MaxRounds: req.MaxRounds,
	}

	// Load orchestrator if needed
	if req.Mode == "orchestrated" && req.OrchestratorID != "" {
		var orch model.Agent
		if err := h.db.Where("id = ? AND (user_id = ? OR is_public = ?)", req.OrchestratorID, userID, true).First(&orch).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "orchestrator agent not found"})
			return
		}
		node, err := h.buildAgentNode(orch)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "orchestrator: " + err.Error()})
			return
		}
		cfg.Orchestrator = node
	}

	result, err := agentpkg.RunMultiAgent(c.Request.Context(), cfg, h.toolRegistry, req.Input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"output":        result.Output,
		"agent_outputs": result.AgentOutputs,
		"usage":         result.TotalUsage,
	})
}

func (h *MultiAgentHandler) buildAgentNode(ag model.Agent) (*agentpkg.AgentNode, error) {
	var modelCfg model.ModelConfig
	if err := h.db.Where("id = ?", ag.ModelID).First(&modelCfg).Error; err != nil {
		return nil, err
	}

	p := resolveProvider(modelCfg, h.providerRegistry)

	var enabledTools []string
	if ag.Tools != "" {
		// parse JSON array
		json.Unmarshal([]byte(ag.Tools), &enabledTools)
	}

	return &agentpkg.AgentNode{
		ID:           ag.ID,
		Name:         ag.Name,
		SystemPrompt: ag.SystemPrompt,
		Model:        modelCfg.ModelName,
		Provider:     p,
		Tools:        enabledTools,
		MaxTokens:    modelCfg.MaxTokens,
		Temperature:  modelCfg.Temperature,
	}, nil
}
