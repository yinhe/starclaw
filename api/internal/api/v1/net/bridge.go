package network

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw/internal/agent"
)

// BridgeHandler manages agent bridge subprocess API endpoints.
type BridgeHandler struct {
	manager *agent.BridgeManager
}

// NewBridgeHandler creates a new bridge handler.
func NewBridgeHandler(manager *agent.BridgeManager) *BridgeHandler {
	return &BridgeHandler{manager: manager}
}

// StatusAll returns the status of all bridges.
// GET /v1/agents/bridges
func (h *BridgeHandler) StatusAll(c *gin.Context) {
	all := h.manager.StatusAll()

	result := make([]map[string]interface{}, 0, len(all))
	for _, inst := range all {
		item := map[string]interface{}{
			"agent_id":      inst.AgentID,
			"state":         inst.State,
			"port":          inst.Port,
			"pid":           inst.PID,
			"url":           inst.URL,
			"dashboard_url": inst.DashboardURL,
			"error":         inst.Error,
		}
		if inst.StartedAt != nil {
			item["started_at"] = inst.StartedAt
		}
		result = append(result, item)
	}

	c.JSON(http.StatusOK, gin.H{"bridges": result})
}

// Status returns the status of a specific bridge.
// GET /v1/agents/:id/bridge
func (h *BridgeHandler) Status(c *gin.Context) {
	agentID := c.Param("id")
	inst := h.manager.Status(agentID)
	if inst == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "bridge not found for agent " + agentID})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"agent_id":      inst.AgentID,
		"state":         inst.State,
		"port":          inst.Port,
		"pid":           inst.PID,
		"url":           inst.URL,
		"dashboard_url": inst.DashboardURL,
		"error":         inst.Error,
		"started_at":    inst.StartedAt,
	})
}

// Start launches a bridge subprocess.
// POST /v1/agents/:id/bridge/start
func (h *BridgeHandler) Start(c *gin.Context) {
	agentID := c.Param("id")
	inst := h.manager.Status(agentID)
	if inst == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "bridge not registered for agent " + agentID})
		return
	}
	if inst.State == agent.BridgeRunning {
		c.JSON(http.StatusConflict, gin.H{"error": "bridge already running", "port": inst.Port})
		return
	}
	if err := h.manager.Start(agentID, inst.Manifest, inst.Dir); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "bridge starting", "agent_id": agentID})
}

// Stop stops a bridge subprocess.
// POST /v1/agents/:id/bridge/stop
func (h *BridgeHandler) Stop(c *gin.Context) {
	agentID := c.Param("id")
	if err := h.manager.Stop(agentID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "bridge stopped", "agent_id": agentID})
}

// Restart restarts a bridge subprocess.
// POST /v1/agents/:id/bridge/restart
func (h *BridgeHandler) Restart(c *gin.Context) {
	agentID := c.Param("id")
	if err := h.manager.Restart(agentID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "bridge restarting", "agent_id": agentID})
}

// Discovered returns all agents discovered from the agents/ directory.
// GET /v1/agents/discovered
func (h *BridgeHandler) Discovered(c *gin.Context) {
	// This is a static list — re-scan would be needed for hot-reload
	c.JSON(http.StatusOK, gin.H{"message": "use GET /v1/agents for the full list (includes discovered agents)"})
}
