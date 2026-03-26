package handler

import (
	"encoding/json"
	"log"
	"sync"

	"gorm.io/gorm"

	"starclaw.net/hive/api/model"
	pheromone "starclaw.net/pheromone/sdk"
)

// ph holds the Pheromone client singleton for use by handlers.
var (
	ph   *pheromone.Client
	phMu sync.RWMutex
	phDB *gorm.DB
)

// SetPheromone stores the Pheromone client for handler use.
func SetPheromone(client *pheromone.Client) {
	phMu.Lock()
	ph = client
	phMu.Unlock()
}

// SetPheromoneDB stores the DB reference for RPC handlers.
func SetPheromoneDB(db *gorm.DB) {
	phMu.Lock()
	phDB = db
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

// --- Convenience wrappers (called from hive.go without importing SDK) ---

// NotifyInstanceCreated publishes instance.created event.
func NotifyInstanceCreated(instanceID, ownerID, slug, domain, plan string) {
	PublishInstanceEvent(pheromone.SubjectInstanceCreated, pheromone.InstanceEvent{
		InstanceID: instanceID,
		UserID:     ownerID,
		Domain:     domain,
		Status:     "created",
		Plan:       plan,
	})
}

// NotifyInstanceStarted publishes instance.started event.
func NotifyInstanceStarted(instanceID, slug string) {
	PublishInstanceEvent(pheromone.SubjectInstanceStarted, pheromone.InstanceEvent{
		InstanceID: instanceID,
		Status:     "started",
	})
}

// NotifyInstanceStopped publishes instance.stopped event.
func NotifyInstanceStopped(instanceID, slug string) {
	PublishInstanceEvent(pheromone.SubjectInstanceStopped, pheromone.InstanceEvent{
		InstanceID: instanceID,
		Status:     "stopped",
	})
}

// NotifyInstanceDeleted publishes instance.deleted event.
func NotifyInstanceDeleted(instanceID, slug string) {
	PublishInstanceEvent(pheromone.SubjectInstanceDeleted, pheromone.InstanceEvent{
		InstanceID: instanceID,
		Status:     "deleted",
	})
}

// NotifyInstanceError publishes instance.error event.
func NotifyInstanceError(instanceID, slug, errMsg string) {
	PublishInstanceEvent(pheromone.SubjectInstanceError, pheromone.InstanceEvent{
		InstanceID: instanceID,
		Status:     "error",
		Error:      errMsg,
	})
}

// RegisterPheromoneRPC registers Hive's RPC handlers.
func RegisterPheromoneRPC(client *pheromone.Client) {
	if err := client.HandleRPC(pheromone.RPCListInstances, handleListInstances); err != nil {
		log.Printf("[hive] pheromone RPC register %s failed: %v", pheromone.RPCListInstances, err)
	}
	log.Printf("[hive] pheromone RPC handlers registered: %s", pheromone.RPCListInstances)
}

type listInstancesRequest struct {
	Owner  string `json:"owner"`
	Status string `json:"status"`
}

type instanceSummary struct {
	ID     string  `json:"id"`
	Slug   string  `json:"slug"`
	Status string  `json:"status"`
	Owner  string  `json:"owner_id"`
	Mode   string  `json:"deploy_mode"`
	CPU    float64 `json:"cpu"`
	Memory int64   `json:"memory_mb"`
}

func handleListInstances(data []byte) (interface{}, error) {
	phMu.RLock()
	db := phDB
	phMu.RUnlock()
	if db == nil {
		return nil, nil
	}

	var req listInstancesRequest
	_ = json.Unmarshal(data, &req)

	query := db.Model(&model.ClawInstance{}).Where("status != 'destroying'")
	if req.Owner != "" {
		query = query.Where("owner_id = ?", req.Owner)
	}
	if req.Status != "" {
		query = query.Where("status = ?", req.Status)
	}

	var instances []model.ClawInstance
	if err := query.Order("created_at DESC").Limit(100).Find(&instances).Error; err != nil {
		return nil, err
	}

	result := make([]instanceSummary, 0, len(instances))
	for _, inst := range instances {
		result = append(result, instanceSummary{
			ID:     inst.ID,
			Slug:   inst.Slug,
			Status: inst.Status,
			Owner:  inst.OwnerID,
			Mode:   inst.DeployMode,
			CPU:    inst.CPULimit,
			Memory: inst.MemoryLimit / (1024 * 1024),
		})
	}
	return result, nil
}
