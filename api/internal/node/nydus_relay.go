package node

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

// RelayClient forwards traffic through a public relay node when hole-punching fails.
// Protocol: HTTP-based relay (simple, works through any firewall).
type RelayClient struct {
	relayURLs []string // relay server base URLs, e.g. ["https://starclaw.me"]
	identity  *Identity
	httpC     *http.Client
	mu        sync.RWMutex
	active    string // currently active relay URL
}

// NewRelayClient creates a relay client with fallback relay servers.
func NewRelayClient(identity *Identity, relayURLs ...string) *RelayClient {
	return &RelayClient{
		identity:  identity,
		relayURLs: relayURLs,
		httpC:     &http.Client{Timeout: 30 * time.Second},
	}
}

// RelayMessage is a message forwarded through a relay node.
type RelayMessage struct {
	FromNodeID string          `json:"from_node_id"`
	ToNodeID   string          `json:"to_node_id"`
	Type       string          `json:"type"`       // "punch_request", "punch_response", "data"
	Payload    json.RawMessage `json:"payload"`
	Timestamp  int64           `json:"timestamp"`
	Signature  string          `json:"signature"`  // Ed25519 signature of payload
}

// SendPunchRequest sends a hole-punch coordination request via relay.
// This is the signaling channel: tells the remote peer our STUN result
// so they know where to send UDP packets.
func (rc *RelayClient) SendPunchRequest(ctx context.Context, req *PunchRequest) error {
	payload, _ := json.Marshal(req)

	msg := &RelayMessage{
		FromNodeID: rc.identity.NodeID,
		ToNodeID:   req.ToNodeID,
		Type:       "punch_request",
		Payload:    payload,
		Timestamp:  time.Now().Unix(),
	}
	msg.Signature = rc.sign(payload)

	return rc.send(ctx, msg)
}

// Forward sends data to a remote node through the relay (fallback when punch fails).
func (rc *RelayClient) Forward(ctx context.Context, toNodeID string, data []byte) ([]byte, error) {
	msg := &RelayMessage{
		FromNodeID: rc.identity.NodeID,
		ToNodeID:   toNodeID,
		Type:       "data",
		Payload:    data,
		Timestamp:  time.Now().Unix(),
	}
	msg.Signature = rc.sign(data)

	return rc.sendAndReceive(ctx, msg)
}

// send dispatches a relay message to the first available relay server.
func (rc *RelayClient) send(ctx context.Context, msg *RelayMessage) error {
	data, _ := json.Marshal(msg)

	for _, relayURL := range rc.relayURLs {
		url := relayURL + "/v1/peer/relay"
		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Node-ID", rc.identity.NodeID)

		resp, err := rc.httpC.Do(req)
		if err != nil {
			log.Printf("[nydus/relay] relay %s unreachable: %v", relayURL, err)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			rc.mu.Lock()
			rc.active = relayURL
			rc.mu.Unlock()
			return nil
		}
		log.Printf("[nydus/relay] relay %s returned %d", relayURL, resp.StatusCode)
	}

	return fmt.Errorf("all relay servers unreachable")
}

// sendAndReceive sends a relay message and reads the response body.
func (rc *RelayClient) sendAndReceive(ctx context.Context, msg *RelayMessage) ([]byte, error) {
	data, _ := json.Marshal(msg)

	for _, relayURL := range rc.relayURLs {
		url := relayURL + "/v1/peer/relay"
		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Node-ID", rc.identity.NodeID)

		resp, err := rc.httpC.Do(req)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return nil, err
			}
			rc.mu.Lock()
			rc.active = relayURL
			rc.mu.Unlock()
			return body, nil
		}
	}

	return nil, fmt.Errorf("relay forward failed")
}

// sign produces a hex-encoded Ed25519 signature of data.
func (rc *RelayClient) sign(data []byte) string {
	sig := rc.identity.Sign(data)
	return fmt.Sprintf("%x", sig)
}

// ActiveRelay returns the currently active relay URL (empty if none).
func (rc *RelayClient) ActiveRelay() string {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.active
}

// RelayHandler is the server-side handler for relay requests.
// Any Claw with a public IP can serve as a relay.
type RelayHandler struct {
	// pending stores messages waiting for the target node to pick up
	pending   map[string][]*RelayMessage // toNodeID -> messages
	mu        sync.Mutex
	maxPerNode int
}

// NewRelayHandler creates a relay handler for serving relay requests.
func NewRelayHandler() *RelayHandler {
	rh := &RelayHandler{
		pending:    make(map[string][]*RelayMessage),
		maxPerNode: 100,
	}
	// Start cleanup goroutine
	go rh.cleanup()
	return rh
}

// Enqueue stores a relay message for the target node to pick up.
func (rh *RelayHandler) Enqueue(msg *RelayMessage) error {
	rh.mu.Lock()
	defer rh.mu.Unlock()

	msgs := rh.pending[msg.ToNodeID]
	if len(msgs) >= rh.maxPerNode {
		// Drop oldest
		rh.pending[msg.ToNodeID] = msgs[1:]
	}
	rh.pending[msg.ToNodeID] = append(rh.pending[msg.ToNodeID], msg)
	return nil
}

// Dequeue retrieves and removes pending messages for a node.
func (rh *RelayHandler) Dequeue(nodeID string) []*RelayMessage {
	rh.mu.Lock()
	defer rh.mu.Unlock()

	msgs := rh.pending[nodeID]
	delete(rh.pending, nodeID)
	return msgs
}

// cleanup removes stale messages older than 60 seconds.
func (rh *RelayHandler) cleanup() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		rh.mu.Lock()
		now := time.Now().Unix()
		for nodeID, msgs := range rh.pending {
			var fresh []*RelayMessage
			for _, m := range msgs {
				if now-m.Timestamp < 60 {
					fresh = append(fresh, m)
				}
			}
			if len(fresh) == 0 {
				delete(rh.pending, nodeID)
			} else {
				rh.pending[nodeID] = fresh
			}
		}
		rh.mu.Unlock()
	}
}

// Stats returns relay handler statistics.
func (rh *RelayHandler) Stats() map[string]interface{} {
	rh.mu.Lock()
	defer rh.mu.Unlock()

	totalMsgs := 0
	for _, msgs := range rh.pending {
		totalMsgs += len(msgs)
	}

	return map[string]interface{}{
		"pending_nodes":    len(rh.pending),
		"pending_messages": totalMsgs,
	}
}
