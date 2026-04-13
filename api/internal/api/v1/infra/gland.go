package infra

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/yinhe/starclaw/internal/model"
	"gorm.io/gorm"
)

// GlandHandler manages agent gland (腺体) configuration CRUD.
type GlandHandler struct {
	db *gorm.DB
}

func NewGlandHandler(db *gorm.DB) *GlandHandler {
	return &GlandHandler{db: db}
}

// ── requests ────────────────────────────────────────────────

type CreateGlandRequest struct {
	AgentID   string `json:"agent_id" binding:"required"`
	Key       string `json:"key" binding:"required"`
	Value     string `json:"value"`
	Category  string `json:"category"`
	Encrypted bool   `json:"encrypted"`
	Required  bool   `json:"required"`
	Label     string `json:"label"`
	HelpText  string `json:"help_text"`
	SortOrder int    `json:"sort_order"`
}

type UpdateGlandRequest struct {
	Value     *string `json:"value"`
	Category  *string `json:"category"`
	Encrypted *bool   `json:"encrypted"`
	Required  *bool   `json:"required"`
	Label     *string `json:"label"`
	HelpText  *string `json:"help_text"`
	SortOrder *int    `json:"sort_order"`
}

type BatchUpsertGlandRequest struct {
	AgentID string               `json:"agent_id" binding:"required"`
	Glands  []CreateGlandRequest `json:"glands" binding:"required"`
}

// ── List glands for an agent ────────────────────────────────

func (h *GlandHandler) List(c *gin.Context) {
	userID := c.GetString("user_id")
	agentID := c.Query("agent_id")

	q := h.db.Where("user_id = ?", userID)
	if agentID != "" {
		q = q.Where("agent_id = ?", agentID)
	}

	var glands []model.AgentGland
	if err := q.Order("agent_id, sort_order, key").Find(&glands).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list glands"})
		return
	}

	// Mask encrypted values in response
	for i := range glands {
		if glands[i].Encrypted && glands[i].Value != "" {
			glands[i].Value = maskValue(glands[i].Value)
		}
	}

	c.JSON(http.StatusOK, gin.H{"glands": glands})
}

// ── Get single gland ────────────────────────────────────────

func (h *GlandHandler) Get(c *gin.Context) {
	userID := c.GetString("user_id")
	id := c.Param("id")

	var gland model.AgentGland
	if err := h.db.Where("id = ? AND user_id = ?", id, userID).First(&gland).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "gland not found"})
		return
	}

	if gland.Encrypted && gland.Value != "" {
		gland.Value = maskValue(gland.Value)
	}

	c.JSON(http.StatusOK, gland)
}

// ── Create gland ────────────────────────────────────────────

func (h *GlandHandler) Create(c *gin.Context) {
	userID := c.GetString("user_id")

	var req CreateGlandRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify agent belongs to user
	var agent model.Agent
	if err := h.db.Where("id = ? AND user_id = ?", req.AgentID, userID).First(&agent).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "agent not found or not owned"})
		return
	}

	// Check duplicate key
	var existing model.AgentGland
	if err := h.db.Where("agent_id = ? AND `key` = ?", req.AgentID, req.Key).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "gland key already exists for this agent"})
		return
	}

	cat := req.Category
	if cat == "" {
		cat = "general"
	}

	val := req.Value
	if req.Encrypted && val != "" {
		val = encrypt(val)
	}

	gland := model.AgentGland{
		ID:        uuid.New().String(),
		AgentID:   req.AgentID,
		UserID:    userID,
		Key:       req.Key,
		Value:     val,
		Category:  cat,
		Encrypted: req.Encrypted,
		Required:  req.Required,
		Label:     req.Label,
		HelpText:  req.HelpText,
		SortOrder: req.SortOrder,
	}

	if err := h.db.Create(&gland).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create gland"})
		return
	}

	if gland.Encrypted && gland.Value != "" {
		gland.Value = maskValue(req.Value)
	}

	c.JSON(http.StatusCreated, gland)
}

// ── Update gland ────────────────────────────────────────────

func (h *GlandHandler) Update(c *gin.Context) {
	userID := c.GetString("user_id")
	id := c.Param("id")

	var gland model.AgentGland
	if err := h.db.Where("id = ? AND user_id = ?", id, userID).First(&gland).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "gland not found"})
		return
	}

	var req UpdateGlandRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := map[string]interface{}{}
	if req.Value != nil {
		v := *req.Value
		enc := gland.Encrypted
		if req.Encrypted != nil {
			enc = *req.Encrypted
		}
		if enc && v != "" {
			v = encrypt(v)
		}
		updates["value"] = v
	}
	if req.Category != nil {
		updates["category"] = *req.Category
	}
	if req.Encrypted != nil {
		updates["encrypted"] = *req.Encrypted
	}
	if req.Required != nil {
		updates["required"] = *req.Required
	}
	if req.Label != nil {
		updates["label"] = *req.Label
	}
	if req.HelpText != nil {
		updates["help_text"] = *req.HelpText
	}
	if req.SortOrder != nil {
		updates["sort_order"] = *req.SortOrder
	}

	if len(updates) > 0 {
		h.db.Model(&gland).Updates(updates)
	}

	// Re-fetch
	h.db.First(&gland, "id = ?", id)
	if gland.Encrypted && gland.Value != "" {
		gland.Value = maskValue(gland.Value)
	}

	c.JSON(http.StatusOK, gland)
}

// ── Delete gland ────────────────────────────────────────────

func (h *GlandHandler) Delete(c *gin.Context) {
	userID := c.GetString("user_id")
	id := c.Param("id")

	result := h.db.Where("id = ? AND user_id = ?", id, userID).Delete(&model.AgentGland{})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "gland not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

// ── Batch upsert (for marketplace install / seed) ───────────

func (h *GlandHandler) BatchUpsert(c *gin.Context) {
	userID := c.GetString("user_id")

	var req BatchUpsertGlandRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify agent ownership
	var agent model.Agent
	if err := h.db.Where("id = ? AND user_id = ?", req.AgentID, userID).First(&agent).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "agent not found or not owned"})
		return
	}

	var results []model.AgentGland
	for _, g := range req.Glands {
		cat := g.Category
		if cat == "" {
			cat = "general"
		}
		val := g.Value
		if g.Encrypted && val != "" {
			val = encrypt(val)
		}

		var existing model.AgentGland
		if err := h.db.Where("agent_id = ? AND `key` = ?", req.AgentID, g.Key).First(&existing).Error; err == nil {
			// Update existing
			updates := map[string]interface{}{
				"value":      val,
				"category":   cat,
				"encrypted":  g.Encrypted,
				"required":   g.Required,
				"label":      g.Label,
				"help_text":  g.HelpText,
				"sort_order": g.SortOrder,
			}
			h.db.Model(&existing).Updates(updates)
			existing.Value = val
			existing.Category = cat
			results = append(results, existing)
		} else {
			// Create new
			gland := model.AgentGland{
				ID:        uuid.New().String(),
				AgentID:   req.AgentID,
				UserID:    userID,
				Key:       g.Key,
				Value:     val,
				Category:  cat,
				Encrypted: g.Encrypted,
				Required:  g.Required,
				Label:     g.Label,
				HelpText:  g.HelpText,
				SortOrder: g.SortOrder,
			}
			h.db.Create(&gland)
			results = append(results, gland)
		}
	}

	// Mask encrypted values
	for i := range results {
		if results[i].Encrypted && results[i].Value != "" {
			results[i].Value = "••••••••"
		}
	}

	c.JSON(http.StatusOK, gin.H{"glands": results, "count": len(results)})
}

// ── GetDecrypted returns the plaintext value (for internal tool use) ────

func (h *GlandHandler) GetDecrypted(c *gin.Context) {
	userID := c.GetString("user_id")
	agentID := c.Query("agent_id")
	key := c.Query("key")

	if agentID == "" || key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "agent_id and key are required"})
		return
	}

	var gland model.AgentGland
	if err := h.db.Where("agent_id = ? AND `key` = ? AND user_id = ?", agentID, key, userID).First(&gland).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "gland not found"})
		return
	}

	val := gland.Value
	if gland.Encrypted && val != "" {
		val = decrypt(val)
	}

	c.JSON(http.StatusOK, gin.H{"key": gland.Key, "value": val})
}

// ── encryption helpers ──────────────────────────────────────

var glandKey []byte

func glandEncryptionKey() []byte {
	if glandKey != nil {
		return glandKey
	}
	k := os.Getenv("GLAND_ENCRYPTION_KEY")
	if k == "" {
		// Fallback: derive from node secret (always present)
		k = os.Getenv("NODE_SECRET")
	}
	if k == "" {
		k = "starclaw-default-gland-key-0000" // 32 bytes
	}
	// Pad or truncate to 32 bytes for AES-256
	padded := make([]byte, 32)
	copy(padded, []byte(k))
	glandKey = padded
	return glandKey
}

func encrypt(plaintext string) string {
	block, err := aes.NewCipher(glandEncryptionKey())
	if err != nil {
		return plaintext
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return plaintext
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return plaintext
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return "enc:" + base64.StdEncoding.EncodeToString(ciphertext)
}

func decrypt(encoded string) string {
	if !strings.HasPrefix(encoded, "enc:") {
		return encoded // not encrypted
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(encoded, "enc:"))
	if err != nil {
		return ""
	}
	block, err := aes.NewCipher(glandEncryptionKey())
	if err != nil {
		return ""
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return ""
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return ""
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return ""
	}
	return string(plaintext)
}

func maskValue(val string) string {
	if len(val) <= 4 {
		return "••••"
	}
	return "••••••••"
}
