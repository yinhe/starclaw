package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

// ForgeHandler proxies Nydus API data for the Overlord console Forge dashboard.
type ForgeHandler struct {
	nydusURL    string
	nydusSecret string
	client      *http.Client
}

func NewForgeHandler() *ForgeHandler {
	url := os.Getenv("NYDUS_URL")
	if url == "" {
		url = "https://nydus.starclaw.net"
	}
	return &ForgeHandler{
		nydusURL:    url,
		nydusSecret: os.Getenv("NYDUS_SECRET"),
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
