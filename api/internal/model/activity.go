package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ActivityType represents the instinct category
type ActivityType string

const (
	ActivityTypeCare     ActivityType = "care"     // 关怀本能：生日、节日、情感关怀
	ActivityTypeSchedule ActivityType = "schedule" // 时间本能：定时任务（早报、周报、日程）
	ActivityTypeMonitor  ActivityType = "monitor"  // 监控本能：数据阈值、服务状态
	ActivityTypeEvent    ActivityType = "event"    // 事件本能：外部事件触发
	ActivityTypeLearn    ActivityType = "learn"    // 学习本能：空闲时间自主学习
)

// Activity represents an instinct-driven proactive behavior.
// It extends the Schedule concept with conditions, channels, cooldowns, and LLM-powered actions.
type Activity struct {
	ID          string       `json:"id" gorm:"type:varchar(36);primaryKey"`
	UserID      string       `json:"user_id" gorm:"type:varchar(36);index;not null"`
	AgentID     string       `json:"agent_id" gorm:"type:varchar(36);index"` // agent to execute the action
	Name        string       `json:"name" gorm:"type:varchar(200);not null"` // e.g. "birthday_greeting"
	Title       string       `json:"title" gorm:"type:varchar(500)"`         // human-readable title
	Description string       `json:"description" gorm:"type:text"`
	Type        ActivityType `json:"type" gorm:"type:varchar(20);index;default:'schedule'"`

	// Trigger: cron expression (5-field) or special keywords
	// "0 9 * * *"          — every day at 9:00
	// "@idle"              — when system is idle (no tasks running)
	// "@event:github_issue" — on external event
	Trigger string `json:"trigger" gorm:"type:varchar(200);not null"`

	// Condition: optional CEL-like expression evaluated before executing
	// "user.birthday == today"
	// "service.status == 'down'"
	// Empty string = always execute when trigger fires
	Condition string `json:"condition" gorm:"type:text"`

	// Action: the goal/prompt sent to the Agent when triggered
	// "生成今日新闻摘要，聚焦 AI 和科技领域"
	// "检查所有监控服务状态，生成报告"
	Action string `json:"action" gorm:"type:longtext;not null"`

	// Channel: which tentacle to deliver results through (empty = in-app notification only)
	// "wechat", "telegram", "email", "slack", "dingtalk"
	Channel string `json:"channel" gorm:"type:varchar(50)"`

	// Cooldown: minimum interval between executions (Go duration string)
	// "24h", "168h" (1 week), "8760h" (1 year)
	Cooldown string `json:"cooldown" gorm:"type:varchar(50);default:'24h'"`

	// Template: built-in template name (empty = custom user activity)
	// "birthday_greeting", "daily_news", "weekly_report", etc.
	Template string `json:"template" gorm:"type:varchar(100);index"`

	// Config: JSON blob for template-specific settings
	// e.g. {"topics": ["AI", "tech"], "language": "zh"}
	Config string `json:"config" gorm:"type:text"`

	Enabled       bool       `json:"enabled" gorm:"default:true"`
	LastRunAt     *time.Time `json:"last_run_at"`
	NextRunAt     *time.Time `json:"next_run_at"`
	LastResult    string     `json:"last_result" gorm:"type:text"`   // last execution result (truncated)
	TotalRuns     int        `json:"total_runs" gorm:"default:0"`    // total execution count
	SuccessRuns   int        `json:"success_runs" gorm:"default:0"`  // successful execution count
	ConsecFails   int        `json:"consec_fails" gorm:"default:0"`  // consecutive failures (auto-disable at 5)
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

func (a *Activity) BeforeCreate(tx *gorm.DB) error {
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	return nil
}

// ActivityLog records each execution of an activity
type ActivityLog struct {
	ID         string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	ActivityID string    `json:"activity_id" gorm:"type:varchar(36);index;not null"`
	UserID     string    `json:"user_id" gorm:"type:varchar(36);index;not null"`
	TaskID     string    `json:"task_id" gorm:"type:varchar(36);index"` // the task created for this execution
	Status     string    `json:"status" gorm:"type:varchar(20)"`        // ok / failed / skipped
	Result     string    `json:"result" gorm:"type:text"`
	Error      string    `json:"error" gorm:"type:text"`
	Duration   int       `json:"duration"` // execution time in milliseconds
	CreatedAt  time.Time `json:"created_at"`
}

func (l *ActivityLog) BeforeCreate(tx *gorm.DB) error {
	if l.ID == "" {
		l.ID = uuid.New().String()
	}
	return nil
}
