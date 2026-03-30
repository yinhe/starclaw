package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"starclaw.net/extractor/api/internal/model"
)

type BacktestHandler struct {
	DB *gorm.DB
}

func (h *BacktestHandler) Submit(c *gin.Context) {
	var job model.BacktestJob
	if err := c.ShouldBindJSON(&job); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	job.ID = uuid.New().String()
	job.Status = "pending"
	h.DB.Create(&job)
	// TODO: dispatch to Python backtest engine
	c.JSON(http.StatusOK, job)
}

func (h *BacktestHandler) List(c *gin.Context) {
	var list []model.BacktestJob
	h.DB.Order("created_at desc").Limit(50).Find(&list)
	c.JSON(http.StatusOK, list)
}

func (h *BacktestHandler) Get(c *gin.Context) {
	var job model.BacktestJob
	if err := h.DB.First(&job, "id = ?", c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, job)
}
