package media

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
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
	q := h.db.Where("user_id = ?", userID)
	if scene := c.Query("scene"); scene != "" {
		q = q.Where("scene = ?", scene)
	}
	if status := c.Query("status"); status != "" {
		q = q.Where("status = ?", status)
	}
	if model := c.Query("model"); model != "" {
		q = q.Where("model = ?", model)
	}
	if taskID := c.Query("task_id"); taskID != "" {
		q = q.Where("task_id = ?", taskID)
	}
	var records []model.VideoRecord
	q.Order("created_at DESC").Limit(500).Find(&records)
	c.JSON(200, gin.H{"videos": records})
}

func (h *VideoHandler) Generate(c *gin.Context) {
	userID := c.GetString("user_id")
	var req struct {
		Prompt          string `json:"prompt"`
		Model           string `json:"model"`
		ImgURL          string `json:"img_url"`
		Size            string `json:"size"`
		Resolution      string `json:"resolution"` // 480p/720p/1080p (Seedance) or 720P/1080P (wan2.7)
		Ratio           string `json:"ratio"`      // 21:9/16:9/4:3/1:1/3:4/9:16 (Seedance only)
		Duration        int    `json:"duration"`
		Scene           string `json:"scene"`
		StylePrefix     string `json:"style_prefix"`
		RefVideoID      string `json:"ref_video_id"`
		RefVideoURL     string `json:"ref_video_url"`
		RefAudioURL     string `json:"ref_audio_url"`
		GenerateAudio   *bool  `json:"generate_audio"`
		Watermark       *bool  `json:"watermark"`
		ReturnLastFrame *bool  `json:"return_last_frame"`
		Category        string `json:"category"`
		ConversationID  string `json:"conversation_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request body", "detail": err.Error()})
		return
	}
	if strings.TrimSpace(req.Prompt) == "" {
		c.JSON(400, gin.H{"error": "prompt is required"})
		return
	}
	videoTool, ok := h.toolRegistry.Get("video_generation")
	if !ok {
		c.JSON(500, gin.H{"error": "video tool not available"})
		return
	}
	duration := req.Duration
	if duration == 0 || duration < -1 {
		duration = 5
	}
	args := map[string]interface{}{
		"action":       "generate_video",
		"prompt":       req.Prompt,
		"model":        req.Model,
		"img_url":      req.ImgURL,
		"size":         req.Size,
		"resolution":   req.Resolution,
		"ratio":        req.Ratio,
		"duration":     fmt.Sprintf("%d", duration),
		"scene":        req.Scene,
		"style_prefix": req.StylePrefix,
		"ref_video_id": req.RefVideoID,
		"category":     req.Category,
	}
	if strings.TrimSpace(req.RefVideoURL) != "" {
		args["ref_video_url"] = strings.TrimSpace(req.RefVideoURL)
	}
	if strings.TrimSpace(req.RefAudioURL) != "" {
		args["ref_audio_url"] = strings.TrimSpace(req.RefAudioURL)
	}
	if req.GenerateAudio != nil {
		args["generate_audio"] = *req.GenerateAudio
	}
	if req.Watermark != nil {
		args["watermark"] = *req.Watermark
	}
	if req.ReturnLastFrame != nil {
		args["return_last_frame"] = *req.ReturnLastFrame
	}
	argsBytes, err := json.Marshal(args)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to marshal video args"})
		return
	}
	argsJSON := string(argsBytes)
	ctx := context.WithValue(context.Background(), tool.CtxKeyUserID, userID)
	if strings.TrimSpace(req.ConversationID) != "" {
		ctx = context.WithValue(ctx, tool.CtxKeyConversationID, strings.TrimSpace(req.ConversationID))
	}
	result, err := videoTool.Execute(ctx, argsJSON)
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

	// Look up record to get task_id and model for Ark cancellation
	var rec model.VideoRecord
	if err := h.db.Where("id = ? AND user_id = ? AND status IN ?", id, userID, []string{"running", "pending"}).First(&rec).Error; err != nil {
		c.JSON(400, gin.H{"error": "video not found or cannot be cancelled"})
		return
	}

	// Try to cancel via Ark DELETE API for Seedance tasks (only works for queued tasks)
	arkCancelled := false
	if strings.HasPrefix(rec.Model, "doubao-seedance-") && rec.TaskID != "" {
		apiKey, baseURL := tool.GetVolcengineAPIKey(h.db, userID)
		if apiKey != "" {
			go func() {
				cancelArkTask(apiKey, baseURL, rec.TaskID)
			}()
			arkCancelled = true
		}
	}

	// Update local DB status
	h.db.Model(&model.VideoRecord{}).Where("id = ?", id).Update("status", "cancelled")
	c.JSON(200, gin.H{"status": "cancelled", "ark_cancel_attempted": arkCancelled})
}

// cancelArkTask sends DELETE to Volcengine Ark API to cancel a queued task.
// Running tasks cannot be cancelled via the API — this is best-effort.
func cancelArkTask(apiKey, baseURL, taskID string) {
	reqURL := strings.TrimRight(baseURL, "/") + "/contents/generations/tasks/" + taskID
	req, err := http.NewRequest("DELETE", reqURL, nil)
	if err != nil {
		log.Printf("[cancelArkTask] build request failed: %v", err)
		return
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("[cancelArkTask] DELETE %s failed: %v", taskID, err)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	log.Printf("[cancelArkTask] DELETE %s → HTTP %d: %s", taskID, resp.StatusCode, string(body))
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

// MergeByTaskIDs merges video clips identified by their task_ids into a single video.
// POST /v1/videos/merge  { "task_ids": ["tid1","tid2",...], "episode": "EP05 夜袭", "season": 1, "episode_number": 5, "title": "夜袭" }
func (h *VideoHandler) MergeByTaskIDs(c *gin.Context) {
	userID := c.GetString("user_id")
	var req struct {
		TaskIDs       []string `json:"task_ids"`
		Episode       string   `json:"episode"`
		Season        int      `json:"season"`
		EpisodeNumber int      `json:"episode_number"`
		Title         string   `json:"title"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.TaskIDs) < 2 {
		c.JSON(400, gin.H{"error": "task_ids (at least 2) is required"})
		return
	}
	videoTool, ok := h.toolRegistry.Get("video_generation")
	if !ok {
		c.JSON(500, gin.H{"error": "video tool not available"})
		return
	}
	taskIDsStr := strings.Join(req.TaskIDs, ",")
	// Use episode as a synthetic conversation ID for grouping
	convID := fmt.Sprintf("workflow-%s", req.Episode)
	go func() {
		argsJSON := fmt.Sprintf(`{"action":"merge_videos","task_ids":%q}`, taskIDsStr)
		ctx := context.WithValue(context.Background(), tool.CtxKeyUserID, userID)
		ctx = context.WithValue(ctx, tool.CtxKeyConversationID, convID)
		result, err := videoTool.Execute(ctx, argsJSON)
		if err != nil {
			log.Printf("[MergeAPI] merge failed for %s: %v", req.Episode, err)
			return
		}
		log.Printf("[MergeAPI] merge success for %s: %s", req.Episode, result)

		// Apply title cards if episode metadata provided
		if req.Season > 0 && req.EpisodeNumber > 0 && req.Title != "" {
			var res map[string]interface{}
			if json.Unmarshal([]byte(result), &res) == nil {
				if dlURL, _ := res["download_url"].(string); dlURL != "" {
					mergedPath := filepath.Join(tool.MergedVideosDir(), filepath.Base(dlURL))
					titledPath := mergedPath + ".titled.mp4"
					if err := tool.FfmpegAddTitleCards(ctx, mergedPath, titledPath, req.Season, req.EpisodeNumber, req.Title); err != nil {
						log.Printf("[MergeAPI] title cards failed: %v", err)
					} else {
						// Replace merged file with titled version
						os.Remove(mergedPath)
						os.Rename(titledPath, mergedPath)
						log.Printf("[MergeAPI] title cards applied to %s", mergedPath)
					}
				}
			}
		}
	}()
	c.JSON(200, gin.H{
		"status":  "merging",
		"message": fmt.Sprintf("正在合成 %d 个片段为成片（含片头字幕）", len(req.TaskIDs)),
		"conv_id": convID,
	})
}

func (h *VideoHandler) Dub(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("user_id")

	var req struct {
		Text          string          `json:"text"`
		Narrations    json.RawMessage `json:"narrations"`
		Voice         string          `json:"voice"`
		SubtitleStyle string          `json:"subtitle_style"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request body"})
		return
	}
	if strings.TrimSpace(req.Text) == "" && len(req.Narrations) == 0 {
		c.JSON(400, gin.H{"error": "text or narrations is required"})
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

	var segments []tool.NarrationSegment
	if len(req.Narrations) > 0 {
		if err := json.Unmarshal(req.Narrations, &segments); err != nil || len(segments) == 0 {
			c.JSON(400, gin.H{"error": "invalid narrations"})
			return
		}
	} else {
		segments = tool.SplitNarrationToSegments(req.Text, 0, float64(rec.Duration), 15)
	}
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
		"message":  fmt.Sprintf("配音任务已开始，默认音色: %s，共%d段配音", voice, len(segments)),
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
