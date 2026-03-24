package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"starclaw.net/forge/internal/config"
	"starclaw.net/forge/internal/model"
)

type DashboardHandler struct {
	DB  *gorm.DB
	Cfg *config.Config
}

// Overview returns the main dashboard data.
func (h *DashboardHandler) Overview(c *gin.Context) {
	// Issue stats
	var totalIssues, openIssues, doneIssues int64
	h.DB.Model(&model.ForgeIssue{}).Count(&totalIssues)
	h.DB.Model(&model.ForgeIssue{}).Where("status NOT IN ('done','closed')").Count(&openIssues)
	h.DB.Model(&model.ForgeIssue{}).Where("status IN ('done','closed')").Count(&doneIssues)

	// Active sprint
	var activeSprint model.ForgeSprint
	hasActiveSprint := h.DB.Where("status = 'active'").Order("created_at DESC").First(&activeSprint).Error == nil

	var sprintIssues, sprintDone int64
	if hasActiveSprint {
		h.DB.Model(&model.ForgeIssue{}).Where("sprint_id = ?", activeSprint.ID).Count(&sprintIssues)
		h.DB.Model(&model.ForgeIssue{}).Where("sprint_id = ? AND status IN ('done','closed')", activeSprint.ID).Count(&sprintDone)
	}

	// Recent activities
	var activities []model.ForgeActivity
	h.DB.Order("created_at DESC").Limit(20).Find(&activities)

	// Agent pool
	var agents []model.ForgeAgent
	h.DB.Find(&agents)

	// Project count
	var projectCount int64
	h.DB.Model(&model.ForgeProject{}).Where("status = 'active'").Count(&projectCount)

	result := gin.H{
		"projects":     projectCount,
		"total_issues": totalIssues,
		"open_issues":  openIssues,
		"done_issues":  doneIssues,
		"activities":   activities,
		"agents":       agents,
	}
	if hasActiveSprint {
		progress := 0
		if sprintIssues > 0 {
			progress = int(sprintDone * 100 / sprintIssues)
		}
		result["active_sprint"] = gin.H{
			"sprint":       activeSprint,
			"total_issues": sprintIssues,
			"done_issues":  sprintDone,
			"progress":     progress,
		}
	}
	c.JSON(http.StatusOK, result)
}

// Services returns health status of all monorepo services.
func (h *DashboardHandler) Services(c *gin.Context) {
	type serviceStatus struct {
		Name    string `json:"name"`
		Env     string `json:"env"`    // online / local
		Status  string `json:"status"` // healthy / unhealthy / unknown
		Latency int    `json:"latency_ms"`
	}

	services := []struct {
		name string
		url  string
		env  string // "online" or "local"
	}{
		{"devclaw", fmt.Sprintf("%s/health", h.Cfg.DevClawURL), "local"},
		{"queen-api", fmt.Sprintf("%s/health", h.Cfg.QueenURL), "online"},
		{"overlord-api", fmt.Sprintf("%s/health", h.Cfg.OverlordURL), "local"},
		{"nydus", fmt.Sprintf("%s/health", h.Cfg.NydusURL), "online"},
		{"forge-api", "http://localhost:8099/health", "local"},
		{"dev-bridge", fmt.Sprintf("%s/health", h.Cfg.DevBridgeURL), "local"},
	}
	// Optional services (only check if URL configured)
	if h.Cfg.HiveURL != "" {
		services = append(services, struct{ name, url, env string }{"hive", fmt.Sprintf("%s/health", h.Cfg.HiveURL), "online"})
	}
	if h.Cfg.SynapseURL != "" {
		services = append(services, struct{ name, url, env string }{"synapse-api", fmt.Sprintf("%s/health", h.Cfg.SynapseURL), "online"})
	}

	client := &http.Client{Timeout: 5 * time.Second}
	var results []serviceStatus
	for _, svc := range services {
		start := time.Now()
		status := "unknown"
		latency := 0

		resp, err := client.Get(svc.url)
		latency = int(time.Since(start).Milliseconds())
		if err != nil {
			status = "unhealthy"
		} else {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				status = "healthy"
			} else {
				status = "unhealthy"
			}
		}
		results = append(results, serviceStatus{
			Name:    svc.name,
			Env:     svc.env,
			Status:  status,
			Latency: latency,
		})
	}

	c.JSON(http.StatusOK, gin.H{"services": results})
}

// Activity returns recent activity feed.
func (h *DashboardHandler) Activity(c *gin.Context) {
	var activities []model.ForgeActivity
	q := h.DB.Order("created_at DESC").Limit(50)
	if s := c.Query("source"); s != "" {
		q = q.Where("source = ?", s)
	}
	if s := c.Query("service"); s != "" {
		q = q.Where("service = ?", s)
	}
	q.Find(&activities)
	c.JSON(http.StatusOK, gin.H{"activities": activities})
}

// Branches returns active feature branches from Dev Bridge.
func (h *DashboardHandler) Branches(c *gin.Context) {
	data, err := h.callDevBridge("git_branches", map[string]interface{}{})
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"branches": []string{}, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"branches_raw": data})
}

// DevClaws returns DevClaw instance status from Overlord.
func (h *DashboardHandler) DevClaws(c *gin.Context) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(h.Cfg.OverlordURL + "/brood/team-agent/instances")
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"devclaws": []string{}, "error": err.Error()})
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var result interface{}
	json.Unmarshal(body, &result)
	c.JSON(http.StatusOK, gin.H{"devclaws": result})
}

// Stats returns aggregate statistics.
func (h *DashboardHandler) Stats(c *gin.Context) {
	var totalIssues, openIssues, doneIssues int64
	h.DB.Model(&model.ForgeIssue{}).Count(&totalIssues)
	h.DB.Model(&model.ForgeIssue{}).Where("status NOT IN ('done','closed')").Count(&openIssues)
	h.DB.Model(&model.ForgeIssue{}).Where("status IN ('done','closed')").Count(&doneIssues)

	// By service
	type svcStat struct {
		Service string `json:"service"`
		Count   int64  `json:"count"`
	}
	var byService []svcStat
	h.DB.Model(&model.ForgeIssue{}).Select("service, count(*) as count").Where("service != ''").Group("service").Scan(&byService)

	// By priority
	type priStat struct {
		Priority string `json:"priority"`
		Count    int64  `json:"count"`
	}
	var byPriority []priStat
	h.DB.Model(&model.ForgeIssue{}).Select("priority, count(*) as count").Group("priority").Scan(&byPriority)

	c.JSON(http.StatusOK, gin.H{
		"total_issues": totalIssues,
		"open_issues":  openIssues,
		"done_issues":  doneIssues,
		"by_service":   byService,
		"by_priority":  byPriority,
	})
}

// callDevBridge makes an MCP tool call to Dev Bridge.
func (h *DashboardHandler) callDevBridge(tool string, args map[string]interface{}) (string, error) {
	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      tool,
			"arguments": args,
		},
	}
	bodyJSON, _ := json.Marshal(reqBody)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(h.Cfg.DevBridgeURL, "application/json", io.NopCloser(jsonReader(bodyJSON)))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return string(body), nil
}

func jsonReader(data []byte) io.Reader {
	return io.NopCloser(readerFromBytes(data))
}

type bytesReader struct {
	data []byte
	pos  int
}

func (r *bytesReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}
func readerFromBytes(data []byte) io.Reader { return &bytesReader{data: data} }
