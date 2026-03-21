package claw

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client calls a Claw node's internal API for Team Agent orchestration.
type Client struct {
	httpClient *http.Client
}

// NewClient creates a new Claw client.
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// ── Request/Response types ──

type CreateSquadReq struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	MaxMembers  int      `json:"max_members"`
	Tags        []string `json:"tags"`
	OverlordRef string   `json:"overlord_ref"` // TeamInstance ID for back-reference
}

type CreateSquadResp struct {
	Squad struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		CaptainNode string `json:"captain_node"`
		Status      string `json:"status"`
	} `json:"squad"`
}

type CreateMissionReq struct {
	SquadID string `json:"squad_id"`
	Title   string `json:"title"`
	Goal    string `json:"goal"`
}

type CreateMissionResp struct {
	Mission struct {
		ID          string `json:"id"`
		SquadID     string `json:"squad_id"`
		Title       string `json:"title"`
		Status      string `json:"status"`
		TotalSteps  int    `json:"total_steps"`
		DoneSteps   int    `json:"done_steps"`
	} `json:"mission"`
}

type GetMissionResp struct {
	Mission struct {
		ID          string  `json:"id"`
		Status      string  `json:"status"`
		TotalSteps  int     `json:"total_steps"`
		DoneSteps   int     `json:"done_steps"`
		PreviewURL  string  `json:"preview_url"`
		FinalResult string  `json:"final_result"`
	} `json:"mission"`
	Steps []struct {
		ID     string `json:"id"`
		Task   string `json:"task"`
		Status string `json:"status"`
	} `json:"steps"`
}

type GetSquadResp struct {
	Squad struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Status string `json:"status"`
	} `json:"squad"`
	Members []struct {
		ID        string `json:"id"`
		NodeID    string `json:"node_id"`
		Role      string `json:"role"`
		Specialty string `json:"specialty"`
		Status    string `json:"status"`
	} `json:"members"`
}

// ── API calls ──

// CreateSquad creates a Squad on the Claw node for the Team Agent.
func (c *Client) CreateSquad(nodeAddr, overlordToken string, req CreateSquadReq) (*CreateSquadResp, error) {
	var resp CreateSquadResp
	if err := c.post(nodeAddr, "/v1/internal/squad/create", overlordToken, req, &resp); err != nil {
		return nil, fmt.Errorf("create squad: %w", err)
	}
	return &resp, nil
}

// DisbandSquad disbands a Squad on the Claw node.
func (c *Client) DisbandSquad(nodeAddr, overlordToken, squadID string) error {
	return c.post(nodeAddr, "/v1/internal/squad/disband", overlordToken,
		map[string]string{"squad_id": squadID}, nil)
}

// CreateMission creates a Mission inside a Squad.
func (c *Client) CreateMission(nodeAddr, overlordToken string, req CreateMissionReq) (*CreateMissionResp, error) {
	var resp CreateMissionResp
	if err := c.post(nodeAddr, "/v1/internal/mission/create", overlordToken, req, &resp); err != nil {
		return nil, fmt.Errorf("create mission: %w", err)
	}
	return &resp, nil
}

// StartMission triggers the Squad Engine to plan and execute a mission.
func (c *Client) StartMission(nodeAddr, overlordToken, missionID string) error {
	return c.post(nodeAddr, "/v1/internal/mission/start", overlordToken,
		map[string]string{"mission_id": missionID}, nil)
}

// GetMission retrieves mission status from the Claw node.
func (c *Client) GetMission(nodeAddr, overlordToken, missionID string) (*GetMissionResp, error) {
	var resp GetMissionResp
	if err := c.get(nodeAddr, fmt.Sprintf("/v1/internal/mission/%s", missionID), overlordToken, &resp); err != nil {
		return nil, fmt.Errorf("get mission: %w", err)
	}
	return &resp, nil
}

// GetSquad retrieves squad status from the Claw node.
func (c *Client) GetSquad(nodeAddr, overlordToken, squadID string) (*GetSquadResp, error) {
	var resp GetSquadResp
	if err := c.get(nodeAddr, fmt.Sprintf("/v1/internal/squad/%s", squadID), overlordToken, &resp); err != nil {
		return nil, fmt.Errorf("get squad: %w", err)
	}
	return &resp, nil
}

// ── HTTP helpers ──

func (c *Client) post(nodeAddr, path, token string, body interface{}, out interface{}) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}

	url := normalizeAddr(nodeAddr) + path
	req, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Overlord-Token", token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("claw returned %d: %s", resp.StatusCode, string(body))
	}

	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

func (c *Client) get(nodeAddr, path, token string, out interface{}) error {
	url := normalizeAddr(nodeAddr) + path
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Overlord-Token", token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("claw returned %d: %s", resp.StatusCode, string(body))
	}

	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

func normalizeAddr(addr string) string {
	if len(addr) > 0 && addr[len(addr)-1] == '/' {
		addr = addr[:len(addr)-1]
	}
	// Add scheme if missing
	if len(addr) > 0 && addr[0] != 'h' {
		addr = "http://" + addr
	}
	return addr
}
