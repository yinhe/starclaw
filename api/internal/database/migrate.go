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
	); err != nil {
		return err
	}

	return nil
}
