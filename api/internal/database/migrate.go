package database

import (
	"github.com/yinhe/starclaw/internal/model"
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
	); err != nil {
		return err
	}

	return nil
}
