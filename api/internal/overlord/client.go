package overlord

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
)

// Client handles registration and heartbeat with an Overlord node
type Client struct {
	cfg    config.OverlordConfig
	nodeID string
	token  string
	clawID string
	mu     sync.RWMutex
	stopCh chan struct{}
	httpC  *http.Client
}

// NewClient creates an overlord client from config
func NewClient(cfg config.OverlordConfig) *Client {
	return &Client{
		cfg:    cfg,
		stopCh: make(chan struct{}),
		httpC:  &http.Client{Timeout: 10 * time.Second},
	}
}

// Start registers with Overlord and begins heartbeat loop
func (c *Client) Start() {
	if !c.cfg.Enabled || c.cfg.OverlordURL == "" {
		log.Println("[overlord] monitoring disabled or overlord_url not set")
		return
	}

	log.Printf("[overlord] connecting to Overlord at %s", c.cfg.OverlordURL)

	// Try to load persisted credentials
	nid, tok := LoadCredentials()
	if nid != "" && tok != "" {
		c.mu.Lock()
		c.nodeID = nid
		c.token = tok
		c.mu.Unlock()
		log.Printf("[overlord] loaded persisted node %s", nid)
	}

	if c.nodeID == "" {
		if err := c.register(); err != nil {
			log.Printf("[overlord] registration failed: %v (will retry)", err)
		} else {
			log.Printf("[overlord] registered as node %s", c.nodeID)
		}
	}

	interval := time.Duration(c.cfg.HeartbeatInterval) * time.Second
	if interval < 10*time.Second {
		interval = 30 * time.Second
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if c.nodeID == "" {
					if err := c.register(); err != nil {
						log.Printf("[overlord] re-registration failed: %v", err)
						continue
					}
					log.Printf("[overlord] registered as node %s", c.nodeID)
				}
				if err := c.heartbeat(); err != nil {
					log.Printf("[overlord] heartbeat failed: %v", err)
				}
			case <-c.stopCh:
				return
			}
		}
	}()
}

// Stop gracefully stops the heartbeat loop
func (c *Client) Stop() {
	select {
	case <-c.stopCh:
	default:
		close(c.stopCh)
	}
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

// OverlordURL returns the configured Overlord URL
func (c *Client) OverlordURL() string {
	return c.cfg.OverlordURL
}

// Connected returns whether this client is registered with Overlord
func (c *Client) Connected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.nodeID != "" && c.cfg.Enabled
}

// Resolve queries Overlord's brood registry for a claw: address
func (c *Client) Resolve(clawID string) (address string, found bool) {
	if !c.Connected() || c.cfg.OverlordURL == "" {
		return "", false
	}
	url := fmt.Sprintf("%s/brood/resolve?claw_id=%s", c.cfg.OverlordURL, clawID)
	resp, err := c.httpC.Get(url)
	if err != nil {
		log.Printf("[overlord] resolve via Overlord failed: %v", err)
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

	body := map[string]interface{}{
		"name":    name,
		"role":    "claw",
		"version": molt.Version,
		"address": fmt.Sprintf("%s:8080", getHostname()),
		"region":  c.cfg.Region,
		"claw_id": cid,
	}

	resp, err := c.post("/overlord/register", body)
	if err != nil {
		return err
	}

	c.mu.Lock()
	if nid, ok := resp["node_id"].(string); ok {
		c.nodeID = nid
	}
	if tok, ok := resp["token"].(string); ok {
		c.token = tok
	}
	c.mu.Unlock()

	SaveCredentials(c.nodeID, c.token)
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
	memMB := memStats.Alloc / 1024 / 1024

	c.mu.RLock()
	cid := c.clawID
	c.mu.RUnlock()

	body := map[string]interface{}{
		"node_id":       nid,
		"token":         tok,
		"version":       molt.Version,
		"claw_id":       cid,
		"cpu_percent":   0,
		"mem_used_mb":   memMB,
		"tasks_running": 0,
		"tasks_queued":  0,
		"go_routines":   runtime.NumGoroutine(),
	}

	_, err := c.post("/overlord/heartbeat", body)
	return err
}

func (c *Client) post(path string, body map[string]interface{}) (map[string]interface{}, error) {
	data, _ := json.Marshal(body)
	url := c.cfg.OverlordURL + path

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

// SaveCredentials persists node_id and token
func SaveCredentials(nodeID, token string) {
	data := fmt.Sprintf("%s\n%s\n", nodeID, token)
	os.WriteFile(".overlord_credentials", []byte(data), 0600)
}

// LoadCredentials reads persisted credentials
func LoadCredentials() (nodeID, token string) {
	data, err := os.ReadFile(".overlord_credentials")
	if err != nil {
		return "", ""
	}
	parts := bytes.Split(data, []byte("\n"))
	if len(parts) >= 2 {
		return string(parts[0]), string(parts[1])
	}
	return "", ""
}

func getHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "127.0.0.1"
	}
	return hostname
}
