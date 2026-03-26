package handler

import (
	"encoding/json"
	"log"

	pheromone "starclaw.net/pheromone/sdk"
	"starclaw.net/overlord/api/internal/ws"
)

// Overlord WebSocket event types for pheromone events
const (
	EventDeployUpdate   = "deploy_update"
	EventInstanceUpdate = "instance_update"
	EventServiceUpdate  = "service_update"
)

// SubscribePheromoneEvents subscribes Overlord to ESB events and forwards
// them to connected WebSocket clients for real-time dashboard updates.
func SubscribePheromoneEvents(ph *pheromone.Client) {
	hub := ws.GetHub()

	// Subscribe to all deploy events (nydus.deploy.*)
	if err := ph.Subscribe("nydus.deploy.>", func(subject string, data []byte) {
		var evt pheromone.DeployEvent
		_ = json.Unmarshal(data, &evt)
		log.Printf("[overlord] deploy event: %s → %s (%s)", evt.Service, evt.Status, subject)
		hub.Broadcast(EventDeployUpdate, map[string]interface{}{
			"subject": subject,
			"service": evt.Service,
			"status":  evt.Status,
			"duration": evt.Duration,
			"error":   evt.Error,
		})
	}); err != nil {
		log.Printf("[overlord] subscribe deploy events failed: %v", err)
	}

	// Subscribe to Hive instance events (hive.instance.*)
	if err := ph.Subscribe("hive.instance.>", func(subject string, data []byte) {
		var evt pheromone.InstanceEvent
		_ = json.Unmarshal(data, &evt)
		log.Printf("[overlord] instance event: %s → %s", evt.InstanceID, evt.Status)
		hub.Broadcast(EventInstanceUpdate, map[string]interface{}{
			"subject":     subject,
			"instance_id": evt.InstanceID,
			"user_id":     evt.UserID,
			"domain":      evt.Domain,
			"status":      evt.Status,
		})
	}); err != nil {
		log.Printf("[overlord] subscribe instance events failed: %v", err)
	}

	// Subscribe to all service registry events (announcements)
	if err := ph.Subscribe("registry.>", func(subject string, data []byte) {
		hub.Broadcast(EventServiceUpdate, map[string]interface{}{
			"subject": subject,
			"raw":     json.RawMessage(data),
		})
	}); err != nil {
		log.Printf("[overlord] subscribe registry events failed: %v", err)
	}

	log.Printf("[overlord] pheromone event subscriptions active: deploy, instance, registry")
}
