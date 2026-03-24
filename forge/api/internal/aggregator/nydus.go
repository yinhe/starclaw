package aggregator

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// NydusClient fetches data from the Nydus Git server for dashboard aggregation.
type NydusClient struct {
	BaseURL string
	Secret  string
	client  *http.Client
}

func NewNydusClient(baseURL, secret string) *NydusClient {
	return &NydusClient{
		BaseURL: baseURL,
		Secret:  secret,
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

// HeatmapEntry represents daily commit count.
type HeatmapEntry struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

// HeatmapResult is the response from Nydus heatmap endpoint.
type HeatmapResult struct {
	Heatmap      []HeatmapEntry   `json:"heatmap"`
	TotalCommits int              `json:"total_commits"`
	ActiveDays   int              `json:"active_days"`
	Authors      map[string]int   `json:"authors"`
	Days         string           `json:"days"`
	Repo         string           `json:"repo"`
}

// GetCommitHeatmap fetches daily commit counts from Nydus.
func (c *NydusClient) GetCommitHeatmap(repo string, days int) (*HeatmapResult, error) {
	url := fmt.Sprintf("%s/v1/commits/heatmap?repo=%s&days=%d", c.BaseURL, repo, days)
	req, _ := http.NewRequest("GET", url, nil)
	if c.Secret != "" {
		req.Header.Set("X-Nydus-Secret", c.Secret)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("nydus heatmap: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("nydus heatmap %d: %s", resp.StatusCode, body)
	}
	var result HeatmapResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CommitEntry represents a single commit.
type CommitEntry struct {
	Hash    string `json:"hash"`
	Message string `json:"message"`
	Time    string `json:"time"`
	Author  string `json:"author"`
}

// GetRecentCommits fetches latest commits from Nydus.
func (c *NydusClient) GetRecentCommits(repo string, limit int) ([]CommitEntry, error) {
	url := fmt.Sprintf("%s/v1/commits?repo=%s&limit=%d", c.BaseURL, repo, limit)
	req, _ := http.NewRequest("GET", url, nil)
	if c.Secret != "" {
		req.Header.Set("X-Nydus-Secret", c.Secret)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result struct {
		Commits []CommitEntry `json:"commits"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	return result.Commits, nil
}

// DeployEntry represents a deploy record.
type DeployEntry struct {
	ID       string `json:"id"`
	Repo     string `json:"repo"`
	Branch   string `json:"branch"`
	Target   string `json:"target"`
	Status   string `json:"status"`
	Duration int    `json:"duration_ms"`
	Time     string `json:"created_at"`
}

// GetRecentDeploys fetches latest deploys from Nydus.
func (c *NydusClient) GetRecentDeploys(limit int) ([]DeployEntry, error) {
	url := fmt.Sprintf("%s/v1/deploys?limit=%d", c.BaseURL, limit)
	req, _ := http.NewRequest("GET", url, nil)
	if c.Secret != "" {
		req.Header.Set("X-Nydus-Secret", c.Secret)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result struct {
		Deploys []DeployEntry `json:"deploys"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	return result.Deploys, nil
}
