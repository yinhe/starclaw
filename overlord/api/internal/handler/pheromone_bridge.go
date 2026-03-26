package handler

import (
	"encoding/json"
	"log"

	"starclaw.net/overlord/api/internal/ws"
	pheromone "starclaw.net/pheromone/sdk"
)

// Overlord WebSocket event types for pheromone events
const (
	EventDeployUpdate   = "deploy_update"
	EventInstanceUpdate = "instance_update"
	EventServiceUpdate  = "service_update"
	EventUserUpdate     = "user_update"
	EventPaymentUpdate  = "payment_update"
	EventUsageAlert     = "usage_alert"
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
			"subject":  subject,
			"service":  evt.Service,
			"status":   evt.Status,
			"duration": evt.Duration,
			"error":    evt.Error,
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

	// Subscribe to Queen user events (queen.user.*)
	if err := ph.Subscribe("queen.user.>", func(subject string, data []byte) {
		var evt pheromone.UserEvent
		_ = json.Unmarshal(data, &evt)
		log.Printf("[overlord] user event: %s → %s (%s)", evt.UserID, evt.Action, subject)
		hub.Broadcast(EventUserUpdate, map[string]interface{}{
			"subject":  subject,
			"user_id":  evt.UserID,
			"username": evt.Username,
			"action":   evt.Action,
			"plan":     evt.Plan,
		})
	}); err != nil {
		log.Printf("[overlord] subscribe queen user events failed: %v", err)
	}

	// Subscribe to Queen billing events (queen.billing.*)
	if err := ph.Subscribe("queen.billing.>", func(subject string, data []byte) {
		var evt pheromone.PaymentEvent
		_ = json.Unmarshal(data, &evt)
		log.Printf("[overlord] payment event: user=%s amount=%d order=%s", evt.UserID, evt.Amount, evt.OrderNo)
		hub.Broadcast(EventPaymentUpdate, map[string]interface{}{
			"subject":  subject,
			"user_id":  evt.UserID,
			"amount":   evt.Amount,
			"order_no": evt.OrderNo,
		})
	}); err != nil {
		log.Printf("[overlord] subscribe queen billing events failed: %v", err)
	}

	// Subscribe to Synapse usage alerts (synapse.usage.*)
	if err := ph.Subscribe("synapse.usage.>", func(subject string, data []byte) {
		var evt pheromone.UsageAlertEvent
		_ = json.Unmarshal(data, &evt)
		log.Printf("[overlord] usage alert: user=%s model=%s cost=%.2f", evt.UserID, evt.Model, evt.Cost)
		hub.Broadcast(EventUsageAlert, map[string]interface{}{
			"subject":   subject,
			"user_id":   evt.UserID,
			"model":     evt.Model,
			"cost":      evt.Cost,
			"threshold": evt.Threshold,
			"message":   evt.Message,
		})
	}); err != nil {
		log.Printf("[overlord] subscribe synapse usage events failed: %v", err)
	}

	log.Printf("[overlord] pheromone event subscriptions active: deploy, instance, registry, user, payment, usage")
}
