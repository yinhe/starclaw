package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ── 生长发育门诊随访系统 (Growth & Development Clinic Follow-up) ──

// ── 患者档案 ──

type GCPatient struct {
	ID              string     `json:"id" gorm:"type:varchar(36);primaryKey"`
	UserID          string     `json:"user_id" gorm:"type:varchar(36);index;not null"`
	AgentID         string     `json:"agent_id" gorm:"type:varchar(36);index"`
	Name            string     `json:"name" gorm:"type:varchar(100);not null"`
	Gender          string     `json:"gender" gorm:"type:varchar(10);not null"`  // male / female
	BirthDate       time.Time  `json:"birth_date" gorm:"not null"`
	FatherHeight    float64    `json:"father_height" gorm:"default:0"`           // cm
	MotherHeight    float64    `json:"mother_height" gorm:"default:0"`           // cm
	TargetHeight    float64    `json:"target_height" gorm:"default:0"`           // 遗传靶身高 cm
	GuardianName    string     `json:"guardian_name" gorm:"type:varchar(100)"`
	GuardianPhone   string     `json:"guardian_phone" gorm:"type:varchar(20)"`
	GuardianRelation string   `json:"guardian_relation" gorm:"type:varchar(20)"` // mother/father/other
	FamilyHistory   string     `json:"family_history" gorm:"type:text"`          // JSON
	PastHistory     string     `json:"past_history" gorm:"type:text"`
	AllergyHistory  string     `json:"allergy_history" gorm:"type:text"`
	Diagnosis       string     `json:"diagnosis" gorm:"type:text"`               // 当前诊断
	TreatmentPlan   string     `json:"treatment_plan" gorm:"type:text"`          // 当前治疗方案 JSON
	Status          string     `json:"status" gorm:"type:varchar(20);default:active;index"` // active/paused/discharged/lost
	ConsentExpiry   *time.Time `json:"consent_expiry"`                           // 授权到期
	Notes           string     `json:"notes" gorm:"type:text"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

func (p *GCPatient) BeforeCreate(tx *gorm.DB) error {
	if p.ID == "" {
		p.ID = uuid.New().String()
	}
	return nil
}

// ── 随访计划 ──

type GCFollowupPlan struct {
	ID              string     `json:"id" gorm:"type:varchar(36);primaryKey"`
	UserID          string     `json:"user_id" gorm:"type:varchar(36);index;not null"`
	PatientID       string     `json:"patient_id" gorm:"type:varchar(36);index;not null"`
	AgentID         string     `json:"agent_id" gorm:"type:varchar(36);index"`
	Name            string     `json:"name" gorm:"type:varchar(200)"`
	FrequencyMode   string     `json:"frequency_mode" gorm:"type:varchar(20);default:smart"` // manual / smart
	FrequencyDays   int        `json:"frequency_days" gorm:"default:90"`                     // 手动: 间隔天数
	Indicators      string     `json:"indicators" gorm:"type:json"`                          // JSON: 需采集的指标列表
	AlertThresholds string     `json:"alert_thresholds" gorm:"type:json"`                    // JSON: 预警阈值配置
	EducationTags   string     `json:"education_tags" gorm:"type:json"`                      // JSON: 宣教内容标签
	Status          string     `json:"status" gorm:"type:varchar(20);default:active;index"`  // active/paused/terminated
	NextFollowupAt  *time.Time `json:"next_followup_at"`
	LastFollowupAt  *time.Time `json:"last_followup_at"`
	ChangeReason    string     `json:"change_reason" gorm:"type:text"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

func (p *GCFollowupPlan) BeforeCreate(tx *gorm.DB) error {
	if p.ID == "" {
		p.ID = uuid.New().String()
	}
	if p.Indicators == "" {
		p.Indicators = `["height","weight","bmi"]`
	}
	if p.AlertThresholds == "" {
		p.AlertThresholds = `{"z_mild":-1,"z_moderate":-2,"z_severe":-3}`
	}
	return nil
}

// ── 生长数据记录 (每次上报) ──

type GCGrowthRecord struct {
	ID              string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	UserID          string    `json:"user_id" gorm:"type:varchar(36);index;not null"`
	PatientID       string    `json:"patient_id" gorm:"type:varchar(36);index;not null"`
	PlanID          string    `json:"plan_id" gorm:"type:varchar(36);index"`
	RecordDate      time.Time `json:"record_date" gorm:"index;not null"`
	AgeMonths       float64   `json:"age_months"`                                      // 月龄
	HeightMorning   float64   `json:"height_morning"`                                  // 早晨身高 cm
	HeightEvening   float64   `json:"height_evening"`                                  // 晚间身高 cm
	Height          float64   `json:"height"`                                          // 取用身高 cm
	Weight          float64   `json:"weight"`                                          // kg
	BMI             float64   `json:"bmi"`                                             // 自动计算
	HeightZScore    float64   `json:"height_z_score"`                                  // 年龄别身高Z评分
	WeightZScore    float64   `json:"weight_z_score"`                                  // 年龄别体重Z评分
	BMIZScore       float64   `json:"bmi_z_score"`                                     // BMI Z评分
	HeightPercentile float64  `json:"height_percentile"`                               // 百分位
	GrowthVelocity  float64   `json:"growth_velocity"`                                 // 年生长速率 cm/year
	BoneAge         float64   `json:"bone_age"`                                        // 骨龄(年)
	TannerStage     string    `json:"tanner_stage" gorm:"type:varchar(20)"`            // B1-B5/G1-G5/PH1-PH5
	ExerciseMinutes int       `json:"exercise_minutes"`                                // 日均运动时长
	ExerciseType    string    `json:"exercise_type" gorm:"type:varchar(100)"`
	ExerciseWeekly  int       `json:"exercise_weekly"`                                 // 每周次数
	SleepHours      float64   `json:"sleep_hours"`                                     // 睡眠时长
	SleepTime       string    `json:"sleep_time" gorm:"type:varchar(10)"`              // 入睡时间 HH:MM
	DietNote        string    `json:"diet_note" gorm:"type:text"`                      // 饮食情况
	DataSource      string    `json:"data_source" gorm:"type:varchar(20);default:manual"` // manual/device/ocr/his
	DataQuality     string    `json:"data_quality" gorm:"type:varchar(20);default:normal"` // normal/suspect/invalid
	QualityNote     string    `json:"quality_note" gorm:"type:text"`                   // 质控备注
	RawReport       string    `json:"raw_report" gorm:"type:text"`                     // 原始上传(图片URL/PDF路径)
	CreatedAt       time.Time `json:"created_at"`
}

func (r *GCGrowthRecord) BeforeCreate(tx *gorm.DB) error {
	if r.ID == "" {
		r.ID = uuid.New().String()
	}
	if r.Height == 0 && r.HeightMorning > 0 {
		r.Height = r.HeightMorning
	}
	if r.Height > 0 && r.Weight > 0 {
		hm := r.Height / 100
		r.BMI = r.Weight / (hm * hm)
	}
	return nil
}

// ── 预警记录 ──

type GCAlertLevel string

const (
	GCAlertNormal   GCAlertLevel = "normal"
	GCAlertMild     GCAlertLevel = "mild"     // 轻度异常
	GCAlertModerate GCAlertLevel = "moderate" // 中度异常
	GCAlertSevere   GCAlertLevel = "severe"   // 危急值
)

type GCAlert struct {
	ID          string       `json:"id" gorm:"type:varchar(36);primaryKey"`
	UserID      string       `json:"user_id" gorm:"type:varchar(36);index;not null"`
	PatientID   string       `json:"patient_id" gorm:"type:varchar(36);index;not null"`
	RecordID    string       `json:"record_id" gorm:"type:varchar(36);index"`
	Level       GCAlertLevel `json:"level" gorm:"type:varchar(20);index;not null"`
	Category    string       `json:"category" gorm:"type:varchar(50)"` // height/weight/bmi/bone_age/puberty/velocity
	Title       string       `json:"title" gorm:"type:varchar(200)"`
	Description string       `json:"description" gorm:"type:text"`
	Indicators  string       `json:"indicators" gorm:"type:json"`              // 触发指标详情 JSON
	AgentAction string       `json:"agent_action" gorm:"type:text"`            // Agent建议动作
	DoctorAction string      `json:"doctor_action" gorm:"type:text"`           // 医生处理动作
	Status      string       `json:"status" gorm:"type:varchar(20);default:pending;index"` // pending/reviewed/resolved/dismissed
	ReviewedBy  string       `json:"reviewed_by" gorm:"type:varchar(100)"`
	ReviewedAt  *time.Time   `json:"reviewed_at"`
	ResolvedAt  *time.Time   `json:"resolved_at"`
	CreatedAt   time.Time    `json:"created_at"`
}

func (a *GCAlert) BeforeCreate(tx *gorm.DB) error {
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	return nil
}

// ── 宣教记录 ──

type GCEducation struct {
	ID        string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	UserID    string    `json:"user_id" gorm:"type:varchar(36);index;not null"`
	PatientID string    `json:"patient_id" gorm:"type:varchar(36);index;not null"`
	Category  string    `json:"category" gorm:"type:varchar(50)"` // diet/exercise/sleep/puberty/general
	Title     string    `json:"title" gorm:"type:varchar(200)"`
	Content   string    `json:"content" gorm:"type:longtext"`
	Channel   string    `json:"channel" gorm:"type:varchar(20)"` // app/sms/wechat
	Status    string    `json:"status" gorm:"type:varchar(20);default:sent"`  // draft/sent/read
	SentAt    *time.Time `json:"sent_at"`
	ReadAt    *time.Time `json:"read_at"`
	CreatedAt time.Time  `json:"created_at"`
}

func (e *GCEducation) BeforeCreate(tx *gorm.DB) error {
	if e.ID == "" {
		e.ID = uuid.New().String()
	}
	return nil
}

// ── 操作审计日志 ──

type GCAuditLog struct {
	ID         string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	UserID     string    `json:"user_id" gorm:"type:varchar(36);index;not null"`
	PatientID  string    `json:"patient_id" gorm:"type:varchar(36);index"`
	Actor      string    `json:"actor" gorm:"type:varchar(50)"` // doctor/guardian/agent
	Action     string    `json:"action" gorm:"type:varchar(100);not null"`
	Resource   string    `json:"resource" gorm:"type:varchar(100)"`
	ResourceID string    `json:"resource_id" gorm:"type:varchar(36)"`
	OldValue   string    `json:"old_value" gorm:"type:text"`
	NewValue   string    `json:"new_value" gorm:"type:text"`
	IP         string    `json:"ip" gorm:"type:varchar(50)"`
	UserAgent  string    `json:"user_agent" gorm:"type:varchar(300)"`
	CreatedAt  time.Time `json:"created_at"`
}

func (l *GCAuditLog) BeforeCreate(tx *gorm.DB) error {
	if l.ID == "" {
		l.ID = uuid.New().String()
	}
	return nil
}
