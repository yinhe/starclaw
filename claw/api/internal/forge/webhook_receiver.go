package forge

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw/internal/model"
	"gorm.io/gorm"
)

// HandleNydusWebhook receives webhook events from the Nydus server.
// POST /v1/forge/nydus/webhook
// Headers: X-Nydus-Event, X-Nydus-Signature, X-Nydus-Delivery
func HandleNydusWebhook(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		event := c.GetHeader("X-Nydus-Event")
		signature := c.GetHeader("X-Nydus-Signature")

		body, err := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20)) // 1MB max
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
			return
		}

		// Verify HMAC signature if webhook secret is configured
		webhookSecret := c.GetHeader("X-Webhook-Secret") // set by config
		if webhookSecret == "" {
			// Try env-based secret
			client := GetNydusClient()
			webhookSecret = client.Secret
		}
		if webhookSecret != "" && signature != "" {
			if !verifyHMAC(body, signature, webhookSecret) {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid signature"})
				return
			}
		}

		log.Printf("[forge-webhook] received event=%s delivery=%s", event, c.GetHeader("X-Nydus-Delivery"))

		switch event {
		case "push":
			handlePushEvent(db, body)
		case "pr_opened":
			handlePROpenedEvent(db, body)
		case "pr_merged":
			handlePRMergedEvent(db, body)
		case "pr_closed":
			handlePRClosedEvent(db, body)
		default:
			log.Printf("[forge-webhook] unhandled event: %s", event)
		}

		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}

func handlePushEvent(db *gorm.DB, body []byte) {
	// Push events can update issue status (branch pushed = in_progress)
	var payload struct {
		Repo   string `json:"repo"`
		Branch string `json:"branch"`
		Pusher string `json:"pusher"`
		Commit string `json:"commit"`
	}
	if err := parseJSON(body, &payload); err != nil {
		return
	}

	// Find issues linked to this branch and mark in_progress
	var issues []model.ForgeIssue
	db.Where("branch = ? AND status = ?", payload.Branch, "open").Find(&issues)
	for _, issue := range issues {
		db.Model(&issue).Update("status", "in_progress")
	}

	log.Printf("[forge-webhook] push on %s/%s by %s → %d issues updated",
		payload.Repo, payload.Branch, payload.Pusher, len(issues))
}

func handlePROpenedEvent(db *gorm.DB, body []byte) {
	var payload struct {
		Repo         string `json:"repo"`
		PRNumber     int    `json:"pr_number"`
		Title        string `json:"title"`
		SourceBranch string `json:"source_branch"`
		TargetBranch string `json:"target_branch"`
		Author       string `json:"author"`
	}
	if err := parseJSON(body, &payload); err != nil {
		return
	}

	// Link PR to issues on the same branch
	var issues []model.ForgeIssue
	db.Where("branch = ?", payload.SourceBranch).Find(&issues)
	for _, issue := range issues {
		db.Model(&issue).Updates(map[string]interface{}{
			"pr_number": payload.PRNumber,
			"status":    "review",
		})
	}

	log.Printf("[forge-webhook] PR #%d opened on %s (%s → %s) → %d issues linked",
		payload.PRNumber, payload.Repo, payload.SourceBranch, payload.TargetBranch, len(issues))
}

func handlePRMergedEvent(db *gorm.DB, body []byte) {
	var payload struct {
		Repo         string `json:"repo"`
		PRNumber     int    `json:"pr_number"`
		SourceBranch string `json:"source_branch"`
		MergeCommit  string `json:"merge_commit"`
		MergedBy     string `json:"merged_by"`
	}
	if err := parseJSON(body, &payload); err != nil {
		return
	}

	// Mark linked issues as done
	var issues []model.ForgeIssue
	db.Where("pr_number = ? OR branch = ?", payload.PRNumber, payload.SourceBranch).Find(&issues)
	for _, issue := range issues {
		db.Model(&issue).Update("status", "done")
	}

	log.Printf("[forge-webhook] PR #%d merged on %s → %d issues closed",
		payload.PRNumber, payload.Repo, len(issues))
}

func handlePRClosedEvent(db *gorm.DB, body []byte) {
	var payload struct {
		Repo     string `json:"repo"`
		PRNumber int    `json:"pr_number"`
	}
	if err := parseJSON(body, &payload); err != nil {
		return
	}

	// Revert linked issues to open (PR closed without merge)
	db.Model(&model.ForgeIssue{}).
		Where("pr_number = ? AND status = ?", payload.PRNumber, "review").
		Updates(map[string]interface{}{"status": "open", "pr_number": 0})

	log.Printf("[forge-webhook] PR #%d closed on %s → issues reverted to open", payload.PRNumber, payload.Repo)
}

func verifyHMAC(body []byte, signature, secret string) bool {
	// Signature format: sha256=<hex>
	parts := strings.SplitN(signature, "=", 2)
	if len(parts) != 2 {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(parts[1]), []byte(expected))
}

func parseJSON(body []byte, v interface{}) error {
	return json.Unmarshal(body, v)
}
