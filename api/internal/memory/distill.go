package memory

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/yinhe/starclaw/internal/broodmind"
	"github.com/yinhe/starclaw/internal/model"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
// Skill Distillation — Hermes-inspired Learning Loop
//
// Bridges BroodMind ReflectionArtifacts → Cerebrate skill memories.
// When a trajectory is completed successfully and the reflection engine
// produces high-quality artifacts, we distill them into reusable
// "skill" category memories that persist across sessions.
//
// This is the core of the closed learning loop:
//   trajectory → reflection → distillation → memory → future prompts
// ════════════════════════════════════════════════════════════════

// DistillConfig controls skill distillation behavior.
type DistillConfig struct {
	MinQuality        float64 // minimum quality score to distill (default 0.6)
	MinReusability    float64 // minimum reusability score to distill (default 0.5)
	MaxSkillsPerAgent int     // max skill memories per agent (default 50)
	Enabled           bool
}

// DefaultDistillConfig returns sensible defaults.
func DefaultDistillConfig() *DistillConfig {
	return &DistillConfig{
		MinQuality:        0.6,
		MinReusability:    0.5,
		MaxSkillsPerAgent: 50,
		Enabled:           true,
	}
}

// DistillFromArtifacts converts BroodMind reflection artifacts into Cerebrate skill memories.
// Should be called after ReflectionEngine.Reflect() produces artifacts.
func (c *Cerebrate) DistillFromArtifacts(artifacts []*broodmind.ReflectionArtifact, trajectory *broodmind.Trajectory, cfg *DistillConfig) int {
	if cfg == nil {
		cfg = DefaultDistillConfig()
	}
	if !cfg.Enabled || len(artifacts) == 0 || trajectory == nil {
		return 0
	}

	distilled := 0
	for _, art := range artifacts {
		// Only distill high-quality, reusable artifacts
		if art.QualityScore < cfg.MinQuality || art.ReusabilityScore < cfg.MinReusability {
			continue
		}

		// Skip failure patterns for now (they could be useful but add noise)
		if art.ArtifactType == broodmind.ArtFailurePattern {
			continue
		}

		// Build a skill memory from the artifact
		skillKey := buildSkillKey(art, trajectory)
		skillContent := buildSkillContent(art, trajectory)

		if skillKey == "" || skillContent == "" {
			continue
		}

		// Check if a similar skill already exists (dedup by key)
		var existing model.Memory
		err := c.db.Where("user_id = ? AND agent_id = ? AND category = ? AND `key` = ?",
			trajectory.UserID, trajectory.AgentID, model.MemCatSkill, skillKey).
			First(&existing).Error

		if err == nil {
			// Update existing skill — increment importance (it's been reinforced)
			newImportance := existing.Importance + 0.05
			if newImportance > 1.0 {
				newImportance = 1.0
			}
			c.db.Model(&existing).Updates(map[string]interface{}{
				"content":      skillContent,
				"importance":   newImportance,
				"access_count": gorm.Expr("access_count + 1"),
			})
			log.Printf("[distill] reinforced skill: %s (importance=%.2f)", skillKey, newImportance)
			distilled++
			continue
		}

		// Enforce capacity limit
		var skillCount int64
		c.db.Model(&model.Memory{}).Where("user_id = ? AND agent_id = ? AND category = ?",
			trajectory.UserID, trajectory.AgentID, model.MemCatSkill).Count(&skillCount)

		if int(skillCount) >= cfg.MaxSkillsPerAgent {
			// Delete least important/least accessed skill
			var oldest model.Memory
			c.db.Where("user_id = ? AND agent_id = ? AND category = ?",
				trajectory.UserID, trajectory.AgentID, model.MemCatSkill).
				Order("importance ASC, access_count ASC, updated_at ASC").
				First(&oldest)
			if oldest.ID != "" {
				c.db.Delete(&oldest)
				log.Printf("[distill] evicted low-value skill: %s", oldest.Key)
			}
		}

		// Create new skill memory
		importance := (art.QualityScore + art.ReusabilityScore) / 2
		if importance < 0.5 {
			importance = 0.5
		}

		mem := model.Memory{
			UserID:     trajectory.UserID,
			AgentID:    trajectory.AgentID,
			Key:        skillKey,
			Content:    skillContent,
			Category:   model.MemCatSkill,
			Source:     "distill",
			Scope:      model.MemScopeAgent,
			Importance: importance,
		}
		c.db.Create(&mem)

		// Embed for vector recall
		go c.embedMemory(&mem)

		log.Printf("[distill] new skill: %s (quality=%.2f, reuse=%.2f)", skillKey, art.QualityScore, art.ReusabilityScore)
		distilled++
	}

	if distilled > 0 {
		log.Printf("[distill] distilled %d skills from trajectory %s", distilled, trajectory.ID)
		if c.notifyFunc != nil && trajectory.UserID != "" {
			c.notifyFunc(trajectory.UserID, "skill_distilled", map[string]interface{}{
				"count":   distilled,
				"message": fmt.Sprintf("🧬 从任务执行中蒸馏了 %d 条新技能", distilled),
			})
		}
	}

	return distilled
}

// buildSkillKey generates a unique, descriptive key for a skill memory.
func buildSkillKey(art *broodmind.ReflectionArtifact, t *broodmind.Trajectory) string {
	// Use artifact type + tool chain as the key basis
	var parts []string

	switch art.ArtifactType {
	case broodmind.ArtSuccessPattern:
		parts = append(parts, "success")
	case broodmind.ArtLesson:
		parts = append(parts, "lesson")
	case broodmind.ArtRoutingHint:
		parts = append(parts, "routing")
	case broodmind.ArtPolicyCandidate:
		parts = append(parts, "policy")
	default:
		parts = append(parts, "skill")
	}

	// Add tool chain signature (first 3 unique tools)
	seen := map[string]bool{}
	toolCount := 0
	for _, step := range t.Steps {
		if !seen[step.ToolName] && toolCount < 3 {
			seen[step.ToolName] = true
			parts = append(parts, step.ToolName)
			toolCount++
		}
	}

	// Add a timestamp suffix for uniqueness
	parts = append(parts, fmt.Sprintf("%d", time.Now().UnixMilli()%100000))

	return strings.Join(parts, "_")
}

// buildSkillContent generates human-readable skill content from artifact + trajectory.
func buildSkillContent(art *broodmind.ReflectionArtifact, t *broodmind.Trajectory) string {
	var sb strings.Builder

	// Task description
	if t.Task != "" {
		task := t.Task
		if len(task) > 200 {
			task = task[:200] + "..."
		}
		sb.WriteString(fmt.Sprintf("任务: %s\n", task))
	}

	// Artifact summary
	if art.Summary != "" {
		sb.WriteString(fmt.Sprintf("经验: %s\n", art.Summary))
	}

	// Tool chain (procedural knowledge)
	if len(t.Steps) > 0 {
		sb.WriteString("工具链: ")
		var tools []string
		for _, step := range t.Steps {
			tools = append(tools, step.ToolName)
		}
		sb.WriteString(strings.Join(tools, " → "))
		sb.WriteString("\n")
	}

	// Model used
	if t.Model != "" {
		sb.WriteString(fmt.Sprintf("模型: %s\n", t.Model))
	}

	// Duration
	if t.DurationMs > 0 {
		sb.WriteString(fmt.Sprintf("耗时: %dms (%d步)\n", t.DurationMs, len(t.Steps)))
	}

	return sb.String()
}
