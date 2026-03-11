package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

// DashboardHandler proxies to the swarm service for global stats and node management
type DashboardHandler struct {
	swarmURL string
	httpC    *http.Client
}

func NewDashboardHandler() *DashboardHandler {
	swarmURL := os.Getenv("SWARM_URL")
	if swarmURL == "" {
		swarmURL = "http://localhost:8090"
	}
	return &DashboardHandler{
		swarmURL: swarmURL,
		httpC:    &http.Client{Timeout: 10 * time.Second},
	}
}

// GlobalStats returns aggregated swarm statistics
func (h *DashboardHandler) GlobalStats(c *gin.Context) {
	data, err := h.proxyGet("/swarm/stats")
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to reach swarm service", "detail": err.Error()})
		return
	}
	c.JSON(http.StatusOK, data)
}

// ListNodes returns all registered nodes
func (h *DashboardHandler) ListNodes(c *gin.Context) {
	path := "/swarm/nodes"
	if role := c.Query("role"); role != "" {
		path += "?role=" + role
	}
	if status := c.Query("status"); status != "" {
		sep := "?"
		if c.Query("role") != "" {
			sep = "&"
		}
		path += sep + "status=" + status
	}

	data, err := h.proxyGet(path)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to reach swarm service"})
		return
	}
	c.JSON(http.StatusOK, data)
}

// GetNode returns a single node's details
func (h *DashboardHandler) GetNode(c *gin.Context) {
	id := c.Param("id")
	data, err := h.proxyGet(fmt.Sprintf("/swarm/nodes/%s", id))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to reach swarm service"})
		return
	}
	c.JSON(http.StatusOK, data)
}

// RemoveNode removes a node from the swarm
func (h *DashboardHandler) RemoveNode(c *gin.Context) {
	id := c.Param("id")
	req, _ := http.NewRequest("DELETE", h.swarmURL+fmt.Sprintf("/swarm/nodes/%s", id), nil)
	resp, err := h.httpC.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to reach swarm service"})
		return
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	c.JSON(resp.StatusCode, result)
}

// NotifyUpdate triggers an update notification to all nodes
func (h *DashboardHandler) NotifyUpdate(c *gin.Context) {
	body, _ := io.ReadAll(c.Request.Body)
	resp, err := h.httpC.Post(h.swarmURL+"/swarm/update/notify", "application/json", bytes.NewReader(body))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to reach swarm service"})
		return
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	c.JSON(resp.StatusCode, result)
}

// ============================================================
// Molt — Version Update Management (proxied to swarm)
// ============================================================

// POST /admin/molt/releases — create a release
func (h *DashboardHandler) CreateRelease(c *gin.Context) {
	h.proxyPost(c, "/swarm/molt/releases")
}

// GET /admin/molt/releases — list releases
func (h *DashboardHandler) ListReleases(c *gin.Context) {
	path := "/swarm/molt/releases?page=" + c.DefaultQuery("page", "1") + "&size=" + c.DefaultQuery("size", "20")
	data, err := h.proxyGet(path)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to reach swarm service"})
		return
	}
	c.JSON(http.StatusOK, data)
}

// GET /admin/molt/releases/:id — get release detail
func (h *DashboardHandler) GetRelease(c *gin.Context) {
	id := c.Param("id")
	data, err := h.proxyGet(fmt.Sprintf("/swarm/molt/releases/%s", id))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to reach swarm service"})
		return
	}
	c.JSON(http.StatusOK, data)
}

// POST /admin/molt/releases/:id/start — start rollout
func (h *DashboardHandler) StartRelease(c *gin.Context) {
	h.proxyPost(c, fmt.Sprintf("/swarm/molt/releases/%s/start", c.Param("id")))
}

// POST /admin/molt/releases/:id/pause — pause rollout
func (h *DashboardHandler) PauseRelease(c *gin.Context) {
	h.proxyPost(c, fmt.Sprintf("/swarm/molt/releases/%s/pause", c.Param("id")))
}

func (h *DashboardHandler) proxyPost(c *gin.Context, path string) {
	body, _ := io.ReadAll(c.Request.Body)
	resp, err := h.httpC.Post(h.swarmURL+path, "application/json", bytes.NewReader(body))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to reach swarm service"})
		return
	}
	defer resp.Body.Close()
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	c.JSON(resp.StatusCode, result)
}

func (h *DashboardHandler) proxyGet(path string) (map[string]interface{}, error) {
	resp, err := h.httpC.Get(h.swarmURL + path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}
