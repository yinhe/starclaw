package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// DashboardHandler proxies to the swarm service for global stats and node management
type DashboardHandler struct {
	swarmURL string
	httpC    *http.Client
}

func NewDashboardHandler(swarmURL string) *DashboardHandler {
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
	resp, err := h.httpC.Post(h.swarmURL+"/swarm/update/notify", "application/json", io.NopCloser(io.Reader(nil)))
	if err != nil {
		// Fallback: forward body properly
		resp, err = h.httpC.Post(h.swarmURL+"/swarm/update/notify", "application/json", io.NopCloser(newBytesReader(body)))
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "failed to reach swarm service"})
			return
		}
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

type bytesReaderCloser struct {
	*bytesReader
}

type bytesReader struct {
	data []byte
	pos  int
}

func newBytesReader(data []byte) *bytesReader {
	return &bytesReader{data: data}
}

func (r *bytesReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}
