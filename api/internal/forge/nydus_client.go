package forge

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

// NydusClient is an HTTP client for the Nydus API.
// Used by Claw to manage repos, PRs, and webhooks on the Nydus server.
type NydusClient struct {
	BaseURL string
	Secret  string
	Token   string // Bearer token (from node registration)
	client  *http.Client
}

// NewNydusClient creates a client from environment variables.
// NYDUS_URL — Nydus API base URL (e.g. http://nydus.starclaw.net:8095)
// NYDUS_SECRET — shared API secret
// NYDUS_TOKEN — Bearer token for authenticated node requests
func NewNydusClient() *NydusClient {
	baseURL := os.Getenv("NYDUS_URL")
	if baseURL == "" {
		baseURL = "https://nydus.starclaw.net"
	}
	return &NydusClient{
		BaseURL: baseURL,
		Secret:  os.Getenv("NYDUS_SECRET"),
		Token:   os.Getenv("NYDUS_TOKEN"),
		client:  &http.Client{Timeout: 15 * time.Second},
	}
}

// ────────────────────────────────────────────────────────────
// Repos
// ────────────────────────────────────────────────────────────

type NydusRepo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Owner       string `json:"owner"`
	TeamID      string `json:"team_id"`
	Public      bool   `json:"public"`
	Source      string `json:"source"`
	SSHURL      string `json:"ssh_url"`
	HTTPSURL    string `json:"https_url"`
	Initialized bool   `json:"initialized"`
}

func (c *NydusClient) ListRepos() ([]NydusRepo, error) {
	var result struct {
		Repos []NydusRepo `json:"repos"`
	}
	if err := c.get("/api/repos", &result); err != nil {
		return nil, err
	}
	return result.Repos, nil
}

func (c *NydusClient) CreateRepo(name, description string, public bool, teamID string) (*NydusRepo, error) {
	body := map[string]interface{}{
		"name":        name,
		"description": description,
		"public":      public,
		"team_id":     teamID,
	}
	var result struct {
		Repo NydusRepo `json:"repo"`
	}
	if err := c.post("/api/repos", body, &result); err != nil {
		return nil, err
	}
	return &result.Repo, nil
}

func (c *NydusClient) ForkRepo(parentName, newName, teamID string) (*NydusRepo, error) {
	body := map[string]interface{}{
		"new_name": newName,
		"team_id":  teamID,
	}
	var result struct {
		Repo NydusRepo `json:"repo"`
	}
	if err := c.post(fmt.Sprintf("/api/repos/%s/fork", parentName), body, &result); err != nil {
		return nil, err
	}
	return &result.Repo, nil
}

// ────────────────────────────────────────────────────────────
// Pull Requests
// ────────────────────────────────────────────────────────────

type NydusPR struct {
	ID           string `json:"id"`
	Number       int    `json:"number"`
	Title        string `json:"title"`
	Body         string `json:"body"`
	SourceBranch string `json:"source_branch"`
	TargetBranch string `json:"target_branch"`
	Status       string `json:"status"`
	AuthorNodeID string `json:"author_node_id"`
	MergeCommit  string `json:"merge_commit"`
}

func (c *NydusClient) CreatePR(repoName, title, body, sourceBranch, targetBranch string) (*NydusPR, error) {
	reqBody := map[string]interface{}{
		"title":         title,
		"body":          body,
		"source_branch": sourceBranch,
		"target_branch": targetBranch,
	}
	var result struct {
		PR NydusPR `json:"pull_request"`
	}
	if err := c.post(fmt.Sprintf("/api/repos/%s/pulls", repoName), reqBody, &result); err != nil {
		return nil, err
	}
	return &result.PR, nil
}

func (c *NydusClient) ListPRs(repoName, status string) ([]NydusPR, error) {
	url := fmt.Sprintf("/api/repos/%s/pulls?status=%s", repoName, status)
	var result struct {
		PRs []NydusPR `json:"pull_requests"`
	}
	if err := c.get(url, &result); err != nil {
		return nil, err
	}
	return result.PRs, nil
}

func (c *NydusClient) MergePR(repoName string, number int, strategy string) error {
	body := map[string]interface{}{"strategy": strategy}
	return c.post(fmt.Sprintf("/api/repos/%s/pulls/%d/merge", repoName, number), body, nil)
}

// ────────────────────────────────────────────────────────────
// Webhooks
// ────────────────────────────────────────────────────────────

func (c *NydusClient) RegisterWebhook(repoName, callbackURL, secret, events string) error {
	body := map[string]interface{}{
		"url":    callbackURL,
		"secret": secret,
		"events": events,
	}
	return c.post(fmt.Sprintf("/api/repos/%s/webhooks", repoName), body, nil)
}

// ────────────────────────────────────────────────────────────
// HTTP helpers
// ────────────────────────────────────────────────────────────

func (c *NydusClient) get(path string, result interface{}) error {
	req, err := http.NewRequest("GET", c.BaseURL+path, nil)
	if err != nil {
		return err
	}
	c.setAuth(req)
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("nydus GET %s: %w", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("nydus GET %s: %d %s", path, resp.StatusCode, body)
	}
	if result != nil {
		return json.NewDecoder(resp.Body).Decode(result)
	}
	return nil
}

func (c *NydusClient) post(path string, body interface{}, result interface{}) error {
	jsonBody, _ := json.Marshal(body)
	req, err := http.NewRequest("POST", c.BaseURL+path, bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	c.setAuth(req)
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("nydus POST %s: %w", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("nydus POST %s: %d %s", path, resp.StatusCode, respBody)
	}
	if result != nil {
		return json.NewDecoder(resp.Body).Decode(result)
	}
	return nil
}

func (c *NydusClient) setAuth(req *http.Request) {
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	if c.Secret != "" {
		req.Header.Set("X-Secret", c.Secret)
	}
}

// Global singleton
var nydusClient *NydusClient

func GetNydusClient() *NydusClient {
	if nydusClient == nil {
		nydusClient = NewNydusClient()
		log.Printf("[forge] Nydus client → %s", nydusClient.BaseURL)
	}
	return nydusClient
}
