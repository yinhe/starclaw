package worker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/yinhe/starclaw/internal/model"
	"github.com/yinhe/starclaw/internal/tool"
	"gorm.io/gorm"
)

// WeChatWatcher polls enabled WeChat watches, compares screenshot hashes,
// and creates customer-service tasks when the UI changes.
type WeChatWatcher struct {
	db       *gorm.DB
	desktop  *tool.DesktopTool
	stopCh   chan struct{}
	wg       sync.WaitGroup
	lastPoll map[string]time.Time
}

func NewWeChatWatcher(db *gorm.DB) *WeChatWatcher {
	return &WeChatWatcher{
		db:       db,
		desktop:  tool.NewDesktopTool(),
		stopCh:   make(chan struct{}),
		lastPoll: make(map[string]time.Time),
	}
}

func (w *WeChatWatcher) Start() {
	log.Println("[WeChatWatcher] Starting...")
	w.wg.Add(1)
	go w.loop()
}

func (w *WeChatWatcher) Stop() {
	close(w.stopCh)
	w.wg.Wait()
}

func (w *WeChatWatcher) loop() {
	defer w.wg.Done()
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.scan()
		}
	}
}

func (w *WeChatWatcher) scan() {
	var watches []model.WeChatWatch
	if err := w.db.Where("enabled = ?", true).Find(&watches).Error; err != nil {
		log.Printf("[WeChatWatcher] list watches failed: %v", err)
		return
	}
	for _, watch := range watches {
		if !w.shouldPoll(watch) {
			continue
		}
		if err := w.processWatch(&watch); err != nil {
			log.Printf("[WeChatWatcher] watch %s (%s) failed: %v", watch.ID, watch.Target, err)
		}
		w.lastPoll[watch.ID] = time.Now()
	}
}

func (w *WeChatWatcher) shouldPoll(watch model.WeChatWatch) bool {
	last, ok := w.lastPoll[watch.ID]
	if !ok {
		return true
	}
	interval := time.Duration(watch.PollIntervalSec) * time.Second
	if interval <= 0 {
		interval = 20 * time.Second
	}
	return time.Since(last) >= interval
}

func (w *WeChatWatcher) processWatch(watch *model.WeChatWatch) error {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	ctx = context.WithValue(ctx, tool.CtxKeyUserID, watch.UserID)

	focusArgs, _ := json.Marshal(map[string]interface{}{
		"action": "focus_window",
		"title":  watch.WindowTitle,
	})
	_, _ = w.desktop.Execute(ctx, string(focusArgs))

	screenArgs, _ := json.Marshal(map[string]interface{}{
		"action": "screenshot",
		"region": "full",
	})
	out, err := w.desktop.Execute(ctx, string(screenArgs))
	if err != nil {
		return err
	}
	var result struct {
		ImageURL string `json:"image_url"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		return fmt.Errorf("parse screenshot result failed: %w", err)
	}
	if result.ImageURL == "" {
		return fmt.Errorf("empty screenshot image_url")
	}
	imagePath, err := resolveImagePath(result.ImageURL)
	if err != nil {
		return err
	}
	hash, err := fileSHA256(imagePath)
	if err != nil {
		return err
	}
	if hash == watch.LastImageHash {
		return nil
	}

	if w.hasPendingWatchTask(watch) {
		_ = w.db.Model(watch).Updates(map[string]interface{}{
			"last_image_hash": hash,
			"last_image_url":  result.ImageURL,
			"updated_at":      time.Now(),
		}).Error
		return nil
	}

	now := time.Now()
	goal := fmt.Sprintf(`微信[%s]有新消息。请直接调用 wechat_cs(action="send_reply", target="%s", content="你的回复内容") 发送一条友好的回复。回复要自然、像真人朋友聊天一样，不要说自己是机器人。不需要截图、不需要分析、不需要调用其他工具，直接回复即可。`, watch.Target, watch.Target)
	task := model.Task{
		UserID:       watch.UserID,
		AgentID:      watch.AgentID,
		Title:        fmt.Sprintf("微信群客服跟进：%s", watch.Target),
		Description:  fmt.Sprintf("检测到微信群“%s”聊天界面发生变化，需客服机器人跟进。请优先通过 mcp_host_screen_inspect 读取当前微信窗口文本/OCR 结果，再决定回复或转人工。", watch.Target),
		Goal:         goal,
		Status:       model.TaskStatusPending,
		Priority:     model.TaskPriorityHigh,
		ScheduledAt:  nil,
		Progress:     0,
		ProgressNote: "检测到新消息变化，等待客服机器人处理",
	}
	if err := w.db.Create(&task).Error; err != nil {
		return fmt.Errorf("create watch task failed: %w", err)
	}
	if err := w.db.Model(watch).Updates(map[string]interface{}{
		"last_image_hash":   hash,
		"last_image_url":    result.ImageURL,
		"last_triggered_at": &now,
		"updated_at":        now,
	}).Error; err != nil {
		return fmt.Errorf("update watch state failed: %w", err)
	}
	_ = w.db.Create(&model.Notification{
		UserID:  watch.UserID,
		TaskID:  task.ID,
		Type:    model.NotifyInfo,
		Title:   fmt.Sprintf("微信群有新动态：%s", watch.Target),
		Content: fmt.Sprintf("已为“%s”创建客服跟进任务。", watch.Target),
	}).Error
	log.Printf("[WeChatWatcher] created task %s for target %s", task.ID, watch.Target)
	return nil
}

func (w *WeChatWatcher) hasPendingWatchTask(watch *model.WeChatWatch) bool {
	var count int64
	title := fmt.Sprintf("微信群客服跟进：%s", watch.Target)
	w.db.Model(&model.Task{}).
		Where("user_id = ? AND agent_id = ? AND title = ? AND status IN ?", watch.UserID, watch.AgentID, title, []string{string(model.TaskStatusPending), string(model.TaskStatusRunning)}).
		Count(&count)
	return count > 0
}

func resolveImagePath(imageURL string) (string, error) {
	filename := filepath.Base(strings.TrimSpace(imageURL))
	if filename == "." || filename == "" {
		return "", fmt.Errorf("invalid image url: %s", imageURL)
	}
	return filepath.Join(tool.ImagesDir(), filename), nil
}

func fileSHA256(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
