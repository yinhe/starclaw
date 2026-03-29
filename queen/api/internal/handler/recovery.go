package handler

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"starclaw.net/queen/api/internal/database"
	"starclaw.net/queen/api/internal/model"
)

// RecoveryHandler handles phone binding, SMS verification, and cloud backup storage.
type RecoveryHandler struct{}

func NewRecoveryHandler() *RecoveryHandler {
	return &RecoveryHandler{}
}

// POST /api/recovery/bind-phone — initiate phone binding (sends SMS)
func (h *RecoveryHandler) BindPhone(c *gin.Context) {
	var req struct {
		NodeID string `json:"node_id"`
		Phone  string `json:"phone"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.NodeID == "" || req.Phone == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "node_id and phone required"})
		return
	}

	// Generate 6-digit code
	code := generateSMSCode()

	// Store verification record
	verification := model.SMSVerification{
		ID:        uuid.New().String(),
		Phone:     req.Phone,
		Code:      code,
		Purpose:   "bind_phone",
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}
	database.DB.Create(&verification)

	// TODO: Send SMS via Aliyun SMS / Tencent Cloud SMS
	// For now, log the code (development mode)
	log.Printf("[recovery] SMS code for %s: %s (node: %s)", req.Phone, code, req.NodeID)

	c.JSON(http.StatusOK, gin.H{
		"message":    "验证码已发送",
		"expires_in": 300,
	})
}

// POST /api/recovery/verify-phone — verify SMS code and complete binding
func (h *RecoveryHandler) VerifyPhone(c *gin.Context) {
	var req struct {
		NodeID string `json:"node_id"`
		Phone  string `json:"phone"`
		Code   string `json:"code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.NodeID == "" || req.Phone == "" || req.Code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "node_id, phone, and code required"})
		return
	}

	// Find valid verification
	var verification model.SMSVerification
	err := database.DB.Where("phone = ? AND code = ? AND purpose = ? AND used = ? AND expires_at > ?",
		req.Phone, req.Code, "bind_phone", false, time.Now()).
		Order("created_at DESC").First(&verification).Error
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "验证码无效或已过期"})
		return
	}

	// Mark code as used
	database.DB.Model(&verification).Update("used", true)

	// Create or update phone binding
	var binding model.PhoneBinding
	result := database.DB.Where("node_id = ?", req.NodeID).First(&binding)
	if result.Error != nil {
		binding = model.PhoneBinding{
			ID:       uuid.New().String(),
			NodeID:   req.NodeID,
			Phone:    req.Phone,
			Verified: true,
		}
		database.DB.Create(&binding)
	} else {
		database.DB.Model(&binding).Updates(map[string]interface{}{
			"phone":    req.Phone,
			"verified": true,
		})
	}

	c.JSON(http.StatusOK, gin.H{"message": "手机绑定成功"})
}

// POST /api/recovery/backup — store encrypted backup blob
func (h *RecoveryHandler) StoreBackup(c *gin.Context) {
	var req struct {
		LookupKey string `json:"lookup_key"`
		NodeID    string `json:"node_id"`
		Data      string `json:"data"`      // base64-encoded encrypted blob
		DataSize  int64  `json:"data_size"`
		Version   int    `json:"version"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.LookupKey == "" || req.Data == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "lookup_key and data required"})
		return
	}

	encrypted, err := base64.StdEncoding.DecodeString(req.Data)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid base64 data"})
		return
	}

	// Upsert backup (one per lookup_key)
	var backup model.CloudBackup
	result := database.DB.Where("lookup_key = ?", req.LookupKey).First(&backup)
	if result.Error != nil {
		backup = model.CloudBackup{
			ID:        uuid.New().String(),
			LookupKey: req.LookupKey,
			NodeID:    req.NodeID,
			Data:      encrypted,
			DataSize:  int64(len(encrypted)),
			Version:   req.Version,
		}
		database.DB.Create(&backup)
	} else {
		database.DB.Model(&backup).Updates(map[string]interface{}{
			"data":      encrypted,
			"data_size": int64(len(encrypted)),
			"version":   req.Version,
		})
	}

	c.JSON(http.StatusOK, gin.H{"message": "备份存储成功", "size": len(encrypted)})
}

// GET /api/recovery/backup?lookup_key=xxx — retrieve encrypted backup
func (h *RecoveryHandler) GetBackup(c *gin.Context) {
	lookupKey := c.Query("lookup_key")
	if lookupKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "lookup_key required"})
		return
	}

	var backup model.CloudBackup
	if err := database.DB.Where("lookup_key = ?", lookupKey).First(&backup).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "备份不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":       base64.StdEncoding.EncodeToString(backup.Data),
		"node_id":    backup.NodeID,
		"version":    backup.Version,
		"created_at": backup.CreatedAt,
		"updated_at": backup.UpdatedAt,
	})
}

// GET /api/recovery/backup/exists?lookup_key=xxx — check if backup exists (no data returned)
func (h *RecoveryHandler) BackupExists(c *gin.Context) {
	lookupKey := c.Query("lookup_key")
	if lookupKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "lookup_key required"})
		return
	}

	var count int64
	database.DB.Model(&model.CloudBackup{}).Where("lookup_key = ?", lookupKey).Count(&count)
	if count > 0 {
		c.JSON(http.StatusOK, gin.H{"exists": true})
	} else {
		c.JSON(http.StatusNotFound, gin.H{"exists": false})
	}
}

// GET /api/recovery/status/:node_id — check recovery setup for a node
func (h *RecoveryHandler) NodeStatus(c *gin.Context) {
	nodeID := c.Param("node_id")
	if nodeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "node_id required"})
		return
	}

	var binding model.PhoneBinding
	phoneBound := database.DB.Where("node_id = ? AND verified = ?", nodeID, true).First(&binding).Error == nil

	phone := ""
	if phoneBound {
		phone = maskPhone(binding.Phone)
	}

	c.JSON(http.StatusOK, gin.H{
		"phone_bound": phoneBound,
		"phone":       phone,
	})
}

func generateSMSCode() string {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "123456" // fallback
	}
	return fmt.Sprintf("%06d", n.Int64())
}

func maskPhone(phone string) string {
	if len(phone) < 7 {
		return phone
	}
	return phone[:3] + "****" + phone[len(phone)-4:]
}
