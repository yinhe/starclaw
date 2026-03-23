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
		ID         string `json:"id"`
		SquadID    string `json:"squad_id"`
		Title      string `json:"title"`
		Status     string `json:"status"`
		TotalSteps int    `json:"total_steps"`
		DoneSteps  int    `json:"done_steps"`
	} `json:"mission"`
}

type GetMissionResp struct {
	Mission struct {
		ID          string `json:"id"`
		Status      string `json:"status"`
		TotalSteps  int    `json:"total_steps"`
		DoneSteps   int    `json:"done_steps"`
		PreviewURL  string `json:"preview_url"`
		FinalResult string `json:"final_result"`
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

// ── Chat Completion (proxy to Claw's OpenAI-compatible chat API) ──

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatCompletionReq struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature float64       `json:"temperature,omitempty"`
	Stream      bool          `json:"stream"`
}

type ChatCompletionResp struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int         `json:"index"`
		Message ChatMessage `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// ChatCompletion sends a chat completion request to the Claw node's API.
func (c *Client) ChatCompletion(nodeAddr, overlordToken string, req ChatCompletionReq) (*ChatCompletionResp, error) {
	var resp ChatCompletionResp
	if err := c.post(nodeAddr, "/v1/internal/chat/completions", overlordToken, req, &resp); err != nil {
		return nil, fmt.Errorf("chat completion: %w", err)
	}
	return &resp, nil
}

// ChatCompletionStream sends a streaming chat completion request.
// Returns the raw HTTP response body (SSE stream). Caller must close it.
func (c *Client) ChatCompletionStream(nodeAddr, overlordToken string, req ChatCompletionReq) (io.ReadCloser, error) {
	req.Stream = true
	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	url := normalizeAddr(nodeAddr) + "/v1/internal/chat/completions"
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Overlord-Token", overlordToken)

	// Use a longer timeout for streaming
	streamClient := &http.Client{Timeout: 5 * time.Minute}
	resp, err := streamClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http error: %w", err)
	}

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("claw returned %d: %s", resp.StatusCode, string(body))
	}

	return resp.Body, nil
}

// ── Auth Exchange (Overlord ↔ Claw token bridge) ──

type AuthExchangeReq struct {
	OverlordUserID string `json:"overlord_user_id"`
	Username       string `json:"username"`
	Role           string `json:"role"`
}

type AuthExchangeResp struct {
	Token     string `json:"token"`
	UserID    string `json:"user_id"`
	Username  string `json:"username"`
	ExpiresAt string `json:"expires_at"`
}

// AuthExchange sends Overlord user info to Claw and gets back a Claw JWT.
func (c *Client) AuthExchange(nodeAddr, overlordToken string, req AuthExchangeReq) (*AuthExchangeResp, error) {
	var resp AuthExchangeResp
	if err := c.post(nodeAddr, "/v1/internal/auth/exchange", overlordToken, req, &resp); err != nil {
		return nil, fmt.Errorf("auth exchange: %w", err)
	}
	return &resp, nil
}

type AuthVerifyReq struct {
	Token string `json:"token"`
}

type AuthVerifyResp struct {
	Valid    bool   `json:"valid"`
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

// AuthVerify asks a Claw node to verify a JWT and return user info.
func (c *Client) AuthVerify(nodeAddr, overlordToken string, req AuthVerifyReq) (*AuthVerifyResp, error) {
	var resp AuthVerifyResp
	if err := c.post(nodeAddr, "/v1/internal/auth/verify", overlordToken, req, &resp); err != nil {
		return nil, fmt.Errorf("auth verify: %w", err)
	}
	return &resp, nil
}

// ── Models ──

type ClawModel struct {
	ID          string  `json:"id"`
	Provider    string  `json:"provider"`
	ModelName   string  `json:"model_name"`
	DisplayName string  `json:"display_name"`
	MaxTokens   int     `json:"max_tokens"`
	Temperature float64 `json:"temperature"`
	IsPlatform  bool    `json:"is_platform"`
}

type ListModelsResp struct {
	Models []ClawModel `json:"models"`
	Total  int         `json:"total"`
}

// ListModels fetches available models from a Claw node.
func (c *Client) ListModels(nodeAddr, overlordToken string) (*ListModelsResp, error) {
	var resp ListModelsResp
	if err := c.get(nodeAddr, "/v1/internal/models", overlordToken, &resp); err != nil {
		return nil, fmt.Errorf("list models: %w", err)
	}
	return &resp, nil
}

// ── Skills ──

type ClawSkill struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Status      string `json:"status"`
}

type ClawPlugin struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Icon        string `json:"icon"`
	Version     string `json:"version"`
	Pricing     string `json:"pricing"`
}

type ClawMCPServer struct {
	Name    string `json:"name"`
	BaseURL string `json:"base_url"`
}

type ListSkillsResp struct {
	Skills     []ClawSkill     `json:"skills"`
	Plugins    []ClawPlugin    `json:"plugins"`
	MCPServers []ClawMCPServer `json:"mcp_servers"`
	Total      int             `json:"total"`
}

// ListSkills fetches available skills/tools from a Claw node.
func (c *Client) ListSkills(nodeAddr, overlordToken string) (*ListSkillsResp, error) {
	var resp ListSkillsResp
	if err := c.get(nodeAddr, "/v1/internal/skills", overlordToken, &resp); err != nil {
		return nil, fmt.Errorf("list skills: %w", err)
	}
	return &resp, nil
}

// ── Agents ──

type ClawAgent struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Description  string  `json:"description"`
	Category     string  `json:"category"`
	SystemPrompt string  `json:"system_prompt"`
	Model        string  `json:"model"`
	Tools        string  `json:"tools"`
	Icon         string  `json:"icon"`
	Rating       float64 `json:"rating"`
	InstallCount int     `json:"install_count"`
	IsOfficial   bool    `json:"is_official"`
}

type ListAgentsResp struct {
	Agents     []ClawAgent `json:"agents"`
	Categories []struct {
		Category string `json:"category"`
		Count    int    `json:"count"`
	} `json:"categories"`
	Total int `json:"total"`
}

// ListAgents fetches published agent templates from a Claw node's marketplace.
func (c *Client) ListAgents(nodeAddr, overlordToken string) (*ListAgentsResp, error) {
	var resp ListAgentsResp
	if err := c.get(nodeAddr, "/v1/internal/agents", overlordToken, &resp); err != nil {
		return nil, fmt.Errorf("list agents: %w", err)
	}
	return &resp, nil
}

// ── Agent Development (Sandbox + Publish) ──

type AgentSandboxReq struct {
	Name         string `json:"name"`
	SystemPrompt string `json:"system_prompt"`
	Model        string `json:"model"`
	Tools        string `json:"tools"`
	Config       string `json:"config"`
	TestMessages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"test_messages"`
}

type AgentSandboxResult struct {
	Input   string          `json:"input"`
	Output  string          `json:"output"`
	Verdict string          `json:"verdict"`
	Checks  map[string]bool `json:"checks"`
	Error   string          `json:"error,omitempty"`
}

type AgentSandboxResp struct {
	Results        []AgentSandboxResult `json:"results"`
	OverallScore   float64              `json:"overall_score"`
	PassCount      int                  `json:"pass_count"`
	TotalTests     int                  `json:"total_tests"`
	ReadyToPublish bool                 `json:"ready_to_publish"`
}

// AgentSandbox tests an agent config in a sandbox on a Claw node.
func (c *Client) AgentSandbox(nodeAddr, overlordToken string, req AgentSandboxReq) (*AgentSandboxResp, error) {
	var resp AgentSandboxResp
	if err := c.post(nodeAddr, "/v1/internal/agent-sandbox", overlordToken, req, &resp); err != nil {
		return nil, fmt.Errorf("agent sandbox: %w", err)
	}
	return &resp, nil
}

type AgentPublishReq struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	SystemPrompt string `json:"system_prompt"`
	Model        string `json:"model"`
	Tools        string `json:"tools"`
	Config       string `json:"config"`
	Category     string `json:"category"`
	Tags         string `json:"tags"`
	Icon         string `json:"icon"`
	Pricing      string `json:"pricing"`
}

type AgentPublishResp struct {
	TemplateID string `json:"template_id"`
	Name       string `json:"name"`
	Category   string `json:"category"`
	Status     string `json:"status"`
}

// AgentPublish publishes an agent to the marketplace on a Claw node.
func (c *Client) AgentPublish(nodeAddr, overlordToken string, req AgentPublishReq) (*AgentPublishResp, error) {
	var resp AgentPublishResp
	if err := c.post(nodeAddr, "/v1/internal/agent-publish", overlordToken, req, &resp); err != nil {
		return nil, fmt.Errorf("agent publish: %w", err)
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
