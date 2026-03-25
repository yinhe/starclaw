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

	http.HandleFunc("/health", withCORS(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status":         "ok",
			"service":        "pheromone-api",
			"nats_url":       natsURL,
			"nats_connected": nc.IsConnected(),
		})
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
