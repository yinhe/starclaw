package sdk

import (
	"encoding/json"
	"fmt"

	"github.com/nats-io/nats.go"
)

// Publish sends an event to pheromone.events.{subject}.
func (c *Client) Publish(subject string, data interface{}) error {
	full := "pheromone.events." + subject
	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("pheromone: marshal event: %w", err)
	}
	return c.nc.Publish(full, payload)
}

// EventHandler is called when an event is received.
type EventHandler func(subject string, data []byte)

// Subscribe listens to pheromone.events.{pattern}.
// Pattern supports NATS wildcards: * (single token) and > (tail match).
func (c *Client) Subscribe(pattern string, handler EventHandler) error {
	full := "pheromone.events." + pattern
	sub, err := c.nc.Subscribe(full, func(msg *nats.Msg) {
		handler(msg.Subject, msg.Data)
	})
	if err != nil {
		return fmt.Errorf("pheromone: subscribe %s: %w", full, err)
	}
	c.addSub(sub)
	return nil
}

// QueueSubscribe works like Subscribe but uses a queue group for load balancing.
func (c *Client) QueueSubscribe(pattern string, queue string, handler EventHandler) error {
	full := "pheromone.events." + pattern
	sub, err := c.nc.QueueSubscribe(full, queue, func(msg *nats.Msg) {
		handler(msg.Subject, msg.Data)
	})
	if err != nil {
		return fmt.Errorf("pheromone: queue subscribe %s: %w", full, err)
	}
	c.addSub(sub)
	return nil
}
