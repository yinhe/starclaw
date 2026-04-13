package broodmind

import (
	"log"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ════════════════════════════════════════════════════════════
// BroodMind v1 — Reflection Engine
//
// Analyzes completed trajectories to produce ReflectionArtifacts:
//   - Quality scoring (success, efficiency, error recovery)
//   - Reusability scoring (generality, pattern frequency)
//   - Artifact type classification (lesson, success/failure pattern, etc.)
//
// High-scoring artifacts become candidates for distillation.
// ════════════════════════════════════════════════════════════

// ArtifactType classifies what kind of reflection was produced
type ArtifactType string

const (
	ArtLesson         ArtifactType = "lesson"
	ArtSuccessPattern ArtifactType = "success_pattern"
	ArtFailurePattern ArtifactType = "failure_pattern"
	ArtRoutingHint    ArtifactType = "routing_hint"
	ArtWorkflowHint   ArtifactType = "workflow_hint"
	ArtPolicyCandidate ArtifactType = "policy_candidate"
)

// ReflectionArtifact represents a single reflection product from a trajectory
type ReflectionArtifact struct {
	ID               string       `json:"id"`
	TraceID          string       `json:"trace_id"`
	AgentID          string       `json:"agent_id"`
	NodeID           string       `json:"node_id,omitempty"`
	ArtifactType     ArtifactType `json:"artifact_type"`
	Summary          string       `json:"summary"`
	ToolChain        []string     `json:"tool_chain,omitempty"`
	QualityScore     float64      `json:"quality_score"`
	ReusabilityScore float64      `json:"reusability_score"`
	PromoteTo        string       `json:"promote_to,omitempty"`
	IsDistilled      bool         `json:"is_distilled"`
	CreatedAt        time.Time    `json:"created_at"`
}

// ReflectionEngine analyzes trajectories and produces artifacts
type ReflectionEngine struct {
	mu        sync.RWMutex
	artifacts []*ReflectionArtifact
	byID      map[string]*ReflectionArtifact
	maxSize   int
}

// NewReflectionEngine creates a new reflection engine
func NewReflectionEngine(maxSize int) *ReflectionEngine {
	if maxSize <= 0 {
		maxSize = 1000
	}
	return &ReflectionEngine{
		artifacts: make([]*ReflectionArtifact, 0),
		byID:      make(map[string]*ReflectionArtifact),
		maxSize:   maxSize,
	}
}

// Reflect analyzes a completed trajectory and produces reflection artifacts
func (re *ReflectionEngine) Reflect(t *Trajectory) []*ReflectionArtifact {
	if t == nil || (t.Status != TraceCompleted && t.Status != TraceFailed) {
		return nil
	}

	var results []*ReflectionArtifact

	// Extract tool chain
	toolChain := make([]string, 0, len(t.Steps))
	for _, s := range t.Steps {
		toolChain = append(toolChain, s.ToolName)
	}

	// Score the trajectory
	quality := scoreQuality(t)
	reusability := scoreReusability(t)

	// Determine artifact type based on outcome and scores
	if t.Status == TraceCompleted {
		if quality >= 0.7 {
			art := &ReflectionArtifact{
				ID:               "ref:" + uuid.New().String()[:8],
				TraceID:          t.ID,
				AgentID:          t.AgentID,
				NodeID:           t.NodeID,
				ArtifactType:     ArtSuccessPattern,
				Summary:          buildSuccessSummary(t),
				ToolChain:        toolChain,
				QualityScore:     quality,
				ReusabilityScore: reusability,
				CreatedAt:        time.Now(),
			}
			if quality >= 0.8 && reusability >= 0.7 {
				art.PromoteTo = "skill_template"
			}
			results = append(results, art)
		}

		// If there's an interesting tool chain, create workflow hint
		if len(t.Steps) >= 3 && reusability >= 0.6 {
			art := &ReflectionArtifact{
				ID:               "ref:" + uuid.New().String()[:8],
				TraceID:          t.ID,
				AgentID:          t.AgentID,
				NodeID:           t.NodeID,
				ArtifactType:     ArtWorkflowHint,
				Summary:          buildWorkflowSummary(t),
				ToolChain:        toolChain,
				QualityScore:     quality,
				ReusabilityScore: reusability,
				CreatedAt:        time.Now(),
			}
			if quality >= 0.8 && reusability >= 0.7 {
				art.PromoteTo = "workflow_template"
			}
			results = append(results, art)
		}
	} else {
		// Failed trajectory — extract failure pattern
		art := &ReflectionArtifact{
			ID:               "ref:" + uuid.New().String()[:8],
			TraceID:          t.ID,
			AgentID:          t.AgentID,
			NodeID:           t.NodeID,
			ArtifactType:     ArtFailurePattern,
			Summary:          buildFailureSummary(t),
			ToolChain:        toolChain,
			QualityScore:     quality,
			ReusabilityScore: reusability * 0.8, // failure patterns slightly less reusable
			CreatedAt:        time.Now(),
		}
		results = append(results, art)
	}

	// Routing hint: if the model/agent combo was notably fast or slow
	if t.DurationMs > 0 {
		avgStepMs := t.DurationMs / int64(max(len(t.Steps), 1))
		if (t.Status == TraceCompleted && avgStepMs < 500) || avgStepMs > 5000 {
			art := &ReflectionArtifact{
				ID:               "ref:" + uuid.New().String()[:8],
				TraceID:          t.ID,
				AgentID:          t.AgentID,
				NodeID:           t.NodeID,
				ArtifactType:     ArtRoutingHint,
				Summary:          buildRoutingHint(t, avgStepMs),
				ToolChain:        toolChain,
				QualityScore:     quality,
				ReusabilityScore: 0.5,
				CreatedAt:        time.Now(),
			}
			results = append(results, art)
		}
	}

	// Store all artifacts
	re.mu.Lock()
	for _, a := range results {
		re.artifacts = append(re.artifacts, a)
		re.byID[a.ID] = a
	}
	// Evict if over capacity
	for len(re.artifacts) > re.maxSize {
		old := re.artifacts[0]
		re.artifacts = re.artifacts[1:]
		delete(re.byID, old.ID)
	}
	re.mu.Unlock()

	if len(results) > 0 {
		log.Printf("[broodmind/reflect] trajectory %s → %d artifacts (quality=%.2f, reuse=%.2f)",
			t.ID, len(results), quality, reusability)
	}

	return results
}

// Candidates returns artifacts eligible for distillation (quality >= threshold)
func (re *ReflectionEngine) Candidates(qualityThreshold float64) []*ReflectionArtifact {
	re.mu.RLock()
	defer re.mu.RUnlock()

	var result []*ReflectionArtifact
	for _, a := range re.artifacts {
		if !a.IsDistilled && a.QualityScore >= qualityThreshold && a.PromoteTo != "" {
			result = append(result, a)
		}
	}
	return result
}

// MarkDistilled marks artifacts as already distilled
func (re *ReflectionEngine) MarkDistilled(ids []string) {
	re.mu.Lock()
	defer re.mu.Unlock()
	for _, id := range ids {
		if a, ok := re.byID[id]; ok {
			a.IsDistilled = true
		}
	}
}

// Recent returns the N most recent artifacts
func (re *ReflectionEngine) Recent(limit int) []*ReflectionArtifact {
	re.mu.RLock()
	defer re.mu.RUnlock()

	if limit <= 0 || limit > len(re.artifacts) {
		limit = len(re.artifacts)
	}
	result := make([]*ReflectionArtifact, limit)
	for i := 0; i < limit; i++ {
		result[i] = re.artifacts[len(re.artifacts)-1-i]
	}
	return result
}

// Stats returns reflection engine statistics
func (re *ReflectionEngine) Stats() map[string]interface{} {
	re.mu.RLock()
	defer re.mu.RUnlock()

	byType := map[ArtifactType]int{}
	distilled := 0
	var totalQuality, totalReuse float64
	promotable := 0

	for _, a := range re.artifacts {
		byType[a.ArtifactType]++
		totalQuality += a.QualityScore
		totalReuse += a.ReusabilityScore
		if a.IsDistilled {
			distilled++
		}
		if a.PromoteTo != "" && !a.IsDistilled {
			promotable++
		}
	}

	n := len(re.artifacts)
	avgQ := 0.0
	avgR := 0.0
	if n > 0 {
		avgQ = totalQuality / float64(n)
		avgR = totalReuse / float64(n)
	}

	return map[string]interface{}{
		"total":           n,
		"by_type":         byType,
		"distilled":       distilled,
		"promotable":      promotable,
		"avg_quality":     avgQ,
		"avg_reusability": avgR,
	}
}

// ── Scoring Functions ──

func scoreQuality(t *Trajectory) float64 {
	score := 0.0

	// Base: success = 0.5, failure = 0.2
	if t.Status == TraceCompleted {
		score = 0.5
	} else {
		score = 0.2
	}

	// Efficiency: fewer steps for same task is better
	stepCount := len(t.Steps)
	if stepCount > 0 && stepCount <= 3 {
		score += 0.2
	} else if stepCount <= 7 {
		score += 0.1
	}

	// Speed bonus
	if t.DurationMs > 0 && t.DurationMs < 10000 {
		score += 0.1
	}

	// Error recovery: had errors but still succeeded
	errorSteps := 0
	for _, s := range t.Steps {
		if s.Error != "" {
			errorSteps++
		}
	}
	if t.Status == TraceCompleted && errorSteps > 0 {
		score += 0.15 // resilience bonus
	}

	// Penalty for too many errors
	if errorSteps > 3 {
		score -= 0.1
	}

	// Token efficiency
	if t.TotalTokens > 0 && t.TotalTokens < 5000 {
		score += 0.05
	}

	if score > 1.0 {
		score = 1.0
	}
	if score < 0.0 {
		score = 0.0
	}
	return score
}

func scoreReusability(t *Trajectory) float64 {
	score := 0.0

	// Structured tool chain is more reusable
	if len(t.Steps) >= 2 {
		score += 0.3
	}

	// Unique tool diversity indicates complex reusable pattern
	uniqueTools := map[string]bool{}
	for _, s := range t.Steps {
		uniqueTools[s.ToolName] = true
	}
	if len(uniqueTools) >= 2 {
		score += 0.2
	}
	if len(uniqueTools) >= 4 {
		score += 0.1
	}

	// Common agent patterns are more reusable
	if t.AgentID != "" {
		score += 0.1
	}

	// Success is more reusable than failure
	if t.Status == TraceCompleted {
		score += 0.3
	}

	if score > 1.0 {
		score = 1.0
	}
	return score
}

// ── Summary Builders ──

func buildSuccessSummary(t *Trajectory) string {
	var sb strings.Builder
	sb.WriteString("成功执行: ")
	sb.WriteString(truncate(t.Task, 80))
	sb.WriteString(" | Agent: ")
	sb.WriteString(t.AgentID)
	sb.WriteString(" | 模型: ")
	sb.WriteString(t.Model)
	sb.WriteString(" | 步骤: ")
	for i, s := range t.Steps {
		if i > 0 {
			sb.WriteString(" → ")
		}
		sb.WriteString(s.ToolName)
	}
	return sb.String()
}

func buildWorkflowSummary(t *Trajectory) string {
	var sb strings.Builder
	sb.WriteString("工作流模式: ")
	for i, s := range t.Steps {
		if i > 0 {
			sb.WriteString(" → ")
		}
		sb.WriteString(s.ToolName)
		if s.Duration > 1000 {
			sb.WriteString("(慢)")
		}
	}
	sb.WriteString(" | 总耗时: ")
	sb.WriteString(formatDuration(t.DurationMs))
	return sb.String()
}

func buildFailureSummary(t *Trajectory) string {
	var sb strings.Builder
	sb.WriteString("失败模式: ")
	sb.WriteString(truncate(t.Task, 60))
	sb.WriteString(" | 错误: ")
	sb.WriteString(truncate(t.ErrorMsg, 100))
	if len(t.Steps) > 0 {
		last := t.Steps[len(t.Steps)-1]
		if last.Error != "" {
			sb.WriteString(" | 最后失败工具: ")
			sb.WriteString(last.ToolName)
		}
	}
	return sb.String()
}

func buildRoutingHint(t *Trajectory, avgStepMs int64) string {
	var sb strings.Builder
	if avgStepMs < 500 {
		sb.WriteString("高效路由: ")
	} else {
		sb.WriteString("低效路由: ")
	}
	sb.WriteString("Agent=")
	sb.WriteString(t.AgentID)
	sb.WriteString(", Model=")
	sb.WriteString(t.Model)
	sb.WriteString(", 平均步骤耗时=")
	sb.WriteString(formatDuration(avgStepMs))
	return sb.String()
}

func formatDuration(ms int64) string {
	if ms < 1000 {
		return "<1s"
	}
	return time.Duration(ms * int64(time.Millisecond)).Round(time.Second).String()
}
