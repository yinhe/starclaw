package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"starclaw.net/extractor/api/internal/bridge"
	"starclaw.net/extractor/api/internal/engine"
	"starclaw.net/extractor/api/internal/model"
)

type StrategyHandler struct {
	DB        *gorm.DB
	Bridge    *bridge.Client
	Scheduler *engine.Scheduler
}

func (h *StrategyHandler) List(c *gin.Context) {
	var list []model.Strategy
	h.DB.Order("created_at desc").Find(&list)
	c.JSON(http.StatusOK, list)
}

func (h *StrategyHandler) Create(c *gin.Context) {
	var s model.Strategy
	if err := c.ShouldBindJSON(&s); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	s.ID = uuid.New().String()
	s.Status = "stopped"
	if err := h.DB.Create(&s).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, s)
}

func (h *StrategyHandler) Get(c *gin.Context) {
	var s model.Strategy
	if err := h.DB.First(&s, "id = ?", c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, s)
}

func (h *StrategyHandler) Update(c *gin.Context) {
	var s model.Strategy
	if err := h.DB.First(&s, "id = ?", c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	if err := c.ShouldBindJSON(&s); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.DB.Save(&s)
	c.JSON(http.StatusOK, s)
}

func (h *StrategyHandler) Delete(c *gin.Context) {
	h.DB.Delete(&model.Strategy{}, "id = ?", c.Param("id"))
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *StrategyHandler) Start(c *gin.Context) {
	var s model.Strategy
	if err := h.DB.First(&s, "id = ?", c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	if err := h.Scheduler.StartStrategy(&s); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "running"})
}

func (h *StrategyHandler) Stop(c *gin.Context) {
	if err := h.Scheduler.StopStrategy(c.Param("id")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "stopped"})
}
