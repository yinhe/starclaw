package agent

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
//  LoRA Adapter Management
// ════════════════════════════════════════════════════════════════

// LoRAAdapter represents a fine-tuned LoRA adapter.
type LoRAAdapter struct {
	ID              string     `json:"id" gorm:"type:varchar(36);primaryKey"`
	UserID          string     `json:"user_id" gorm:"type:varchar(36);index;not null"`
	Name            string     `json:"name" gorm:"type:varchar(200);not null"`
	Description     string     `json:"description" gorm:"type:text"`
	BaseModel       string     `json:"base_model" gorm:"type:varchar(100);not null"`         // e.g. llama-3-8b, mistral-7b
	Rank            int        `json:"rank" gorm:"default:16"`                               // LoRA rank (r)
	Alpha           int        `json:"alpha" gorm:"default:32"`                              // LoRA alpha
	TargetModules   string     `json:"target_modules" gorm:"type:json"`                      // ["q_proj","v_proj","k_proj","o_proj"]
	Status          string     `json:"status" gorm:"type:varchar(20);default:pending;index"` // pending, training, ready, failed, archived
	TrainingSamples int64      `json:"training_samples" gorm:"default:0"`
	TrainingEpochs  int        `json:"training_epochs" gorm:"default:3"`
	LearningRate    float64    `json:"learning_rate" gorm:"default:0.0002"`
	BatchSize       int        `json:"batch_size" gorm:"default:4"`
	LossHistory     string     `json:"loss_history" gorm:"type:json"` // [{epoch, train_loss, eval_loss}]
	FinalLoss       float64    `json:"final_loss" gorm:"default:0"`
	AdapterPath     string     `json:"adapter_path" gorm:"type:varchar(500)"` // path to saved adapter weights
	AdapterSizeMB   float64    `json:"adapter_size_mb" gorm:"default:0"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	FinishedAt      *time.Time `json:"finished_at"`
}

func (a *LoRAAdapter) BeforeCreate(tx *gorm.DB) error {
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	return nil
}

// TrainingSample represents a single training example for fine-tuning.
type TrainingSample struct {
	ID        string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	AdapterID string    `json:"adapter_id" gorm:"type:varchar(36);index;not null"`
	UserID    string    `json:"user_id" gorm:"type:varchar(36);index;not null"`
	Input     string    `json:"input" gorm:"type:text;not null"`  // user message or prompt
	Output    string    `json:"output" gorm:"type:text;not null"` // desired assistant response
	System    string    `json:"system" gorm:"type:text"`          // optional system prompt
	Source    string    `json:"source" gorm:"type:varchar(50)"`   // manual, conversation, distillation
	Quality   float64   `json:"quality" gorm:"default:1.0"`       // 0.0-1.0 quality score
	CreatedAt time.Time `json:"created_at"`
}

func (s *TrainingSample) BeforeCreate(tx *gorm.DB) error {
	if s.ID == "" {
		s.ID = uuid.New().String()
	}
	return nil
}

// ════════════════════════════════════════════════════════════════
//  Knowledge Distillation
// ════════════════════════════════════════════════════════════════

// DistillationJob represents a knowledge distillation pipeline run.
type DistillationJob struct {
	ID             string     `json:"id" gorm:"type:varchar(36);primaryKey"`
	UserID         string     `json:"user_id" gorm:"type:varchar(36);index;not null"`
	Name           string     `json:"name" gorm:"type:varchar(200);not null"`
	TeacherModel   string     `json:"teacher_model" gorm:"type:varchar(100);not null"`      // e.g. gpt-4o, claude-3-opus
	StudentModel   string     `json:"student_model" gorm:"type:varchar(100);not null"`      // e.g. llama-3-8b, mistral-7b
	AdapterID      string     `json:"adapter_id" gorm:"type:varchar(36);index"`             // resulting LoRA adapter
	Status         string     `json:"status" gorm:"type:varchar(20);default:pending;index"` // pending, generating, training, completed, failed
	SeedPrompts    string     `json:"seed_prompts" gorm:"type:json"`                        // initial prompts to generate training data
	GeneratedCount int64      `json:"generated_count" gorm:"default:0"`                     // number of teacher outputs generated
	TargetCount    int64      `json:"target_count" gorm:"default:1000"`
	Temperature    float64    `json:"temperature" gorm:"default:0.7"`
	Config         string     `json:"config" gorm:"type:json"`
	Progress       float64    `json:"progress" gorm:"default:0"`
	Error          string     `json:"error" gorm:"type:text"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	CompletedAt    *time.Time `json:"completed_at"`
}

func (j *DistillationJob) BeforeCreate(tx *gorm.DB) error {
	if j.ID == "" {
		j.ID = uuid.New().String()
	}
	return nil
}

// ════════════════════════════════════════════════════════════════
//  Fine-Tune Engine
// ════════════════════════════════════════════════════════════════

// FineTuneEngine manages LoRA adapters and distillation pipelines.
type FineTuneEngine struct {
	db     *gorm.DB
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewFineTuneEngine creates the engine.
func NewFineTuneEngine(db *gorm.DB) *FineTuneEngine {
	return &FineTuneEngine{
		db:     db,
		stopCh: make(chan struct{}),
	}
}

// Start begins the background job processing loop.
func (e *FineTuneEngine) Start() {
	log.Println("[FineTune] Engine starting...")
	e.wg.Add(1)
	go e.jobLoop()
}

// Stop gracefully shuts down.
func (e *FineTuneEngine) Stop() {
	close(e.stopCh)
	e.wg.Wait()
	log.Println("[FineTune] Engine stopped")
}

// jobLoop checks for pending jobs periodically.
func (e *FineTuneEngine) jobLoop() {
	defer e.wg.Done()

	select {
	case <-e.stopCh:
		return
	case <-time.After(30 * time.Second):
	}

	log.Println("[FineTune] Job loop started (every 5m)")
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-e.stopCh:
			return
		case <-ticker.C:
			e.processPendingJobs()
		}
	}
}

// processPendingJobs moves pending jobs to the next stage.
func (e *FineTuneEngine) processPendingJobs() {
	// Check for pending distillation jobs
	var jobs []DistillationJob
	e.db.Where("status = ?", "pending").Limit(5).Find(&jobs)

	for _, job := range jobs {
		e.db.Model(&job).Update("status", "generating")
		log.Printf("[FineTune] Started distillation job: %s (%s → %s)", job.Name, job.TeacherModel, job.StudentModel)
	}

	// Check for pending training jobs (adapters)
	var adapters []LoRAAdapter
	e.db.Where("status = ?", "pending").Limit(5).Find(&adapters)

	for _, adapter := range adapters {
		// Check if adapter has enough training samples
		var count int64
		e.db.Model(&TrainingSample{}).Where("adapter_id = ?", adapter.ID).Count(&count)
		if count >= 10 {
			e.db.Model(&adapter).Updates(map[string]interface{}{
				"status":           "training",
				"training_samples": count,
			})
			log.Printf("[FineTune] Started training adapter: %s (samples: %d)", adapter.Name, count)
		}
	}
}

// ── LoRA Adapter CRUD ──

// CreateAdapter creates a new LoRA adapter configuration.
func (e *FineTuneEngine) CreateAdapter(adapter *LoRAAdapter) error {
	if adapter.TargetModules == "" {
		adapter.TargetModules = `["q_proj","v_proj","k_proj","o_proj"]`
	}
	if adapter.LossHistory == "" {
		adapter.LossHistory = "[]"
	}
	return e.db.Create(adapter).Error
}

// GetAdapter retrieves an adapter by ID.
func (e *FineTuneEngine) GetAdapter(id string) (*LoRAAdapter, error) {
	var adapter LoRAAdapter
	err := e.db.Where("id = ?", id).First(&adapter).Error
	return &adapter, err
}

// ListAdapters returns adapters for a user.
func (e *FineTuneEngine) ListAdapters(userID string, page, pageSize int) ([]LoRAAdapter, int64) {
	q := e.db.Model(&LoRAAdapter{}).Where("user_id = ?", userID)
	var total int64
	q.Count(&total)

	var adapters []LoRAAdapter
	q.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&adapters)
	return adapters, total
}

// DeleteAdapter removes an adapter (only if not training).
func (e *FineTuneEngine) DeleteAdapter(id, userID string) error {
	return e.db.Where("id = ? AND user_id = ? AND status NOT IN ?", id, userID, []string{"training"}).
		Delete(&LoRAAdapter{}).Error
}

// ── Training Samples ──

// AddSample adds a training sample.
func (e *FineTuneEngine) AddSample(sample *TrainingSample) error {
	return e.db.Create(sample).Error
}

// AddSamplesBatch adds multiple training samples at once.
func (e *FineTuneEngine) AddSamplesBatch(samples []TrainingSample) error {
	if len(samples) == 0 {
		return nil
	}
	return e.db.CreateInBatches(samples, 100).Error
}

// ListSamples returns training samples for an adapter.
func (e *FineTuneEngine) ListSamples(adapterID string, page, pageSize int) ([]TrainingSample, int64) {
	q := e.db.Model(&TrainingSample{}).Where("adapter_id = ?", adapterID)
	var total int64
	q.Count(&total)

	var samples []TrainingSample
	q.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&samples)
	return samples, total
}

// DeleteSample removes a training sample.
func (e *FineTuneEngine) DeleteSample(id, userID string) error {
	return e.db.Where("id = ? AND user_id = ?", id, userID).Delete(&TrainingSample{}).Error
}

// ExportSamplesJSONL exports samples as JSONL for training.
func (e *FineTuneEngine) ExportSamplesJSONL(adapterID string) ([]byte, error) {
	var samples []TrainingSample
	e.db.Where("adapter_id = ?", adapterID).Order("created_at ASC").Find(&samples)

	var lines []byte
	for _, s := range samples {
		entry := map[string]interface{}{
			"messages": []map[string]string{},
		}
		msgs := []map[string]string{}
		if s.System != "" {
			msgs = append(msgs, map[string]string{"role": "system", "content": s.System})
		}
		msgs = append(msgs, map[string]string{"role": "user", "content": s.Input})
		msgs = append(msgs, map[string]string{"role": "assistant", "content": s.Output})
		entry["messages"] = msgs

		line, _ := json.Marshal(entry)
		lines = append(lines, line...)
		lines = append(lines, '\n')
	}
	return lines, nil
}

// ── Distillation Jobs ──

// CreateDistillationJob creates a new distillation pipeline.
func (e *FineTuneEngine) CreateDistillationJob(job *DistillationJob) error {
	if job.Config == "" {
		job.Config = "{}"
	}
	if job.SeedPrompts == "" {
		job.SeedPrompts = "[]"
	}
	return e.db.Create(job).Error
}

// GetDistillationJob retrieves a job by ID.
func (e *FineTuneEngine) GetDistillationJob(id string) (*DistillationJob, error) {
	var job DistillationJob
	err := e.db.Where("id = ?", id).First(&job).Error
	return &job, err
}

// ListDistillationJobs returns jobs for a user.
func (e *FineTuneEngine) ListDistillationJobs(userID string, page, pageSize int) ([]DistillationJob, int64) {
	q := e.db.Model(&DistillationJob{}).Where("user_id = ?", userID)
	var total int64
	q.Count(&total)

	var jobs []DistillationJob
	q.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&jobs)
	return jobs, total
}

// CancelDistillationJob cancels a running job.
func (e *FineTuneEngine) CancelDistillationJob(id, userID string) error {
	return e.db.Model(&DistillationJob{}).
		Where("id = ? AND user_id = ? AND status IN ?", id, userID, []string{"pending", "generating"}).
		Update("status", "failed").Error
}

// Stats returns fine-tuning engine statistics.
func (e *FineTuneEngine) Stats(userID string) map[string]interface{} {
	var totalAdapters, readyAdapters, trainingAdapters int64
	e.db.Model(&LoRAAdapter{}).Where("user_id = ?", userID).Count(&totalAdapters)
	e.db.Model(&LoRAAdapter{}).Where("user_id = ? AND status = ?", userID, "ready").Count(&readyAdapters)
	e.db.Model(&LoRAAdapter{}).Where("user_id = ? AND status = ?", userID, "training").Count(&trainingAdapters)

	var totalSamples int64
	e.db.Model(&TrainingSample{}).Where("user_id = ?", userID).Count(&totalSamples)

	var totalJobs, activeJobs int64
	e.db.Model(&DistillationJob{}).Where("user_id = ?", userID).Count(&totalJobs)
	e.db.Model(&DistillationJob{}).Where("user_id = ? AND status IN ?", userID, []string{"generating", "training"}).Count(&activeJobs)

	return map[string]interface{}{
		"total_adapters":    totalAdapters,
		"ready_adapters":    readyAdapters,
		"training_adapters": trainingAdapters,
		"total_samples":     totalSamples,
		"total_jobs":        totalJobs,
		"active_jobs":       activeJobs,
	}
}

// GetDistillationPrompt returns the prompt template for knowledge distillation.
func GetDistillationPrompt() string {
	return `You are a knowledge distillation engine. Your role is to generate high-quality training examples.

Given a seed topic or prompt, generate diverse training examples that capture the teacher model's knowledge.

Each example should be a JSON object:
{
  "input": "user question or instruction",
  "output": "ideal assistant response",
  "system": "optional system prompt"
}

Guidelines:
- Generate diverse, high-quality examples
- Cover edge cases and nuances
- Maintain consistent quality and style
- Ensure outputs are factually accurate
- Vary complexity from simple to advanced`
}
