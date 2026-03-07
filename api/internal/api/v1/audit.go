package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw/internal/model"
	"gorm.io/gorm"
)

type AuditHandler struct {
	db *gorm.DB
}

func NewAuditHandler(db *gorm.DB) *AuditHandler {
	return &AuditHandler{db: db}
}

// List returns recent audit logs for the current user
func (h *AuditHandler) List(c *gin.Context) {
	userID := c.GetString("user_id")
	var logs []model.AuditLog
	h.db.Where("user_id = ?", userID).Order("created_at DESC").Limit(100).Find(&logs)
	c.JSON(http.StatusOK, gin.H{"logs": logs})
}

// LogAction is a helper to record an audit event (called internally)
func LogAction(db *gorm.DB, userID, action, resource, resourceID, detail, ip string) {
	log := model.AuditLog{
		UserID:     userID,
		Action:     action,
		Resource:   resource,
		ResourceID: resourceID,
		Detail:     detail,
		IP:         ip,
	}
	db.Create(&log)
}
