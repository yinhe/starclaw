package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	agentpkg "github.com/yinhe/starclaw/internal/agent"
	"github.com/yinhe/starclaw/internal/model"
	"github.com/yinhe/starclaw/internal/provider"
	"github.com/yinhe/starclaw/internal/tool"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TaskWorker is the 24/7 background processor that picks up tasks and runs them via Agent Runtime
type TaskWorker struct {
	db               *gorm.DB
	providerRegistry *provider.Registry
	toolRegistry     *tool.Registry
	concurrency      int
	stopCh           chan struct{}
	wg               sync.WaitGroup
	paused           bool
	mu               sync.RWMutex
}

// Pause pauses the worker (won't pick new tasks)
func (w *TaskWorker) Pause() {
	w.mu.Lock()
	w.paused = true
	w.mu.Unlock()
	log.Println("[TaskWorker] Paused")
}

// Resume resumes the worker
func (w *TaskWorker) Resume() {
	w.mu.Lock()
	w.paused = false
	w.mu.Unlock()
	log.Println("[TaskWorker] Resumed")
}

// IsPaused returns whether the worker is paused
func (w *TaskWorker) IsPaused() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.paused
}

// NewTaskWorker creates a new background task processor
func NewTaskWorker(db *gorm.DB, pr *provider.Registry, tr *tool.Registry, concurrency int) *TaskWorker {
	if concurrency < 1 {
		concurrency = 2
	}
	return &TaskWorker{
		db:               db,
		providerRegistry: pr,
		toolRegistry:     tr,
		concurrency:      concurrency,
		stopCh:           make(chan struct{}),
	}
}

// Start begins the worker loop
func (w *TaskWorker) Start() {
	log.Printf("[TaskWorker] Starting with concurrency=%d", w.concurrency)
	for i := 0; i < w.concurrency; i++ {
		w.wg.Add(1)
		go w.loop(i)
	}
	// Scheduler: promotes waiting ↀpending when scheduled_at arrives
	w.wg.Add(1)
	go w.schedulerLoop()
	// Cron: creates tasks from Schedule records
	w.wg.Add(1)
	go w.cronLoop()
	// Heartbeat monitor: detects dead/stuck tasks
	w.wg.Add(1)
	go w.heartbeatMonitor()
}

// Stop gracefully stops the worker
func (w *TaskWorker) Stop() {
	log.Println("[TaskWorker] Stopping...")
	close(w.stopCh)
	w.wg.Wait()
	log.Println("[TaskWorker] Stopped")
}

// loop is a single worker goroutine that continuously picks and executes tasks
func (w *TaskWorker) loop(workerID int) {
	defer w.wg.Done()
	for {
		select {
		case <-w.stopCh:
			return
		default:
		}

		if w.IsPaused() {
			select {
			case <-w.stopCh:
				return
			case <-time.After(2 * time.Second):
				continue
			}
		}

		task, err := w.claimTask()
		if err != nil || task == nil {
			// No tasks available, wait before polling again
			select {
			case <-w.stopCh:
				return
			case <-time.After(5 * time.Second):
				continue
			}
		}

		log.Printf("[TaskWorker-%d] Processing task %s: %s", workerID, task.ID, task.Title)
		w.executeTask(task)
	}
}

// schedulerLoop promotes "waiting" tasks to "pending" when their scheduled time arrives
func (w *TaskWorker) schedulerLoop() {
	defer w.wg.Done()
	for {
		select {
		case <-w.stopCh:
			return
		case <-time.After(30 * time.Second):
		}

		now := time.Now()
		w.db.Model(&model.Task{}).
			Where("status = ? AND scheduled_at IS NOT NULL AND scheduled_at <= ?", model.TaskStatusWaiting, now).
			Updates(map[string]interface{}{
				"status":     model.TaskStatusPending,
				"updated_at": now,
			})
	}
}

// claimTask atomically claims the next pending task (prevents double-processing)
func (w *TaskWorker) claimTask() (*model.Task, error) {
	var task model.Task
	silent := w.db.Session(&gorm.Session{Logger: logger.Discard})
	err := silent.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("status = ?", model.TaskStatusPending).
			Order("FIELD(priority, 'urgent', 'high', 'normal', 'low'), created_at ASC").
			First(&task).Error; err != nil {
			return err
		}
		now := time.Now()
		task.Status = model.TaskStatusRunning
		task.StartedAt = &now
		return tx.Save(&task).Error
	})
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // no tasks available, not an error
		}
		return nil, err
	}
	return &task, nil
}

// executeTask runs a single task through the Agent Runtime
func (w *TaskWorker) executeTask(task *model.Task) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Start heartbeat goroutine  updates heartbeat column every 30s while task runs
	hbDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-hbDone:
				return
			case <-ticker.C:
				now := time.Now()
				w.db.Model(&model.Task{}).Where("id = ?", task.ID).Update("heartbeat", &now)
			}
		}
	}()
	defer close(hbDone)

	// Set initial heartbeat
	nowHB := time.Now()
	w.db.Model(&model.Task{}).Where("id = ?", task.ID).Update("heartbeat", &nowHB)

	// Inject user_id and conversation_id so sub-tools work correctly
	ctx = context.WithValue(ctx, tool.CtxKeyUserID, task.UserID)
	if task.ConversationID != "" {
		ctx = context.WithValue(ctx, tool.CtxKeyConversationID, task.ConversationID)
	}

	// Look up agent (fallback to SuperAgent if not found)
	var agent model.Agent
	foundAgent := false
	if task.AgentID != "" {
		if err := w.db.Where("id = ? AND user_id = ?", task.AgentID, task.UserID).First(&agent).Error; err == nil {
			foundAgent = true
		} else {
			log.Printf("[TaskWorker] Agent %s not found, falling back to SuperAgent", task.AgentID)
		}
	}
	if !foundAgent {
		if err := w.db.Where("name = ? AND user_id = ?", "全能助手", task.UserID).First(&agent).Error; err != nil {
			w.failTask(task, "SuperAgent not found for user")
			return
		}
	}

	// Look up model config
	var modelCfg model.ModelConfig
	if err := w.db.Where("id = ?", agent.ModelID).First(&modelCfg).Error; err != nil {
		w.failTask(task, fmt.Sprintf("model config not found: %s", agent.ModelID))
		return
	}

	// Create provider and runtime
	p := provider.CreateFromConfig(w.providerRegistry, modelCfg)
	var enabledTools []string
	if agent.Tools != "" {
		json.Unmarshal([]byte(agent.Tools), &enabledTools)
	}

	// Build task-specific system prompt addition
	taskContext := fmt.Sprintf(`

## 当前正在执行后台任务
- 任务ID: %s
- 任务标题: %s
- 父任务ID: %s

你正在后台自主执行任务。完成后请用 system.notify_user 通知用户结果。如果需要拆分子任务，用 system.create_task 创建新任务。用 system.update_task 更新进度（progress 0-100，progress_note 描述当前步骤）。`, task.ID, task.Title, task.ParentTaskID)

	systemPrompt := agent.SystemPrompt + taskContext

	messages := []provider.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: task.Goal},
	}

	rt := agentpkg.NewRuntime(p, w.toolRegistry)
	runReq := &agentpkg.RunRequest{
		Model:       modelCfg.ModelName,
		Messages:    messages,
		Tools:       enabledTools,
		Temperature: modelCfg.Temperature,
		MaxTokens:   modelCfg.MaxTokens,
	}

	result, err := rt.Run(ctx, runReq)
	if err != nil {
		if task.RetryCount < task.MaxRetries {
			// Retry
			w.db.Model(task).Updates(map[string]interface{}{
				"status":      model.TaskStatusPending,
				"retry_count": task.RetryCount + 1,
				"error_msg":   err.Error(),
				"updated_at":  time.Now(),
			})
			log.Printf("[TaskWorker] Task %s failed, retrying (%d/%d): %v", task.ID, task.RetryCount+1, task.MaxRetries, err)
			return
		}
		w.failTask(task, err.Error())
		return
	}

	// Success
	now := time.Now()
	w.db.Model(task).Updates(map[string]interface{}{
		"status":       model.TaskStatusCompleted,
		"result":       result.Content,
		"progress":     100,
		"completed_at": &now,
		"updated_at":   now,
	})

	// Auto-notify user on completion
	w.db.Create(&model.Notification{
		UserID:  task.UserID,
		TaskID:  task.ID,
		Type:    model.NotifyTaskComplete,
		Title:   fmt.Sprintf("任务完成: %s", task.Title),
		Content: truncate(result.Content, 2000),
	})

	log.Printf("[TaskWorker] Task %s completed: %d chars", task.ID, len(result.Content))
}

// failTask marks a task as failed and notifies the user
func (w *TaskWorker) failTask(task *model.Task, errMsg string) {
	now := time.Now()
	w.db.Model(task).Updates(map[string]interface{}{
		"status":       model.TaskStatusFailed,
		"error_msg":    errMsg,
		"completed_at": &now,
		"updated_at":   now,
	})

	w.db.Create(&model.Notification{
		UserID:  task.UserID,
		TaskID:  task.ID,
		Type:    model.NotifyTaskFailed,
		Title:   fmt.Sprintf("任务失败: %s", task.Title),
		Content: errMsg,
	})

	log.Printf("[TaskWorker] Task %s failed: %s", task.ID, errMsg)
}

// cronLoop scans Schedule records every 60s and creates Tasks when cron matches
func (w *TaskWorker) cronLoop() {
	defer w.wg.Done()
	// Wait a bit on startup to let DB migrate
	select {
	case <-w.stopCh:
		return
	case <-time.After(10 * time.Second):
	}

	for {
		select {
		case <-w.stopCh:
			return
		case <-time.After(60 * time.Second):
		}

		var schedules []model.Schedule
		w.db.Where("enabled = ?", true).Find(&schedules)

		now := time.Now()
		for _, sched := range schedules {
			cron, err := ParseCron(sched.CronExpr)
			if err != nil {
				log.Printf("[CronLoop] Invalid cron expression for schedule %s: %v", sched.ID, err)
				continue
			}

			// Calculate next run if not set
			if sched.NextRunAt == nil || sched.NextRunAt.IsZero() {
				next := cron.NextAfter(now.Add(-61 * time.Second)) // look from ~1min ago
				if !next.IsZero() {
					w.db.Model(&sched).Update("next_run_at", &next)
					sched.NextRunAt = &next
				}
				continue
			}

			// Check if it's time to run
			if sched.NextRunAt.After(now) {
				continue
			}

			// Check max concurrent instances
			if sched.MaxInstances > 0 {
				var running int64
				w.db.Model(&model.Task{}).Where("schedule_id = ? AND status IN ?", sched.ID,
					[]string{string(model.TaskStatusRunning), string(model.TaskStatusPending)}).Count(&running)
				if int(running) >= sched.MaxInstances {
					log.Printf("[CronLoop] Schedule %s: %d instances running, max %d, skipping", sched.ID, running, sched.MaxInstances)
					// Still advance next_run_at so we don't pile up
					next := cron.NextAfter(now)
					w.db.Model(&sched).Updates(map[string]interface{}{"next_run_at": &next, "updated_at": now})
					continue
				}
			}

			// Create a task from this schedule
			title := sched.Title
			if title == "" {
				title = sched.Input // legacy field
			}
			if title == "" {
				title = fmt.Sprintf("定时任务 %s", sched.CronExpr)
			}
			goal := sched.Goal
			if goal == "" {
				goal = title
			}

			task := model.Task{
				UserID:         sched.UserID,
				AgentID:        sched.AgentID,
				ConversationID: sched.ConversationID,
				ScheduleID:     sched.ID,
				Title:          title,
				Goal:           goal,
				Status:         model.TaskStatusPending,
				Priority:       model.TaskPriorityNormal,
			}

			if err := w.db.Create(&task).Error; err != nil {
				log.Printf("[CronLoop] Failed to create task for schedule %s: %v", sched.ID, err)
				continue
			}

			// Update schedule: last_run_at, next_run_at
			next := cron.NextAfter(now)
			w.db.Model(&sched).Updates(map[string]interface{}{
				"last_run_at": &now,
				"next_run_at": &next,
				"updated_at":  now,
			})

			log.Printf("[CronLoop] Created task %s from schedule %s (%s), next run: %s",
				task.ID, sched.ID, sched.CronExpr, next.Format(time.RFC3339))
		}
	}
}

// heartbeatMonitor checks for running tasks whose heartbeat has gone stale (>3min)
// and marks them as failed or retries them
func (w *TaskWorker) heartbeatMonitor() {
	defer w.wg.Done()
	for {
		select {
		case <-w.stopCh:
			return
		case <-time.After(60 * time.Second):
		}

		// Find tasks that are "running" but heartbeat is older than 3 minutes
		staleThreshold := time.Now().Add(-3 * time.Minute)
		var staleTasks []model.Task
		w.db.Where("status = ? AND heartbeat IS NOT NULL AND heartbeat < ?",
			model.TaskStatusRunning, staleThreshold).Find(&staleTasks)

		for _, task := range staleTasks {
			if task.RetryCount < task.MaxRetries {
				// Retry
				w.db.Model(&task).Updates(map[string]interface{}{
					"status":      model.TaskStatusPending,
					"retry_count": task.RetryCount + 1,
					"error_msg":   "heartbeat timeout - task appears stuck, retrying",
					"heartbeat":   nil,
					"updated_at":  time.Now(),
				})
				log.Printf("[Heartbeat] Task %s stuck, retrying (%d/%d)", task.ID, task.RetryCount+1, task.MaxRetries)
			} else {
				w.failTask(&task, "heartbeat timeout - task appears stuck after max retries")
				log.Printf("[Heartbeat] Task %s stuck and out of retries, marking failed", task.ID)
			}
		}

		// Also detect tasks stuck in "running" without ANY heartbeat for >5min (legacy tasks)
		oldThreshold := time.Now().Add(-5 * time.Minute)
		w.db.Model(&model.Task{}).Where(
			"status = ? AND heartbeat IS NULL AND started_at IS NOT NULL AND started_at < ?",
			model.TaskStatusRunning, oldThreshold,
		).Updates(map[string]interface{}{
			"status":     model.TaskStatusFailed,
			"error_msg":  "task stuck without heartbeat",
			"updated_at": time.Now(),
		})
	}
}

func truncate(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}
