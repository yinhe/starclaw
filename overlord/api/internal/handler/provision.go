package handler

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw-overlord/api/internal/middleware"
	"github.com/yinhe/starclaw-overlord/api/internal/model"
	"gorm.io/gorm"
)

// ProvisionHandler handles one-click Claw node deployment via Hive
type ProvisionHandler struct {
	db      *gorm.DB
	hiveURL string
	httpC   *http.Client
}

func NewProvisionHandler(db *gorm.DB) *ProvisionHandler {
	hiveURL := os.Getenv("HIVE_URL")
	if hiveURL == "" {
		hiveURL = "https://starclaw.me"
	}
	return &ProvisionHandler{
		db:      db,
		hiveURL: hiveURL,
		httpC:   &http.Client{Timeout: 15 * time.Second},
	}
}

// POST /brood/provision-node
// Creates a free Claw node via Hive, waits for it to register with Overlord.
func (h *ProvisionHandler) ProvisionNode(c *gin.Context) {
	var req struct {
		Name string `json:"name"` // optional display name
	}
	c.ShouldBindJSON(&req)

	// Generate slug: team-{6 hex}
	slug := "team-" + randomHexShort(6)
	displayName := req.Name
	if displayName == "" {
		displayName = slug
	}

	actor := middleware.GetAdminActor(c)
	log.Printf("[provision] %s requesting new node: slug=%s name=%s", actor, slug, displayName)

	// Call Hive API to create a free instance
	hiveReq := map[string]string{
		"slug":         slug,
		"display_name": displayName,
		"plan_id":      "free",
	}
	body, _ := json.Marshal(hiveReq)

	resp, err := h.httpC.Post(h.hiveURL+"/hive/claws", "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("[provision] hive call failed: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "无法连接蜂巢部署服务: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	var hiveResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&hiveResp)

	if resp.StatusCode >= 400 {
		errMsg, _ := hiveResp["error"].(string)
		log.Printf("[provision] hive returned %d: %s", resp.StatusCode, errMsg)
		c.JSON(resp.StatusCode, gin.H{"error": "蜂巢部署失败: " + errMsg})
		return
	}

	address := fmt.Sprintf("https://%s.starclaw.me", slug)
	log.Printf("[provision] hive created %s, waiting for node registration...", address)

	// Poll for the node to register with Overlord (heartbeat)
	var node model.ClawNode
	deadline := time.Now().Add(90 * time.Second)
	found := false
	for time.Now().Before(deadline) {
		time.Sleep(3 * time.Second)
		if err := h.db.Where("address = ? AND status = ?", address, "online").First(&node).Error; err == nil {
			found = true
			break
		}
		// Also try matching by slug in name
		if err := h.db.Where("name LIKE ? AND status = ?", "%"+slug+"%", "online").First(&node).Error; err == nil {
			found = true
			break
		}
	}

	if found {
		log.Printf("[provision] node %s registered as %s", slug, node.ID)
		c.JSON(http.StatusOK, gin.H{
			"status":  "ready",
			"node_id": node.ID,
			"name":    node.Name,
			"address": node.Address,
			"slug":    slug,
		})
	} else {
		// Node created but not yet registered — return provisioning status
		log.Printf("[provision] node %s created but not yet registered (timeout)", slug)
		c.JSON(http.StatusAccepted, gin.H{
			"status":  "provisioning",
			"slug":    slug,
			"address": address,
			"message": "节点已创建，正在启动中，请稍后刷新",
		})
	}
}

// GET /brood/provision-status?slug=xxx
// Check if a provisioned node has come online.
func (h *ProvisionHandler) ProvisionStatus(c *gin.Context) {
	slug := c.Query("slug")
	if slug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing slug"})
		return
	}

	address := fmt.Sprintf("https://%s.starclaw.me", slug)
	var node model.ClawNode
	if err := h.db.Where("address = ? AND status = ?", address, "online").First(&node).Error; err == nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ready",
			"node_id": node.ID,
			"name":    node.Name,
			"address": node.Address,
			"slug":    slug,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "provisioning",
		"slug":    slug,
		"address": address,
	})
}

func randomHexShort(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)[:n]
}
