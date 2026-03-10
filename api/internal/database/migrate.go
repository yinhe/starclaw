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

	// Generate API tokens for existing users who don't have one
	var users []model.User
	db.Where("api_token = '' OR api_token IS NULL").Find(&users)
	for _, u := range users {
		db.Model(&u).Update("api_token", model.GenerateAPIToken())
	}

	return nil
}
