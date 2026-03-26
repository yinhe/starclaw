package sdk

import (
	"encoding/json"
	"log"
	"runtime"
	"time"
)

// HeartbeatPayload is sent periodically to indicate liveness.
type HeartbeatPayload struct {
	Service   ServiceInfo `json:"service"`
	Uptime    string      `json:"uptime"`
	Goroutines int       `json:"goroutines"`
	Timestamp time.Time   `json:"ts"`
}

// StartHeartbeat sends periodic heartbeats on pheromone.heartbeat.{name}.
// Returns immediately; heartbeats stop when the client is closed.
func (c *Client) StartHeartbeat(interval time.Duration) {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	startedAt := time.Now()
	subject := "pheromone.heartbeat." + c.info.Name

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		// Send first heartbeat immediately
		c.sendHeartbeat(subject, startedAt)

		for {
			select {
			case <-c.stopCh:
				return
			case <-ticker.C:
				c.sendHeartbeat(subject, startedAt)
			}
		}
	}()

	log.Printf("[pheromone] %s heartbeat started (every %s)", c.info.Name, interval)
}

func (c *Client) sendHeartbeat(subject string, startedAt time.Time) {
	payload, _ := json.Marshal(HeartbeatPayload{
		Service:    c.info,
		Uptime:     time.Since(startedAt).Round(time.Second).String(),
		Goroutines: runtime.NumGoroutine(),
		Timestamp:  time.Now().UTC(),
	})
	if err := c.nc.Publish(subject, payload); err != nil {
		log.Printf("[pheromone] heartbeat publish error: %v", err)
	}
}
