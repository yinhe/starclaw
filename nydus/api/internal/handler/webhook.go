package handler

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw/nydus/internal/database"
	"github.com/yinhe/starclaw/nydus/internal/model"
)

// ════════════════════════════════════════════════════════════
// Webhook CRUD
// ════════════════════════════════════════════════════════════

// CreateWebhook registers a new webhook for a repository.
// POST /api/repos/:name/webhooks
func CreateWebhook(c *gin.Context) {
	repoName := c.Param("name")
	var repo model.NydusRepo
	if err := database.DB.Where("name = ? AND status = ?", repoName, "active").First(&repo).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "repo not found"})
		return
	}

	var req struct {
		URL    string `json:"url" binding:"required"`
		Secret string `json:"secret"`
		Events string `json:"events"` // comma-separated, default "push"
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Events == "" {
		req.Events = "push"
	}

	createdBy := ""
	if v, exists := c.Get("node_id"); exists && v != nil {
		createdBy = v.(string)
	}

	wh := model.Webhook{
		RepoID:    repo.ID,
		URL:       req.URL,
		Secret:    req.Secret,
		Events:    req.Events,
		Active:    true,
		CreatedBy: createdBy,
	}
	if err := database.DB.Create(&wh).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create webhook"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"webhook": wh})
}

// ListWebhooks returns all webhooks for a repository.
// GET /api/repos/:name/webhooks
func ListWebhooks(c *gin.Context) {
	repoName := c.Param("name")
	var repo model.NydusRepo
	if err := database.DB.Where("name = ? AND status = ?", repoName, "active").First(&repo).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "repo not found"})
		return
	}

	var hooks []model.Webhook
	database.DB.Where("repo_id = ?", repo.ID).Order("created_at DESC").Find(&hooks)
	c.JSON(http.StatusOK, gin.H{"webhooks": hooks, "total": len(hooks)})
}

// DeleteWebhook removes a webhook.
// DELETE /api/repos/:name/webhooks/:id
func DeleteWebhook(c *gin.Context) {
	whID := c.Param("id")
	result := database.DB.Where("id = ?", whID).Delete(&model.Webhook{})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "webhook not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "webhook deleted"})
}

// ListDeliveries returns recent delivery attempts for a webhook.
// GET /api/repos/:name/webhooks/:id/deliveries
func ListDeliveries(c *gin.Context) {
	whID := c.Param("id")
	var deliveries []model.WebhookDelivery
	database.DB.Where("webhook_id = ?", whID).Order("created_at DESC").Limit(50).Find(&deliveries)
	c.JSON(http.StatusOK, gin.H{"deliveries": deliveries, "total": len(deliveries)})
}

// ════════════════════════════════════════════════════════════
// Event Dispatch (called internally by other handlers)
// ════════════════════════════════════════════════════════════

// FireEvent dispatches a webhook event to all matching hooks for a repo.
// Called asynchronously — does not block the caller.
func FireEvent(repoID, event string, payload gin.H) {
	go func() {
		var hooks []model.Webhook
		database.DB.Where("repo_id = ? AND active = ?", repoID, true).Find(&hooks)

		for _, hook := range hooks {
			if !eventMatches(hook.Events, event) {
				continue
			}
			deliverWebhook(hook, event, payload)
		}
	}()
}

func eventMatches(hookEvents, event string) bool {
	for _, e := range strings.Split(hookEvents, ",") {
		if strings.TrimSpace(e) == event || strings.TrimSpace(e) == "*" {
			return true
		}
	}
	return false
}

func deliverWebhook(hook model.Webhook, event string, payload gin.H) {
	body, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", hook.URL, bytes.NewReader(body))
	if err != nil {
		log.Printf("[webhook] failed to create request for %s: %v", hook.URL, err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Nydus-Event", event)
	req.Header.Set("X-Nydus-Delivery", fmt.Sprintf("%d", time.Now().UnixNano()))

	// HMAC signature
	if hook.Secret != "" {
		mac := hmac.New(sha256.New, []byte(hook.Secret))
		mac.Write(body)
		sig := hex.EncodeToString(mac.Sum(nil))
		req.Header.Set("X-Nydus-Signature", "sha256="+sig)
	}

	start := time.Now()
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)

	delivery := model.WebhookDelivery{
		WebhookID: hook.ID,
		Event:     event,
		Payload:   string(body),
	}

	if err != nil {
		delivery.StatusCode = 0
		delivery.Response = err.Error()
		delivery.Success = false
	} else {
		delivery.StatusCode = resp.StatusCode
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		resp.Body.Close()
		delivery.Response = string(respBody)
		delivery.Success = resp.StatusCode >= 200 && resp.StatusCode < 300
		delivery.Duration = time.Since(start).Milliseconds()
	}

	database.DB.Create(&delivery)

	if delivery.Success {
		log.Printf("[webhook] %s → %s: %d (%dms)", event, hook.URL, delivery.StatusCode, delivery.Duration)
	} else {
		log.Printf("[webhook] %s → %s: FAILED %d (%s)", event, hook.URL, delivery.StatusCode, delivery.Response)
	}
}
