package v1

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	agentpkg "github.com/yinhe/starclaw/internal/agent"
	"gorm.io/gorm"
)

// FineTuneHandler manages LoRA adapters, training samples, and distillation jobs.
type FineTuneHandler struct {
	db     *gorm.DB
	engine *agentpkg.FineTuneEngine
}

// NewFineTuneHandler creates the handler.
func NewFineTuneHandler(db *gorm.DB, engine *agentpkg.FineTuneEngine) *FineTuneHandler {
	return &FineTuneHandler{db: db, engine: engine}
}

// ════════════════════════════════════════════════════════════════
//  LoRA Adapters
// ════════════════════════════════════════════════════════════════

// ListAdapters returns the user's LoRA adapters.
func (h *FineTuneHandler) ListAdapters(c *gin.Context) {
	userID := c.GetString("user_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}

	adapters, total := h.engine.ListAdapters(userID, page, pageSize)
	c.JSON(http.StatusOK, gin.H{
		"items":     adapters,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// CreateAdapter creates a new LoRA adapter.
func (h *FineTuneHandler) CreateAdapter(c *gin.Context) {
	userID := c.GetString("user_id")

	var req struct {
		Name          string  `json:"name" binding:"required"`
		Description   string  `json:"description"`
		BaseModel     string  `json:"base_model" binding:"required"`
		Rank          int     `json:"rank"`
		Alpha         int     `json:"alpha"`
		TargetModules string  `json:"target_modules"`
		TrainingEpochs int    `json:"training_epochs"`
		LearningRate  float64 `json:"learning_rate"`
		BatchSize     int     `json:"batch_size"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	adapter := agentpkg.LoRAAdapter{
		UserID:         userID,
		Name:           req.Name,
		Description:    req.Description,
		BaseModel:      req.BaseModel,
		Rank:           req.Rank,
		Alpha:          req.Alpha,
		TargetModules:  req.TargetModules,
		TrainingEpochs: req.TrainingEpochs,
		LearningRate:   req.LearningRate,
		BatchSize:      req.BatchSize,
	}
	if adapter.Rank == 0 {
		adapter.Rank = 16
	}
	if adapter.Alpha == 0 {
		adapter.Alpha = 32
	}
	if adapter.TrainingEpochs == 0 {
		adapter.TrainingEpochs = 3
	}
	if adapter.LearningRate == 0 {
		adapter.LearningRate = 0.0002
	}
	if adapter.BatchSize == 0 {
		adapter.BatchSize = 4
	}

	if err := h.engine.CreateAdapter(&adapter); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create adapter"})
		return
	}
	c.JSON(http.StatusCreated, adapter)
}

// GetAdapter returns a single adapter.
func (h *FineTuneHandler) GetAdapter(c *gin.Context) {
	id := c.Param("id")
	adapter, err := h.engine.GetAdapter(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "adapter not found"})
		return
	}
	c.JSON(http.StatusOK, adapter)
}

// DeleteAdapter deletes a LoRA adapter.
func (h *FineTuneHandler) DeleteAdapter(c *gin.Context) {
	userID := c.GetString("user_id")
	id := c.Param("id")
	if err := h.engine.DeleteAdapter(id, userID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "adapter not found or cannot delete while training"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

// StartTraining triggers training for an adapter.
func (h *FineTuneHandler) StartTraining(c *gin.Context) {
	userID := c.GetString("user_id")
	id := c.Param("id")

	adapter, err := h.engine.GetAdapter(id)
	if err != nil || adapter.UserID != userID {
		c.JSON(http.StatusNotFound, gin.H{"error": "adapter not found"})
		return
	}

	if adapter.Status != "pending" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "adapter must be in pending status to start training"})
		return
	}

	// Count samples
	var count int64
	h.db.Model(&agentpkg.TrainingSample{}).Where("adapter_id = ?", id).Count(&count)
	if count < 10 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "need at least 10 training samples", "current": count})
		return
	}

	h.db.Model(adapter).Updates(map[string]interface{}{
		"status":           "training",
		"training_samples": count,
	})

	c.JSON(http.StatusOK, gin.H{"status": "training", "samples": count})
}

// ExportSamples exports training samples as JSONL.
func (h *FineTuneHandler) ExportSamples(c *gin.Context) {
	id := c.Param("id")
	data, err := h.engine.ExportSamplesJSONL(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "export failed"})
		return
	}
	c.Header("Content-Disposition", "attachment; filename=training-data-"+id+".jsonl")
	c.Data(http.StatusOK, "application/jsonl", data)
}

// ════════════════════════════════════════════════════════════════
//  Training Samples
// ════════════════════════════════════════════════════════════════

// ListSamples returns training samples for an adapter.
func (h *FineTuneHandler) ListSamples(c *gin.Context) {
	adapterID := c.Param("id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))
	if page < 1 {
		page = 1
	}

	samples, total := h.engine.ListSamples(adapterID, page, pageSize)
	c.JSON(http.StatusOK, gin.H{
		"items":     samples,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// AddSample adds a single training sample.
func (h *FineTuneHandler) AddSample(c *gin.Context) {
	userID := c.GetString("user_id")
	adapterID := c.Param("id")

	var req struct {
		Input   string  `json:"input" binding:"required"`
		Output  string  `json:"output" binding:"required"`
		System  string  `json:"system"`
		Source  string  `json:"source"`
		Quality float64 `json:"quality"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	sample := agentpkg.TrainingSample{
		AdapterID: adapterID,
		UserID:    userID,
		Input:     req.Input,
		Output:    req.Output,
		System:    req.System,
		Source:    req.Source,
		Quality:   req.Quality,
	}
	if sample.Source == "" {
		sample.Source = "manual"
	}
	if sample.Quality == 0 {
		sample.Quality = 1.0
	}

	if err := h.engine.AddSample(&sample); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to add sample"})
		return
	}
	c.JSON(http.StatusCreated, sample)
}

// AddSamplesBatch adds multiple training samples at once.
func (h *FineTuneHandler) AddSamplesBatch(c *gin.Context) {
	userID := c.GetString("user_id")
	adapterID := c.Param("id")

	var req struct {
		Samples []struct {
			Input   string  `json:"input"`
			Output  string  `json:"output"`
			System  string  `json:"system"`
			Quality float64 `json:"quality"`
		} `json:"samples" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var samples []agentpkg.TrainingSample
	for _, s := range req.Samples {
		q := s.Quality
		if q == 0 {
			q = 1.0
		}
		samples = append(samples, agentpkg.TrainingSample{
			AdapterID: adapterID,
			UserID:    userID,
			Input:     s.Input,
			Output:    s.Output,
			System:    s.System,
			Source:    "batch",
			Quality:   q,
		})
	}

	if err := h.engine.AddSamplesBatch(samples); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to add samples"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"added": len(samples)})
}

// DeleteSample removes a training sample.
func (h *FineTuneHandler) DeleteSample(c *gin.Context) {
	userID := c.GetString("user_id")
	sampleID := c.Param("sample_id")
	if err := h.engine.DeleteSample(sampleID, userID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "sample not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

// ════════════════════════════════════════════════════════════════
//  Distillation Jobs
// ════════════════════════════════════════════════════════════════

// ListDistillationJobs returns the user's distillation jobs.
func (h *FineTuneHandler) ListDistillationJobs(c *gin.Context) {
	userID := c.GetString("user_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}

	jobs, total := h.engine.ListDistillationJobs(userID, page, pageSize)
	c.JSON(http.StatusOK, gin.H{
		"items":     jobs,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// CreateDistillationJob creates a new distillation pipeline.
func (h *FineTuneHandler) CreateDistillationJob(c *gin.Context) {
	userID := c.GetString("user_id")

	var req struct {
		Name          string  `json:"name" binding:"required"`
		TeacherModel  string  `json:"teacher_model" binding:"required"`
		StudentModel  string  `json:"student_model" binding:"required"`
		SeedPrompts   string  `json:"seed_prompts"`
		TargetCount   int64   `json:"target_count"`
		Temperature   float64 `json:"temperature"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	job := agentpkg.DistillationJob{
		UserID:       userID,
		Name:         req.Name,
		TeacherModel: req.TeacherModel,
		StudentModel: req.StudentModel,
		SeedPrompts:  req.SeedPrompts,
		TargetCount:  req.TargetCount,
		Temperature:  req.Temperature,
	}
	if job.TargetCount == 0 {
		job.TargetCount = 1000
	}
	if job.Temperature == 0 {
		job.Temperature = 0.7
	}

	if err := h.engine.CreateDistillationJob(&job); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create job"})
		return
	}
	c.JSON(http.StatusCreated, job)
}

// GetDistillationJob returns a single job.
func (h *FineTuneHandler) GetDistillationJob(c *gin.Context) {
	id := c.Param("id")
	job, err := h.engine.GetDistillationJob(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}
	c.JSON(http.StatusOK, job)
}

// CancelDistillationJob cancels a running job.
func (h *FineTuneHandler) CancelDistillationJob(c *gin.Context) {
	userID := c.GetString("user_id")
	id := c.Param("id")
	if err := h.engine.CancelDistillationJob(id, userID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found or cannot cancel"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "cancelled"})
}

// DistillationPrompt returns the prompt template for knowledge distillation.
func (h *FineTuneHandler) DistillationPrompt(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"prompt": agentpkg.GetDistillationPrompt()})
}

// FineTuneStats returns engine statistics.
func (h *FineTuneHandler) FineTuneStats(c *gin.Context) {
	userID := c.GetString("user_id")
	stats := h.engine.Stats(userID)
	c.JSON(http.StatusOK, stats)
}
