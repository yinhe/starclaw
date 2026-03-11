package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

// AdminProxyHandler proxies admin requests to bounty/forum/arena services
type AdminProxyHandler struct {
	bountyURL string
	forumURL  string
	arenaURL  string
	httpC     *http.Client
}

func NewAdminProxyHandler() *AdminProxyHandler {
	return &AdminProxyHandler{
		bountyURL: envOr("BOUNTY_URL", "http://localhost:8092"),
		forumURL:  envOr("FORUM_URL", "http://localhost:8093"),
		arenaURL:  envOr("ARENA_URL", "http://localhost:8094"),
		httpC:     &http.Client{Timeout: 10 * time.Second},
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
