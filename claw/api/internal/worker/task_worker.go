package worker

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	agentpkg "github.com/yinhe/starclaw/internal/agent"
	"github.com/yinhe/starclaw/internal/model"
	"github.com/yinhe/starclaw/internal/provider"
	"github.com/yinhe/starclaw/internal/tool"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const maxTaskImageDataURLLength = 90000
const maxTaskImageRawBytes = 65000

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
			Order("CASE priority WHEN 'urgent' THEN 0 WHEN 'high' THEN 1 WHEN 'normal' THEN 2 WHEN 'low' THEN 3 ELSE 4 END, created_at ASC").
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
	ctx = context.WithValue(ctx, tool.CtxKeyTaskExecution, true)
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

	// Look up model config (fallback to user's first available model if agent has none)
	var modelCfg model.ModelConfig
	if agent.ModelID != "" {
		if err := w.db.Where("id = ?", agent.ModelID).First(&modelCfg).Error; err != nil {
			log.Printf("[TaskWorker] Agent model %s not found, trying user's default model", agent.ModelID)
			agent.ModelID = "" // trigger fallback
		}
	}
	if agent.ModelID == "" {
		if err := w.db.Where("user_id = ? AND is_enabled = ?", task.UserID, true).Order("created_at ASC").First(&modelCfg).Error; err != nil {
			// Last resort: try any enabled model in the system
			if err := w.db.Where("is_enabled = ?", true).Order("created_at ASC").First(&modelCfg).Error; err != nil {
				w.failTask(task, "no model config available — please add a model in Settings → Models")
				return
			}
		}
		log.Printf("[TaskWorker] Using fallback model: %s (%s)", modelCfg.ModelName, modelCfg.ID)
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

你正在后台自主执行任务。完成后请用 system.notify_user 通知用户结果。如果需要拆分子任务，用 system.create_task 创建新任务。用 system.update_task 更新进度（progress 0-100，progress_note 描述当前步骤）。

%s`, task.ID, task.Title, task.ParentTaskID, tool.DataDirSummary())

	systemPrompt := agent.SystemPrompt + taskContext
	userMessage := buildTaskUserMessage(task)
	wechatObservation := ""
	if shouldPreObserveWeChatTask(task) {
		if observed, err := w.collectWeChatObservation(ctx, p, modelCfg.Provider, modelCfg.ModelName); err == nil && strings.TrimSpace(observed) != "" {
			wechatObservation = observed
			w.db.Model(task).Updates(map[string]interface{}{
				"progress_note": truncate("微信窗口观测结果: "+observed, 1000),
				"updated_at":    time.Now(),
			})
			userMessage.Content = strings.TrimSpace(userMessage.Content) + "\n\n以下是通过 mcp_host_open_app + mcp_host_screen_capture 并经视觉提取获得的当前微信窗口真实观测结果。你必须仅基于这些观测文本判断是否有待处理客户消息，不得编造不可见内容。若观测结果里没有明确聊天文本，就明确写无法判断。\n\n微信窗口观测结果：\n" + observed
			userMessage.MultiContent = nil
		} else if err != nil {
			w.db.Model(task).Updates(map[string]interface{}{
				"progress_note": truncate("微信窗口观测失败: "+err.Error(), 1000),
				"updated_at":    time.Now(),
			})
		}
	}
	visionSummary := ""
	if len(userMessage.MultiContent) > 0 {
		if extracted, err := extractTaskVisionSummary(ctx, p, modelCfg.Provider, modelCfg.ModelName, userMessage); err == nil && strings.TrimSpace(extracted) != "" {
			visionSummary = extracted
			w.db.Model(task).Updates(map[string]interface{}{
				"progress_note": truncate("截图提取结果: "+extracted, 1000),
				"updated_at":    time.Now(),
			})
			userMessage = provider.ChatMessage{
				Role:    "user",
				Content: userMessage.Content + "\n\n以下是从截图中提取的结构化结果。你只能依据这些字段行动，不得编造监控ID、模式、系统限制、不可见消息内容或任何截图中看不到的信息。若字段为空、unknown 或无法判断，就明确写无法判断。\n\n截图提取结果：\n" + extracted,
			}
			log.Printf("[TaskWorker] Vision extraction completed for task %s", task.ID)
		} else if err != nil {
			log.Printf("[TaskWorker] Vision extraction failed for task %s: %v", task.ID, err)
			w.db.Model(task).Updates(map[string]interface{}{
				"progress_note": truncate("截图提取失败: "+err.Error(), 1000),
				"updated_at":    time.Now(),
			})
		}
	}

	messages := []provider.ChatMessage{
		{Role: "system", Content: systemPrompt},
		userMessage,
	}
	chatModel := modelCfg.ModelName
	if len(userMessage.MultiContent) > 0 && !isVisionTaskModel(chatModel) {
		if vm := pickTaskVisionModel(p.Models(), modelCfg.Provider); vm != "" {
			log.Printf("[TaskWorker] Images detected — switching from %q to vision model %q", chatModel, vm)
			chatModel = vm
		}
	}

	rt := agentpkg.NewRuntime(p, w.toolRegistry)
	runReq := &agentpkg.RunRequest{
		Model:       chatModel,
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
		"status":   model.TaskStatusCompleted,
		"result":   result.Content,
		"progress": 100,
		"progress_note": func() string {
			if wechatObservation != "" {
				return truncate("已完成。微信窗口观测结果已记录。", 1000)
			}
			if visionSummary != "" {
				return truncate("已完成。截图提取结果已记录。", 1000)
			}
			return task.ProgressNote
		}(),
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

func shouldPreObserveWeChatTask(task *model.Task) bool {
	text := task.Title + "\n" + task.Description + "\n" + task.Goal
	return strings.Contains(text, "微信") || strings.Contains(text, "mcp_host_screen_inspect") || strings.Contains(text, "mcp_host_screen_capture")
}

func (w *TaskWorker) collectWeChatObservation(ctx context.Context, p provider.ModelProvider, providerName, currentModel string) (string, error) {
	if w.toolRegistry == nil {
		return "", fmt.Errorf("tool registry unavailable")
	}
	var sections []string
	if _, ok := w.toolRegistry.Get("mcp_host_open_app"); ok {
		openArgs := `{"target":"微信"}`
		if out, err := w.toolRegistry.Execute(ctx, "mcp_host_open_app", openArgs); err == nil && strings.TrimSpace(out) != "" {
			sections = append(sections, "open_app:\n"+out)
		}
	}
	if _, ok := w.toolRegistry.Get("mcp_host_screen_capture"); !ok {
		return "", fmt.Errorf("mcp_host_screen_capture not available")
	}
	captureOut, err := w.toolRegistry.Execute(ctx, "mcp_host_screen_capture", `{}`)
	if err != nil {
		return "", err
	}
	imageURL := extractMCPScreenshotDataURL(captureOut)
	if imageURL == "" {
		return "", fmt.Errorf("screen_capture returned no screenshot data")
	}
	visionPrompt := "请严格读取这张当前微信窗口截图中可见的聊天内容，只输出结构化可见信息。禁止编造任何截图外的信息；如果看不清或没有聊天文本，就明确写无法判断。"
	visionMsg := provider.ChatMessage{
		Role:    "user",
		Content: visionPrompt,
		MultiContent: []provider.ContentPart{
			{Type: "text", Text: visionPrompt},
			{Type: "image_url", ImageURL: &provider.ImageURL{URL: imageURL, Detail: "auto"}},
		},
	}
	visionOut, err := extractTaskVisionSummary(ctx, p, providerName, currentModel, visionMsg)
	if err != nil {
		return "", err
	}
	visionOut = strings.TrimSpace(visionOut)
	if visionOut == "" {
		return "", fmt.Errorf("vision extraction returned empty result")
	}
	sections = append(sections, "vision_extract:\n"+visionOut)
	return truncate(strings.Join(sections, "\n\n"), 12000), nil
}

func extractMCPScreenshotDataURL(toolResult string) string {
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(toolResult), &parsed); err != nil {
		return ""
	}
	url, _ := parsed["screenshot"].(string)
	return strings.TrimSpace(url)
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

var taskImageURLPattern = regexp.MustCompile(`/v1/images/[^\s"')]+`)

func buildTaskUserMessage(task *model.Task) provider.ChatMessage {
	content := strings.TrimSpace(task.Goal)
	if desc := strings.TrimSpace(task.Description); desc != "" {
		content = desc + "\n\n" + content
	}
	msg := provider.ChatMessage{
		Role:    "user",
		Content: content,
	}
	images := extractTaskImages(task)
	if len(images) == 0 {
		return msg
	}
	parts := []provider.ContentPart{{Type: "text", Text: content}}
	for _, img := range images {
		parts = append(parts, provider.ContentPart{
			Type:     "image_url",
			ImageURL: &provider.ImageURL{URL: img, Detail: "auto"},
		})
	}
	msg.MultiContent = parts
	return msg
}

func extractTaskImages(task *model.Task) []string {
	joined := task.Description + "\n" + task.Goal
	matches := taskImageURLPattern.FindAllString(joined, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[string]bool)
	var out []string
	for _, m := range matches {
		if seen[m] {
			continue
		}
		seen[m] = true
		if dataURL, err := localImageURLToDataURL(m); err == nil {
			out = append(out, dataURL)
			continue
		}
		if strings.HasPrefix(m, "http://") || strings.HasPrefix(m, "https://") || strings.HasPrefix(m, "data:image/") {
			out = append(out, m)
		}
	}
	return out
}

func localImageURLToDataURL(imageURL string) (string, error) {
	filename := filepath.Base(strings.TrimSpace(imageURL))
	if filename == "" || filename == "." {
		return "", fmt.Errorf("invalid image url: %s", imageURL)
	}
	path := filepath.Join(tool.ImagesDir(), filename)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if len("data:image/jpeg;base64,")+base64.StdEncoding.EncodedLen(len(data)) <= maxTaskImageDataURLLength {
		ext := strings.ToLower(filepath.Ext(filename))
		mime := "image/png"
		switch ext {
		case ".jpg", ".jpeg":
			mime = "image/jpeg"
		case ".gif":
			mime = "image/gif"
		}
		return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data), nil
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	img = cropTaskRelevantRegion(img)
	scales := []float64{0.5, 0.35, 0.25, 0.18, 0.12, 0.08}
	qualities := []int{45, 35, 28, 22, 18}
	for _, scale := range scales {
		resized := resizeImageNearest(img, scale)
		for _, quality := range qualities {
			var buf bytes.Buffer
			if err := jpeg.Encode(&buf, resized, &jpeg.Options{Quality: quality}); err != nil {
				return "", err
			}
			if buf.Len() <= maxTaskImageRawBytes {
				encoded := base64.StdEncoding.EncodeToString(buf.Bytes())
				dataURL := "data:image/jpeg;base64," + encoded
				if len(dataURL) <= maxTaskImageDataURLLength {
					return dataURL, nil
				}
			}
		}
	}
	return "", fmt.Errorf("image too large after compression: %s", imageURL)
}

func cropTaskRelevantRegion(src image.Image) image.Image {
	b := src.Bounds()
	w := b.Dx()
	h := b.Dy()
	if w < 400 || h < 300 {
		return src
	}
	left := b.Min.X + int(float64(w)*0.38)
	top := b.Min.Y + int(float64(h)*0.10)
	right := b.Max.X - int(float64(w)*0.02)
	bottom := b.Max.Y - int(float64(h)*0.14)
	if right-left < 200 || bottom-top < 120 {
		return src
	}
	dst := image.NewRGBA(image.Rect(0, 0, right-left, bottom-top))
	for y := top; y < bottom; y++ {
		for x := left; x < right; x++ {
			dst.Set(x-left, y-top, src.At(x, y))
		}
	}
	return dst
}

func isVisionTaskModel(model string) bool {
	m := strings.ToLower(model)
	if strings.Contains(m, "-vl") || strings.Contains(m, "vl-") || strings.Contains(m, "vision") {
		return true
	}
	visionModels := []string{
		"gpt-4o", "gpt-4o-mini", "gpt-4-turbo", "gpt-4.1", "gpt-4.1-mini", "gpt-4.1-nano",
		"chatgpt-4o-latest",
		"claude-sonnet-4", "claude-3-7-sonnet", "claude-3-5-sonnet", "claude-3-opus",
		"gemini-2.5-pro", "gemini-2.5-flash", "gemini-2.0-flash", "gemini-1.5-pro", "gemini-1.5-flash",
		"o3", "o3-mini", "o4-mini", "o1", "o1-mini",
	}
	for _, vm := range visionModels {
		if strings.HasPrefix(m, vm) {
			return true
		}
	}
	return false
}

func pickTaskVisionModel(available []string, providerName string) string {
	preferred := map[string][]string{
		"star-ai":   {"qwen-vl-max", "qwen-vl-plus", "gpt-4o", "gemini-2.0-flash"},
		"qwen":      {"qwen-vl-max", "qwen-vl-plus", "qwen3-vl-plus", "qwen3-vl-flash"},
		"openai":    {"gpt-4o", "gpt-4o-mini", "gpt-4-turbo"},
		"google":    {"gemini-2.5-flash", "gemini-2.0-flash", "gemini-1.5-flash"},
		"anthropic": {"claude-sonnet-4-20250514", "claude-3-7-sonnet-20250219", "claude-3-5-sonnet-20241022"},
	}
	if prefs, ok := preferred[providerName]; ok {
		for _, pref := range prefs {
			for _, avail := range available {
				if strings.HasPrefix(strings.ToLower(avail), strings.ToLower(pref)) {
					return avail
				}
			}
		}
	}
	for _, avail := range available {
		if isVisionTaskModel(avail) {
			return avail
		}
	}
	return ""
}

func extractTaskVisionSummary(ctx context.Context, p provider.ModelProvider, providerName, currentModel string, userMessage provider.ChatMessage) (string, error) {
	visionModel := currentModel
	if !isVisionTaskModel(visionModel) {
		if vm := pickTaskVisionModel(p.Models(), providerName); vm != "" {
			visionModel = vm
		}
	}
	if !isVisionTaskModel(visionModel) {
		return "", fmt.Errorf("no vision model available for provider %s", providerName)
	}
	visionReq := &provider.ChatRequest{
		Model: visionModel,
		Messages: []provider.ChatMessage{
			{
				Role:    "system",
				Content: "你是微信群客服截图解析器。你只能根据图片中真正可见的内容提取信息，绝不能编造。请只输出一个 JSON 对象，不要输出 markdown，不要输出额外解释。JSON 结构固定为：{\"conversation_name\":string,\"latest_visible_messages\":[string],\"latest_customer_message\":string,\"has_image\":boolean,\"has_voice\":boolean,\"needs_human_review\":boolean,\"confidence\":\"high|medium|low\",\"notes\":string}。如果看不清或无法判断，字符串字段填 \"unknown\"，数组填 []，布尔填 false。",
			},
			userMessage,
		},
		Temperature: 0.1,
		MaxTokens:   800,
		Stream:      false,
	}
	result, err := p.ChatSync(ctx, visionReq)
	if err != nil {
		return "", err
	}
	return normalizeTaskVisionSummary(result.Content), nil
}

type taskVisionSummary struct {
	ConversationName      string   `json:"conversation_name"`
	LatestVisibleMessage  []string `json:"latest_visible_messages"`
	LatestCustomerMessage string   `json:"latest_customer_message"`
	HasImage              bool     `json:"has_image"`
	HasVoice              bool     `json:"has_voice"`
	NeedsHumanReview      bool     `json:"needs_human_review"`
	Confidence            string   `json:"confidence"`
	Notes                 string   `json:"notes"`
}

func normalizeTaskVisionSummary(raw string) string {
	parsed := taskVisionSummary{
		ConversationName:      "unknown",
		LatestVisibleMessage:  []string{},
		LatestCustomerMessage: "unknown",
		Confidence:            "low",
		Notes:                 "unknown",
	}
	jsonText := extractJSONObject(strings.TrimSpace(raw))
	if jsonText != "" {
		_ = json.Unmarshal([]byte(jsonText), &parsed)
	}
	if strings.TrimSpace(parsed.ConversationName) == "" {
		parsed.ConversationName = "unknown"
	}
	if len(parsed.LatestVisibleMessage) == 0 {
		parsed.LatestVisibleMessage = []string{}
	}
	if strings.TrimSpace(parsed.LatestCustomerMessage) == "" {
		parsed.LatestCustomerMessage = "unknown"
	}
	if parsed.Confidence != "high" && parsed.Confidence != "medium" && parsed.Confidence != "low" {
		parsed.Confidence = "low"
	}
	if strings.TrimSpace(parsed.Notes) == "" {
		parsed.Notes = "unknown"
	}
	out, _ := json.MarshalIndent(parsed, "", "  ")
	return string(out)
}

func extractJSONObject(s string) string {
	start := strings.IndexByte(s, '{')
	if start < 0 {
		return ""
	}
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(s); i++ {
		c := s[i]
		if escaped {
			escaped = false
			continue
		}
		if c == '\\' && inString {
			escaped = true
			continue
		}
		if c == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		if c == '{' {
			depth++
		}
		if c == '}' {
			depth--
			if depth == 0 {
				candidate := s[start : i+1]
				if json.Valid([]byte(candidate)) {
					return candidate
				}
			}
		}
	}
	return ""
}

func resizeImageNearest(src image.Image, scale float64) image.Image {
	if scale >= 0.999 {
		return src
	}
	b := src.Bounds()
	srcW := b.Dx()
	srcH := b.Dy()
	if srcW <= 0 || srcH <= 0 {
		return src
	}
	dstW := int(float64(srcW) * scale)
	dstH := int(float64(srcH) * scale)
	if dstW < 1 {
		dstW = 1
	}
	if dstH < 1 {
		dstH = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	for y := 0; y < dstH; y++ {
		srcY := b.Min.Y + (y * srcH / dstH)
		for x := 0; x < dstW; x++ {
			srcX := b.Min.X + (x * srcW / dstW)
			dst.Set(x, y, src.At(srcX, srcY))
		}
	}
	return dst
}
