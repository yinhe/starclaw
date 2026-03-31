package v1

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw/internal/mcp"
	"github.com/yinhe/starclaw/internal/model"
	"github.com/yinhe/starclaw/internal/security"
	"github.com/yinhe/starclaw/internal/tool"
	"gorm.io/gorm"
)

type MCPHandler struct {
	db           *gorm.DB
	toolRegistry *tool.Registry
}

func NewMCPHandler(db *gorm.DB, tr *tool.Registry) *MCPHandler {
	return &MCPHandler{db: db, toolRegistry: tr}
}

// ReloadSavedServers re-registers all saved MCP servers from DB on startup.
// This ensures MCP tools survive Claw restarts.
func ReloadSavedServers(db *gorm.DB, registry *tool.Registry) {
	go func() {
		// Wait for DB to be ready
		time.Sleep(5 * time.Second)

		var servers []model.MCPServer
		db.Where("status = ?", "active").Find(&servers)
		if len(servers) == 0 {
			return
		}

		for _, server := range servers {
			cfg := mcp.ServerConfig{
				BaseURL: server.BaseURL,
				APIKey:  security.DecryptAPIKey(server.APIKey),
				Name:    server.Name,
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			client := mcp.NewClient(cfg)
			tools, err := client.ListTools(ctx)
			cancel()

			if err != nil {
				log.Printf("[MCP] Failed to reload server %s (%s): %v", server.Name, server.BaseURL, err)
				db.Model(&server).Update("status", "error")
				continue
			}

			for _, info := range tools {
				mcpTool := mcp.NewMCPTool(client, info, server.Name)
				registry.Register(mcpTool)
			}
			log.Printf("[MCP] ✓ Reloaded %d tools from %s (%s)", len(tools), server.Name, server.BaseURL)
		}
	}()
}

func (h *MCPHandler) ListServers(c *gin.Context) {
	userID := c.GetString("user_id")
	var servers []model.MCPServer
	h.db.Where("user_id = ?", userID).Order("created_at DESC").Find(&servers)
	// Mask API keys
	for i := range servers {
		if servers[i].APIKey != "" {
			servers[i].APIKey = "***"
		}
	}
	c.JSON(http.StatusOK, gin.H{"servers": servers})
}

func (h *MCPHandler) AddServer(c *gin.Context) {
	userID := c.GetString("user_id")

	var req struct {
		Name    string `json:"name" binding:"required"`
		BaseURL string `json:"base_url" binding:"required"`
		APIKey  string `json:"api_key"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Test connection and discover tools
	cfg := mcp.ServerConfig{
		BaseURL: req.BaseURL,
		APIKey:  req.APIKey,
		Name:    req.Name,
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	client := mcp.NewClient(cfg)
	tools, err := client.ListTools(ctx)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to connect to MCP server: " + err.Error()})
		return
	}

	server := model.MCPServer{
		UserID:    userID,
		Name:      req.Name,
		BaseURL:   req.BaseURL,
		APIKey:    security.EncryptAPIKey(req.APIKey),
		Status:    "active",
		ToolCount: len(tools),
	}
	if err := h.db.Create(&server).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save MCP server"})
		return
	}

	// Register tools into the global registry
	for _, info := range tools {
		mcpTool := mcp.NewMCPTool(client, info, req.Name)
		h.toolRegistry.Register(mcpTool)
	}

	// Return tool names
	toolNames := make([]string, len(tools))
	for i, t := range tools {
		toolNames[i] = t.Name
	}

	c.JSON(http.StatusOK, gin.H{
		"server": server,
		"tools":  toolNames,
	})
}

func (h *MCPHandler) DeleteServer(c *gin.Context) {
	userID := c.GetString("user_id")
	id := c.Param("id")

	result := h.db.Where("id = ? AND user_id = ?", id, userID).Delete(&model.MCPServer{})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "MCP server not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func (h *MCPHandler) TestServer(c *gin.Context) {
	userID := c.GetString("user_id")
	id := c.Param("id")

	var server model.MCPServer
	if err := h.db.Where("id = ? AND user_id = ?", id, userID).First(&server).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "MCP server not found"})
		return
	}

	cfg := mcp.ServerConfig{BaseURL: server.BaseURL, APIKey: security.DecryptAPIKey(server.APIKey), Name: server.Name}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	client := mcp.NewClient(cfg)
	tools, err := client.ListTools(ctx)
	if err != nil {
		h.db.Model(&server).Update("status", "error")
		c.JSON(http.StatusOK, gin.H{"status": "error", "error": err.Error()})
		return
	}

	h.db.Model(&server).Updates(map[string]interface{}{"status": "active", "tool_count": len(tools)})

	type toolDetail struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	details := make([]toolDetail, len(tools))
	for i, t := range tools {
		details[i] = toolDetail{Name: t.Name, Description: t.Description}
	}
	c.JSON(http.StatusOK, gin.H{"status": "active", "tools": details})
}
