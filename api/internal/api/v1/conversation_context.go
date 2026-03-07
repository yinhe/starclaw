package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw/internal/model"
	"gorm.io/gorm"
)

type ConversationContextHandler struct {
	db *gorm.DB
}

func NewConversationContextHandler(db *gorm.DB) *ConversationContextHandler {
	return &ConversationContextHandler{db: db}
}

type taskSummary struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Status      string  `json:"status"`
	Priority    string  `json:"priority"`
	Progress    int     `json:"progress"`
	AgentID     string  `json:"agent_id"`
	CreatedAt   string  `json:"created_at"`
	CompletedAt *string `json:"completed_at,omitempty"`
}

type workflowSummary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type videoSummary struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Status      string `json:"status"`
	Prompt      string `json:"prompt"`
	VideoURL    string `json:"video_url"`
	NarratedURL string `json:"narrated_url,omitempty"`
	Duration    int    `json:"duration"`
	Scene       string `json:"scene"`
	CreatedAt   string `json:"created_at"`
}

type musicSummary struct {
	ID        string `json:"id"`
	Model     string `json:"model"`
	Status    string `json:"status"`
	Prompt    string `json:"prompt"`
	AudioURL  string `json:"audio_url,omitempty"`
	Duration  int    `json:"duration"`
	CreatedAt string `json:"created_at"`
}

type contextResponse struct {
	ConversationID string            `json:"conversation_id"`
	Tasks          []taskSummary     `json:"tasks"`
	Workflows      []workflowSummary `json:"workflows"`
	Videos         []videoSummary    `json:"videos"`
	Music          []musicSummary    `json:"music"`
	Stats          contextStats      `json:"stats"`
}

type contextStats struct {
	TasksTotal     int `json:"tasks_total"`
	TasksRunning   int `json:"tasks_running"`
	TasksCompleted int `json:"tasks_completed"`
	TasksFailed    int `json:"tasks_failed"`
	WorkflowsTotal int `json:"workflows_total"`
	VideosTotal    int `json:"videos_total"`
	VideosMerged   int `json:"videos_merged"`
	VideosNarrated int `json:"videos_narrated"`
	MusicTotal     int `json:"music_total"`
}

// GetContext returns all related tasks, workflows, and videos for a conversation
func (h *ConversationContextHandler) GetContext(c *gin.Context) {
	userID := c.GetString("user_id")
	convID := c.Param("id")

	resp := contextResponse{
		ConversationID: convID,
		Tasks:          []taskSummary{},
		Workflows:      []workflowSummary{},
		Videos:         []videoSummary{},
		Music:          []musicSummary{},
	}

	// Fetch related tasks
	var tasks []model.Task
	h.db.Where("user_id = ? AND conversation_id = ?", userID, convID).
		Order("created_at DESC").Limit(50).Find(&tasks)

	for _, t := range tasks {
		ts := taskSummary{
			ID:        t.ID,
			Title:     t.Title,
			Status:    string(t.Status),
			Priority:  string(t.Priority),
			Progress:  t.Progress,
			AgentID:   t.AgentID,
			CreatedAt: t.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}
		if t.CompletedAt != nil {
			s := t.CompletedAt.Format("2006-01-02T15:04:05Z")
			ts.CompletedAt = &s
		}
		resp.Tasks = append(resp.Tasks, ts)

		resp.Stats.TasksTotal++
		switch t.Status {
		case model.TaskStatusRunning:
			resp.Stats.TasksRunning++
		case model.TaskStatusCompleted:
			resp.Stats.TasksCompleted++
		case model.TaskStatusFailed:
			resp.Stats.TasksFailed++
		}
	}

	// Fetch related workflows
	var workflows []model.Workflow
	h.db.Where("user_id = ? AND conversation_id = ?", userID, convID).
		Order("created_at DESC").Limit(20).Find(&workflows)

	for _, w := range workflows {
		resp.Workflows = append(resp.Workflows, workflowSummary{
			ID:          w.ID,
			Name:        w.Name,
			Description: w.Description,
			CreatedAt:   w.CreatedAt.Format("2006-01-02T15:04:05Z"),
			UpdatedAt:   w.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		})
		resp.Stats.WorkflowsTotal++
	}

	// Fetch related videos
	var videos []model.VideoRecord
	h.db.Where("user_id = ? AND conversation_id = ?", userID, convID).
		Order("scene ASC, created_at DESC").Limit(50).Find(&videos)

	for _, v := range videos {
		vs := videoSummary{
			ID:        v.ID,
			Type:      v.Type,
			Status:    v.Status,
			Prompt:    v.Prompt,
			VideoURL:  v.VideoURL,
			Duration:  v.Duration,
			Scene:     v.Scene,
			CreatedAt: v.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}
		if v.NarratedURL != "" {
			vs.NarratedURL = v.NarratedURL
		}
		resp.Videos = append(resp.Videos, vs)

		resp.Stats.VideosTotal++
		if v.Type == "merged" {
			resp.Stats.VideosMerged++
		}
		if v.NarratedURL != "" {
			resp.Stats.VideosNarrated++
		}
	}

	// Fetch related music
	var musicRecords []model.MusicRecord
	h.db.Where("user_id = ? AND conversation_id = ?", userID, convID).
		Order("created_at DESC").Limit(20).Find(&musicRecords)

	for _, m := range musicRecords {
		ms := musicSummary{
			ID:        m.ID,
			Model:     m.Model,
			Status:    m.Status,
			Prompt:    m.Prompt,
			Duration:  m.Duration,
			CreatedAt: m.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}
		if m.LocalURL != "" {
			ms.AudioURL = m.LocalURL
		} else if m.AudioURL != "" {
			ms.AudioURL = m.AudioURL
		}
		resp.Music = append(resp.Music, ms)
		resp.Stats.MusicTotal++
	}

	c.JSON(http.StatusOK, resp)
}
