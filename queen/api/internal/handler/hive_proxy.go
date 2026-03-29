package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

// HiveProxyHandler proxies requests from Queen web to the Hive cloud fleet API.
type HiveProxyHandler struct {
	hiveURL string
	httpC   *http.Client
}

func NewHiveProxyHandler() *HiveProxyHandler {
	url := os.Getenv("HIVE_URL")
	if url == "" {
		url = "http://localhost:9090"
	}
	return &HiveProxyHandler{
		hiveURL: url,
		httpC:   &http.Client{Timeout: 30 * time.Second},
	}
}

// ListPlans returns available Hive plans (public).
// GET /v1/cloud/plans
func (h *HiveProxyHandler) ListPlans(c *gin.Context) {
	h.proxyGet(c, "/hive/plans")
}

// CreateInstance creates a new Claw instance via Hive.
// POST /v1/cloud/claws
func (h *HiveProxyHandler) CreateInstance(c *gin.Context) {
	h.proxyPost(c, "/hive/claws")
}

// ListMyInstances lists instances owned by the current user.
// GET /v1/cloud/claws
func (h *HiveProxyHandler) ListMyInstances(c *gin.Context) {
	clawID := c.GetString("claw_id")
	if clawID == "" {
		clawID = c.Query("claw_id")
	}
	h.proxyGet(c, "/hive/claws?owner_id="+clawID)
}

// GetInstance returns details of a specific instance.
// GET /v1/cloud/claws/:slug
func (h *HiveProxyHandler) GetInstance(c *gin.Context) {
	slug := c.Param("slug")
	h.proxyGet(c, "/hive/claws/"+slug)
}

// StopInstance stops a running instance.
// POST /v1/cloud/claws/:slug/stop
func (h *HiveProxyHandler) StopInstance(c *gin.Context) {
	slug := c.Param("slug")
	h.proxyPost(c, "/hive/claws/"+slug+"/stop")
}

// StartInstance starts a stopped instance.
// POST /v1/cloud/claws/:slug/start
func (h *HiveProxyHandler) StartInstance(c *gin.Context) {
	slug := c.Param("slug")
	h.proxyPost(c, "/hive/claws/"+slug+"/start")
}

// RestartInstance restarts an instance.
// POST /v1/cloud/claws/:slug/restart
func (h *HiveProxyHandler) RestartInstance(c *gin.Context) {
	slug := c.Param("slug")
	h.proxyPost(c, "/hive/claws/"+slug+"/restart")
}

// DestroyInstance destroys an instance.
// DELETE /v1/cloud/claws/:slug
func (h *HiveProxyHandler) DestroyInstance(c *gin.Context) {
	slug := c.Param("slug")
	req, err := http.NewRequest("DELETE", h.hiveURL+"/hive/claws/"+slug, nil)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to create request"})
		return
	}
	resp, err := h.httpC.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "hive service unreachable"})
		return
	}
	defer resp.Body.Close()
	var result interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	c.JSON(resp.StatusCode, result)
}

// CheckBalance checks credit balance for creating instances.
// GET /v1/cloud/balance
func (h *HiveProxyHandler) CheckBalance(c *gin.Context) {
	clawID := c.Query("claw_id")
	h.proxyGet(c, "/hive/balance?claw_id="+clawID)
}

func (h *HiveProxyHandler) proxyGet(c *gin.Context, path string) {
	resp, err := h.httpC.Get(h.hiveURL + path)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "hive service unreachable"})
		return
	}
	defer resp.Body.Close()
	var result interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	c.JSON(resp.StatusCode, result)
}

func (h *HiveProxyHandler) proxyPost(c *gin.Context, path string) {
	req, err := http.NewRequest("POST", h.hiveURL+path, c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to create request"})
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := h.httpC.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "hive service unreachable"})
		return
	}
	defer resp.Body.Close()
	var result interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	c.JSON(resp.StatusCode, result)
}
