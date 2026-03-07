package v1

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw/internal/model"
	"gorm.io/gorm"
)

// WorkerController allows the handler to control the TaskWorker
type WorkerController interface {
	Pause()
	Resume()
	Stop()
	IsPaused() bool
}

type TaskHandler struct {
	db     *gorm.DB
	worker WorkerController
}

func NewTaskHandler(db *gorm.DB, w WorkerController) *TaskHandler {
	return &TaskHandler{db: db, worker: w}
}

// ListTasks returns user's tasks with optional status filter
func (h *TaskHandler) ListTasks(c *gin.Context) {
	userID := c.GetString("user_id")
	status := c.Query("status") // optional filter

	var tasks []model.Task
	q := h.db.Where("user_id = ?", userID).Order("created_at DESC").Limit(50)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if err := q.Find(&tasks).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"tasks": tasks})
}

// GetTask returns a single task by ID
func (h *TaskHandler) GetTask(c *gin.Context) {
	userID := c.GetString("user_id")
	taskID := c.Param("id")

	var task model.Task
	if err := h.db.Where("id = ? AND user_id = ?", taskID, userID).First(&task).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return
	}

	// Also get sub-tasks
	var subTasks []model.Task
	h.db.Where("parent_task_id = ?", taskID).Order("created_at ASC").Find(&subTasks)

	c.JSON(http.StatusOK, gin.H{"task": task, "sub_tasks": subTasks})
}

// CreateTask allows user to directly create a background task
func (h *TaskHandler) CreateTask(c *gin.Context) {
	userID := c.GetString("user_id")

	var req struct {
		Title          string `json:"title" binding:"required"`
		Goal           string `json:"goal" binding:"required"`
		AgentID        string `json:"agent_id"`
		ConversationID string `json:"conversation_id"`
		Priority       string `json:"priority"`
		ScheduledAt    string `json:"scheduled_at"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	task := model.Task{
		UserID:         userID,
		AgentID:        req.AgentID,
		ConversationID: req.ConversationID,
		Title:          req.Title,
		Goal:           req.Goal,
		Priority:       model.TaskPriority(req.Priority),
	}
	if task.Priority == "" {
		task.Priority = model.TaskPriorityNormal
	}
	if req.ScheduledAt != "" {
		if scheduled, err := time.Parse(time.RFC3339, req.ScheduledAt); err == nil {
			task.ScheduledAt = &scheduled
			task.Status = model.TaskStatusWaiting
		}
	}

	if err := h.db.Create(&task).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"task": task})
}

// CancelTask cancels a pending/waiting/running task
func (h *TaskHandler) CancelTask(c *gin.Context) {
	userID := c.GetString("user_id")
	taskID := c.Param("id")

	result := h.db.Model(&model.Task{}).
		Where("id = ? AND user_id = ? AND status IN ?", taskID, userID, []string{"pending", "waiting", "running"}).
		Updates(map[string]interface{}{
			"status":     model.TaskStatusCancelled,
			"updated_at": time.Now(),
		})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "task not found or cannot be cancelled"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "cancelled"})
}

// PauseTask pauses a single running/pending task (sets status to waiting)
func (h *TaskHandler) PauseTask(c *gin.Context) {
	userID := c.GetString("user_id")
	taskID := c.Param("id")

	result := h.db.Model(&model.Task{}).
		Where("id = ? AND user_id = ? AND status IN ?", taskID, userID, []string{"running", "pending"}).
		Updates(map[string]interface{}{
			"status":     model.TaskStatusWaiting,
			"updated_at": time.Now(),
		})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "task not found or cannot be paused"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "waiting"})
}

// ResumeTask resumes a waiting task (sets status to pending)
func (h *TaskHandler) ResumeTask(c *gin.Context) {
	userID := c.GetString("user_id")
	taskID := c.Param("id")

	result := h.db.Model(&model.Task{}).
		Where("id = ? AND user_id = ? AND status = ?", taskID, userID, "waiting").
		Updates(map[string]interface{}{
			"status":     model.TaskStatusPending,
			"updated_at": time.Now(),
		})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "task not found or not paused"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "pending"})
}

// ListNotifications returns user's notifications
func (h *TaskHandler) ListNotifications(c *gin.Context) {
	userID := c.GetString("user_id")
	unreadOnly := c.Query("unread") == "true"

	var notifications []model.Notification
	q := h.db.Where("user_id = ?", userID).Order("created_at DESC").Limit(50)
	if unreadOnly {
		q = q.Where("is_read = ?", false)
	}
	if err := q.Find(&notifications).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Count unread
	var unreadCount int64
	h.db.Model(&model.Notification{}).Where("user_id = ? AND is_read = ?", userID, false).Count(&unreadCount)

	c.JSON(http.StatusOK, gin.H{
		"notifications": notifications,
		"unread_count":  unreadCount,
	})
}

// MarkNotificationsRead marks notifications as read
func (h *TaskHandler) MarkNotificationsRead(c *gin.Context) {
	userID := c.GetString("user_id")

	var req struct {
		IDs []string `json:"ids"` // empty = mark all
	}
	c.ShouldBindJSON(&req)

	q := h.db.Model(&model.Notification{}).Where("user_id = ? AND is_read = ?", userID, false)
	if len(req.IDs) > 0 {
		q = q.Where("id IN ?", req.IDs)
	}
	q.Update("is_read", true)

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// UnreadCount returns just the unread notification count (lightweight polling)
func (h *TaskHandler) UnreadCount(c *gin.Context) {
	userID := c.GetString("user_id")
	var count int64
	h.db.Model(&model.Notification{}).Where("user_id = ? AND is_read = ?", userID, false).Count(&count)
	c.JSON(http.StatusOK, gin.H{"unread_count": count})
}

// WorkerPause pauses the task worker
func (h *TaskHandler) WorkerPause(c *gin.Context) {
	if h.worker != nil {
		h.worker.Pause()
	}
	c.JSON(http.StatusOK, gin.H{"status": "paused"})
}

// WorkerResume resumes the task worker
func (h *TaskHandler) WorkerResume(c *gin.Context) {
	if h.worker != nil {
		h.worker.Resume()
	}
	c.JSON(http.StatusOK, gin.H{"status": "running"})
}

// WorkerStop stops all running tasks and pauses
func (h *TaskHandler) WorkerStop(c *gin.Context) {
	userID := c.GetString("user_id")
	if h.worker != nil {
		h.worker.Pause()
	}
	// Cancel all running/pending tasks for this user
	now := time.Now()
	h.db.Model(&model.Task{}).Where("user_id = ? AND status IN ?", userID, []string{"running", "pending", "waiting"}).
		Updates(map[string]interface{}{"status": model.TaskStatusCancelled, "updated_at": now})
	c.JSON(http.StatusOK, gin.H{"status": "stopped"})
}

// WorkerStatus returns current worker state
func (h *TaskHandler) WorkerStatus(c *gin.Context) {
	paused := false
	if h.worker != nil {
		paused = h.worker.IsPaused()
	}
	c.JSON(http.StatusOK, gin.H{"paused": paused})
}

// Visualization returns full topology data for the real-time visualization dashboard
func (h *TaskHandler) Visualization(c *gin.Context) {
	userID := c.GetString("user_id")
	conversationID := c.Query("conversation_id")

	// Get all agents for this user
	var agents []model.Agent
	h.db.Where("user_id = ?", userID).Find(&agents)

	// Get recent tasks  filtered by conversation_id if provided
	var tasks []model.Task
	taskQ := h.db.Where("user_id = ?", userID)
	if conversationID != "" {
		taskQ = taskQ.Where("conversation_id = ?", conversationID)
	}
	taskQ.Order("created_at DESC").Limit(100).Find(&tasks)

	// Get conversations that have tasks (for frontend selector)
	type ConvSummary struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	}
	var convSummaries []ConvSummary
	h.db.Raw(`SELECT DISTINCT c.id, c.title FROM conversations c 
		INNER JOIN tasks t ON t.conversation_id = c.id 
		WHERE t.user_id = ? AND t.deleted_at IS NULL AND c.deleted_at IS NULL
		ORDER BY c.updated_at DESC LIMIT 20`, userID).Scan(&convSummaries)

	// Build stats
	stats := gin.H{
		"total":     len(tasks),
		"running":   0,
		"pending":   0,
		"completed": 0,
		"failed":    0,
	}
	for _, t := range tasks {
		switch t.Status {
		case model.TaskStatusRunning:
			stats["running"] = stats["running"].(int) + 1
		case model.TaskStatusPending, model.TaskStatusWaiting:
			stats["pending"] = stats["pending"].(int) + 1
		case model.TaskStatusCompleted:
			stats["completed"] = stats["completed"].(int) + 1
		case model.TaskStatusFailed:
			stats["failed"] = stats["failed"].(int) + 1
		}
	}

	// Build edges: task ↀagent relationships
	type Edge struct {
		From   string `json:"from"`
		To     string `json:"to"`
		TaskID string `json:"task_id"`
		Status string `json:"status"`
		Label  string `json:"label"`
	}
	var edges []Edge
	for _, t := range tasks {
		if t.AgentID != "" {
			edges = append(edges, Edge{
				From:   "orchestrator",
				To:     t.AgentID,
				TaskID: t.ID,
				Status: string(t.Status),
				Label:  t.Title,
			})
		}
		if t.ParentTaskID != "" {
			edges = append(edges, Edge{
				From:   t.ParentTaskID,
				To:     t.ID,
				TaskID: t.ID,
				Status: string(t.Status),
				Label:  t.Title,
			})
		}
	}

	// Recent notifications for activity feed
	var notifications []model.Notification
	h.db.Where("user_id = ?", userID).Order("created_at DESC").Limit(10).Find(&notifications)

	workerPaused := false
	if h.worker != nil {
		workerPaused = h.worker.IsPaused()
	}

	c.JSON(http.StatusOK, gin.H{
		"agents":          agents,
		"tasks":           tasks,
		"edges":           edges,
		"stats":           stats,
		"notifications":   notifications,
		"worker_paused":   workerPaused,
		"conversations":   convSummaries,
		"conversation_id": conversationID,
	})
}
