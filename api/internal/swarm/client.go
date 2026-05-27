package swarm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/yinhe/starclaw/internal/config"
	"github.com/yinhe/starclaw/internal/molt"
	"github.com/yinhe/starclaw/internal/node"
)

// Connection state constants
const (
	StateDisconnected = "disconnected" // never connected or disabled
	StateConnected    = "connected"    // heartbeat OK
	StateFeral        = "feral"        // heartbeat failing, running autonomously
)

// FeralThreshold: after this many consecutive heartbeat failures, enter feral mode
const FeralThreshold = 3

// CreditBalance holds cached star energy balance from Queen
type CreditBalance struct {
	Balance       int64     `json:"balance"`
	BalanceEnergy float64   `json:"balance_energy"`
	Frozen        int64     `json:"frozen"`
	FrozenEnergy  float64   `json:"frozen_energy"`
	TotalIn       int64     `json:"total_in"`
	TotalOut      int64     `json:"total_out"`
	Nonce         int64     `json:"nonce"`
	Status        string    `json:"status"`
	HPStatus      string    `json:"hp_status"`
	TrustLevel    string    `json:"trust_level"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// Client handles swarm registration and heartbeat with Queen/Overlord
type Client struct {
	cfg              config.SwarmConfig
	nodeID           string
	token            string
	clawID           string
	nodeAddress      string
	mu               sync.RWMutex
	stopCh           chan struct{}
	httpC            *http.Client
	consecutiveFails int
	lastHeartbeat    time.Time
	feralSince       time.Time // zero if not in feral mode
	credits          *CreditBalance
	creditClient     *CreditClient // star energy operations client
	moltUpdating     bool          // prevents concurrent molt updates
	UpdateFunc       func() error  // set by system handler to perform Docker update

	// Mining: set by inference contributor service to report contributor status
	ContributorInfoFunc func() *ContributorInfo
}

// ContributorInfo holds compute contribution status for heartbeat reporting
type ContributorInfo struct {
	IsContributor bool     `json:"is_contributor"`
	Models        []string `json:"contributor_models"`
	GPUInfo       string   `json:"gpu_info"`
}

// NormalizeQueenURL converts claw:// protocol to https:// (or http:// for local addresses).
//
//	claw://swarm.starclaw.net       → https://swarm.starclaw.net
//	claw://192.168.1.100:8090       → http://192.168.1.100:8090
//	claw://starclaw-queen-swarm:8090→ http://starclaw-queen-swarm:8090
//	https://swarm.starclaw.net      → https://swarm.starclaw.net (unchanged)
//	http://localhost:8090            → http://localhost:8090 (unchanged)
func NormalizeQueenURL(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimRight(raw, "/")

	if strings.HasPrefix(raw, "claw://") {
		hostPort := strings.TrimPrefix(raw, "claw://")
		host := hostPort
		if h, _, err := net.SplitHostPort(hostPort); err == nil {
			host = h
		}
		// Local/internal addresses use http, public domains use https
		if isLocalHost(host) {
			return "http://" + hostPort
		}
		return "https://" + hostPort
	}

	// Already has scheme
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		return raw
	}

	// Bare domain/IP — guess scheme
	host := raw
	if h, _, err := net.SplitHostPort(raw); err == nil {
		host = h
	}
	if isLocalHost(host) {
		return "http://" + raw
	}
	return "https://" + raw
}

// isLocalHost returns true for localhost, IPs, docker container names (no dots), etc.
func isLocalHost(host string) bool {
	if host == "localhost" || host == "127.0.0.1" || host == "0.0.0.0" || host == "::1" {
		return true
	}
	if net.ParseIP(host) != nil {
		return true // any IP address
	}
	if !strings.Contains(host, ".") {
		return true // docker container name like "starclaw-queen-swarm"
	}
	return false
}

// NewClient creates a swarm client from config
func NewClient(cfg config.SwarmConfig) *Client {
	cfg.QueenURL = NormalizeQueenURL(cfg.QueenURL)
	return &Client{
		cfg:    cfg,
		stopCh: make(chan struct{}),
		httpC:  &http.Client{Timeout: 10 * time.Second},
	}
}

// Start registers with Queen and begins heartbeat loop
func (c *Client) Start() {
	if !c.cfg.Enabled || c.cfg.QueenURL == "" {
		log.Println("[swarm] swarm disabled or queen_url not set, running standalone")
		return
	}

	log.Printf("[swarm] will register with Queen at %s (waiting for server ready)", c.cfg.QueenURL)

	// Heartbeat loop with exponential backoff + jitter
	baseInterval := time.Duration(c.cfg.HeartbeatInterval) * time.Second
	if baseInterval < 10*time.Second {
		baseInterval = 60 * time.Second
	}

	// Delay initial registration to allow HTTP server to start (needed for
	// localhost fallback in self-hosted Docker deployments with hairpin NAT).
	go func() {
		time.Sleep(3 * time.Second)
		if err := c.register(); err != nil {
			log.Printf("[swarm] registration failed: %v (will retry in heartbeat loop)", err)
		} else {
			log.Printf("[swarm] registered as node %s", c.nodeID)
			go c.registerArena()
		}
		c.heartbeatLoop(baseInterval)
	}()
}

// heartbeatLoop runs heartbeat with exponential backoff on failure.
// On success: reset to baseInterval. On failure: double delay (cap 5min), ±20% jitter.
func (c *Client) heartbeatLoop(baseInterval time.Duration) {
	const maxBackoff = 5 * time.Minute
	backoff := time.Duration(0) // 0 means use baseInterval
	consecutiveFails := 0

	for {
		// Calculate sleep duration with jitter
		sleep := baseInterval
		if backoff > 0 {
			sleep = backoff
		}
		sleep = addJitter(sleep, 0.2) // ±20%

		select {
		case <-time.After(sleep):
		case <-c.stopCh:
			return
		}

		// Retry registration if not registered
		if c.nodeID == "" {
			if err := c.register(); err != nil {
				consecutiveFails++
				backoff = calcBackoff(consecutiveFails, maxBackoff)
				log.Printf("[swarm] registration failed (retry in %s): %v", backoff, err)
				continue
			}
			log.Printf("[swarm] registered as node %s", c.nodeID)
		}

		// Send heartbeat
		if err := c.heartbeat(); err != nil {
			consecutiveFails++
			c.mu.Lock()
			c.consecutiveFails = consecutiveFails
			if consecutiveFails == FeralThreshold {
				c.feralSince = time.Now()
				log.Printf("[swarm] entering FERAL mode after %d failures — running autonomously", consecutiveFails)
			}
			c.mu.Unlock()
			backoff = calcBackoff(consecutiveFails, maxBackoff)
			log.Printf("[swarm] heartbeat failed (retry in %s): %v", backoff, err)
		} else {
			wasFeral := consecutiveFails >= FeralThreshold
			if consecutiveFails > 0 {
				log.Printf("[swarm] heartbeat recovered after %d failures", consecutiveFails)
			}
			if wasFeral {
				log.Println("[swarm] exiting FERAL mode — reconnected to Queen")
			}
			consecutiveFails = 0
			backoff = 0
			c.mu.Lock()
			c.consecutiveFails = 0
			c.lastHeartbeat = time.Now()
			c.feralSince = time.Time{}
			c.mu.Unlock()
		}
	}
}

// calcBackoff returns exponential backoff: 1s, 2s, 4s, 8s, ... capped at max.
func calcBackoff(failures int, max time.Duration) time.Duration {
	d := time.Second << uint(failures-1) // 2^(n-1) seconds
	if d > max {
		d = max
	}
	return d
}

// addJitter adds ±pct random jitter to a duration.
func addJitter(d time.Duration, pct float64) time.Duration {
	jitterRange := float64(d) * pct * 2
	jitter := float64(time.Now().UnixNano()%1000) / 1000 * jitterRange // pseudo-random [0, range)
	return d - time.Duration(float64(d)*pct) + time.Duration(jitter)
}

// Stop gracefully stops the heartbeat loop
func (c *Client) Stop() {
	close(c.stopCh)
}

// NodeID returns the registered node ID
func (c *Client) NodeID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.nodeID
}

// SetClawID sets the Ed25519-derived claw: address for registration
func (c *Client) SetClawID(id string) {
	c.mu.Lock()
	c.clawID = id
	c.mu.Unlock()
}

// SetIdentity sets the node identity and initializes the CreditClient.
func (c *Client) SetIdentity(identity *node.Identity) {
	c.mu.Lock()
	c.clawID = identity.NodeID
	if c.cfg.QueenURL != "" {
		c.creditClient = NewCreditClient(c.cfg.QueenURL, identity)
	}
	c.mu.Unlock()
}

// CreditClient returns the star energy operations client (may be nil if not connected to Queen).
func (c *Client) CreditClient() *CreditClient {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.creditClient
}

// SetAddress sets the node's public-facing address for swarm registration
func (c *Client) SetAddress(addr string) {
	c.mu.Lock()
	c.nodeAddress = addr
	c.mu.Unlock()
}

// QueenURL returns the configured Queen URL
func (c *Client) QueenURL() string {
	return c.cfg.QueenURL
}

// Connected returns whether this client is registered with Queen
func (c *Client) Connected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.nodeID != "" && c.cfg.Enabled
}

// State returns the current connection state: connected, feral, or disconnected
func (c *Client) State() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if !c.cfg.Enabled || c.nodeID == "" {
		return StateDisconnected
	}
	if c.consecutiveFails >= FeralThreshold {
		return StateFeral
	}
	return StateConnected
}

// ConsecutiveFails returns the number of consecutive heartbeat failures
func (c *Client) ConsecutiveFails() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.consecutiveFails
}

// LastHeartbeat returns the last successful heartbeat time
func (c *Client) LastHeartbeat() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastHeartbeat
}

// FeralSince returns when feral mode started (zero if not feral)
func (c *Client) FeralSince() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.feralSince
}

// Resolve queries Queen's swarm registry for a claw: address
func (c *Client) Resolve(clawID string) (address string, found bool) {
	if !c.Connected() || c.cfg.QueenURL == "" {
		return "", false
	}
	url := fmt.Sprintf("%s/swarm/resolve?claw_id=%s", c.cfg.QueenURL, clawID)
	resp, err := c.httpC.Get(url)
	if err != nil {
		log.Printf("[swarm] resolve via Queen failed: %v", err)
		return "", false
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if json.NewDecoder(resp.Body).Decode(&result) != nil {
		return "", false
	}
	if f, ok := result["found"].(bool); ok && f {
		if addr, ok := result["address"].(string); ok {
			return addr, true
		}
	}
	return "", false
}

func (c *Client) register() error {
	name := c.cfg.NodeName
	if name == "" {
		hostname, _ := os.Hostname()
		name = hostname
	}

	c.mu.RLock()
	cid := c.clawID
	c.mu.RUnlock()

	addr := c.nodeAddress
	if addr == "" {
		addr = fmt.Sprintf("%s:8080", getOutboundIP())
	}

	body := map[string]interface{}{
		"name":    name,
		"role":    "claw",
		"version": molt.Version,
		"address": addr,
		"region":  c.cfg.Region,
		"claw_id": cid,
	}

	resp, err := c.post("/swarm/register", body)
	if err != nil {
		return err
	}

	c.mu.Lock()
	c.nodeID = resp["node_id"].(string)
	c.token = resp["token"].(string)
	c.mu.Unlock()

	// Persist credentials for restart survival
	saveSwarmCredentials(c.nodeID, c.token)

	return nil
}

// registerArena registers this Claw as an ArenaAgent in the 龙虾社区.
// Called once after successful swarm registration. Failures are non-fatal.
func (c *Client) registerArena() {
	c.mu.RLock()
	nid := c.nodeID
	cid := c.clawID
	c.mu.RUnlock()

	name := c.cfg.NodeName
	if name == "" {
		hostname, _ := os.Hostname()
		name = hostname
	}

	body := map[string]interface{}{
		"node_id":     nid,
		"name":        name,
		"description": fmt.Sprintf("Claw %s (v%s)", name, molt.Version),
	}
	if cid != "" {
		body["node_id"] = cid
	}

	resp, err := c.post("/arena/agents", body)
	if err != nil {
		log.Printf("[arena] registration failed (non-fatal): %v", err)
		return
	}
	agentID, _ := resp["agent"].(map[string]interface{})
	if agentID != nil {
		if id, ok := agentID["id"].(string); ok {
			log.Printf("[arena] registered in 龙虾社区 as %s (%s)", name, id)
			return
		}
	}
	log.Printf("[arena] registered in 龙虾社区 as %s", name)
}

func (c *Client) heartbeat() error {
	c.mu.RLock()
	nid, tok := c.nodeID, c.token
	c.mu.RUnlock()

	if nid == "" || tok == "" {
		return fmt.Errorf("not registered")
	}

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	memPct := float64(memStats.Alloc) / float64(memStats.Sys) * 100

	c.mu.RLock()
	cid := c.clawID
	addr := c.nodeAddress
	c.mu.RUnlock()

	body := map[string]interface{}{
		"node_id":       nid,
		"token":         tok,
		"version":       molt.Version,
		"claw_id":       cid,
		"address":       addr,
		"cpu_percent":   0, // TODO: real CPU sampling
		"mem_percent":   memPct,
		"tasks_running": 0,
		"tasks_queued":  0,
		"error_rate":    0,
	}

	// Inject contributor info if available
	if c.ContributorInfoFunc != nil {
		if info := c.ContributorInfoFunc(); info != nil && info.IsContributor {
			body["is_contributor"] = true
			body["contributor_models"] = info.Models
			body["gpu_info"] = info.GPUInfo
		}
	}

	resp, err := c.post("/swarm/heartbeat", body)
	if err != nil {
		return err
	}

	// Parse credit balance from heartbeat response
	if creditsRaw, ok := resp["credits"]; ok && creditsRaw != nil {
		if creditsMap, ok := creditsRaw.(map[string]interface{}); ok {
			cb := &CreditBalance{UpdatedAt: time.Now()}
			if v, ok := creditsMap["balance"].(float64); ok {
				cb.Balance = int64(v)
			}
			if v, ok := creditsMap["balance_energy"].(float64); ok {
				cb.BalanceEnergy = v
			}
			if v, ok := creditsMap["frozen"].(float64); ok {
				cb.Frozen = int64(v)
			}
			if v, ok := creditsMap["frozen_energy"].(float64); ok {
				cb.FrozenEnergy = v
			}
			if v, ok := creditsMap["total_in"].(float64); ok {
				cb.TotalIn = int64(v)
			}
			if v, ok := creditsMap["total_out"].(float64); ok {
				cb.TotalOut = int64(v)
			}
			if v, ok := creditsMap["nonce"].(float64); ok {
				cb.Nonce = int64(v)
			}
			if v, ok := creditsMap["status"].(string); ok {
				cb.Status = v
			}
			if v, ok := creditsMap["hp_status"].(string); ok {
				cb.HPStatus = v
			}
			if v, ok := creditsMap["trust_level"].(string); ok {
				cb.TrustLevel = v
			}
			c.mu.Lock()
			c.credits = cb
			cc := c.creditClient
			c.mu.Unlock()

			// Forward to CreditClient for HP monitoring
			if cc != nil {
				cc.UpdateFromHeartbeat(cb)
			}
		}
	}

	// Parse molt update directive from heartbeat response
	if moltRaw, ok := resp["molt"]; ok && moltRaw != nil {
		if moltMap, ok := moltRaw.(map[string]interface{}); ok {
			if avail, _ := moltMap["update_available"].(bool); avail {
				releaseID, _ := moltMap["release_id"].(string)
				targetVer, _ := moltMap["version"].(string)
				mandatory, _ := moltMap["mandatory"].(bool)
				log.Printf("[molt] Queen instructed update: %s → %s (release=%s, mandatory=%v)", molt.Version, targetVer, releaseID, mandatory)
				go c.executeMoltUpdate(releaseID, targetVer)
			}
		}
	}

	return nil
}

// executeMoltUpdate saves a pending marker file and triggers the Docker update.
// After restart, ReportPendingMolt() reads the marker and reports success/failure to Queen.
func (c *Client) executeMoltUpdate(releaseID, targetVersion string) {
	c.mu.Lock()
	if c.moltUpdating {
		c.mu.Unlock()
		log.Printf("[molt] update already in progress, skipping")
		return
	}
	c.moltUpdating = true
	c.mu.Unlock()

	// Write .molt_pending so we can report back after restart
	pending := fmt.Sprintf("%s\n%s\n", releaseID, targetVersion)
	if err := os.WriteFile(".molt_pending", []byte(pending), 0600); err != nil {
		log.Printf("[molt] failed to write .molt_pending: %v", err)
	}

	if c.UpdateFunc == nil {
		log.Printf("[molt] UpdateFunc not set, cannot perform auto-update. User must update manually.")
		c.mu.Lock()
		c.moltUpdating = false
		c.mu.Unlock()
		return
	}

	log.Printf("[molt] executing auto-update to %s...", targetVersion)
	if err := c.UpdateFunc(); err != nil {
		log.Printf("[molt] auto-update failed: %v", err)
		c.mu.Lock()
		c.moltUpdating = false
		c.mu.Unlock()
	}
	// If update succeeds, the container restarts — we never reach here
}

// ReportPendingMolt checks if there's a .molt_pending file from a previous update
// and reports the result to Queen. Called once on startup.
func (c *Client) ReportPendingMolt() {
	data, err := os.ReadFile(".molt_pending")
	if err != nil {
		return // no pending report
	}
	parts := bytes.Split(data, []byte("\n"))
	if len(parts) < 2 {
		os.Remove(".molt_pending")
		return
	}
	releaseID := string(parts[0])
	targetVersion := string(parts[1])

	// Determine if update succeeded by comparing current version
	status := "ok"
	errMsg := ""
	if molt.Version == "dev" || molt.Version < targetVersion {
		status = "failed"
		errMsg = fmt.Sprintf("version still %s after update (expected %s)", molt.Version, targetVersion)
	}

	log.Printf("[molt] reporting update result to Queen: release=%s status=%s (current=%s, target=%s)", releaseID, status, molt.Version, targetVersion)

	c.mu.RLock()
	nid := c.nodeID
	c.mu.RUnlock()

	body := map[string]interface{}{
		"node_id":    nid,
		"release_id": releaseID,
		"status":     status,
		"error":      errMsg,
	}
	if _, err := c.post("/swarm/molt/report", body); err != nil {
		log.Printf("[molt] failed to report to Queen: %v", err)
		return
	}

	os.Remove(".molt_pending")
	log.Printf("[molt] update report sent successfully")
}

// Credits returns the cached star energy balance (updated each heartbeat)
func (c *Client) Credits() *CreditBalance {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.credits
}

func (c *Client) post(path string, body map[string]interface{}) (map[string]interface{}, error) {
	data, _ := json.Marshal(body)
	url := c.cfg.QueenURL + path

	result, err := c.doPost(url, data)
	if err != nil && strings.Contains(err.Error(), "decode response") {
		// Docker hairpin NAT: container can't reach itself via public URL.
		// Fallback to localhost (embedded queen on same host).
		fallback := "http://127.0.0.1:8080" + path
		if r, e := c.doPost(fallback, data); e == nil {
			log.Printf("[swarm] queen reachable via localhost fallback (hairpin NAT workaround)")
			return r, nil
		}
	}
	return result, err
}

func (c *Client) doPost(url string, data []byte) (map[string]interface{}, error) {
	resp, err := c.httpC.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("POST %s: %w", url, err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if resp.StatusCode >= 400 {
		errMsg, _ := result["error"].(string)
		return nil, fmt.Errorf("POST %s: %d %s", url, resp.StatusCode, errMsg)
	}

	return result, nil
}

// saveSwarmCredentials persists node_id and token to a file for restart survival
func saveSwarmCredentials(nodeID, token string) {
	data := fmt.Sprintf("%s\n%s\n", nodeID, token)
	os.WriteFile(".swarm_credentials", []byte(data), 0600)
}

// LoadSwarmCredentials reads persisted credentials
func LoadSwarmCredentials() (nodeID, token string) {
	data, err := os.ReadFile(".swarm_credentials")
	if err != nil {
		return "", ""
	}
	parts := bytes.Split(data, []byte("\n"))
	if len(parts) >= 2 {
		return string(parts[0]), string(parts[1])
	}
	return "", ""
}

func getOutboundIP() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "127.0.0.1"
	}
	return hostname
}
