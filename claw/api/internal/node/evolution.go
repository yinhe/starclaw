package node

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/big"
	"sort"
	"sync"
	"time"
)

// ── Agent Variant (Individual in the population) ──

// AgentVariant represents one variant of an Agent with a specific configuration.
type AgentVariant struct {
	ID          string          `json:"id"`
	AgentID     string          `json:"agent_id"`     // parent agent
	Generation  int             `json:"generation"`
	SystemPrompt string         `json:"system_prompt"`
	Model       string          `json:"model"`
	Temperature float64         `json:"temperature"`
	Tools       []string        `json:"tools"`        // tool IDs
	MaxTokens   int             `json:"max_tokens"`
	Params      json.RawMessage `json:"params"`       // additional config
	Fitness     *FitnessScore   `json:"fitness"`
	CreatedAt   int64           `json:"created_at"`
	Lineage     []string        `json:"lineage"`      // ancestor variant IDs
}

// FitnessScore measures how well a variant performs.
type FitnessScore struct {
	TaskCompletion float64 `json:"task_completion"` // 0-1: % of tasks completed successfully
	UserSatisfaction float64 `json:"user_satisfaction"` // 0-1: user rating average
	Latency        float64 `json:"latency"`           // average response latency (ms)
	CostEfficiency float64 `json:"cost_efficiency"`   // output quality per token cost
	ErrorRate      float64 `json:"error_rate"`        // 0-1: % of errors/failures
	SampleSize     int     `json:"sample_size"`       // number of evaluations
	EvaluatedAt    int64   `json:"evaluated_at"`
}

// Score computes the composite fitness score (higher = better).
func (f *FitnessScore) Score() float64 {
	if f == nil || f.SampleSize == 0 {
		return 0
	}
	// Weighted composite: completion (40%) + satisfaction (30%) + efficiency (20%) + reliability (10%)
	completion := f.TaskCompletion * 40.0
	satisfaction := f.UserSatisfaction * 30.0
	efficiency := f.CostEfficiency * 20.0
	reliability := (1.0 - f.ErrorRate) * 10.0

	// Latency penalty (soft cap at 5000ms)
	latencyPenalty := math.Min(f.Latency/5000.0, 1.0) * 5.0

	return completion + satisfaction + efficiency + reliability - latencyPenalty
}

// ── Mutation Operators ──

// MutationType defines the kind of mutation applied.
type MutationType string

const (
	MutatePrompt      MutationType = "prompt"       // modify system prompt
	MutateModel       MutationType = "model"        // switch model
	MutateTemperature MutationType = "temperature"  // adjust temperature
	MutateTools       MutationType = "tools"        // add/remove tools
	MutateMaxTokens   MutationType = "max_tokens"   // adjust output length
	MutateCrossover   MutationType = "crossover"    // combine two variants
)

// MutationRecord logs what mutation was applied.
type MutationRecord struct {
	Type      MutationType `json:"type"`
	From      string       `json:"from"`     // original value summary
	To        string       `json:"to"`       // new value summary
	AppliedAt int64        `json:"applied_at"`
}

// ── Evolution Engine ──

// EvolutionConfig configures the evolution process.
type EvolutionConfig struct {
	PopulationSize   int           `json:"population_size"`    // max variants per agent (default 10)
	EliteCount       int           `json:"elite_count"`        // top N preserved (default 2)
	MutationRate     float64       `json:"mutation_rate"`      // 0-1 (default 0.3)
	CrossoverRate    float64       `json:"crossover_rate"`     // 0-1 (default 0.2)
	MinSamples       int           `json:"min_samples"`        // min evals before selection (default 10)
	EvalInterval     time.Duration `json:"eval_interval"`      // how often to evolve (default 24h)
	AvailableModels  []string      `json:"available_models"`
	AvailableTools   []string      `json:"available_tools"`
	PromptTemplates  []string      `json:"prompt_templates"`   // seed prompt variations
}

// DefaultEvolutionConfig returns sensible defaults.
func DefaultEvolutionConfig() EvolutionConfig {
	return EvolutionConfig{
		PopulationSize:  10,
		EliteCount:      2,
		MutationRate:    0.3,
		CrossoverRate:   0.2,
		MinSamples:      10,
		EvalInterval:    24 * time.Hour,
		AvailableModels: []string{"deepseek-chat", "gpt-4o-mini", "qwen-plus"},
		AvailableTools:  []string{},
	}
}

// EvolutionEngine manages the evolutionary process for Agent self-improvement.
type EvolutionEngine struct {
	config      EvolutionConfig
	populations map[string][]*AgentVariant // agentID -> variants
	history     map[string][]MutationRecord // variantID -> mutations
	mu          sync.RWMutex
	stopCh      chan struct{}
	started     bool

	// Callbacks
	onNewBest func(agentID string, variant *AgentVariant)
}

// NewEvolutionEngine creates the evolution engine.
func NewEvolutionEngine(config EvolutionConfig) *EvolutionEngine {
	return &EvolutionEngine{
		config:      config,
		populations: make(map[string][]*AgentVariant),
		history:     make(map[string][]MutationRecord),
		stopCh:      make(chan struct{}),
	}
}

// OnNewBest sets a callback for when a better variant is found.
func (ee *EvolutionEngine) OnNewBest(fn func(string, *AgentVariant)) {
	ee.onNewBest = fn
}

// Start begins the evolution loop.
func (ee *EvolutionEngine) Start() {
	ee.mu.Lock()
	if ee.started {
		ee.mu.Unlock()
		return
	}
	ee.started = true
	ee.mu.Unlock()

	go ee.evolutionLoop()
	log.Printf("[evolution] started: pop=%d elite=%d mutation=%.0f%% interval=%s",
		ee.config.PopulationSize, ee.config.EliteCount,
		ee.config.MutationRate*100, ee.config.EvalInterval)
}

// Stop halts the evolution engine.
func (ee *EvolutionEngine) Stop() {
	select {
	case <-ee.stopCh:
	default:
		close(ee.stopCh)
	}
}

// ── Population management ──

// SeedVariant adds an initial variant to an agent's population.
func (ee *EvolutionEngine) SeedVariant(variant *AgentVariant) {
	ee.mu.Lock()
	defer ee.mu.Unlock()

	variant.ID = generateVariantID()
	variant.Generation = 0
	variant.CreatedAt = time.Now().Unix()

	ee.populations[variant.AgentID] = append(ee.populations[variant.AgentID], variant)
}

// RecordEvaluation records a task evaluation result for a variant.
func (ee *EvolutionEngine) RecordEvaluation(variantID string, completion, satisfaction, latencyMs, costEff, errorRate float64) {
	ee.mu.Lock()
	defer ee.mu.Unlock()

	for _, pop := range ee.populations {
		for _, v := range pop {
			if v.ID == variantID {
				if v.Fitness == nil {
					v.Fitness = &FitnessScore{}
				}
				n := float64(v.Fitness.SampleSize)
				// Running average
				v.Fitness.TaskCompletion = (v.Fitness.TaskCompletion*n + completion) / (n + 1)
				v.Fitness.UserSatisfaction = (v.Fitness.UserSatisfaction*n + satisfaction) / (n + 1)
				v.Fitness.Latency = (v.Fitness.Latency*n + latencyMs) / (n + 1)
				v.Fitness.CostEfficiency = (v.Fitness.CostEfficiency*n + costEff) / (n + 1)
				v.Fitness.ErrorRate = (v.Fitness.ErrorRate*n + errorRate) / (n + 1)
				v.Fitness.SampleSize++
				v.Fitness.EvaluatedAt = time.Now().Unix()
				return
			}
		}
	}
}

// GetBestVariant returns the highest-scoring variant for an agent.
func (ee *EvolutionEngine) GetBestVariant(agentID string) *AgentVariant {
	ee.mu.RLock()
	defer ee.mu.RUnlock()

	pop := ee.populations[agentID]
	if len(pop) == 0 {
		return nil
	}

	var best *AgentVariant
	bestScore := -1.0
	for _, v := range pop {
		if v.Fitness != nil && v.Fitness.Score() > bestScore {
			bestScore = v.Fitness.Score()
			best = v
		}
	}
	return best
}

// ── Evolution operators ──

// Evolve runs one generation of evolution for an agent.
func (ee *EvolutionEngine) Evolve(agentID string) {
	ee.mu.Lock()
	defer ee.mu.Unlock()

	pop := ee.populations[agentID]
	if len(pop) == 0 {
		return
	}

	// Filter variants with enough evaluations
	var evaluated []*AgentVariant
	for _, v := range pop {
		if v.Fitness != nil && v.Fitness.SampleSize >= ee.config.MinSamples {
			evaluated = append(evaluated, v)
		}
	}

	if len(evaluated) < 2 {
		return // need at least 2 evaluated variants to evolve
	}

	// Sort by fitness (descending)
	sort.Slice(evaluated, func(i, j int) bool {
		return evaluated[i].Fitness.Score() > evaluated[j].Fitness.Score()
	})

	// Preserve elite
	newPop := make([]*AgentVariant, 0, ee.config.PopulationSize)
	eliteCount := min(ee.config.EliteCount, len(evaluated))
	for i := 0; i < eliteCount; i++ {
		newPop = append(newPop, evaluated[i])
	}

	// Track the best before evolution
	oldBest := evaluated[0]

	// Fill remaining slots with mutations and crossovers
	for len(newPop) < ee.config.PopulationSize && len(evaluated) > 0 {
		if randomFloat() < ee.config.CrossoverRate && len(evaluated) >= 2 {
			// Crossover: combine two parents
			p1 := tournamentSelect(evaluated)
			p2 := tournamentSelect(evaluated)
			child := ee.crossover(p1, p2, agentID)
			newPop = append(newPop, child)
		} else if randomFloat() < ee.config.MutationRate {
			// Mutation: modify a parent
			parent := tournamentSelect(evaluated)
			child := ee.mutate(parent, agentID)
			newPop = append(newPop, child)
		} else {
			// Clone a good parent
			parent := tournamentSelect(evaluated)
			child := ee.clone(parent, agentID)
			newPop = append(newPop, child)
		}
	}

	// Find new generation number
	maxGen := 0
	for _, v := range pop {
		if v.Generation > maxGen {
			maxGen = v.Generation
		}
	}

	ee.populations[agentID] = newPop

	// Check if we found a new best
	newBest := ee.getBestLocked(agentID)
	if newBest != nil && oldBest != nil &&
		newBest.ID != oldBest.ID &&
		newBest.Fitness.Score() > oldBest.Fitness.Score() {
		log.Printf("[evolution] agent=%s new best variant=%s score=%.1f (was %.1f)",
			agentID, newBest.ID[:8], newBest.Fitness.Score(), oldBest.Fitness.Score())
		if ee.onNewBest != nil {
			go ee.onNewBest(agentID, newBest)
		}
	}

	log.Printf("[evolution] agent=%s evolved gen=%d pop=%d best=%.1f",
		agentID, maxGen+1, len(newPop),
		func() float64 {
			if newBest != nil && newBest.Fitness != nil {
				return newBest.Fitness.Score()
			}
			return 0
		}())
}

func (ee *EvolutionEngine) getBestLocked(agentID string) *AgentVariant {
	pop := ee.populations[agentID]
	var best *AgentVariant
	bestScore := -1.0
	for _, v := range pop {
		if v.Fitness != nil && v.Fitness.Score() > bestScore {
			bestScore = v.Fitness.Score()
			best = v
		}
	}
	return best
}

// mutate creates a mutated variant from a parent.
func (ee *EvolutionEngine) mutate(parent *AgentVariant, agentID string) *AgentVariant {
	child := ee.clone(parent, agentID)

	// Pick a random mutation type
	mutations := []MutationType{MutatePrompt, MutateTemperature, MutateMaxTokens}
	if len(ee.config.AvailableModels) > 1 {
		mutations = append(mutations, MutateModel)
	}
	if len(ee.config.AvailableTools) > 0 {
		mutations = append(mutations, MutateTools)
	}

	mutType := mutations[randomInt(len(mutations))]
	var record MutationRecord
	record.Type = mutType
	record.AppliedAt = time.Now().Unix()

	switch mutType {
	case MutatePrompt:
		record.From = truncate(parent.SystemPrompt, 50)
		child.SystemPrompt = mutatePrompt(parent.SystemPrompt, ee.config.PromptTemplates)
		record.To = truncate(child.SystemPrompt, 50)

	case MutateModel:
		record.From = parent.Model
		models := ee.config.AvailableModels
		child.Model = models[randomInt(len(models))]
		record.To = child.Model

	case MutateTemperature:
		record.From = fmt.Sprintf("%.2f", parent.Temperature)
		// Perturb by ±0.1, clamp to [0.0, 2.0]
		delta := (randomFloat() - 0.5) * 0.2
		child.Temperature = math.Max(0, math.Min(2.0, parent.Temperature+delta))
		record.To = fmt.Sprintf("%.2f", child.Temperature)

	case MutateTools:
		record.From = fmt.Sprintf("%d tools", len(parent.Tools))
		if randomFloat() < 0.5 && len(ee.config.AvailableTools) > 0 {
			// Add a tool
			tool := ee.config.AvailableTools[randomInt(len(ee.config.AvailableTools))]
			child.Tools = append(child.Tools, tool)
		} else if len(child.Tools) > 0 {
			// Remove a tool
			idx := randomInt(len(child.Tools))
			child.Tools = append(child.Tools[:idx], child.Tools[idx+1:]...)
		}
		record.To = fmt.Sprintf("%d tools", len(child.Tools))

	case MutateMaxTokens:
		record.From = fmt.Sprintf("%d", parent.MaxTokens)
		// Perturb by ±256, clamp to [128, 16384]
		delta := (randomInt(513) - 256)
		child.MaxTokens = max(128, min(16384, parent.MaxTokens+delta))
		record.To = fmt.Sprintf("%d", child.MaxTokens)
	}

	ee.history[child.ID] = append(ee.history[child.ID], record)
	return child
}

// crossover combines two parent variants.
func (ee *EvolutionEngine) crossover(p1, p2 *AgentVariant, agentID string) *AgentVariant {
	child := &AgentVariant{
		ID:        generateVariantID(),
		AgentID:   agentID,
		CreatedAt: time.Now().Unix(),
		Lineage:   append(append([]string{}, p1.Lineage...), p1.ID, p2.ID),
	}

	// Find max generation
	child.Generation = max(p1.Generation, p2.Generation) + 1

	// Crossover: take attributes from both parents
	if randomFloat() < 0.5 {
		child.SystemPrompt = p1.SystemPrompt
	} else {
		child.SystemPrompt = p2.SystemPrompt
	}

	if randomFloat() < 0.5 {
		child.Model = p1.Model
	} else {
		child.Model = p2.Model
	}

	child.Temperature = (p1.Temperature + p2.Temperature) / 2.0
	child.MaxTokens = (p1.MaxTokens + p2.MaxTokens) / 2

	// Merge tools (union)
	toolSet := make(map[string]bool)
	for _, t := range p1.Tools {
		toolSet[t] = true
	}
	for _, t := range p2.Tools {
		toolSet[t] = true
	}
	for t := range toolSet {
		child.Tools = append(child.Tools, t)
	}

	record := MutationRecord{
		Type:      MutateCrossover,
		From:      fmt.Sprintf("%s × %s", p1.ID[:8], p2.ID[:8]),
		To:        child.ID[:8],
		AppliedAt: time.Now().Unix(),
	}
	ee.history[child.ID] = append(ee.history[child.ID], record)

	return child
}

// clone creates an identical copy with a new ID.
func (ee *EvolutionEngine) clone(parent *AgentVariant, agentID string) *AgentVariant {
	tools := make([]string, len(parent.Tools))
	copy(tools, parent.Tools)

	return &AgentVariant{
		ID:           generateVariantID(),
		AgentID:      agentID,
		Generation:   parent.Generation + 1,
		SystemPrompt: parent.SystemPrompt,
		Model:        parent.Model,
		Temperature:  parent.Temperature,
		Tools:        tools,
		MaxTokens:    parent.MaxTokens,
		Params:       parent.Params,
		CreatedAt:    time.Now().Unix(),
		Lineage:      append(append([]string{}, parent.Lineage...), parent.ID),
	}
}

// ── Selection ──

// tournamentSelect picks a variant using tournament selection (size=3).
func tournamentSelect(pop []*AgentVariant) *AgentVariant {
	tournamentSize := min(3, len(pop))
	var best *AgentVariant
	bestScore := -1.0

	for i := 0; i < tournamentSize; i++ {
		idx := randomInt(len(pop))
		v := pop[idx]
		score := 0.0
		if v.Fitness != nil {
			score = v.Fitness.Score()
		}
		if score > bestScore {
			bestScore = score
			best = v
		}
	}
	return best
}

// ── Prompt mutation helpers ──

// mutatePrompt applies a random modification to a system prompt.
func mutatePrompt(prompt string, templates []string) string {
	strategies := []string{"suffix", "rephrase", "template"}

	strategy := strategies[randomInt(len(strategies))]
	switch strategy {
	case "suffix":
		suffixes := []string{
			"\n请用简洁精确的语言回答。",
			"\n回答时先思考，再给出结论。",
			"\n如果不确定，请说明不确定的部分。",
			"\nBe concise and precise in your responses.",
			"\nThink step by step before answering.",
			"\nProvide examples when helpful.",
		}
		return prompt + suffixes[randomInt(len(suffixes))]

	case "rephrase":
		// Simple rephrasing: swap sentence order if multi-sentence
		if len(prompt) > 100 {
			mid := len(prompt) / 2
			// Find nearest sentence boundary
			for i := mid; i < len(prompt); i++ {
				if prompt[i] == '.' || prompt[i] == '\n' {
					return prompt[i+1:] + " " + prompt[:i+1]
				}
			}
		}
		return prompt

	case "template":
		if len(templates) > 0 {
			return templates[randomInt(len(templates))]
		}
		return prompt
	}
	return prompt
}

// ── Evolution loop ──

func (ee *EvolutionEngine) evolutionLoop() {
	ticker := time.NewTicker(ee.config.EvalInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ee.mu.RLock()
			agentIDs := make([]string, 0, len(ee.populations))
			for id := range ee.populations {
				agentIDs = append(agentIDs, id)
			}
			ee.mu.RUnlock()

			for _, agentID := range agentIDs {
				ee.Evolve(agentID)
			}

		case <-ee.stopCh:
			return
		}
	}
}

// Stats returns evolution statistics.
func (ee *EvolutionEngine) Stats() map[string]interface{} {
	ee.mu.RLock()
	defer ee.mu.RUnlock()

	agentStats := make(map[string]interface{})
	for agentID, pop := range ee.populations {
		var bestScore float64
		maxGen := 0
		evaluated := 0
		for _, v := range pop {
			if v.Generation > maxGen {
				maxGen = v.Generation
			}
			if v.Fitness != nil && v.Fitness.SampleSize > 0 {
				evaluated++
				if s := v.Fitness.Score(); s > bestScore {
					bestScore = s
				}
			}
		}
		agentStats[agentID] = map[string]interface{}{
			"population": len(pop),
			"generation": maxGen,
			"evaluated":  evaluated,
			"best_score": fmt.Sprintf("%.1f", bestScore),
		}
	}

	return map[string]interface{}{
		"config": map[string]interface{}{
			"population_size": ee.config.PopulationSize,
			"elite_count":     ee.config.EliteCount,
			"mutation_rate":   ee.config.MutationRate,
			"crossover_rate":  ee.config.CrossoverRate,
		},
		"agents":    agentStats,
		"total_agents": len(ee.populations),
	}
}

// ── Helpers ──

func generateVariantID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return "var_" + hex.EncodeToString(b)
}

func randomFloat() float64 {
	n, _ := rand.Int(rand.Reader, big.NewInt(10000))
	return float64(n.Int64()) / 10000.0
}

func randomInt(max int) int {
	if max <= 0 {
		return 0
	}
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(max)))
	return int(n.Int64())
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
