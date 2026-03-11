package v1

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw/internal/model"
	"gorm.io/gorm"
)

// TeamHandler provides lightweight team-related APIs.
type TeamHandler struct {
	db *gorm.DB
}

func NewTeamHandler(db *gorm.DB) *TeamHandler {
	return &TeamHandler{db: db}
}

// GetOrchestrator returns the best matched orchestrator agent for a team.
// Current supported team IDs:
// - team-devops-rnd
func (h *TeamHandler) GetOrchestrator(c *gin.Context) {
	userID := c.GetString("user_id")
	teamID := c.Param("id")

	if teamID != "team-devops-rnd" {
		c.JSON(http.StatusNotFound, gin.H{"error": "team not found"})
		return
	}

	var agents []model.Agent
	if err := h.db.Select("id, name, description").Where("user_id = ?", userID).Find(&agents).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	preferred := []string{"全能助手", "编程agent", "研发", "devops", "orchestrator"}
	for _, keyword := range preferred {
		for _, ag := range agents {
			if strings.Contains(strings.ToLower(ag.Name), strings.ToLower(keyword)) {
				c.JSON(http.StatusOK, gin.H{
					"team_id": teamID,
					"orchestrator_agent": gin.H{
						"id":          ag.ID,
						"name":        ag.Name,
						"description": ag.Description,
					},
					"strategy": "keyword_match",
				})
				return
			}
		}
	}

	if len(agents) > 0 {
		ag := agents[0]
		c.JSON(http.StatusOK, gin.H{
			"team_id": teamID,
			"orchestrator_agent": gin.H{
				"id":          ag.ID,
				"name":        ag.Name,
				"description": ag.Description,
			},
			"strategy": "fallback_first_agent",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"team_id":            teamID,
		"orchestrator_agent": nil,
		"strategy":           "none",
	})
}
