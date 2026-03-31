package v1

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw/internal/model"
	"github.com/yinhe/starclaw/internal/sandbox"
	"gorm.io/gorm"
)

type WorkspaceHandler struct {
	db      *gorm.DB
	baseDir string
}

func NewWorkspaceHandler(db *gorm.DB) *WorkspaceHandler {
	return &WorkspaceHandler{db: db, baseDir: sandbox.WorkspacesDir()}
}

// FolderInfo represents a conversation-level folder with aggregated stats
type FolderInfo struct {
	ConversationID string `json:"conversation_id"`
	Title          string `json:"title"`
	FileCount      int    `json:"file_count"`
	DocCount       int    `json:"doc_count"`
	CodeCount      int    `json:"code_count"`
	TotalSize      int64  `json:"total_size"`
	LastModified   string `json:"last_modified"`
	Locked         bool   `json:"locked"`
}

// ListFolders returns conversation-level folder summaries (no individual files)
func (h *WorkspaceHandler) ListFolders(c *gin.Context) {
	userID := c.GetString("user_id")

	// 1. Aggregate from DB: group by conversation_id
	type FolderRow struct {
		ConversationID string
		FileCount      int
		DocCount       int
		CodeCount      int
		TotalSize      int64
		LastModified   time.Time
	}

	var rows []FolderRow
	h.db.Model(&model.WorkspaceFile{}).
		Select(`conversation_id,
			COUNT(*) as file_count,
			SUM(CASE WHEN category = 'document' THEN 1 ELSE 0 END) as doc_count,
			SUM(CASE WHEN category = 'code' THEN 1 ELSE 0 END) as code_count,
			SUM(size) as total_size,
			MAX(updated_at) as last_modified`).
		Where("user_id = ? AND conversation_id != ''", userID).
		Group("conversation_id").
		Order("last_modified DESC").
		Find(&rows)

	// 2. Collect conversation IDs for title lookup
	convIDs := make([]string, 0, len(rows))
	for _, r := range rows {
		convIDs = append(convIDs, r.ConversationID)
	}

	// 3. Fetch conversation titles
	convTitles := map[string]string{}
	if len(convIDs) > 0 {
		var convs []model.Conversation
		h.db.Select("id, title").Where("id IN ?", convIDs).Find(&convs)
		for _, cv := range convs {
			convTitles[cv.ID] = cv.Title
		}
	}

	// 4. Fetch lock states
	lockMap := map[string]bool{}
	if len(convIDs) > 0 {
		var folders []model.WorkspaceFolder
		h.db.Where("conversation_id IN ? AND locked = ?", convIDs, true).Find(&folders)
		for _, f := range folders {
			lockMap[f.ConversationID] = true
		}
	}

	// 5. Count untracked files (no conversation_id) from filesystem
	cst := time.FixedZone("CST", 8*3600)
	var untrackedCount int
	var untrackedSize int64
	var untrackedLastMod time.Time
	wsPath := filepath.Join(h.baseDir, userID)
	if info, err := os.Stat(wsPath); err == nil && info.IsDir() {
		// Walk only top-level files in user workspace (not in conversation subdirs)
		entries, _ := os.ReadDir(wsPath)
		for _, e := range entries {
			if !e.IsDir() {
				fi, err := e.Info()
				if err != nil {
					continue
				}
				untrackedCount++
				untrackedSize += fi.Size()
				if fi.ModTime().After(untrackedLastMod) {
					untrackedLastMod = fi.ModTime()
				}
			}
		}
	}

	// 6. Build response
	folders := make([]FolderInfo, 0, len(rows)+1)
	for _, r := range rows {
		title := convTitles[r.ConversationID]
		if title == "" {
			title = "对话 " + r.LastModified.In(cst).Format("2006-01-02")
		}
		folders = append(folders, FolderInfo{
			ConversationID: r.ConversationID,
			Title:          title,
			FileCount:      r.FileCount,
			DocCount:       r.DocCount,
			CodeCount:      r.CodeCount,
			TotalSize:      r.TotalSize,
			LastModified:   r.LastModified.In(cst).Format("2006-01-02 15:04:05"),
			Locked:         lockMap[r.ConversationID],
		})
	}

	// Add "untracked" virtual folder if any
	if untrackedCount > 0 {
		folders = append(folders, FolderInfo{
			ConversationID: "_untracked",
			Title:          "未归类文件",
			FileCount:      untrackedCount,
			TotalSize:      untrackedSize,
			LastModified:   untrackedLastMod.In(cst).Format("2006-01-02 15:04:05"),
		})
	}

	// Count totals
	totalFiles := 0
	for _, f := range folders {
		totalFiles += f.FileCount
	}

	c.JSON(200, gin.H{
		"folders":       folders,
		"total_folders": len(folders),
		"total_files":   totalFiles,
	})
}

// ListFolderFiles returns paginated files within a specific conversation folder
func (h *WorkspaceHandler) ListFolderFiles(c *gin.Context) {
	userID := c.GetString("user_id")
	convID := c.Param("conv_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 50
	}

	cst := time.FixedZone("CST", 8*3600)

	type FileItem struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		Path      string `json:"path"`
		Workspace string `json:"workspace"`
		Size      int64  `json:"size"`
		ModTime   string `json:"mod_time"`
		URL       string `json:"url"`
		Category  string `json:"category"`
	}

	// Handle untracked files (top-level in user workspace)
	if convID == "_untracked" {
		wsPath := filepath.Join(h.baseDir, userID)
		var files []FileItem
		entries, _ := os.ReadDir(wsPath)
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			fi, err := e.Info()
			if err != nil {
				continue
			}
			cat := "document"
			ext := strings.ToLower(filepath.Ext(fi.Name()))
			if isCodeExt(ext) {
				cat = "code"
			}
			files = append(files, FileItem{
				Name:      fi.Name(),
				Path:      fi.Name(),
				Workspace: userID,
				Size:      fi.Size(),
				ModTime:   fi.ModTime().In(cst).Format("2006-01-02 15:04:05"),
				URL:       fmt.Sprintf("/v1/documents/%s/%s", userID, fi.Name()),
				Category:  cat,
			})
		}
		// Sort by mod_time desc
		sort.Slice(files, func(i, j int) bool {
			return files[i].ModTime > files[j].ModTime
		})
		total := len(files)
		start := (page - 1) * pageSize
		if start > total {
			start = total
		}
		end := start + pageSize
		if end > total {
			end = total
		}
		c.JSON(200, gin.H{
			"conversation_id": "_untracked",
			"title":           "未归类文件",
			"files":           files[start:end],
			"total":           total,
			"page":            page,
			"page_size":       pageSize,
			"locked":          false,
		})
		return
	}

	// Regular conversation folder
	var total int64
	h.db.Model(&model.WorkspaceFile{}).
		Where("user_id = ? AND conversation_id = ?", userID, convID).
		Count(&total)

	var dbFiles []model.WorkspaceFile
	h.db.Where("user_id = ? AND conversation_id = ?", userID, convID).
		Order("updated_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&dbFiles)

	files := make([]FileItem, 0, len(dbFiles))
	for _, f := range dbFiles {
		size := f.Size
		modTime := f.UpdatedAt.In(cst).Format("2006-01-02 15:04:05")
		absPath := filepath.Join(h.baseDir, f.WorkspaceID, f.Path)
		if info, err := os.Stat(absPath); err == nil {
			size = info.Size()
			modTime = info.ModTime().In(cst).Format("2006-01-02 15:04:05")
		}
		files = append(files, FileItem{
			ID:        f.ID,
			Name:      f.Name,
			Path:      f.Path,
			Workspace: f.WorkspaceID,
			Size:      size,
			ModTime:   modTime,
			URL:       fmt.Sprintf("/v1/documents/%s/%s", f.WorkspaceID, f.Path),
			Category:  f.Category,
		})
	}

	// Get conversation title
	title := ""
	var conv model.Conversation
	if h.db.Select("title").Where("id = ?", convID).First(&conv).Error == nil {
		title = conv.Title
	}
	if title == "" {
		title = "对话 " + convID[:8]
	}

	// Get lock state
	locked := false
	var folder model.WorkspaceFolder
	if h.db.Where("conversation_id = ?", convID).First(&folder).Error == nil {
		locked = folder.Locked
	}

	c.JSON(200, gin.H{
		"conversation_id": convID,
		"title":           title,
		"files":           files,
		"total":           total,
		"page":            page,
		"page_size":       pageSize,
		"locked":          locked,
	})
}

// DeleteFolder deletes all files in a conversation folder (DB + disk)
func (h *WorkspaceHandler) DeleteFolder(c *gin.Context) {
	userID := c.GetString("user_id")
	convID := c.Param("conv_id")

	if convID == "_untracked" {
		c.JSON(400, gin.H{"error": "不能删除未归类文件夹，请逐个删除文件"})
		return
	}

	// Check lock
	var folder model.WorkspaceFolder
	if h.db.Where("conversation_id = ? AND user_id = ?", convID, userID).First(&folder).Error == nil {
		if folder.Locked {
			c.JSON(403, gin.H{"error": "此文件夹已锁定，请先解锁再删除"})
			return
		}
	}

	// Get all files for this conversation
	var files []model.WorkspaceFile
	h.db.Where("user_id = ? AND conversation_id = ?", userID, convID).Find(&files)

	// Delete from filesystem
	deletedCount := 0
	for _, f := range files {
		absPath := filepath.Join(h.baseDir, f.WorkspaceID, f.Path)
		if err := os.Remove(absPath); err == nil {
			deletedCount++
		}
	}

	// Try to remove the conversation directory (will only succeed if empty or all files deleted)
	// Find workspace IDs used
	wsIDs := map[string]bool{}
	for _, f := range files {
		wsIDs[f.WorkspaceID] = true
	}
	for wsID := range wsIDs {
		convDir := filepath.Join(h.baseDir, wsID, convID)
		os.Remove(convDir) // Remove dir if empty, ignore error
	}

	// Delete DB records
	h.db.Where("user_id = ? AND conversation_id = ?", userID, convID).Delete(&model.WorkspaceFile{})

	// Delete folder lock record if exists
	h.db.Where("user_id = ? AND conversation_id = ?", userID, convID).Delete(&model.WorkspaceFolder{})

	log.Printf("[workspace] deleted folder %s: %d files removed", convID, deletedCount)
	c.JSON(200, gin.H{
		"message":       "文件夹已删除",
		"deleted_files": deletedCount,
	})
}

// LockFolder locks a conversation folder to prevent deletion
func (h *WorkspaceHandler) LockFolder(c *gin.Context) {
	userID := c.GetString("user_id")
	convID := c.Param("conv_id")
	now := time.Now()

	var folder model.WorkspaceFolder
	result := h.db.Where("conversation_id = ? AND user_id = ?", convID, userID).First(&folder)
	if result.Error != nil {
		// Create new
		folder = model.WorkspaceFolder{
			UserID:         userID,
			ConversationID: convID,
			Locked:         true,
			LockedAt:       &now,
		}
		h.db.Create(&folder)
	} else {
		h.db.Model(&folder).Updates(map[string]interface{}{
			"locked":    true,
			"locked_at": &now,
		})
	}
	c.JSON(200, gin.H{"message": "已锁定", "locked": true})
}

// UnlockFolder unlocks a conversation folder
func (h *WorkspaceHandler) UnlockFolder(c *gin.Context) {
	userID := c.GetString("user_id")
	convID := c.Param("conv_id")

	h.db.Model(&model.WorkspaceFolder{}).
		Where("conversation_id = ? AND user_id = ?", convID, userID).
		Updates(map[string]interface{}{
			"locked":    false,
			"locked_at": nil,
		})
	c.JSON(200, gin.H{"message": "已解锁", "locked": false})
}

func isCodeExt(ext string) bool {
	codeExts := map[string]bool{
		".py": true, ".js": true, ".ts": true, ".jsx": true, ".tsx": true,
		".go": true, ".java": true, ".c": true, ".cpp": true, ".h": true,
		".rs": true, ".rb": true, ".php": true, ".sh": true, ".bat": true,
		".css": true, ".scss": true, ".vue": true, ".svelte": true,
		".json": true, ".yaml": true, ".yml": true, ".toml": true, ".xml": true,
		".html": true, ".htm": true, ".sql": true,
	}
	return codeExts[ext]
}
