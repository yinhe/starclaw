package swarm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
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

// CreditBalance holds cached star credit balance from Queen
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
	creditClient     *CreditClient // star credit operations client
}

// NewClient creates a swarm client from config
func NewClient(cfg config.SwarmConfig) *Client {
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

	log.Printf("[swarm] registering with Queen at %s", c.cfg.QueenURL)

	if err := c.register(); err != nil {
		log.Printf("[swarm] registration failed: %v (will retry in heartbeat loop)", err)
	} else {
		log.Printf("[swarm] registered as node %s", c.nodeID)
	}

	// Heartbeat loop with exponential backoff + jitter
	baseInterval := time.Duration(c.cfg.HeartbeatInterval) * time.Second
	if baseInterval < 10*time.Second {
		baseInterval = 60 * time.Second
	}

	go c.heartbeatLoop(baseInterval)
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

// CreditClient returns the star credit operations client (may be nil if not connected to Queen).
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

	return nil
}

// Credits returns the cached star credit balance (updated each heartbeat)
func (c *Client) Credits() *CreditBalance {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.credits
}

func (c *Client) post(path string, body map[string]interface{}) (map[string]interface{}, error) {
	data, _ := json.Marshal(body)
	url := c.cfg.QueenURL + path

	resp, err := c.httpC.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("POST %s: %w", path, err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if resp.StatusCode >= 400 {
		errMsg, _ := result["error"].(string)
		return nil, fmt.Errorf("POST %s: %d %s", path, resp.StatusCode, errMsg)
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
