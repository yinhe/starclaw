package database

import (
	agentpkg "github.com/yinhe/starclaw/internal/agent"
	"github.com/yinhe/starclaw/internal/model"
	"github.com/yinhe/starclaw/internal/observe"
	"github.com/yinhe/starclaw/internal/rag"
	"github.com/yinhe/starclaw/internal/security"
	"github.com/yinhe/starclaw/internal/webhook"
	"gorm.io/gorm"
)

func AutoMigrate(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&model.User{},
		&model.Agent{},
		&model.Conversation{},
		&model.Message{},
		&model.ModelConfig{},
		&model.Workflow{},
		&model.KnowledgeBase{},
		&model.Document{},
		&model.DocumentChunk{},
		&model.MCPServer{},
		&model.WorkflowRun{},
		&model.Schedule{},
		&model.AuditLog{},
		&model.WorkflowTemplate{},
		&model.Memory{},
		&model.AgentTestCase{},
		&model.AgentTestRun{},
		&model.VideoRecord{},
		&model.WorkspaceFile{},
		&model.Tenant{},
		&model.TenantMember{},
		&model.Plan{},
		&model.UsageRecord{},
		&model.Transaction{},
		&model.Invoice{},
		&model.AgentTemplate{},
		&model.AuthorizedDevice{},
		&model.Integration{},
		&model.WorkspaceFolder{},
		&model.AgentListing{},
		&model.AgentPurchase{},
		&model.CreatorRevenue{},
		&model.AgentRating{},
		&model.AgentVersion{},
		&model.CreatorProfile{},
		&observe.TraceSpan{},
		&observe.AlertRule{},
		&observe.AlertHistory{},
		&observe.LogEntry{},
		&rag.KGNode{},
		&rag.KGEdge{},
		&webhook.EventRule{},
		&webhook.EventLog{},
		&model.PluginListing{},
		&model.PluginInstall{},
		&model.PluginRating{},
		&model.PlaygroundRequest{},
		&security.AuditChainEntry{},
		&agentpkg.Goal{},
		&agentpkg.GoalStep{},
		&agentpkg.Collaboration{},
		&agentpkg.CollaborationMember{},
		&agentpkg.CollaborationMessage{},
		&agentpkg.LoRAAdapter{},
		&agentpkg.TrainingSample{},
		&agentpkg.DistillationJob{},
		&model.GenerationLog{},
		&model.ImageRecord{},
		&model.MusicRecord{},
		&model.ForgeProject{},
		&model.ForgeIssue{},
		&model.ForgeIssueComment{},
		&model.ForgeMilestone{},
		&model.ForgeBoard{},
		&model.AgentSkill{},
		&model.AgentMCPBinding{},
		&model.Team{},
		&model.TeamMember{},
		&model.ServiceToken{},
		&model.SwarmUnit{},
		&model.StardustTransaction{},
		&model.AgentGland{},
		&model.GCPatient{},
		&model.GCFollowupPlan{},
		&model.GCGrowthRecord{},
		&model.GCAlert{},
		&model.GCEducation{},
		&model.GCAuditLog{},
		// Phase 4D persistence — Abathur
		&model.EvolutionPlanRecord{},
		&model.SprintRecord{},
		&model.TaskRecord{},
		&model.HotfixRecord{},
		// Phase 4D persistence — SenseClaw
		&model.FeedbackRecord{},
		&model.AlertRecord{},
		&model.AnomalyRecord{},
		// Phase 4D persistence — TestClaw
		&model.TestSuiteRecord{},
		&model.TestCaseRecord{},
		&model.BenchmarkRecord{},
		// Phase 4D persistence — Cocoon
		&model.CocoonSpecRecord{},
		&model.BuildRecord{},
		&model.PublishRecord{},
		// Phase 4D persistence — Chitin
		&model.RuntimeInstance{},
		&model.RuntimeEvent{},
		// Phase 4D persistence — Lair
		&model.LairNodeRecord{},
		&model.DeploymentRecord{},
		&model.RolloutRecord{},
		// Phase 4D persistence — Partner
		&model.PartnerRecord{},
		&model.CommissionDBRecord{},
		&model.SettlementRecord{},
		// Phase 4D persistence — BroodNet
		&model.MarketTask{},
		&model.MarketBid{},
		&model.ReputationEntry{},
		&model.GossipPeer{},
		// Phase 5 persistence — Autonomy
		&model.AutonomyDecision{},
		&model.AutonomyRule{},
		&model.AutonomyInsight{},
		&model.AutonomySnapshot{},
		// Phase 5 persistence — Exchange
		&model.ExchangeOrder{},
		&model.ExchangeTrade{},
		&model.ExchangeService{},
		&model.ExchangeRequest{},
		&model.ExchangeBid{},
		&model.ExchangeRating{},
		// Phase 5 persistence — Federation
		&model.FederationSwarm{},
		&model.FederationHandshake{},
		&model.FederationTaskRoute{},
		&model.FederationTrustEvent{},
		// Phase 5 persistence — SwarmCtl
		&model.SwarmCtlUnit{},
		&model.SwarmCtlFormation{},
		&model.SwarmCtlMission{},
		&model.SwarmCtlMissionLog{},
	); err != nil {
		return err
	}

	return nil
}
