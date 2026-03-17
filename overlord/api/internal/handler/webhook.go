package handler

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw-overlord/api/internal/middleware"
	"github.com/yinhe/starclaw-overlord/api/internal/model"
	"gorm.io/gorm"
)

type WebhookHandler struct {
	db *gorm.DB
}

func NewWebhookHandler(db *gorm.DB) *WebhookHandler {
	return &WebhookHandler{db: db}
}

// ---------- POST /brood/webhooks ----------

func (h *WebhookHandler) CreateWebhook(c *gin.Context) {
	var req struct {
		Name   string `json:"name" binding:"required"`
		URL    string `json:"url" binding:"required"`
		Events string `json:"events"`
		TeamID string `json:"team_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	events := req.Events
	if events == "" {
		events = "*"
	}

	secret := generateToken(16)
	wh := model.Webhook{
		Name:   req.Name,
		URL:    req.URL,
		Secret: secret,
		TeamID: req.TeamID,
		Events: events,
		Status: "active",
	}

	if err := h.db.Create(&wh).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create webhook"})
		return
	}

	audit(h.db, c, "create_webhook", wh.ID, "webhook created: "+req.Name)
	c.JSON(http.StatusCreated, gin.H{"webhook": wh, "secret": secret})
}

// ---------- GET /brood/webhooks ----------

func (h *WebhookHandler) ListWebhooks(c *gin.Context) {
	q := middleware.TeamScope(c, h.db)
	var webhooks []model.Webhook
	q.Order("created_at DESC").Find(&webhooks)
	c.JSON(http.StatusOK, gin.H{"webhooks": webhooks, "total": len(webhooks)})
}

// ---------- GET /brood/webhooks/:id ----------

func (h *WebhookHandler) GetWebhook(c *gin.Context) {
	id := c.Param("id")
	var wh model.Webhook
	if err := h.db.First(&wh, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "webhook not found"})
		return
	}

	var logs []model.WebhookLog
	h.db.Where("webhook_id = ?", id).Order("created_at DESC").Limit(20).Find(&logs)

	c.JSON(http.StatusOK, gin.H{"webhook": wh, "recent_logs": logs})
}

// ---------- PUT /brood/webhooks/:id ----------

func (h *WebhookHandler) UpdateWebhook(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Name   *string `json:"name"`
		URL    *string `json:"url"`
		Events *string `json:"events"`
		Status *string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := map[string]interface{}{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.URL != nil {
		updates["url"] = *req.URL
	}
	if req.Events != nil {
		updates["events"] = *req.Events
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}

	if err := h.db.Model(&model.Webhook{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update webhook"})
		return
	}

	audit(h.db, c, "update_webhook", id, "webhook updated")
	c.JSON(http.StatusOK, gin.H{"message": "webhook updated"})
}

// ---------- DELETE /brood/webhooks/:id ----------

func (h *WebhookHandler) DeleteWebhook(c *gin.Context) {
	id := c.Param("id")
	if err := h.db.Where("id = ?", id).Delete(&model.Webhook{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete webhook"})
		return
	}
	audit(h.db, c, "delete_webhook", id, "webhook deleted")
	c.JSON(http.StatusOK, gin.H{"message": "webhook deleted"})
}

// ---------- POST /brood/webhooks/:id/test ----------

func (h *WebhookHandler) TestWebhook(c *gin.Context) {
	id := c.Param("id")
	var wh model.Webhook
	if err := h.db.First(&wh, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "webhook not found"})
		return
	}

	payload := map[string]interface{}{
		"event":     "test",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"message":   "Overlord webhook test delivery",
	}

	code, err := h.deliver(&wh, "test", payload)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error(), "status_code": code})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "status_code": code})
}

// ---------- Delivery engine ----------

// Dispatch sends an event to all matching active webhooks (call from goroutine)
func (h *WebhookHandler) Dispatch(event string, payload map[string]interface{}) {
	var webhooks []model.Webhook
	h.db.Where("status = ?", "active").Find(&webhooks)

	for i := range webhooks {
		wh := webhooks[i]
		if !h.matchesEvent(&wh, event) {
			continue
		}
		go func(w model.Webhook) {
			h.deliver(&w, event, payload)
		}(wh)
	}
}

func (h *WebhookHandler) matchesEvent(wh *model.Webhook, event string) bool {
	if wh.Events == "*" || wh.Events == "" {
		return true
	}
	parts := strings.Split(wh.Events, ",")
	for _, p := range parts {
		if strings.TrimSpace(p) == event {
			return true
		}
	}
	return false
}

func (h *WebhookHandler) deliver(wh *model.Webhook, event string, payload map[string]interface{}) (int, error) {
	payload["event"] = event

	body, _ := json.Marshal(payload)
	start := time.Now()

	req, err := http.NewRequest("POST", wh.URL, bytes.NewReader(body))
	if err != nil {
		h.logDelivery(wh.ID, event, string(body), 0, err.Error(), 0)
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Overlord-Event", event)

	// HMAC signature
	if wh.Secret != "" {
		mac := hmac.New(sha256.New, []byte(wh.Secret))
		mac.Write(body)
		sig := hex.EncodeToString(mac.Sum(nil))
		req.Header.Set("X-Overlord-Signature", "sha256="+sig)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	durationMs := int(time.Since(start).Milliseconds())

	if err != nil {
		h.logDelivery(wh.ID, event, string(body), 0, err.Error(), durationMs)
		h.updateStats(wh.ID, false, err.Error())
		return 0, err
	}
	defer resp.Body.Close()

	errMsg := ""
	if resp.StatusCode >= 400 {
		errMsg = resp.Status
	}
	h.logDelivery(wh.ID, event, string(body), resp.StatusCode, errMsg, durationMs)
	h.updateStats(wh.ID, resp.StatusCode < 400, errMsg)

	if resp.StatusCode >= 400 {
		return resp.StatusCode, &webhookError{msg: resp.Status}
	}
	return resp.StatusCode, nil
}

func (h *WebhookHandler) logDelivery(webhookID, event, payload string, statusCode int, errMsg string, durationMs int) {
	entry := model.WebhookLog{
		WebhookID:  webhookID,
		Event:      event,
		Payload:    payload,
		StatusCode: statusCode,
		Error:      errMsg,
		DurationMs: durationMs,
	}
	if err := h.db.Create(&entry).Error; err != nil {
		log.Printf("[webhook] failed to log delivery: %v", err)
	}
}

func (h *WebhookHandler) updateStats(webhookID string, success bool, lastErr string) {
	now := time.Now()
	if success {
		h.db.Model(&model.Webhook{}).Where("id = ?", webhookID).Updates(map[string]interface{}{
			"total_sent":  gorm.Expr("total_sent + 1"),
			"last_sent_at": &now,
		})
	} else {
		h.db.Model(&model.Webhook{}).Where("id = ?", webhookID).Updates(map[string]interface{}{
			"total_failed": gorm.Expr("total_failed + 1"),
			"last_error":   lastErr,
		})
	}
}

type webhookError struct {
	msg string
}

func (e *webhookError) Error() string {
	return e.msg
}
