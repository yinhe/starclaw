package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw/internal/model"
	"gorm.io/gorm"
)

type DeviceHandler struct {
	db *gorm.DB
}

func NewDeviceHandler(db *gorm.DB) *DeviceHandler {
	return &DeviceHandler{db: db}
}

// ListDevices returns all authorized devices for the current user.
func (h *DeviceHandler) ListDevices(c *gin.Context) {
	userID, _ := c.Get("user_id")
	var devices []model.AuthorizedDevice
	h.db.Where("user_id = ?", userID).Order("created_at DESC").Find(&devices)
	c.JSON(http.StatusOK, gin.H{"devices": devices})
}

// ApproveDevice approves a pending device.
func (h *DeviceHandler) ApproveDevice(c *gin.Context) {
	userID, _ := c.Get("user_id")
	deviceID := c.Param("id")

	var device model.AuthorizedDevice
	if err := h.db.Where("id = ? AND user_id = ?", deviceID, userID).First(&device).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "设备不存在"})
		return
	}
	if device.Revoked {
		c.JSON(http.StatusBadRequest, gin.H{"error": "设备已被撤销，无法审批"})
		return
	}
	if device.Approved {
		c.JSON(http.StatusOK, gin.H{"message": "设备已经审批通过"})
		return
	}

	h.db.Model(&device).Update("approved", true)
	c.JSON(http.StatusOK, gin.H{"message": "设备审批通过", "device": device})
}

// RejectDevice rejects a pending device (marks as revoked).
func (h *DeviceHandler) RejectDevice(c *gin.Context) {
	userID, _ := c.Get("user_id")
	deviceID := c.Param("id")

	var device model.AuthorizedDevice
	if err := h.db.Where("id = ? AND user_id = ?", deviceID, userID).First(&device).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "设备不存在"})
		return
	}

	h.db.Model(&device).Updates(map[string]interface{}{
		"revoked":  true,
		"approved": false,
	})
	c.JSON(http.StatusOK, gin.H{"message": "设备已拒绝"})
}

// RevokeDevice revokes an approved device.
func (h *DeviceHandler) RevokeDevice(c *gin.Context) {
	userID, _ := c.Get("user_id")
	deviceID := c.Param("id")

	var device model.AuthorizedDevice
	if err := h.db.Where("id = ? AND user_id = ?", deviceID, userID).First(&device).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "设备不存在"})
		return
	}

	h.db.Model(&device).Updates(map[string]interface{}{
		"revoked":  true,
		"approved": false,
	})
	c.JSON(http.StatusOK, gin.H{"message": "设备已撤销"})
}
