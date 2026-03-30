package trading

import (
	"log"
	"net/http"
	"sync"
	"time"
)

// Heartbeat monitors Master connectivity and triggers autonomous mode switch.
type Heartbeat struct {
	state       *NodeState
	url         string
	interval    time.Duration
	timeout     time.Duration
	client      *http.Client
	failCount   int
	maxFails    int
	stopCh      chan struct{}
	stoppedOnce sync.Once
}

// NewHeartbeat creates a heartbeat monitor for the given node state.
func NewHeartbeat(state *NodeState, cfg MasterConfig) *Heartbeat {
	interval := time.Duration(cfg.HeartbeatInterval) * time.Second
	if interval < time.Second {
		interval = 10 * time.Second
	}
	timeout := time.Duration(cfg.HeartbeatTimeout) * time.Second
	if timeout < interval {
		timeout = 3 * interval
	}
	maxFails := int(timeout / interval)
	if maxFails < 1 {
		maxFails = 3
	}
	return &Heartbeat{
		state:    state,
		url:      cfg.HeartbeatURL,
		interval: interval,
		timeout:  timeout,
		maxFails: maxFails,
		client:   &http.Client{Timeout: 5 * time.Second},
		stopCh:   make(chan struct{}),
	}
}

// Start begins the heartbeat loop in a background goroutine.
func (h *Heartbeat) Start() {
	if h.url == "" {
		log.Printf("[trading] No master heartbeat URL configured, heartbeat disabled")
		return
	}
	go h.loop()
	log.Printf("[trading] Heartbeat started: url=%s interval=%s timeout=%s", h.url, h.interval, h.timeout)
}

// Stop terminates the heartbeat loop.
func (h *Heartbeat) Stop() {
	h.stoppedOnce.Do(func() {
		close(h.stopCh)
	})
}

func (h *Heartbeat) loop() {
	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	for {
		select {
		case <-h.stopCh:
			return
		case <-ticker.C:
			h.ping()
		}
	}
}

func (h *Heartbeat) ping() {
	resp, err := h.client.Get(h.url)
	if err != nil {
		h.failCount++
		if h.failCount >= h.maxFails {
			h.state.SetMasterOnline(false)
		}
		if h.failCount == h.maxFails {
			log.Printf("[trading] Master heartbeat failed %d times, switching to autonomous", h.failCount)
		}
		return
	}
	resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		if h.failCount >= h.maxFails {
			log.Printf("[trading] Master heartbeat recovered after %d failures", h.failCount)
		}
		h.failCount = 0
		h.state.SetMasterOnline(true)
	} else {
		h.failCount++
		if h.failCount >= h.maxFails {
			h.state.SetMasterOnline(false)
		}
	}
}
