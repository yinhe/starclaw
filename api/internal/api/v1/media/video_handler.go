package media

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw/internal/model"
	"github.com/yinhe/starclaw/internal/tool"
	"gorm.io/gorm"
)

// VideoHandler handles video gallery CRUD and operations (dub, add-music, retry, etc.)
type VideoHandler struct {
	db           *gorm.DB
	toolRegistry *tool.Registry
}

func NewVideoHandler(db *gorm.DB, toolRegistry *tool.Registry) *VideoHandler {
	return &VideoHandler{db: db, toolRegistry: toolRegistry}
}

func (h *VideoHandler) List(c *gin.Context) {
	userID := c.GetString("user_id")
	var records []model.VideoRecord
	h.db.Where("user_id = ?", userID).Order("created_at DESC").Limit(500).Find(&records)
	c.JSON(200, gin.H{"videos": records})
}

func (h *VideoHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("user_id")
	result := h.db.Where("id = ? AND user_id = ?", id, userID).Delete(&model.VideoRecord{})
	if result.RowsAffected == 0 {
		c.JSON(404, gin.H{"error": "video not found"})
		return
	}
	c.JSON(200, gin.H{"message": "deleted"})
}

func (h *VideoHandler) Cancel(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("user_id")
	result := h.db.Model(&model.VideoRecord{}).
		Where("id = ? AND user_id = ? AND status IN ?", id, userID, []string{"running", "pending"}).
		Update("status", "cancelled")
	if result.RowsAffected == 0 {
		c.JSON(400, gin.H{"error": "video not found or cannot be cancelled"})
		return
	}
	c.JSON(200, gin.H{"status": "cancelled"})
}

func (h *VideoHandler) Retry(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("user_id")
	var rec model.VideoRecord
	if err := h.db.Where("id = ? AND user_id = ? AND status IN ?", id, userID, []string{"failed", "cancelled"}).First(&rec).Error; err != nil {
		c.JSON(400, gin.H{"error": "video not found or cannot be retried"})
		return
	}
	videoTool, ok := h.toolRegistry.Get("video_generation")
	if !ok {
		c.JSON(500, gin.H{"error": "video tool not available"})
		return
	}
	// Reset status
	h.db.Model(&rec).Updates(map[string]interface{}{"status": "running", "video_url": ""})
	// Re-generate in background
	go func() {
		argsJSON := fmt.Sprintf(`{"action":"generate_video","prompt":%q,"model":%q,"size":%q,"duration":"%d","scene":%q}`,
			rec.Prompt, rec.Model, rec.Size, rec.Duration, rec.Scene)
		ctx := context.WithValue(context.Background(), tool.CtxKeyUserID, rec.UserID)
		ctx = context.WithValue(ctx, tool.CtxKeyConversationID, rec.ConversationID)
		// Delete old record before re-generating (new one will be created)
		h.db.Where("id = ?", rec.ID).Delete(&model.VideoRecord{})
		videoTool.Execute(ctx, argsJSON)
	}()
	c.JSON(200, gin.H{"status": "retrying"})
}

func (h *VideoHandler) Regenerate(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("user_id")
	var rec model.VideoRecord
	if err := h.db.Where("id = ? AND user_id = ?", id, userID).First(&rec).Error; err != nil {
		c.JSON(400, gin.H{"error": "video not found"})
		return
	}
	if rec.Type == "merged" || rec.Type == "mv" || rec.Type == "narrated" {
		c.JSON(400, gin.H{"error": "cannot regenerate a merged/mv/narrated video, use remerge instead"})
		return
	}
	videoTool, ok := h.toolRegistry.Get("video_generation")
	if !ok {
		c.JSON(500, gin.H{"error": "video tool not available"})
		return
	}
	// Delete old record and re-generate with same params
	oldConvID := rec.ConversationID
	oldUserID := rec.UserID
	go func() {
		argsJSON := fmt.Sprintf(`{"action":"generate_video","prompt":%q,"model":%q,"size":%q,"duration":"%d","scene":%q}`,
			rec.Prompt, rec.Model, rec.Size, rec.Duration, rec.Scene)
		ctx := context.WithValue(context.Background(), tool.CtxKeyUserID, oldUserID)
		ctx = context.WithValue(ctx, tool.CtxKeyConversationID, oldConvID)
		h.db.Where("id = ?", rec.ID).Delete(&model.VideoRecord{})
		videoTool.Execute(ctx, argsJSON)
	}()
	c.JSON(200, gin.H{"status": "regenerating", "message": "片段正在重新生成"})
}

func (h *VideoHandler) Remerge(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("user_id")
	var rec model.VideoRecord
	if err := h.db.Where("id = ? AND user_id = ? AND type IN ?", id, userID, []string{"merged", "mv"}).First(&rec).Error; err != nil {
		c.JSON(400, gin.H{"error": "merged video not found"})
		return
	}
	videoTool, ok := h.toolRegistry.Get("video_generation")
	if !ok {
		c.JSON(500, gin.H{"error": "video tool not available"})
		return
	}
	convID := rec.ConversationID
	if convID == "" {
		c.JSON(400, gin.H{"error": "no conversation_id on this merged video"})
		return
	}
	// Parse original clip IDs from the merged record
	var clipIDs []string
	if err := json.Unmarshal([]byte(rec.ClipIDs), &clipIDs); err != nil || len(clipIDs) == 0 {
		c.JSON(400, gin.H{"error": "no clip_ids found in merged video"})
		return
	}
	// Get task_ids for these specific clips
	var clips []model.VideoRecord
	h.db.Where("id IN ?", clipIDs).Find(&clips)
	var taskIDs []string
	for _, clip := range clips {
		if clip.TaskID != "" {
			taskIDs = append(taskIDs, clip.TaskID)
		}
	}
	if len(taskIDs) == 0 {
		c.JSON(400, gin.H{"error": "no valid clips found for remerge"})
		return
	}
	// Delete old merged record
	h.db.Where("id = ?", rec.ID).Delete(&model.VideoRecord{})
	// Re-merge in background with specific task_ids
	taskIDsStr := strings.Join(taskIDs, ",")
	go func() {
		argsJSON := fmt.Sprintf(`{"action":"merge_videos","task_ids":%q}`, taskIDsStr)
		ctx := context.WithValue(context.Background(), tool.CtxKeyUserID, userID)
		ctx = context.WithValue(ctx, tool.CtxKeyConversationID, convID)
		videoTool.Execute(ctx, argsJSON)
	}()
	c.JSON(200, gin.H{"status": "remerging", "message": "正在重新合成视频"})
}

func (h *VideoHandler) Dub(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("user_id")

	var req struct {
		Text          string `json:"text"`
		Voice         string `json:"voice"`
		SubtitleStyle string `json:"subtitle_style"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Text == "" {
		c.JSON(400, gin.H{"error": "text (配音文案) is required"})
		return
	}

	var rec model.VideoRecord
	if err := h.db.Where("id = ? AND user_id = ?", id, userID).First(&rec).Error; err != nil {
		c.JSON(400, gin.H{"error": "video not found"})
		return
	}
	if rec.Status != "succeeded" || rec.VideoURL == "" {
		c.JSON(400, gin.H{"error": "video not ready yet"})
		return
	}

	dubbingTool, ok := h.toolRegistry.Get("dubbing")
	if !ok {
		c.JSON(500, gin.H{"error": "dubbing tool not available"})
		return
	}

	// Auto-split text into timed segments
	segments := tool.SplitNarrationToSegments(req.Text, 0, float64(rec.Duration), 15)
	segJSON, _ := json.Marshal(segments)

	voice := req.Voice
	if voice == "" {
		voice = "longyuan"
	}
	subStyle := req.SubtitleStyle
	if subStyle == "" {
		subStyle = "auto"
	}

	go func() {
		argsJSON := fmt.Sprintf(`{"action":"add_voiceover","video_id":%q,"narrations":%q,"voice":%q,"subtitle_style":%q}`,
			rec.ID, string(segJSON), voice, subStyle)
		ctx := context.WithValue(context.Background(), tool.CtxKeyUserID, userID)
		ctx = context.WithValue(ctx, tool.CtxKeyConversationID, rec.ConversationID)
		result, err := dubbingTool.Execute(ctx, argsJSON)
		if err != nil {
			log.Printf("[DubAPI] failed for video %s: %v", rec.ID, err)
		} else {
			log.Printf("[DubAPI] success for video %s: %s", rec.ID, result)
		}
	}()

	c.JSON(200, gin.H{
		"status":   "dubbing",
		"message":  fmt.Sprintf("配音任务已开始，音色: %s，共%d段旁白", voice, len(segments)),
		"segments": len(segments),
	})
}

func (h *VideoHandler) AddMusic(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("user_id")

	var req struct {
		MusicID       string `json:"music_id"`
		LyricsSRT     string `json:"lyrics_srt"`
		SubtitleStyle string `json:"subtitle_style"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.MusicID == "" {
		c.JSON(400, gin.H{"error": "music_id is required"})
		return
	}

	var rec model.VideoRecord
	if err := h.db.Where("id = ? AND user_id = ?", id, userID).First(&rec).Error; err != nil {
		c.JSON(400, gin.H{"error": "video not found"})
		return
	}
	if rec.Status != "succeeded" || rec.VideoURL == "" {
		c.JSON(400, gin.H{"error": "video not ready yet"})
		return
	}

	mvTool, ok := h.toolRegistry.Get("mv_production")
	if !ok {
		c.JSON(500, gin.H{"error": "mv tool not available"})
		return
	}

	go func() {
		argsJSON := fmt.Sprintf(`{"action":"compose_mv","video_id":%q,"music_id":%q,"lyrics_srt":%q,"subtitle_style":%q}`,
			rec.ID, req.MusicID, req.LyricsSRT, req.SubtitleStyle)
		ctx := context.WithValue(context.Background(), tool.CtxKeyUserID, userID)
		ctx = context.WithValue(ctx, tool.CtxKeyConversationID, rec.ConversationID)
		result, err := mvTool.Execute(ctx, argsJSON)
		if err != nil {
			log.Printf("[AddMusicAPI] failed for video %s: %v", rec.ID, err)
		} else {
			log.Printf("[AddMusicAPI] success for video %s: %s", rec.ID, result)
		}
	}()

	c.JSON(200, gin.H{"status": "composing", "message": "正在合成配乐视频"})
}

func (h *VideoHandler) ListVoices(c *gin.Context) {
	dubbingTool, ok := h.toolRegistry.Get("dubbing")
	if !ok {
		c.JSON(500, gin.H{"error": "dubbing tool not available"})
		return
	}
	result, err := dubbingTool.Execute(context.Background(), `{"action":"list_voices"}`)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	var data map[string]interface{}
	json.Unmarshal([]byte(result), &data)
	c.JSON(200, data)
}
