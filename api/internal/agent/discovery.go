package agent

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/yinhe/starclaw/internal/model"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════
//  Agent Discovery Engine — scans agents/ directory on startup,
//  parses manifest.yaml (Octad properties), and seeds the DB.
// ════════════════════════════════════════════════════════════

// I18nString supports bilingual text: {zh: "中文", en: "English"}
type I18nString map[string]string

func (s I18nString) Get(locale string) string {
	if v, ok := s[locale]; ok && v != "" {
		return v
	}
	if v, ok := s["zh"]; ok && v != "" {
		return v
	}
	if v, ok := s["en"]; ok && v != "" {
		return v
	}
	for _, v := range s {
		return v
	}
	return ""
}

// ── Manifest Types ──────────────────────────────────────────

type AgentManifest struct {
	ID          string     `yaml:"id"`
	Type        string     `yaml:"type"` // solo | team
	Name        I18nString `yaml:"name"`
	Description I18nString `yaml:"description"`
	Icon        string     `yaml:"icon"`
	Category    string     `yaml:"category"`
	Tags        []string   `yaml:"tags"`
	Version     string     `yaml:"version"`
	Status      string     `yaml:"status"`     // draft|beta|stable|deprecated
	Visibility  string     `yaml:"visibility"` // public|private|unlisted
	IsBuiltin   bool       `yaml:"is_builtin"`
	Featured    bool       `yaml:"featured"`

	Author ManifestAuthor `yaml:"author"`

	PromptFile  string            `yaml:"prompt_file"`
	PromptFiles map[string]string `yaml:"prompt_files"` // lang → file

	Model ManifestModel `yaml:"model"`

	Tools ManifestTools `yaml:"tools"`

	Skills []ManifestSkill `yaml:"skills"`
	Glands []ManifestGland `yaml:"glands"`

	Bridge *ManifestBridge `yaml:"bridge"`

	WorkflowFile string `yaml:"workflow_file"`

	Team *ManifestTeam `yaml:"team"`

	Marketplace *ManifestMarketplace `yaml:"marketplace"`

	// Set at parse time (not from YAML)
	Dir                string `yaml:"-"` // absolute directory path
	PromptText         string `yaml:"-"` // loaded prompt content
	WorkflowDefinition string `yaml:"-"`
}

type ManifestAuthor struct {
	ID       string     `yaml:"id"`
	Name     I18nString `yaml:"name"`
	Email    string     `yaml:"email"`
	URL      string     `yaml:"url"`
	Verified bool       `yaml:"verified"`
}

type ManifestModel struct {
	Name        string  `yaml:"name"`
	Temperature float64 `yaml:"temperature"`
	MaxTokens   int     `yaml:"max_tokens"`
}

type ManifestTools struct {
	Own     []string `yaml:"own"`
	Shared  []string `yaml:"shared"`
	Foreign []string `yaml:"foreign"`
}

type ManifestRoleTools []string

func (t *ManifestRoleTools) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case 0:
		*t = nil
		return nil
	case yaml.SequenceNode:
		var items []string
		if err := value.Decode(&items); err != nil {
			return err
		}
		*t = items
		return nil
	case yaml.MappingNode:
		var spec ManifestTools
		if err := value.Decode(&spec); err != nil {
			return err
		}
		items := make([]string, 0, len(spec.Own)+len(spec.Shared)+len(spec.Foreign))
		items = append(items, spec.Own...)
		items = append(items, spec.Shared...)
		items = append(items, spec.Foreign...)
		*t = items
		return nil
	default:
		return fmt.Errorf("invalid role tools format")
	}
}

type ManifestSkill struct {
	Name            I18nString `yaml:"name"`
	Trigger         string     `yaml:"trigger"` // passive | proactive
	Description     I18nString `yaml:"description"`
	Tools           []string   `yaml:"tools"`
	ExampleTriggers []string   `yaml:"example_triggers"`
	Schedule        string     `yaml:"schedule"`
	AutoExecute     bool       `yaml:"auto_execute"`
	Notify          bool       `yaml:"notify"`
}

type ManifestGland struct {
	Key       string     `yaml:"key"`
	Label     I18nString `yaml:"label"`
	Category  string     `yaml:"category"` // credential|threshold|toggle|endpoint|general
	Encrypted bool       `yaml:"encrypted"`
	Required  bool       `yaml:"required"`
	HelpText  I18nString `yaml:"help_text"`
	SortOrder int        `yaml:"sort_order"`
}

type ManifestBridge struct {
	Type         string            `yaml:"type"` // python|node|go|external
	Entry        string            `yaml:"entry"`
	Port         int               `yaml:"port"`
	PortRange    [2]int            `yaml:"port_range"`
	HealthCheck  string            `yaml:"health_check"`
	Dashboard    string            `yaml:"dashboard"`
	Requirements string            `yaml:"requirements"`
	AutoStart    bool              `yaml:"auto_start"`
	Env          map[string]string `yaml:"env"`
}

type ManifestTeam struct {
	Topology    string             `yaml:"topology"` // hierarchical|flat|pipeline
	Roles       []ManifestRole     `yaml:"roles"`
	QualityGate ManifestQG         `yaml:"quality_gate"`
	Escalation  ManifestEscalation `yaml:"escalation"`
}

type ManifestRole struct {
	Code       string            `yaml:"code"`
	Name       I18nString        `yaml:"name"`
	PromptFile string            `yaml:"prompt_file"`
	Tools      ManifestRoleTools `yaml:"tools"`
	Model      ManifestModel     `yaml:"model"`
	Count      int               `yaml:"count"`
	IsLead     bool              `yaml:"is_lead"`

	PromptText string `yaml:"-"` // loaded at parse time
}

type ManifestQG struct {
	ReviewRequired bool `yaml:"review_required"`
	MaxRetries     int  `yaml:"max_retries"`
	AutoTest       bool `yaml:"auto_test"`
}

type ManifestEscalation struct {
	OnFailure  string `yaml:"on_failure"`
	OnConflict string `yaml:"on_conflict"`
}

type ManifestMarketplace struct {
	Pricing       ManifestPricing       `yaml:"pricing"`
	Screenshots   []ManifestScreenshot  `yaml:"screenshots"`
	DemoURL       string                `yaml:"demo_url"`
	VideoURL      string                `yaml:"video_url"`
	Docs          ManifestDocs          `yaml:"docs"`
	Keywords      map[string][]string   `yaml:"keywords"`
	Compatibility ManifestCompatibility `yaml:"compatibility"`
}

type ManifestPricing struct {
	Type      string `yaml:"type"`   // free|one_time|subscription
	Price     int64  `yaml:"price"`  // cents
	Period    string `yaml:"period"` // month|quarter|year
	Currency  string `yaml:"currency"`
	Display   string `yaml:"display"`
	TrialDays int    `yaml:"trial_days"`
}

type ManifestScreenshot struct {
	URL     string     `yaml:"url"`
	Caption I18nString `yaml:"caption"`
}

type ManifestDocs struct {
	Readme    string `yaml:"readme"`
	Changelog string `yaml:"changelog"`
	FAQ       string `yaml:"faq"`
}

type ManifestCompatibility struct {
	ClawVersion string   `yaml:"claw_version"`
	OS          []string `yaml:"os"`
	Arch        []string `yaml:"arch"`
}

// ── Auto-derived Badges ─────────────────────────────────────

type AgentBadges struct {
	Source     string // builtin | marketplace | local
	Pricing    string // free | paid | trial
	Type       string // solo | team
	HasBridge  bool
	Status     string   // draft | beta | stable | deprecated
	Locales    []string // ["zh", "en"]
	RoleCount  int      // team only
	ToolCount  int
	SkillCount int
}

func DeriveBadges(m *AgentManifest) AgentBadges {
	source := "local"
	if m.IsBuiltin {
		source = "builtin"
	} else if m.Marketplace != nil && m.Marketplace.Pricing.Type != "" {
		source = "marketplace"
	}

	pricing := "free"
	if m.Marketplace != nil {
		switch m.Marketplace.Pricing.Type {
		case "subscription", "one_time":
			pricing = "paid"
			if m.Marketplace.Pricing.TrialDays > 0 {
				pricing = "trial"
			}
		}
	}

	locales := []string{"zh"}
	for lang := range m.PromptFiles {
		if lang != "zh" {
			locales = append(locales, lang)
		}
	}

	roleCount := 0
	if m.Team != nil {
		for _, r := range m.Team.Roles {
			c := r.Count
			if c < 1 {
				c = 1
			}
			roleCount += c
		}
	}

	agentType := m.Type
	if agentType == "" {
		agentType = "solo"
	}

	return AgentBadges{
		Source:     source,
		Pricing:    pricing,
		Type:       agentType,
		HasBridge:  m.Bridge != nil,
		Status:     m.Status,
		Locales:    locales,
		RoleCount:  roleCount,
		ToolCount:  len(m.Tools.Own) + len(m.Tools.Shared) + len(m.Tools.Foreign),
		SkillCount: len(m.Skills),
	}
}

// ── Global Manifest Registry ─────────────────────────────────

// GlobalManifests stores parsed manifests after ScanAgentsDir, so the API
// can query team structures, role descriptions, workflow stages, etc.
var GlobalManifests []AgentManifest

// GetTeamManifests returns all manifests of type "team".
func GetTeamManifests() []AgentManifest {
	var teams []AgentManifest
	for _, m := range GlobalManifests {
		if m.Type == "team" {
			teams = append(teams, m)
		}
	}
	return teams
}

// GetManifestByID returns a manifest by its ID.
func GetManifestByID(id string) *AgentManifest {
	for i := range GlobalManifests {
		if GlobalManifests[i].ID == id {
			return &GlobalManifests[i]
		}
	}
	return nil
}

// ── Scan ─────────────────────────────────────────────────────

// ScanAgentsDir reads all agent manifest.yaml files from the agents directory.
// Returns parsed manifests. Skips directories starting with "_" or ".".
func ScanAgentsDir(agentsDir string) ([]AgentManifest, error) {
	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		return nil, fmt.Errorf("cannot read agents dir %s: %w", agentsDir, err)
	}

	seen := map[string]string{} // id → dir
	var manifests []AgentManifest

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, "_") || strings.HasPrefix(name, ".") {
			continue
		}

		dir := filepath.Join(agentsDir, name)
		manifestPath := filepath.Join(dir, "manifest.yaml")
		if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
			log.Printf("[discovery] skip %s: no manifest.yaml", name)
			continue
		}

		m, err := ParseManifest(manifestPath, dir)
		if err != nil {
			log.Printf("[discovery] ERROR parsing %s: %v", name, err)
			continue
		}

		// Enforce id == directory name
		if m.ID != name {
			log.Printf("[discovery] WARNING %s: manifest id=%q != dir=%q, using dir name", name, m.ID, name)
			m.ID = name
		}

		// Duplicate check
		if prev, dup := seen[m.ID]; dup {
			log.Printf("[discovery] ERROR duplicate agent id %q: %s and %s — skipping", m.ID, prev, dir)
			continue
		}
		seen[m.ID] = dir

		manifests = append(manifests, *m)
	}

	log.Printf("[discovery] scanned %d agents from %s", len(manifests), agentsDir)
	return manifests, nil
}

// ParseManifest reads and validates a single manifest.yaml.
func ParseManifest(path, dir string) (*AgentManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}

	var m AgentManifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse YAML: %w", err)
	}
	m.Dir = dir

	// Defaults
	if m.Type == "" {
		m.Type = "solo"
	}
	if m.Status == "" {
		m.Status = "stable"
	}
	if m.Visibility == "" {
		m.Visibility = "public"
	}

	// Validate required fields
	if m.ID == "" {
		return nil, fmt.Errorf("missing required field: id")
	}
	if len(m.Name) == 0 {
		return nil, fmt.Errorf("missing required field: name")
	}

	// Load prompt file
	if m.PromptFile != "" {
		promptPath := filepath.Join(dir, m.PromptFile)
		if content, err := os.ReadFile(promptPath); err == nil {
			m.PromptText = string(content)
		} else {
			log.Printf("[discovery] WARNING %s: cannot read prompt file %s: %v", m.ID, m.PromptFile, err)
		}
	}
	if m.WorkflowFile != "" {
		workflowPath := filepath.Join(dir, m.WorkflowFile)
		content, err := os.ReadFile(workflowPath)
		if err != nil {
			return nil, fmt.Errorf("read workflow file %s: %w", m.WorkflowFile, err)
		}
		var workflowDef map[string]interface{}
		if err := json.Unmarshal(content, &workflowDef); err != nil {
			return nil, fmt.Errorf("invalid workflow file %s: %w", m.WorkflowFile, err)
		}
		m.WorkflowDefinition = string(content)
	}

	// Load team role prompts
	if m.Team != nil {
		for i := range m.Team.Roles {
			r := &m.Team.Roles[i]
			if r.Count < 1 {
				r.Count = 1
			}
			if r.PromptFile != "" {
				rolePath := filepath.Join(dir, r.PromptFile)
				if content, err := os.ReadFile(rolePath); err == nil {
					r.PromptText = string(content)
				} else {
					log.Printf("[discovery] WARNING %s role %s: cannot read prompt %s", m.ID, r.Code, r.PromptFile)
				}
			}
		}
	}

	return &m, nil
}

// ── Seed to DB ───────────────────────────────────────────────

// SeedFromManifests upserts all discovered agents into the database.
// It also loads shared skills from the monorepo skills/ directory and
// appends them to each agent's system prompt.
func SeedFromManifests(db *gorm.DB, manifests []AgentManifest, ownerID string) {
	// Load shared skills once (auto-detects monorepo root)
	if len(manifests) > 0 {
		LoadSharedSkills(manifests[0].Dir)
	}

	for _, m := range manifests {
		switch m.Type {
		case "team":
			seedTeamAgent(db, &m, ownerID)
		default:
			seedSoloAgent(db, &m, ownerID)
		}
	}
}

func seedSoloAgent(db *gorm.DB, m *AgentManifest, ownerID string) {
	locale := "zh"
	name := m.Name.Get(locale)
	desc := m.Description.Get(locale)

	// Build tools JSON array (namespaced own + shared + foreign)
	allTools := buildToolsList(m)
	toolsJSON := toJSONArray(allTools)

	// Build config JSON
	configJSON := fmt.Sprintf(`{"temperature":%v,"max_tokens":%d}`, m.Model.Temperature, m.Model.MaxTokens)

	// Append shared skills to prompt
	fullPrompt := m.PromptText + sharedSkillsCache

	var agent model.Agent
	err := db.Where("manifest_id = ? AND (user_id = ? OR user_id = ?)", m.ID, ownerID, model.SystemUserID).First(&agent).Error

	if err != nil {
		// Create new
		agent = model.Agent{
			UserID:       ownerID,
			Name:         name,
			Description:  desc,
			SystemPrompt: fullPrompt,
			ModelName:    m.Model.Name,
			Tools:        toolsJSON,
			Config:       configJSON,
			IsPublic:     m.Visibility == "public",
			IsBuiltin:    m.IsBuiltin,
			ManifestID:   m.ID,
		}
		if m.Bridge != nil {
			agent.BridgePort = m.Bridge.Port
		}
		db.Create(&agent)
		log.Printf("[discovery] created agent: %s (%s)", name, m.ID)
	} else {
		// Update existing
		updates := map[string]interface{}{
			"name":          name,
			"description":   desc,
			"system_prompt": fullPrompt,
			"model_name":    m.Model.Name,
			"tools":         toolsJSON,
			"config":        configJSON,
			"is_public":     m.Visibility == "public",
			"is_builtin":    m.IsBuiltin,
		}
		if m.Bridge != nil {
			updates["bridge_port"] = m.Bridge.Port
		}
		db.Model(&agent).Updates(updates)
		log.Printf("[discovery] updated agent: %s (%s)", name, m.ID)
	}

	// Clean up old hardcoded duplicates: same name + is_builtin but no manifest_id
	if m.IsBuiltin {
		result := db.Where(
			"name = ? AND is_builtin = ? AND (manifest_id IS NULL OR manifest_id = '') AND id != ?",
			name, true, agent.ID,
		).Delete(&model.Agent{})
		if result.RowsAffected > 0 {
			log.Printf("[discovery] cleaned %d old hardcoded duplicate(s) for %s", result.RowsAffected, name)
		}
	}

	// Upsert skills
	seedSkills(db, agent.ID, m)

	// Upsert shared skills as AgentSkill records (visible in skill panel)
	seedSharedSkillRecords(db, agent.ID)

	// Upsert gland definitions (schema only, not values)
	seedGlandDefs(db, agent.ID, ownerID, m)

	// Upsert workflow definition
	seedWorkflow(db, agent.ID, ownerID, m)
}

func seedTeamAgent(db *gorm.DB, m *AgentManifest, ownerID string) {
	if m.Team == nil || len(m.Team.Roles) == 0 {
		log.Printf("[discovery] team %s has no roles, skipping", m.ID)
		return
	}

	locale := "zh"
	teamName := m.Name.Get(locale)

	// Create lead agent with team-level properties
	for _, role := range m.Team.Roles {
		roleName := role.Name.Get(locale)
		count := role.Count
		if count < 1 {
			count = 1
		}

		for i := 0; i < count; i++ {
			suffix := ""
			agentName := fmt.Sprintf("%s · %s", teamName, roleName)
			manifestRoleID := fmt.Sprintf("%s.%s", m.ID, role.Code)
			if count > 1 {
				suffix = fmt.Sprintf("-%d", i+1)
				agentName = fmt.Sprintf("%s · %s%s", teamName, roleName, suffix)
				manifestRoleID = fmt.Sprintf("%s.%s%s", m.ID, role.Code, suffix)
			}

			// Build role tools (shared namespace)
			roleTools := make([]string, 0, len(role.Tools))
			for _, t := range role.Tools {
				if strings.Contains(t, ":") || isSharedTool(t) {
					roleTools = append(roleTools, t)
				} else {
					roleTools = append(roleTools, m.ID+":"+t)
				}
			}
			toolsJSON := toJSONArray(roleTools)

			temp := role.Model.Temperature
			if temp == 0 {
				temp = m.Model.Temperature
			}
			maxTok := role.Model.MaxTokens
			if maxTok == 0 {
				maxTok = m.Model.MaxTokens
			}
			modelName := role.Model.Name
			if modelName == "" {
				modelName = m.Model.Name
			}
			configJSON := fmt.Sprintf(`{"temperature":%v,"max_tokens":%d}`, temp, maxTok)

			var agent model.Agent
			err := db.Where("manifest_id = ? AND (user_id = ? OR user_id = ?)", manifestRoleID, ownerID, model.SystemUserID).First(&agent).Error

			if err != nil {
				agent = model.Agent{
					UserID:       ownerID,
					Name:         agentName,
					Description:  m.Description.Get(locale),
					SystemPrompt: role.PromptText,
					ModelName:    modelName,
					Tools:        toolsJSON,
					Config:       configJSON,
					RoleCode:     role.Code,
					IsPublic:     m.Visibility == "public",
					IsBuiltin:    m.IsBuiltin,
					ManifestID:   manifestRoleID,
				}
				db.Create(&agent)
				log.Printf("[discovery] created team role: %s", agentName)
			} else {
				db.Model(&agent).Updates(map[string]interface{}{
					"name":          agentName,
					"description":   m.Description.Get(locale),
					"system_prompt": role.PromptText,
					"model_name":    modelName,
					"tools":         toolsJSON,
					"config":        configJSON,
					"role_code":     role.Code,
					"is_public":     m.Visibility == "public",
					"is_builtin":    m.IsBuiltin,
				})
			}

			// Lead role gets team-level skills and glands
			if role.IsLead && i == 0 {
				seedSkills(db, agent.ID, m)
				seedSharedSkillRecords(db, agent.ID)
				seedGlandDefs(db, agent.ID, ownerID, m)
				seedWorkflow(db, agent.ID, ownerID, m)
			}
		}
	}
	log.Printf("[discovery] seeded team: %s (%d roles)", teamName, len(m.Team.Roles))
}

// ── Skills ───────────────────────────────────────────────────

func seedSkills(db *gorm.DB, agentID string, m *AgentManifest) {
	for _, s := range m.Skills {
		skillName := s.Name.Get("zh")
		abilityType := "skill"
		if s.Trigger == "proactive" {
			abilityType = "instinct"
		}

		// Build skill spec JSON
		spec := map[string]interface{}{
			"trigger":     s.Trigger,
			"description": s.Description.Get("zh"),
			"tools":       namespacedSkillTools(m.ID, s.Tools),
		}
		if len(s.ExampleTriggers) > 0 {
			spec["example_triggers"] = s.ExampleTriggers
		}
		if s.Schedule != "" {
			spec["schedule"] = s.Schedule
		}
		if s.AutoExecute {
			spec["auto_execute"] = true
		}
		if s.Notify {
			spec["notify"] = true
		}
		specBytes, _ := json.Marshal(spec)

		var existing model.AgentSkill
		err := db.Where("agent_id = ? AND skill_name = ?", agentID, skillName).First(&existing).Error
		if err != nil {
			db.Create(&model.AgentSkill{
				AgentID:     agentID,
				SkillName:   skillName,
				SkillSpec:   string(specBytes),
				AbilityType: abilityType,
				Enabled:     true,
				InstalledAt: time.Now(),
			})
		} else {
			db.Model(&existing).Updates(map[string]interface{}{
				"skill_spec":   string(specBytes),
				"ability_type": abilityType,
			})
		}
	}
}

// ── Glands ───────────────────────────────────────────────────

func seedGlandDefs(db *gorm.DB, agentID, userID string, m *AgentManifest) {
	for _, g := range m.Glands {
		var existing model.AgentGland
		err := db.Where("agent_id = ? AND `key` = ?", agentID, g.Key).First(&existing).Error
		if err != nil {
			// Create schema definition (no value — user fills in later)
			db.Create(&model.AgentGland{
				ID:        uuid.New().String(),
				AgentID:   agentID,
				UserID:    userID,
				Key:       g.Key,
				Label:     g.Label.Get("zh"),
				Category:  g.Category,
				Encrypted: g.Encrypted,
				Required:  g.Required,
				HelpText:  g.HelpText.Get("zh"),
				SortOrder: g.SortOrder,
			})
		} else {
			// Update schema (preserve user-set value)
			db.Model(&existing).Updates(map[string]interface{}{
				"label":      g.Label.Get("zh"),
				"category":   g.Category,
				"encrypted":  g.Encrypted,
				"required":   g.Required,
				"help_text":  g.HelpText.Get("zh"),
				"sort_order": g.SortOrder,
			})
		}
	}
}

// ── Workflow ─────────────────────────────────────────────────

func seedWorkflow(db *gorm.DB, agentID, userID string, m *AgentManifest) {
	if strings.TrimSpace(m.WorkflowDefinition) == "" {
		return
	}
	locale := "zh"
	name := m.Name.Get(locale)
	if name == "" {
		name = m.ID
	}
	workflowName := name + " · 默认流程"
	description := m.Description.Get(locale)
	var existing model.Workflow
	err := db.Where("agent_id = ? AND user_id = ? AND name = ?", agentID, userID, workflowName).First(&existing).Error
	if err != nil {
		db.Create(&model.Workflow{
			ID:          uuid.New().String(),
			UserID:      userID,
			AgentID:     agentID,
			Name:        workflowName,
			Description: description,
			Definition:  m.WorkflowDefinition,
			IsPublic:    m.Visibility == "public",
		})
		return
	}
	db.Model(&existing).Updates(map[string]interface{}{
		"description": description,
		"definition":  m.WorkflowDefinition,
		"is_public":   m.Visibility == "public",
	})
}

// ── Shared Skills ────────────────────────────────────────────

// sharedSkillsCache holds loaded shared skills content (loaded once per startup)
var sharedSkillsCache string

// sharedSkillMeta stores parsed metadata for each shared skill file.
type sharedSkillMeta struct {
	Name        string
	Description string
	Category    string
	Tags        string
}

// sharedSkillsMetaCache holds parsed metadata for creating AgentSkill records.
var sharedSkillsMetaCache []sharedSkillMeta

// LoadSharedSkills reads .md skill files from the monorepo skills/ directory.
// It walks up from agentsDir to find the git root, then loads skills/*.md.
// The result is cached and appended to every agent's system prompt.
// Metadata is also cached for creating AgentSkill records via seedSharedSkillRecords.
func LoadSharedSkills(agentsDir string) string {
	if sharedSkillsCache != "" {
		return sharedSkillsCache
	}

	// Walk up from claw/agents/ to find monorepo root (containing .git)
	abs, err := filepath.Abs(agentsDir)
	if err != nil {
		return ""
	}
	repoRoot := ""
	dir := abs
	for {
		if info, err := os.Stat(filepath.Join(dir, ".git")); err == nil && info.IsDir() {
			repoRoot = dir
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	if repoRoot == "" {
		return ""
	}

	skillsDir := filepath.Join(repoRoot, "skills")
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return ""
	}

	var sb strings.Builder
	count := 0
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".md" || e.Name() == "README.md" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(skillsDir, e.Name()))
		if err != nil {
			continue
		}
		content := string(data)
		name := strings.TrimSuffix(e.Name(), ".md")

		// Parse and cache metadata
		meta := sharedSkillMeta{Name: name}
		body := content
		if strings.HasPrefix(content, "---\n") {
			if parts := strings.SplitN(content[4:], "\n---\n", 2); len(parts) == 2 {
				body = strings.TrimSpace(parts[1])
				// Extract frontmatter fields
				for _, line := range strings.Split(parts[0], "\n") {
					line = strings.TrimSpace(line)
					if strings.HasPrefix(line, "description:") {
						meta.Description = strings.TrimSpace(strings.TrimPrefix(line, "description:"))
					} else if strings.HasPrefix(line, "category:") {
						meta.Category = strings.TrimSpace(strings.TrimPrefix(line, "category:"))
					} else if strings.HasPrefix(line, "tags:") {
						meta.Tags = strings.TrimSpace(strings.TrimPrefix(line, "tags:"))
						meta.Tags = strings.Trim(meta.Tags, "[]")
						meta.Tags = strings.ReplaceAll(meta.Tags, " ", "")
					}
				}
			}
		}
		if meta.Description == "" {
			meta.Description = name + " skill"
		}
		sharedSkillsMetaCache = append(sharedSkillsMetaCache, meta)

		sb.WriteString(fmt.Sprintf("\n### %s\n%s\n", name, body))
		count++
	}

	if count > 0 {
		sharedSkillsCache = "\n## Shared Knowledge\n" + sb.String()
		log.Printf("[discovery] loaded %d shared skills from %s", count, skillsDir)
	}
	return sharedSkillsCache
}

// seedSharedSkillRecords creates AgentSkill records for shared skills on a given agent.
// This makes shared skills visible in the agent's skill panel alongside manifest skills.
func seedSharedSkillRecords(db *gorm.DB, agentID string) {
	for _, meta := range sharedSkillsMetaCache {
		skillName := "shared:" + meta.Name

		spec := map[string]interface{}{
			"trigger":     "passive",
			"description": meta.Description,
			"category":    meta.Category,
			"tags":        meta.Tags,
			"source":      "shared-skills",
		}
		specBytes, _ := json.Marshal(spec)

		var existing model.AgentSkill
		err := db.Where("agent_id = ? AND skill_name = ?", agentID, skillName).First(&existing).Error
		if err != nil {
			db.Create(&model.AgentSkill{
				AgentID:     agentID,
				SkillName:   skillName,
				SkillSpec:   string(specBytes),
				AbilityType: "skill",
				Enabled:     true,
				InstalledAt: time.Now(),
			})
		} else {
			db.Model(&existing).Updates(map[string]interface{}{
				"skill_spec": string(specBytes),
			})
		}
	}
}

// ── Helpers ──────────────────────────────────────────────────

// sharedTools are Claw built-in tools that don't need namespace prefix.
var sharedTools = map[string]bool{
	"web_search": true, "browser": true, "http_request": true,
	"code": true, "system": true, "document": true, "desktop": true,
	"video_generation": true, "music_generation": true, "image_generation": true,
	"dubbing": true, "mv_production": true, "comic_production": true,
	"audio_analysis": true,
}

func isSharedTool(name string) bool {
	return sharedTools[name]
}

func buildToolsList(m *AgentManifest) []string {
	var all []string
	// Own tools — namespaced
	for _, t := range m.Tools.Own {
		all = append(all, m.ID+":"+t)
	}
	// Shared tools — as-is
	all = append(all, m.Tools.Shared...)
	// Foreign tools — as-is (already namespaced)
	all = append(all, m.Tools.Foreign...)
	return all
}

func namespacedSkillTools(agentID string, tools []string) []string {
	out := make([]string, len(tools))
	for i, t := range tools {
		if strings.Contains(t, ":") || isSharedTool(t) {
			out[i] = t
		} else {
			out[i] = agentID + ":" + t
		}
	}
	return out
}

func toJSONArray(items []string) string {
	if len(items) == 0 {
		return "[]"
	}
	parts := make([]string, len(items))
	for i, s := range items {
		parts[i] = fmt.Sprintf("%q", s)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

// ── Marketplace ranking ─────────────────────────────────────

// MarketplaceScore computes a ranking score for agent marketplace listings.
func MarketplaceScore(rating float64, ratingCount, installCount int, updatedAt time.Time, isBuiltin, featured bool) float64 {
	// Wilson lower bound for rating confidence
	wilson := wilsonLowerBound(rating, ratingCount)

	// Popularity (log of installs)
	pop := math.Log10(float64(installCount) + 1)

	// Freshness decay
	days := time.Since(updatedAt).Hours() / 24
	fresh := 1.0 / (1.0 + days/30.0)

	// Official boost
	boost := 1.0
	if isBuiltin {
		boost = 1.2
	}
	if featured {
		boost = 1.5
	}

	return wilson * pop * fresh * boost
}

func wilsonLowerBound(rating float64, n int) float64 {
	if n == 0 {
		return 0
	}
	// Convert 1-5 star rating to 0-1 proportion
	p := (rating - 1.0) / 4.0
	z := 1.96 // 95% confidence
	nf := float64(n)
	denominator := 1 + z*z/nf
	centre := p + z*z/(2*nf)
	spread := z * math.Sqrt((p*(1-p)+z*z/(4*nf))/nf)
	return (centre - spread) / denominator
}
