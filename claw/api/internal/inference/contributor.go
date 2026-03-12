package inference

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/yinhe/starclaw/internal/node"
	"github.com/yinhe/starclaw/internal/provider"
)

// ContributorConfig controls compute contribution behavior.
type ContributorConfig struct {
	Enabled     bool   `json:"enabled" mapstructure:"enabled"`           // opt-in to contribute compute
	OllamaURL  string `json:"ollama_url" mapstructure:"ollama_url"`     // e.g. http://localhost:11434
	MaxJobs    int    `json:"max_jobs" mapstructure:"max_jobs"`         // max concurrent inference jobs
	ExternalAddr string `json:"external_addr" mapstructure:"external_addr"` // address other nodes reach us at
}

// ContributorService auto-detects local models and registers with swarm router nodes.
type ContributorService struct {
	cfg       ContributorConfig
	identity  *node.Identity
	providers *provider.Registry
	peers     func() []string // returns addresses of known peers (potential routers)

	mu         sync.RWMutex
	models     []string
	registered map[string]bool // router address -> registered
	stopCh     chan struct{}
	httpC      *http.Client
}

// NewContributorService creates a new compute contribution service.
func NewContributorService(cfg ContributorConfig, identity *node.Identity, providers *provider.Registry, peersFn func() []string) *ContributorService {
	return &ContributorService{
		cfg:        cfg,
		identity:   identity,
		providers:  providers,
		peers:      peersFn,
		registered: make(map[string]bool),
		stopCh:     make(chan struct{}),
		httpC:      &http.Client{Timeout: 10 * time.Second},
	}
}

// Start begins the contributor lifecycle: detect models → register → heartbeat loop.
func (s *ContributorService) Start() {
	if !s.cfg.Enabled {
		log.Println("[contributor] compute contribution disabled")
		return
	}

	// Detect local models
	models := s.detectModels()
	if len(models) == 0 {
		log.Println("[contributor] no local models detected, compute contribution inactive")
		return
	}

	s.mu.Lock()
	s.models = models
	s.mu.Unlock()

	log.Printf("[contributor] detected %d local models: %v", len(models), models)
	log.Printf("[contributor] starting compute contribution (max_jobs=%d)", s.cfg.MaxJobs)

	go s.registrationLoop()
}

// Stop gracefully stops the contributor service.
func (s *ContributorService) Stop() {
	if s.cfg.Enabled {
		s.unregisterAll()
	}
	close(s.stopCh)
}

// detectModels queries the local Ollama instance for available models.
func (s *ContributorService) detectModels() []string {
	if s.providers == nil {
		return nil
	}
	if p, ok := s.providers.Get("ollama"); ok {
		return p.Models()
	}
	return nil
}

// registrationLoop periodically registers with known peers and sends heartbeats.
func (s *ContributorService) registrationLoop() {
	// Initial registration attempt
	s.registerWithPeers()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Re-detect models (Ollama models may change)
			models := s.detectModels()
			s.mu.Lock()
			s.models = models
			s.mu.Unlock()

			if len(models) == 0 {
				continue
			}

			s.registerWithPeers()

		case <-s.stopCh:
			return
		}
	}
}

// registerWithPeers attempts to register/heartbeat with all known peer router nodes.
func (s *ContributorService) registerWithPeers() {
	peers := s.peers()
	if len(peers) == 0 {
		return
	}

	s.mu.RLock()
	models := make([]string, len(s.models))
	copy(models, s.models)
	s.mu.RUnlock()

	if len(models) == 0 {
		return
	}

	for _, addr := range peers {
		s.mu.RLock()
		alreadyRegistered := s.registered[addr]
		s.mu.RUnlock()

		if alreadyRegistered {
			// Send heartbeat instead
			go s.sendHeartbeat(addr)
		} else {
			// Register
			go s.registerWith(addr, models)
		}
	}
}

// registerWith sends a registration request to a specific router node.
func (s *ContributorService) registerWith(routerAddr string, models []string) {
	payload := map[string]interface{}{
		"address":  s.cfg.ExternalAddr,
		"models":   models,
		"max_jobs": s.cfg.MaxJobs,
	}
	data, _ := json.Marshal(payload)

	url := routerAddr + "/v1/inference/register"
	req, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	s.signHTTPRequest(req, data)

	resp, err := s.httpC.Do(req)
	if err != nil {
		return // peer might not be a router or is offline
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		s.mu.Lock()
		s.registered[routerAddr] = true
		s.mu.Unlock()
		log.Printf("[contributor] registered with router %s (%d models)", routerAddr, len(models))
	}
}

// sendHeartbeat sends a heartbeat to a registered router node.
func (s *ContributorService) sendHeartbeat(routerAddr string) {
	payload := map[string]interface{}{
		"active_jobs": 0, // TODO: track actual active jobs
		"latency_ms":  0,
	}
	data, _ := json.Marshal(payload)

	url := routerAddr + "/v1/inference/heartbeat"
	req, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	s.signHTTPRequest(req, data)

	resp, err := s.httpC.Do(req)
	if err != nil {
		// Router unreachable, mark as unregistered for re-registration
		s.mu.Lock()
		delete(s.registered, routerAddr)
		s.mu.Unlock()
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// Router doesn't know us, re-register next cycle
		s.mu.Lock()
		delete(s.registered, routerAddr)
		s.mu.Unlock()
	}
}

// unregisterAll sends unregister to all known routers on shutdown.
func (s *ContributorService) unregisterAll() {
	s.mu.RLock()
	routers := make([]string, 0, len(s.registered))
	for addr := range s.registered {
		routers = append(routers, addr)
	}
	s.mu.RUnlock()

	for _, addr := range routers {
		url := addr + "/v1/inference/unregister"
		req, err := http.NewRequest("POST", url, bytes.NewReader([]byte("{}")))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		s.signHTTPRequest(req, []byte("{}"))

		resp, err := s.httpC.Do(req)
		if err == nil {
			resp.Body.Close()
		}
	}
	log.Printf("[contributor] unregistered from %d routers", len(routers))
}

// signHTTPRequest signs an outgoing HTTP request with the node's Ed25519 key.
func (s *ContributorService) signHTTPRequest(req *http.Request, body []byte) {
	ts := fmt.Sprintf("%d", time.Now().Unix())
	bodyHash := sha256.Sum256(body)
	message := fmt.Sprintf("%s\n%s\n%s\n%s",
		req.Method,
		req.URL.Path,
		ts,
		hex.EncodeToString(bodyHash[:]),
	)

	sig := s.identity.Sign([]byte(message))

	req.Header.Set("X-Node-ID", s.identity.NodeID)
	req.Header.Set("X-Node-PubKey", s.identity.PublicKeyHex())
	req.Header.Set("X-Node-Signature", hex.EncodeToString(sig))
	req.Header.Set("X-Node-Timestamp", ts)
}

// Models returns currently detected local models.
func (s *ContributorService) Models() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]string, len(s.models))
	copy(out, s.models)
	return out
}

// RegisteredRouters returns the number of routers we're registered with.
func (s *ContributorService) RegisteredRouters() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.registered)
}
