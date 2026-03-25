package aggregator

import (
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
)

const DeploySubjectPattern = "pheromone.events.deploy.*"

// PheromoneClient subscribes to Pheromone (NATS) events.
type PheromoneClient struct {
	URL  string
	conn *nats.Conn
}

func NewPheromoneClient(url string) (*PheromoneClient, error) {
	if url == "" {
		return nil, nil
	}

	conn, err := nats.Connect(url,
		nats.Name("forge-api"),
		nats.Timeout(5*time.Second),
		nats.ReconnectWait(2*time.Second),
		nats.MaxReconnects(-1),
	)
	if err != nil {
		return nil, fmt.Errorf("connect pheromone nats: %w", err)
	}

	return &PheromoneClient{URL: url, conn: conn}, nil
}

func (c *PheromoneClient) SubscribeDeployEvents(handler func(subject string, payload []byte)) (*nats.Subscription, error) {
	if c == nil || c.conn == nil {
		return nil, fmt.Errorf("pheromone client not initialized")
	}

	sub, err := c.conn.Subscribe(DeploySubjectPattern, func(msg *nats.Msg) {
		handler(msg.Subject, msg.Data)
	})
	if err != nil {
		return nil, fmt.Errorf("subscribe deploy events: %w", err)
	}

	return sub, nil
}

func (c *PheromoneClient) Close() {
	if c != nil && c.conn != nil {
		c.conn.Drain()
		c.conn.Close()
	}
}
