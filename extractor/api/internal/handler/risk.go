package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"starclaw.net/extractor/api/internal/engine"
	"starclaw.net/extractor/api/internal/model"
)

type RiskHandler struct {
	DB       *gorm.DB
	RiskCtrl *engine.RiskController
}

func (h *RiskHandler) ListRules(c *gin.Context) {
	var list []model.RiskRule
	h.DB.Order("level, type").Find(&list)
	c.JSON(http.StatusOK, list)
}

func (h *RiskHandler) CreateRule(c *gin.Context) {
	var r model.RiskRule
	if err := c.ShouldBindJSON(&r); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	r.ID = uuid.New().String()
	h.DB.Create(&r)
	c.JSON(http.StatusOK, r)
}

func (h *RiskHandler) UpdateRule(c *gin.Context) {
	var r model.RiskRule
	if err := h.DB.First(&r, "id = ?", c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	if err := c.ShouldBindJSON(&r); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.DB.Save(&r)
	c.JSON(http.StatusOK, r)
}

func (h *RiskHandler) DeleteRule(c *gin.Context) {
	h.DB.Delete(&model.RiskRule{}, "id = ?", c.Param("id"))
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *RiskHandler) ListAlerts(c *gin.Context) {
	var list []model.RiskAlert
	q := h.DB.Order("created_at desc").Limit(100)
	if severity := c.Query("severity"); severity != "" {
		q = q.Where("severity = ?", severity)
	}
	if c.Query("unresolved") == "true" {
		q = q.Where("resolved = false")
	}
	q.Find(&list)
	c.JSON(http.StatusOK, list)
}

func (h *RiskHandler) ResolveAlert(c *gin.Context) {
	now := time.Now()
	h.DB.Model(&model.RiskAlert{}).Where("id = ?", c.Param("id")).Updates(map[string]interface{}{
		"resolved":    true,
		"resolved_at": &now,
	})
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
