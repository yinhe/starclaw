package v1

import (
	"fmt"

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
		Name             string   `json:"name"`
		Description      string   `json:"description"`
		Type             string   `json:"type"`
		Status           string   `json:"status"`
		Category         string   `json:"category"`
		CategoryLabel    string   `json:"category_label"`
		Subcategory      string   `json:"subcategory,omitempty"`
		SubcategoryLabel string   `json:"subcategory_label,omitempty"`
		Industry         string   `json:"industry"`
		Tags             []string `json:"tags,omitempty"`
	}
	var skills []SkillInfo

	// Built-in tools
	builtinNames := map[string]bool{"system": true, "code": true, "web_search": true, "http_request": true, "browser": true, "video_generation": true, "image_generation": true, "music_generation": true, "audio_analysis": true, "mv_production": true, "dubbing": true, "comic_production": true, "document": true, "desktop": true, "deploy_web": true, "bind_domain": true, "verify_online": true}
	for _, name := range h.toolRegistry.List() {
		t, ok := h.toolRegistry.Get(name)
		if !ok {
			continue
		}
		typ := "builtin"
		if !builtinNames[name] && name != "system" {
			if catalog := tool.DescribeCapability(name, "plugin", tool.MetadataFor(t)); catalog.Subcategory == "mcp" {
				typ = "mcp"
			} else {
				typ = "plugin"
			}
		}
		catalog := tool.DescribeCapability(name, typ, tool.MetadataFor(t))
		skills = append(skills, SkillInfo{
			Name:             name,
			Description:      t.Description(),
			Type:             typ,
			Status:           "active",
			Category:         catalog.Category,
			CategoryLabel:    catalog.CategoryLabel,
			Subcategory:      catalog.Subcategory,
			SubcategoryLabel: catalog.SubcategoryLabel,
			Industry:         catalog.Industry,
			Tags:             catalog.Tags,
		})
	}

	// MCP server tools (user-configured)
	userID := c.GetString("user_id")
	var mcpServers []model.MCPServer
	h.db.Where("user_id = ?", userID).Find(&mcpServers)
	for _, srv := range mcpServers {
		catalog := tool.DescribeCapability("mcp:"+srv.Name, "mcp", nil)
		skills = append(skills, SkillInfo{
			Name:             "mcp:" + srv.Name,
			Description:      fmt.Sprintf("MCP 外部服务: %s (%s)", srv.Name, srv.BaseURL),
			Type:             "mcp",
			Status:           srv.Status,
			Category:         catalog.Category,
			CategoryLabel:    catalog.CategoryLabel,
			Subcategory:      catalog.Subcategory,
			SubcategoryLabel: catalog.SubcategoryLabel,
			Industry:         catalog.Industry,
			Tags:             catalog.Tags,
		})
	}

	// MCP Bridge tools (host bridge — auto-detected)
	bridgeStatus := mcp.BridgeStatus()
	if connected, ok := bridgeStatus["connected"].(bool); ok && connected {
		if toolNames, ok := bridgeStatus["tool_names"].([]string); ok {
			for _, tn := range toolNames {
				catalog := tool.DescribeCapability("host."+tn, "mcp", nil)
				skills = append(skills, SkillInfo{
					Name:             "host." + tn,
					Description:      fmt.Sprintf("宿主机工具: %s (MCP Bridge)", tn),
					Type:             "mcp",
					Status:           "active",
					Category:         catalog.Category,
					CategoryLabel:    catalog.CategoryLabel,
					Subcategory:      catalog.Subcategory,
					SubcategoryLabel: catalog.SubcategoryLabel,
					Industry:         catalog.Industry,
					Tags:             catalog.Tags,
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
