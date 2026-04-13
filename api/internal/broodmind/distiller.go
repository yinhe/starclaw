package broodmind

import (
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ════════════════════════════════════════════════════════════
// BroodMind v1 — Distillation Engine
//
// Promotes high-quality ReflectionArtifacts into reusable
// capability assets: skill templates, workflow templates,
// policy rules, and routing priors.
//
// State machine: candidate → scored → distilled → published
// ════════════════════════════════════════════════════════════

// DistillStatus represents the lifecycle of a distilled artifact
type DistillStatus string

const (
	DstCandidate      DistillStatus = "candidate"
	DstScored         DistillStatus = "scored"
	DstDistilled      DistillStatus = "distilled"
	DstReviewRequired DistillStatus = "review_required"
	DstApproved       DistillStatus = "approved"
	DstPublished      DistillStatus = "published"
	DstRejected       DistillStatus = "rejected"
	DstArchived       DistillStatus = "archived"
)

// DistillType classifies the kind of distilled capability
type DistillType string

const (
	DistSkillTemplate    DistillType = "skill_template"
	DistWorkflowTemplate DistillType = "workflow_template"
	DistPolicyRule       DistillType = "policy_rule"
	DistRoutingPrior     DistillType = "routing_prior"
	DistTrainingSlice    DistillType = "training_slice"
)

// DistilledArtifact is a reusable capability asset produced by distillation
type DistilledArtifact struct {
	ID             string        `json:"id"`
	SourceTraceID  string        `json:"source_trace_id"`
	SourceRefID    string        `json:"source_ref_id"`
	Type           DistillType   `json:"type"`
	Status         DistillStatus `json:"status"`
	Title          string        `json:"title"`
	Summary        string        `json:"summary"`
	ToolChain      []string      `json:"tool_chain,omitempty"`
	AgentID        string        `json:"agent_id"`
	NodeID         string        `json:"node_id,omitempty"`
	QualityScore   float64       `json:"quality_score"`
	UsageCount     int           `json:"usage_count"`
	CreatedAt      time.Time     `json:"created_at"`
	PublishedAt    *time.Time    `json:"published_at,omitempty"`
	ApprovedBy     string        `json:"approved_by,omitempty"`
}

// Distiller manages the distillation pipeline
type Distiller struct {
	mu        sync.RWMutex
	artifacts []*DistilledArtifact
	byID      map[string]*DistilledArtifact
	maxSize   int

	// Thresholds
	QualityThreshold     float64
	ReusabilityThreshold float64
	AutoPublish          bool // if true, skip review and auto-publish
}

// NewDistiller creates a new distillation engine
func NewDistiller() *Distiller {
	return &Distiller{
		artifacts:            make([]*DistilledArtifact, 0),
		byID:                make(map[string]*DistilledArtifact),
		maxSize:             500,
		QualityThreshold:     0.8,
		ReusabilityThreshold: 0.7,
		AutoPublish:          true, // v1: auto-publish for simplicity
	}
}

// Distill processes reflection candidates and produces distilled artifacts
func (d *Distiller) Distill(candidates []*ReflectionArtifact) []*DistilledArtifact {
	d.mu.Lock()
	defer d.mu.Unlock()

	var produced []*DistilledArtifact

	for _, ref := range candidates {
		if ref.QualityScore < d.QualityThreshold || ref.ReusabilityScore < d.ReusabilityThreshold {
			continue
		}
		if ref.PromoteTo == "" {
			continue
		}

		// Deduplicate: skip if we already have one from the same trace
		dup := false
		for _, existing := range d.artifacts {
			if existing.SourceTraceID == ref.TraceID && string(existing.Type) == ref.PromoteTo {
				dup = true
				break
			}
		}
		if dup {
			continue
		}

		da := &DistilledArtifact{
			ID:            "dist:" + uuid.New().String()[:8],
			SourceTraceID: ref.TraceID,
			SourceRefID:   ref.ID,
			Type:          DistillType(ref.PromoteTo),
			Status:        DstCandidate,
			Title:         buildDistillTitle(ref),
			Summary:       ref.Summary,
			ToolChain:     ref.ToolChain,
			AgentID:       ref.AgentID,
			NodeID:        ref.NodeID,
			QualityScore:  ref.QualityScore,
			CreatedAt:     time.Now(),
		}

		// Run through state machine
		da.Status = DstScored
		da.Status = DstDistilled
		if d.AutoPublish {
			da.Status = DstPublished
			now := time.Now()
			da.PublishedAt = &now
		} else {
			da.Status = DstReviewRequired
		}

		d.artifacts = append(d.artifacts, da)
		d.byID[da.ID] = da
		produced = append(produced, da)
	}

	// Evict oldest if over capacity
	for len(d.artifacts) > d.maxSize {
		old := d.artifacts[0]
		d.artifacts = d.artifacts[1:]
		delete(d.byID, old.ID)
	}

	if len(produced) > 0 {
		log.Printf("[broodmind/distill] produced %d artifacts from %d candidates", len(produced), len(candidates))
	}

	return produced
}

// Published returns all published distilled artifacts
func (d *Distiller) Published(limit int) []*DistilledArtifact {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var result []*DistilledArtifact
	for i := len(d.artifacts) - 1; i >= 0 && len(result) < limit; i-- {
		if d.artifacts[i].Status == DstPublished {
			result = append(result, d.artifacts[i])
		}
	}
	return result
}

// All returns all distilled artifacts (newest first)
func (d *Distiller) All(limit int) []*DistilledArtifact {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if limit <= 0 || limit > len(d.artifacts) {
		limit = len(d.artifacts)
	}
	result := make([]*DistilledArtifact, limit)
	for i := 0; i < limit; i++ {
		result[i] = d.artifacts[len(d.artifacts)-1-i]
	}
	return result
}

// Get retrieves a specific distilled artifact by ID
func (d *Distiller) Get(id string) *DistilledArtifact {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.byID[id]
}

// Approve transitions an artifact from review_required → approved → published
func (d *Distiller) Approve(id, approvedBy string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	a, ok := d.byID[id]
	if !ok {
		return false
	}
	if a.Status != DstReviewRequired && a.Status != DstDistilled {
		return false
	}
	a.Status = DstPublished
	a.ApprovedBy = approvedBy
	now := time.Now()
	a.PublishedAt = &now
	return true
}

// Reject transitions an artifact to rejected
func (d *Distiller) Reject(id string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	a, ok := d.byID[id]
	if !ok {
		return false
	}
	a.Status = DstRejected
	return true
}

// IncrUsage increments the usage counter for a published artifact
func (d *Distiller) IncrUsage(id string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if a, ok := d.byID[id]; ok {
		a.UsageCount++
	}
}

// ByType returns artifacts of a specific type
func (d *Distiller) ByType(t DistillType, limit int) []*DistilledArtifact {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var result []*DistilledArtifact
	for i := len(d.artifacts) - 1; i >= 0 && len(result) < limit; i-- {
		if d.artifacts[i].Type == t {
			result = append(result, d.artifacts[i])
		}
	}
	return result
}

// Stats returns distiller statistics
func (d *Distiller) Stats() map[string]interface{} {
	d.mu.RLock()
	defer d.mu.RUnlock()

	byType := map[DistillType]int{}
	byStatus := map[DistillStatus]int{}
	totalUsage := 0

	for _, a := range d.artifacts {
		byType[a.Type]++
		byStatus[a.Status]++
		totalUsage += a.UsageCount
	}

	return map[string]interface{}{
		"total":       len(d.artifacts),
		"by_type":     byType,
		"by_status":   byStatus,
		"total_usage": totalUsage,
	}
}

// ── Helpers ──

func buildDistillTitle(ref *ReflectionArtifact) string {
	switch ref.ArtifactType {
	case ArtSuccessPattern:
		return "技能: " + truncate(ref.Summary, 60)
	case ArtWorkflowHint:
		return "工作流: " + truncate(ref.Summary, 60)
	case ArtFailurePattern:
		return "失败规避: " + truncate(ref.Summary, 60)
	case ArtRoutingHint:
		return "路由偏好: " + truncate(ref.Summary, 60)
	default:
		return "经验: " + truncate(ref.Summary, 60)
	}
}
