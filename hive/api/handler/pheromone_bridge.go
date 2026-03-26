package handler

import (
	"log"
	"sync"

	pheromone "starclaw.net/pheromone/sdk"
)

// ph holds the Pheromone client singleton for use by handlers.
var (
	ph   *pheromone.Client
	phMu sync.RWMutex
)

// SetPheromone stores the Pheromone client for handler use.
func SetPheromone(client *pheromone.Client) {
	phMu.Lock()
	ph = client
	phMu.Unlock()
}

// PublishInstanceEvent publishes a Hive instance lifecycle event.
func PublishInstanceEvent(subject string, evt pheromone.InstanceEvent) {
	phMu.RLock()
	c := ph
	phMu.RUnlock()
	if c == nil {
		return
	}
	if err := c.Publish(subject, evt); err != nil {
		log.Printf("[hive] pheromone publish %s failed: %v", subject, err)
	}
}

// RegisterPheromoneRPC registers Hive's RPC handlers.
func RegisterPheromoneRPC(client *pheromone.Client) {
	if err := client.HandleRPC(pheromone.RPCListInstances, handleListInstances); err != nil {
		log.Printf("[hive] pheromone RPC register %s failed: %v", pheromone.RPCListInstances, err)
	}
	log.Printf("[hive] pheromone RPC handlers registered: %s", pheromone.RPCListInstances)
}

func handleListInstances(data []byte) (interface{}, error) {
	// Delegate to the existing instance listing logic
	// Returns a summary of active instances
	return map[string]string{"status": "not yet implemented"}, nil
}
