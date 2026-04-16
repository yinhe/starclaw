package v1

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw/internal/mcp"
	"github.com/yinhe/starclaw/internal/model"
	"github.com/yinhe/starclaw/internal/tool"
	"gorm.io/gorm"
)

// SkillsHandler lists tools, skills, and MCP servers for the frontend.
type SkillsHandler struct {
	db           *gorm.DB
	toolRegistry *tool.Registry
}

func NewSkillsHandler(db *gorm.DB, toolRegistry *tool.Registry) *SkillsHandler {
	return &SkillsHandler{db: db, toolRegistry: toolRegistry}
}

func (h *SkillsHandler) ListTools(c *gin.Context) {
	c.JSON(200, gin.H{"tools": h.toolRegistry.List()})
}

func (h *SkillsHandler) ListSkills(c *gin.Context) {
	type SkillInfo struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Type        string `json:"type"`
		Status      string `json:"status"`
	}
	var skills []SkillInfo

	// Built-in tools
	builtinNames := []string{"system", "code", "web_search", "http_request", "browser", "video_generation", "deploy_web", "bind_domain", "verify_online"}
	for _, name := range h.toolRegistry.List() {
		t, ok := h.toolRegistry.Get(name)
		if !ok {
			continue
		}
		typ := "builtin"
		for _, bn := range builtinNames {
			if name == bn {
				typ = "builtin"
				break
			}
		}
		// Check if it's a plugin (starts with plugin prefix or not in builtin list)
		isBuiltin := false
		for _, bn := range builtinNames {
			if name == bn {
				isBuiltin = true
				break
			}
		}
		if !isBuiltin && name != "system" {
			if strings.HasPrefix(name, "mcp_") {
				typ = "mcp"
			} else {
				typ = "plugin"
			}
		}
		skills = append(skills, SkillInfo{
			Name:        name,
			Description: t.Description(),
			Type:        typ,
			Status:      "active",
		})
	}

	// MCP server tools (user-configured)
	userID := c.GetString("user_id")
	var mcpServers []model.MCPServer
	h.db.Where("user_id = ?", userID).Find(&mcpServers)
	for _, srv := range mcpServers {
		skills = append(skills, SkillInfo{
			Name:        "mcp:" + srv.Name,
			Description: fmt.Sprintf("MCP 外部服务: %s (%s)", srv.Name, srv.BaseURL),
			Type:        "mcp",
			Status:      srv.Status,
		})
	}

	// MCP Bridge tools (host bridge — auto-detected)
	bridgeStatus := mcp.BridgeStatus()
	if connected, ok := bridgeStatus["connected"].(bool); ok && connected {
		if toolNames, ok := bridgeStatus["tool_names"].([]string); ok {
			for _, tn := range toolNames {
				skills = append(skills, SkillInfo{
					Name:        "host." + tn,
					Description: fmt.Sprintf("宿主机工具: %s (MCP Bridge)", tn),
					Type:        "mcp",
					Status:      "active",
				})
			}
		}
	}

	// Count by type
	builtinCount, pluginCount, mcpCount := 0, 0, 0
	for _, s := range skills {
		switch s.Type {
		case "builtin":
			builtinCount++
		case "plugin":
			pluginCount++
		case "mcp":
			mcpCount++
		}
	}

	c.JSON(200, gin.H{
		"skills": skills,
		"summary": gin.H{
			"total":   len(skills),
			"builtin": builtinCount,
			"plugin":  pluginCount,
			"mcp":     mcpCount,
		},
	})
}
