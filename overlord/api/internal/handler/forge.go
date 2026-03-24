package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

// ForgeHandler proxies Nydus API data and Forge API data for the Overlord console.
type ForgeHandler struct {
	nydusURL    string
	nydusSecret string
	forgeURL    string // standalone forge-api URL (e.g. http://forge-api:8099)
	client      *http.Client
}

func NewForgeHandler() *ForgeHandler {
	nurl := os.Getenv("NYDUS_URL")
	if nurl == "" {
		nurl = "https://nydus.starclaw.net"
	}
	return &ForgeHandler{
		nydusURL:    nurl,
		nydusSecret: os.Getenv("NYDUS_SECRET"),
		forgeURL:    os.Getenv("FORGE_URL"), // empty = forge features disabled
		client:      &http.Client{Timeout: 10 * time.Second},
	}
}

// ListRepos proxies GET /api/repos from Nydus.
func (h *ForgeHandler) ListRepos(c *gin.Context) {
	h.proxy(c, "/api/repos")
}

// ListNodes proxies GET /api/nodes from Nydus.
func (h *ForgeHandler) ListNodes(c *gin.Context) {
	h.proxy(c, "/api/nodes")
}

// ListPRs proxies GET /api/repos/:name/pulls from Nydus.
func (h *ForgeHandler) ListPRs(c *gin.Context) {
	name := c.Param("name")
	status := c.DefaultQuery("status", "all")
	h.proxy(c, "/api/repos/"+name+"/pulls?status="+status)
}

// ListForks proxies GET /api/repos/:name/forks from Nydus.
func (h *ForgeHandler) ListForks(c *gin.Context) {
	name := c.Param("name")
	h.proxy(c, "/api/repos/"+name+"/forks")
}

// ListWebhooks proxies GET /api/repos/:name/webhooks from Nydus.
func (h *ForgeHandler) ListWebhooks(c *gin.Context) {
	name := c.Param("name")
	h.proxy(c, "/api/repos/"+name+"/webhooks")
}

// ListBranchProtections proxies GET /api/repos/:name/branches/protect from Nydus.
func (h *ForgeHandler) ListBranchProtections(c *gin.Context) {
	name := c.Param("name")
	h.proxy(c, "/api/repos/"+name+"/branches/protect")
}

// ForgeSummary returns an aggregated summary of Nydus Forge state.
func (h *ForgeHandler) ForgeSummary(c *gin.Context) {
	type repoEntry struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Owner       string `json:"owner"`
		Public      bool   `json:"public"`
		Initialized bool   `json:"initialized"`
		Branches    int    `json:"branches"`
		Tags        int    `json:"tags"`
		CommitCount int    `json:"commit_count"`
	}

	// Fetch repos
	repos, err := h.fetchJSON("/api/repos")
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to reach Nydus: " + err.Error()})
		return
	}

	// Fetch nodes
	nodes, _ := h.fetchJSON("/api/nodes")

	c.JSON(http.StatusOK, gin.H{
		"repos": repos,
		"nodes": nodes,
	})
}

// ForgeAPIProxy proxies any request to the standalone forge-api service.
// Used for /brood/forge/* routes in Overlord console.
func (h *ForgeHandler) ForgeAPIProxy(c *gin.Context) {
	if h.forgeURL == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "forge-api not configured (set FORGE_URL)"})
		return
	}
	subPath := c.Param("path")
	h.proxyTo(c, h.forgeURL, "/api"+subPath)
}

// Heatmap proxies commit heatmap from Nydus.
func (h *ForgeHandler) Heatmap(c *gin.Context) {
	repo := c.DefaultQuery("repo", "starclaw")
	days := c.DefaultQuery("days", "30")
	h.proxy(c, "/v1/commits/heatmap?repo="+repo+"&days="+days)
}

// Commits proxies recent commits from Nydus.
func (h *ForgeHandler) Commits(c *gin.Context) {
	repo := c.DefaultQuery("repo", "starclaw")
	limit := c.DefaultQuery("limit", "20")
	h.proxy(c, "/v1/commits?repo="+repo+"&limit="+limit)
}

// Deploys proxies recent deploys from Nydus.
func (h *ForgeHandler) Deploys(c *gin.Context) {
	h.proxy(c, "/v1/deploys?limit=20")
}

func (h *ForgeHandler) proxy(c *gin.Context, path string) {
	req, err := http.NewRequest("GET", h.nydusURL+path, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if h.nydusSecret != "" {
		req.Header.Set("X-Secret", h.nydusSecret)
	}
	resp, err := h.client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "nydus unreachable: " + err.Error()})
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	c.Data(resp.StatusCode, "application/json", body)
}

func (h *ForgeHandler) proxyTo(c *gin.Context, baseURL, path string) {
	req, err := http.NewRequest(c.Request.Method, baseURL+path, c.Request.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	req.Header.Set("Content-Type", c.GetHeader("Content-Type"))
	resp, err := h.client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "forge-api unreachable: " + err.Error()})
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	c.Data(resp.StatusCode, "application/json", body)
}

func (h *ForgeHandler) fetchJSON(path string) (interface{}, error) {
	req, err := http.NewRequest("GET", h.nydusURL+path, nil)
	if err != nil {
		return nil, err
	}
	if h.nydusSecret != "" {
		req.Header.Set("X-Secret", h.nydusSecret)
	}
	resp, err := h.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	return result, nil
}
