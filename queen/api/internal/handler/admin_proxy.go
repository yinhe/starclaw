package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

// AdminProxyHandler proxies admin requests to bounty/forum/arena/chrysalis/hive services
type AdminProxyHandler struct {
	bountyURL    string
	forumURL     string
	arenaURL     string // forum-only (queen/arena)
	chrysalisURL string // PK battle system (chrysalis)
	hiveURL      string // cloud fleet (hive)
	hiveToken    string // hive admin token
	httpC        *http.Client
}

func NewAdminProxyHandler() *AdminProxyHandler {
	return &AdminProxyHandler{
		bountyURL:    envOr("BOUNTY_URL", "http://localhost:8092"),
		forumURL:     envOr("FORUM_URL", "http://localhost:8093"),
		arenaURL:     envOr("ARENA_URL", "http://localhost:8095"),
		chrysalisURL: envOr("CHRYSALIS_URL", "http://localhost:8094"),
		hiveURL:      envOr("HIVE_URL", "http://localhost:9090"),
		hiveToken:    envOr("HIVE_ADMIN_TOKEN", ""),
		httpC:        &http.Client{Timeout: 10 * time.Second},
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// GET /admin/bounty/stats
func (h *AdminProxyHandler) BountyStats(c *gin.Context) {
	h.proxy(c, h.bountyURL+"/bounty/stats")
}

// GET /admin/bounty/tasks — list all bounty tasks
func (h *AdminProxyHandler) BountyTasks(c *gin.Context) {
	path := "/bounty/tasks?page=" + c.DefaultQuery("page", "1") + "&size=" + c.DefaultQuery("size", "20")
	if s := c.Query("status"); s != "" {
		path += "&status=" + s
	}
	h.proxy(c, h.bountyURL+path)
}

// GET /admin/forum/stats
func (h *AdminProxyHandler) ForumStats(c *gin.Context) {
	h.proxy(c, h.forumURL+"/forum/stats")
}

// GET /admin/forum/posts — list forum posts
func (h *AdminProxyHandler) ForumPosts(c *gin.Context) {
	path := "/forum/posts?page=" + c.DefaultQuery("page", "1") + "&size=" + c.DefaultQuery("size", "20")
	if cat := c.Query("category"); cat != "" {
		path += "&category=" + cat
	}
	h.proxy(c, h.forumURL+path)
}

// DELETE /admin/forum/posts/:id — delete a forum post
func (h *AdminProxyHandler) ForumDeletePost(c *gin.Context) {
	id := c.Param("id")
	req, _ := http.NewRequest("DELETE", h.forumURL+"/forum/posts/"+id, nil)
	resp, err := h.httpC.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "forum service unreachable"})
		return
	}
	defer resp.Body.Close()
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	c.JSON(resp.StatusCode, result)
}

// GET /admin/arena/stats
func (h *AdminProxyHandler) ArenaStats(c *gin.Context) {
	h.proxy(c, h.arenaURL+"/arena/stats")
}

// GET /admin/arena/threads
func (h *AdminProxyHandler) ArenaThreads(c *gin.Context) {
	path := "/arena/threads"
	if t := c.Query("type"); t != "" {
		path += "?type=" + t
	}
	h.proxy(c, h.arenaURL+path)
}

// GET /admin/arena/leaderboard
func (h *AdminProxyHandler) ArenaLeaderboard(c *gin.Context) {
	h.proxy(c, h.arenaURL+"/arena/leaderboard")
}

// GET /admin/chrysalis/stats
func (h *AdminProxyHandler) ChrysalisStats(c *gin.Context) {
	h.proxy(c, h.chrysalisURL+"/chrysalis/stats")
}

// GET /admin/hive/stats
func (h *AdminProxyHandler) HiveStats(c *gin.Context) {
	h.proxyWithToken(c, h.hiveURL+"/hive/admin/stats")
}

// GET /admin/hive/instances
func (h *AdminProxyHandler) HiveInstances(c *gin.Context) {
	h.proxyWithToken(c, h.hiveURL+"/hive/claws")
}

// ProxyArenaPK is a generic proxy for /arena/pk/* endpoints (any method).
// Routes to the Chrysalis service (pet evolution & PK battle system).
func (h *AdminProxyHandler) ProxyArenaPK(c *gin.Context) {
	subPath := c.Param("path")
	targetURL := h.chrysalisURL + "/arena/pk" + subPath

	req, err := http.NewRequest(c.Request.Method, targetURL, c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to create request"})
		return
	}
	req.Header.Set("Content-Type", c.GetHeader("Content-Type"))

	resp, err := h.httpC.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "chrysalis service unreachable"})
		return
	}
	defer resp.Body.Close()

	var result interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	c.JSON(resp.StatusCode, result)
}

// ChrysalisURL returns the chrysalis service base URL.
func (h *AdminProxyHandler) ChrysalisURL() string {
	return h.chrysalisURL
}

// ArenaURL returns the arena (forum) service base URL.
func (h *AdminProxyHandler) ArenaURL() string {
	return h.arenaURL
}

func (h *AdminProxyHandler) proxyWithToken(c *gin.Context, url string) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to create request"})
		return
	}
	if h.hiveToken != "" {
		req.Header.Set("X-Hive-Token", h.hiveToken)
	}
	resp, err := h.httpC.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "service unreachable"})
		return
	}
	defer resp.Body.Close()
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	c.JSON(resp.StatusCode, result)
}

func (h *AdminProxyHandler) proxy(c *gin.Context, url string) {
	resp, err := h.httpC.Get(url)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "service unreachable"})
		return
	}
	defer resp.Body.Close()
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	c.JSON(resp.StatusCode, result)
}
