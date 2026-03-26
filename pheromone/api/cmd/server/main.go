package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
)

type Event struct {
	Subject   string          `json:"subject"`
	Payload   json.RawMessage `json:"payload"`
	Timestamp time.Time       `json:"timestamp"`
}

// ========== Service Registry ==========

type ServiceEntry struct {
	Name       string    `json:"name"`
	Version    string    `json:"version,omitempty"`
	Host       string    `json:"host,omitempty"`
	Port       int       `json:"port,omitempty"`
	Tags       []string  `json:"tags,omitempty"`
	PID        int       `json:"pid,omitempty"`
	Status     string    `json:"status"`
	Uptime     string    `json:"uptime,omitempty"`
	Goroutines int       `json:"goroutines,omitempty"`
	LastSeen   time.Time `json:"last_seen"`
	Registered time.Time `json:"registered"`
}

type ServiceRegistry struct {
	mu       sync.RWMutex
	services map[string]*ServiceEntry
	ttl      time.Duration
}

func NewServiceRegistry(ttl time.Duration) *ServiceRegistry {
	r := &ServiceRegistry{
		services: make(map[string]*ServiceEntry),
		ttl:      ttl,
	}
	go r.reaper()
	return r
}

func (r *ServiceRegistry) Update(name string, data json.RawMessage) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var info struct {
		Service struct {
			Name    string   `json:"name"`
			Version string   `json:"version"`
			Host    string   `json:"host"`
			Port    int      `json:"port"`
			Tags    []string `json:"tags"`
			PID     int      `json:"pid"`
		} `json:"service"`
		Uptime     string `json:"uptime"`
		Goroutines int    `json:"goroutines"`
		Status     string `json:"status"`
	}
	_ = json.Unmarshal(data, &info)

	svcName := info.Service.Name
	if svcName == "" {
		svcName = name
	}

	entry, exists := r.services[svcName]
	if !exists {
		entry = &ServiceEntry{
			Name:       svcName,
			Registered: time.Now().UTC(),
		}
		r.services[svcName] = entry
		log.Printf("[registry] new service: %s", svcName)
	}

	entry.LastSeen = time.Now().UTC()
	entry.Status = "online"
	if info.Service.Version != "" {
		entry.Version = info.Service.Version
	}
	if info.Service.Host != "" {
		entry.Host = info.Service.Host
	}
	if info.Service.Port > 0 {
		entry.Port = info.Service.Port
	}
	if info.Service.PID > 0 {
		entry.PID = info.Service.PID
	}
	if len(info.Service.Tags) > 0 {
		entry.Tags = info.Service.Tags
	}
	if info.Uptime != "" {
		entry.Uptime = info.Uptime
	}
	if info.Goroutines > 0 {
		entry.Goroutines = info.Goroutines
	}
	if info.Status == "offline" {
		entry.Status = "offline"
	}
}

func (r *ServiceRegistry) List() []ServiceEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]ServiceEntry, 0, len(r.services))
	for _, e := range r.services {
		out = append(out, *e)
	}
	return out
}

func (r *ServiceRegistry) Get(name string) *ServiceEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if e, ok := r.services[name]; ok {
		copy := *e
		return &copy
	}
	return nil
}

// reaper marks services as stale if no heartbeat within TTL
func (r *ServiceRegistry) reaper() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		r.mu.Lock()
		now := time.Now()
		for _, e := range r.services {
			if e.Status == "online" && now.Sub(e.LastSeen) > r.ttl {
				e.Status = "stale"
				log.Printf("[registry] %s marked stale (no heartbeat for %s)", e.Name, r.ttl)
			}
		}
		r.mu.Unlock()
	}
}

type EventHub struct {
	mu     sync.RWMutex
	recent []Event
	subs   map[chan Event]struct{}
	max    int
}

func NewEventHub(max int) *EventHub {
	return &EventHub{
		recent: make([]Event, 0, max),
		subs:   make(map[chan Event]struct{}),
		max:    max,
	}
}

func (h *EventHub) Add(evt Event) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.recent = append(h.recent, evt)
	if len(h.recent) > h.max {
		h.recent = h.recent[len(h.recent)-h.max:]
	}

	for ch := range h.subs {
		select {
		case ch <- evt:
		default:
		}
	}
}

func (h *EventHub) Recent(limit int) []Event {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if limit <= 0 || limit > len(h.recent) {
		limit = len(h.recent)
	}
	start := len(h.recent) - limit
	copyOf := make([]Event, limit)
	copy(copyOf, h.recent[start:])
	return copyOf
}

func (h *EventHub) Subscribe() (chan Event, func()) {
	ch := make(chan Event, 32)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	h.mu.Unlock()

	cancel := func() {
		h.mu.Lock()
		delete(h.subs, ch)
		close(ch)
		h.mu.Unlock()
	}
	return ch, cancel
}

func main() {
	port := envOr("PHEROMONE_PORT", "8100")
	natsURL := envOr("PHEROMONE_NATS_URL", "nats://pheromone-nats:4222")
	hub := NewEventHub(300)

	registry := NewServiceRegistry(90 * time.Second)

	nc, err := nats.Connect(natsURL,
		nats.Name("pheromone-api"),
		nats.Timeout(5*time.Second),
		nats.ReconnectWait(2*time.Second),
		nats.MaxReconnects(-1),
	)
	if err != nil {
		log.Fatalf("connect nats: %v", err)
	}
	defer nc.Drain()

	_, err = nc.Subscribe("pheromone.events.>", func(msg *nats.Msg) {
		payload := normalizePayload(msg.Data)
		hub.Add(Event{Subject: msg.Subject, Payload: payload, Timestamp: time.Now().UTC()})
	})
	if err != nil {
		log.Fatalf("subscribe events: %v", err)
	}

	// Service registry: heartbeats
	_, err = nc.Subscribe("pheromone.heartbeat.>", func(msg *nats.Msg) {
		parts := strings.SplitN(msg.Subject, ".", 3)
		name := "unknown"
		if len(parts) >= 3 {
			name = parts[2]
		}
		registry.Update(name, msg.Data)
	})
	if err != nil {
		log.Fatalf("subscribe heartbeat: %v", err)
	}

	// Service registry: announcements (online/offline)
	_, err = nc.Subscribe("pheromone.registry.>", func(msg *nats.Msg) {
		var ann struct {
			Service struct{ Name string } `json:"service"`
		}
		_ = json.Unmarshal(msg.Data, &ann)
		name := ann.Service.Name
		if name == "" {
			name = "unknown"
		}
		registry.Update(name, msg.Data)
		hub.Add(Event{Subject: msg.Subject, Payload: msg.Data, Timestamp: time.Now().UTC()})
	})
	if err != nil {
		log.Fatalf("subscribe registry: %v", err)
	}

	http.HandleFunc("/health", withCORS(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status":         "ok",
			"service":        "pheromone-api",
			"nats_url":       natsURL,
			"nats_connected": nc.IsConnected(),
		})
	}))

	// Service registry API
	http.HandleFunc("/api/services", withCORS(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]interface{}{"services": registry.List()})
	}))

	http.HandleFunc("/api/services/", withCORS(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/api/services/")
		if name == "" {
			writeJSON(w, http.StatusOK, map[string]interface{}{"services": registry.List()})
			return
		}
		entry := registry.Get(name)
		if entry == nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "service not found"})
			return
		}
		writeJSON(w, http.StatusOK, entry)
	}))

	http.HandleFunc("/api/events/recent", withCORS(func(w http.ResponseWriter, r *http.Request) {
		limit := 50
		if q := r.URL.Query().Get("limit"); q != "" {
			if n, err := strconv.Atoi(q); err == nil {
				limit = n
			}
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"events": hub.Recent(limit)})
	}))

	http.HandleFunc("/api/events", withCORS(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}

		var req struct {
			Subject string          `json:"subject"`
			Payload json.RawMessage `json:"payload"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		req.Subject = strings.TrimSpace(req.Subject)
		if req.Subject == "" || !strings.HasPrefix(req.Subject, "pheromone.events.") {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "subject must start with pheromone.events."})
			return
		}
		if len(req.Payload) == 0 {
			req.Payload = json.RawMessage(`{}`)
		}

		if err := nc.Publish(req.Subject, req.Payload); err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "publish failed"})
			return
		}
		if err := nc.FlushTimeout(2 * time.Second); err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "publish timeout"})
			return
		}

		writeJSON(w, http.StatusAccepted, map[string]string{"status": "published"})
	}))

	http.HandleFunc("/api/events/stream", withCORS(func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "stream unsupported"})
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		events, cancel := hub.Subscribe()
		defer cancel()
		ctx := r.Context()

		fmt.Fprintf(w, "event: hello\ndata: {\"status\":\"connected\"}\n\n")
		flusher.Flush()

		for {
			select {
			case <-ctx.Done():
				return
			case evt := <-events:
				b, _ := json.Marshal(evt)
				fmt.Fprintf(w, "event: event\ndata: %s\n\n", b)
				flusher.Flush()
			}
		}
	}))

	addr := ":" + port
	log.Printf("pheromone-api listening on %s (nats=%s)", addr, natsURL)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func withCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next(w, r)
	}
}

func writeJSON(w http.ResponseWriter, code int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(data)
}

func normalizePayload(in []byte) json.RawMessage {
	trimmed := strings.TrimSpace(string(in))
	if trimmed == "" {
		return json.RawMessage(`{}`)
	}
	var any interface{}
	if err := json.Unmarshal([]byte(trimmed), &any); err == nil {
		return json.RawMessage(trimmed)
	}
	wrapped, _ := json.Marshal(map[string]string{"raw": trimmed})
	return json.RawMessage(wrapped)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); strings.TrimSpace(v) != "" {
		return v
	}
	return fallback
}
