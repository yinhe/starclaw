package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"starclaw.net/extractor/api/internal/engine"
	"starclaw.net/extractor/api/internal/model"
)

type SettlementHandler struct {
	DB *gorm.DB
}

func (h *SettlementHandler) RunDaily(c *gin.Context) {
	date := c.DefaultQuery("date", time.Now().Format("2006-01-02"))
	s, err := engine.RunDailySettlement(h.DB, date)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, s)
}

func (h *SettlementHandler) History(c *gin.Context) {
	var list []model.Settlement
	h.DB.Order("date desc").Limit(60).Find(&list)
	c.JSON(http.StatusOK, list)
}
