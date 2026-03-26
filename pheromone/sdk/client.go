// Package sdk provides a lightweight ESB client for StarClaw microservices.
// Services import this package to register with Pheromone, send heartbeats,
// publish/subscribe events, and make RPC calls — all over NATS.
package sdk

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
)

// ServiceInfo describes a microservice instance.
type ServiceInfo struct {
	Name    string   `json:"name"`
	Version string   `json:"version,omitempty"`
	Host    string   `json:"host,omitempty"`
	Port    int      `json:"port,omitempty"`
	Tags    []string `json:"tags,omitempty"`
	PID     int      `json:"pid,omitempty"`
}

// Client is a Pheromone ESB client backed by NATS.
type Client struct {
	nc      *nats.Conn
	info    ServiceInfo
	subs    []*nats.Subscription
	stopCh  chan struct{}
	stopped sync.Once
}

// Option configures the client.
type Option func(*clientCfg)

type clientCfg struct {
	natsOpts []nats.Option
}

// WithNATSOpts passes extra nats.Option to the underlying connection.
func WithNATSOpts(opts ...nats.Option) Option {
	return func(c *clientCfg) { c.natsOpts = append(c.natsOpts, opts...) }
}

// New creates a Pheromone ESB client and connects to NATS.
func New(natsURL string, info ServiceInfo, opts ...Option) (*Client, error) {
	cfg := &clientCfg{}
	for _, o := range opts {
		o(cfg)
	}

	if info.PID == 0 {
		info.PID = os.Getpid()
	}
	if info.Host == "" {
		info.Host, _ = os.Hostname()
	}

	natsOpts := []nats.Option{
		nats.Name(info.Name),
		nats.Timeout(5 * time.Second),
		nats.ReconnectWait(2 * time.Second),
		nats.MaxReconnects(-1),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			log.Printf("[pheromone] %s disconnected: %v", info.Name, err)
		}),
		nats.ReconnectHandler(func(_ *nats.Conn) {
			log.Printf("[pheromone] %s reconnected", info.Name)
		}),
	}
	natsOpts = append(natsOpts, cfg.natsOpts...)

	nc, err := nats.Connect(natsURL, natsOpts...)
	if err != nil {
		return nil, fmt.Errorf("pheromone: connect %s: %w", natsURL, err)
	}

	c := &Client{
		nc:     nc,
		info:   info,
		stopCh: make(chan struct{}),
	}

	// Announce service online
	c.announce("online")

	return c, nil
}

// Info returns the service info.
func (c *Client) Info() ServiceInfo { return c.info }

// NATS returns the underlying NATS connection for advanced use.
func (c *Client) NATS() *nats.Conn { return c.nc }

// Close announces offline, drains NATS, and stops background goroutines.
func (c *Client) Close() {
	c.stopped.Do(func() {
		close(c.stopCh)
		c.announce("offline")
		for _, sub := range c.subs {
			_ = sub.Unsubscribe()
		}
		_ = c.nc.Drain()
	})
}

func (c *Client) announce(status string) {
	payload, _ := json.Marshal(map[string]interface{}{
		"service": c.info,
		"status":  status,
		"ts":      time.Now().UTC(),
	})
	_ = c.nc.Publish("pheromone.registry.announce", payload)
	_ = c.nc.Flush()
}

func (c *Client) addSub(sub *nats.Subscription) {
	c.subs = append(c.subs, sub)
}
