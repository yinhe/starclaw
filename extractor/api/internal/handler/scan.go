package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"starclaw.net/extractor/api/internal/bridge"
)

// ScanHandler triggers the Python strategy executor.
type ScanHandler struct {
	DB     *gorm.DB
	Bridge *bridge.Client
}

// TriggerScan triggers a one-shot scan cycle on the Python executor.
// POST /v1/scan
func (h *ScanHandler) TriggerScan(c *gin.Context) {
	var req struct {
		MinScore float64 `json:"min_score"`
		TopN     int     `json:"top_n"`
	}
	_ = c.ShouldBindJSON(&req)

	// Call Python bridge /scan endpoint
	result, err := h.Bridge.TriggerScan(req.MinScore, req.TopN)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// ScanStatus returns the last scan result.
// GET /v1/scan/status
func (h *ScanHandler) ScanStatus(c *gin.Context) {
	result, err := h.Bridge.GetScanStatus()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}
