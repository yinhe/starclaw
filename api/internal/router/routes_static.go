package router

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw/internal/api/v1/media"
	"github.com/yinhe/starclaw/internal/mcp"
	"github.com/yinhe/starclaw/internal/tool"
)

// registerStaticServeRoutes registers public static file serving routes (no auth required).
// These serve generated media files (videos, images, music, documents) and MCP bridge binaries.
func registerStaticServeRoutes(apiV1 *gin.RouterGroup) {
	// Uploaded files (public, secured by UUID filename)
	apiV1.GET("/uploads/:filename", media.ServeUploadedFile)

	// Browser screenshots (public, secured by UUID)
	apiV1.GET("/screenshots/:id", media.ServeScreenshot)

	// Video clips (individual generated clips, public, secured by UUID filename)
	apiV1.GET("/videos/clips/:filename", func(c *gin.Context) {
		filename := c.Param("filename")
		filePath := filepath.Join(tool.VideosDir(), filename)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			c.JSON(404, gin.H{"error": "video clip not found"})
			return
		}
		c.Header("Content-Disposition", "attachment; filename="+filename)
		c.File(filePath)
	})

	// Merged videos (public, secured by UUID filename)
	apiV1.GET("/videos/merged/:filename", func(c *gin.Context) {
		filename := c.Param("filename")
		filePath := filepath.Join(tool.MergedVideosDir(), filename)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			c.JSON(404, gin.H{"error": "merged video not found"})
			return
		}
		c.Header("Content-Type", "video/mp4")
		if c.Query("download") == "1" {
			c.Header("Content-Disposition", "attachment; filename="+filename)
		} else {
			c.Header("Content-Disposition", "inline")
		}
		c.File(filePath)
	})

	// Video thumbnails (public, secured by UUID filename)
	apiV1.GET("/videos/thumbnails/:filename", func(c *gin.Context) {
		filename := c.Param("filename")
		filePath := filepath.Join(tool.ThumbnailsDir(), filename)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			c.JSON(404, gin.H{"error": "thumbnail not found"})
			return
		}
		c.Header("Cache-Control", "public, max-age=86400")
		c.File(filePath)
	})

	// Generated images (public, secured by UUID filename)
	apiV1.GET("/images/:filename", func(c *gin.Context) {
		filename := c.Param("filename")
		filePath := filepath.Join(tool.ImagesDir(), filename)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			c.JSON(404, gin.H{"error": "image not found"})
			return
		}
		c.Header("Cache-Control", "public, max-age=86400")
		c.File(filePath)
	})

	// Generated Word documents (public, secured by UUID filename)
	apiV1.GET("/docx/:filename", func(c *gin.Context) {
		filename := c.Param("filename")
		if !strings.HasSuffix(filename, ".docx") {
			c.JSON(400, gin.H{"error": "invalid document format"})
			return
		}
		filePath := filepath.Join(tool.GetDataDir(), "documents", filename)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			c.JSON(404, gin.H{"error": "document not found"})
			return
		}
		c.Header("Content-Disposition", "attachment; filename="+filename)
		c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
		c.File(filePath)
	})

	// MCP Bridge binary download (public, no auth — users need this before logging in)
	apiV1.GET("/mcp-bridge/download/:platform", func(c *gin.Context) {
		platform := c.Param("platform")
		filePath, filename := mcp.BridgeBinaryPath(platform)
		if filePath == "" {
			c.JSON(404, gin.H{"error": "unsupported platform, use: windows_amd64, darwin_amd64, darwin_arm64, linux_amd64"})
			return
		}
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			c.JSON(404, gin.H{"error": "binary not available, rebuild with: docker compose build api"})
			return
		}
		c.Header("Content-Disposition", "attachment; filename="+filename)
		c.File(filePath)
	})

	// MCP Bridge one-line installer script (macOS/Linux)
	apiV1.GET("/mcp-bridge/install.sh", func(c *gin.Context) {
		scheme := "http"
		if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
			scheme = "https"
		}
		serverURL := scheme + "://" + c.Request.Host
		script := mcp.GenerateInstallScript(serverURL)
		c.Header("Content-Type", "text/plain; charset=utf-8")
		c.String(200, script)
	})

	// MCP Bridge one-line installer script (Windows PowerShell)
	apiV1.GET("/mcp-bridge/install.ps1", func(c *gin.Context) {
		scheme := "http"
		if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
			scheme = "https"
		}
		serverURL := scheme + "://" + c.Request.Host
		script := mcp.GeneratePowerShellInstallScript(serverURL)
		c.Header("Content-Type", "text/plain; charset=utf-8")
		c.String(200, script)
	})

	// Project assets (character images, scene refs, etc.) from mounted docs directory
	// URL: /v1/projects/:project/*filepath → /app/docs/:project/*filepath
	apiV1.GET("/projects/:project/*filepath", func(c *gin.Context) {
		project := c.Param("project")
		fp := c.Param("filepath")
		// Security: reject path traversal
		if strings.Contains(fp, "..") || strings.Contains(project, "..") {
			c.JSON(400, gin.H{"error": "invalid path"})
			return
		}
		filePath := filepath.Join("/app/docs", project, fp)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			c.JSON(404, gin.H{"error": "project asset not found"})
			return
		}
		c.Header("Cache-Control", "public, max-age=3600")
		c.File(filePath)
	})

	// Music files (public, secured by UUID filename)
	apiV1.GET("/music/:filename", func(c *gin.Context) {
		filename := c.Param("filename")
		filePath := filepath.Join(tool.MusicDir(), filename)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			c.JSON(404, gin.H{"error": "music file not found"})
			return
		}
		c.Header("Cache-Control", "public, max-age=86400")
		c.File(filePath)
	})
}
