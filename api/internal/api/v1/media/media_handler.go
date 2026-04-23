package media

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw/internal/model"
	"github.com/yinhe/starclaw/internal/sandbox"
	"github.com/yinhe/starclaw/internal/tool"
	"gorm.io/gorm"
)

// MediaHandler handles image gallery, music gallery, and document browsing.
type MediaHandler struct {
	db           *gorm.DB
	toolRegistry *tool.Registry
}

func NewMediaHandler(db *gorm.DB, toolRegistry *tool.Registry) *MediaHandler {
	return &MediaHandler{db: db, toolRegistry: toolRegistry}
}

// ── Images ──

func (h *MediaHandler) ListImages(c *gin.Context) {
	userID := c.GetString("user_id")
	var records []model.ImageRecord
	h.db.Where("user_id = ?", userID).Order("created_at DESC").Limit(500).Find(&records)
	c.JSON(200, gin.H{"images": records})
}

func (h *MediaHandler) DeleteImage(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("user_id")
	result := h.db.Where("id = ? AND user_id = ?", id, userID).Delete(&model.ImageRecord{})
	if result.RowsAffected == 0 {
		c.JSON(404, gin.H{"error": "image not found"})
		return
	}
	c.JSON(200, gin.H{"message": "deleted"})
}

// GenerateImage synchronously generates a single image via image_generation tool
// and returns the stable local URL. Used by Character Studio wizard.
// Body: { prompt, model?, image_url?, size?, scene?, style?, negative_prompt? }
// Response: { image_id, url, local_url, display_url, model, size, scene }
func (h *MediaHandler) GenerateImage(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(401, gin.H{"error": "unauthorized"})
		return
	}
	var req struct {
		Prompt         string `json:"prompt"`
		Model          string `json:"model"`
		ImageURL       string `json:"image_url"`
		Size           string `json:"size"`
		Scene          string `json:"scene"`
		Style          string `json:"style"`
		NegativePrompt string `json:"negative_prompt"`
		ConversationID string `json:"conversation_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request body", "detail": err.Error()})
		return
	}
	if strings.TrimSpace(req.Prompt) == "" {
		c.JSON(400, gin.H{"error": "prompt is required"})
		return
	}
	imgTool, ok := h.toolRegistry.Get("image_generation")
	if !ok {
		c.JSON(500, gin.H{"error": "image_generation tool not available"})
		return
	}
	if req.Model == "" {
		req.Model = "nano-banana-2"
	}
	if req.Size == "" {
		req.Size = "landscape_16_9"
	}
	args := map[string]interface{}{
		"action":          "generate_image",
		"prompt":          req.Prompt,
		"model":           req.Model,
		"size":            req.Size,
		"n":               "1",
		"scene":           req.Scene,
		"style":           req.Style,
		"negative_prompt": req.NegativePrompt,
	}
	if strings.TrimSpace(req.ImageURL) != "" {
		args["image_url"] = strings.TrimSpace(req.ImageURL)
	}
	argsBytes, err := json.Marshal(args)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to marshal image args"})
		return
	}
	ctx := context.WithValue(context.Background(), tool.CtxKeyUserID, userID)
	if strings.TrimSpace(req.ConversationID) != "" {
		ctx = context.WithValue(ctx, tool.CtxKeyConversationID, strings.TrimSpace(req.ConversationID))
	}
	result, err := imgTool.Execute(ctx, string(argsBytes))
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(result), &data); err != nil {
		c.JSON(200, gin.H{"result": result})
		return
	}
	c.JSON(200, data)
}

// ── Music ──

func (h *MediaHandler) ListMusic(c *gin.Context) {
	userID := c.GetString("user_id")
	var records []model.MusicRecord
	h.db.Where("user_id = ?", userID).Order("created_at DESC").Limit(500).Find(&records)
	c.JSON(200, gin.H{"music": records})
}

func (h *MediaHandler) DeleteMusic(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("user_id")
	result := h.db.Where("id = ? AND user_id = ?", id, userID).Delete(&model.MusicRecord{})
	if result.RowsAffected == 0 {
		c.JSON(404, gin.H{"error": "music not found"})
		return
	}
	c.JSON(200, gin.H{"message": "deleted"})
}

// ── Documents ──

type docFile struct {
	Name           string `json:"name"`
	Path           string `json:"path"`
	Workspace      string `json:"workspace"`
	Size           int64  `json:"size"`
	ModTime        string `json:"mod_time"`
	URL            string `json:"url"`
	Category       string `json:"category"`
	ConversationID string `json:"conversation_id"`
	ConvTitle      string `json:"conv_title"`
}

type convSummary struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

func (h *MediaHandler) ListDocuments(c *gin.Context) {
	baseDir := sandbox.WorkspacesDir()
	userID := c.GetString("user_id")
	convFilter := c.Query("conversation_id")

	// 1. Query tracked files from DB (user-isolated)
	var dbFiles []model.WorkspaceFile
	q := h.db.Where("user_id = ?", userID).Order("created_at DESC")
	if convFilter != "" {
		q = q.Where("conversation_id = ?", convFilter)
	}
	q.Find(&dbFiles)

	// Build a set of tracked file keys (workspace:path) for dedup
	trackedKeys := map[string]bool{}
	// Collect unique conversation IDs
	convIDs := map[string]bool{}
	var docs []docFile

	cst := time.FixedZone("CST", 8*3600)

	for _, f := range dbFiles {
		key := f.WorkspaceID + ":" + f.Path
		trackedKeys[key] = true
		if f.ConversationID != "" {
			convIDs[f.ConversationID] = true
		}

		// Get actual file size and mod_time from filesystem
		absPath := filepath.Join(baseDir, f.WorkspaceID, f.Path)
		modTime := f.UpdatedAt.In(cst).Format("2006-01-02 15:04:05")
		size := f.Size
		if info, err := os.Stat(absPath); err == nil {
			size = info.Size()
			modTime = info.ModTime().In(cst).Format("2006-01-02 15:04:05")
		}

		docs = append(docs, docFile{
			Name:           f.Name,
			Path:           f.Path,
			Workspace:      f.WorkspaceID,
			Size:           size,
			ModTime:        modTime,
			URL:            fmt.Sprintf("/v1/documents/%s/%s", f.WorkspaceID, f.Path),
			Category:       f.Category,
			ConversationID: f.ConversationID,
		})
	}

	// 2. If no conversation filter, also scan filesystem for untracked files (user's workspace only)
	if convFilter == "" {
		skipExts := map[string]bool{
			".pyc": true, ".class": true, ".o": true, ".exe": true,
		}
		codeExts := map[string]bool{
			".py": true, ".js": true, ".ts": true, ".jsx": true, ".tsx": true,
			".go": true, ".java": true, ".c": true, ".cpp": true, ".h": true,
			".rs": true, ".rb": true, ".php": true, ".sh": true, ".bat": true,
			".ps1": true, ".sql": true, ".r": true, ".m": true, ".swift": true,
			".kt": true, ".scala": true, ".lua": true, ".pl": true, ".dart": true,
			".css": true, ".scss": true, ".less": true, ".vue": true, ".svelte": true,
			".json": true, ".yaml": true, ".yml": true, ".toml": true, ".xml": true,
			".ini": true, ".cfg": true, ".conf": true, ".env": true,
			".html": true, ".htm": true,
		}
		// Only scan user's own workspace directory
		wsID := userID
		wsPath := filepath.Join(baseDir, wsID)
		if _, err := os.Stat(wsPath); err == nil {
			filepath.Walk(wsPath, func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return nil
				}
				name := info.Name()
				if strings.HasPrefix(name, "_exec") || name == "Main.class" {
					return nil
				}
				ext := strings.ToLower(filepath.Ext(name))
				if skipExts[ext] {
					return nil
				}
				relPath, _ := filepath.Rel(wsPath, path)
				key := wsID + ":" + relPath
				if trackedKeys[key] {
					return nil // already from DB
				}
				category := "document"
				if codeExts[ext] {
					category = "code"
				}
				docs = append(docs, docFile{
					Name:      name,
					Path:      relPath,
					Workspace: wsID,
					Size:      info.Size(),
					ModTime:   info.ModTime().In(cst).Format("2006-01-02 15:04:05"),
					URL:       fmt.Sprintf("/v1/documents/%s/%s", wsID, relPath),
					Category:  category,
				})
				return nil
			})
		}
	}

	// 3. Fetch conversation titles for grouping
	var conversations []convSummary
	if len(convIDs) > 0 {
		ids := make([]string, 0, len(convIDs))
		for id := range convIDs {
			ids = append(ids, id)
		}
		var convs []model.Conversation
		h.db.Where("id IN ?", ids).Find(&convs)
		convMap := map[string]string{}
		for _, cv := range convs {
			convMap[cv.ID] = cv.Title
			conversations = append(conversations, convSummary{ID: cv.ID, Title: cv.Title})
		}
		// Backfill conv_title into docs
		for i := range docs {
			if docs[i].ConversationID != "" {
				docs[i].ConvTitle = convMap[docs[i].ConversationID]
			}
		}
	}

	c.JSON(200, gin.H{
		"documents":     docs,
		"conversations": conversations,
	})
}

func (h *MediaHandler) GetDocument(c *gin.Context) {
	wsID := c.Param("workspace")
	filePath := strings.TrimPrefix(c.Param("filepath"), "/")
	baseDir := sandbox.WorkspacesDir()
	absPath := filepath.Join(baseDir, wsID, filePath)
	// Security: ensure path stays within workspace
	if !strings.HasPrefix(absPath, filepath.Join(baseDir, wsID)) {
		c.JSON(403, gin.H{"error": "forbidden"})
		return
	}
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		c.JSON(404, gin.H{"error": "file not found"})
		return
	}
	c.File(absPath)
}

func (h *MediaHandler) DeleteDocument(c *gin.Context) {
	wsID := c.Param("workspace")
	filePath := strings.TrimPrefix(c.Param("filepath"), "/")
	baseDir := sandbox.WorkspacesDir()
	absPath := filepath.Join(baseDir, wsID, filePath)
	if !strings.HasPrefix(absPath, filepath.Join(baseDir, wsID)) {
		c.JSON(403, gin.H{"error": "forbidden"})
		return
	}
	if err := os.Remove(absPath); err != nil {
		c.JSON(404, gin.H{"error": "file not found"})
		return
	}
	c.JSON(200, gin.H{"message": "deleted"})
}
