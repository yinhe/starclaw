package federation

import (
	"encoding/json"
	"log"

	"github.com/yinhe/starclaw/internal/model"
	"gorm.io/gorm"
)

func (e *Engine) persistSwarm(s *SwarmNode) {
	if e.db == nil {
		return
	}
	caps, _ := json.Marshal(s.Capabilities)
	meta, _ := json.Marshal(s.Metadata)
	rec := model.FederationSwarm{
		SwarmID:      s.ID,
		Name:         s.Name,
		Endpoint:     s.Endpoint,
		Region:       s.Region,
		Status:       string(s.Status),
		Trust:        string(s.Trust),
		Capabilities: string(caps),
		NodeCount:    s.NodeCount,
		AgentCount:   s.AgentCount,
		Reputation:   s.Reputation,
		LastSeen:     s.LastSeen,
		JoinedAt:     s.JoinedAt,
		Metadata:     string(meta),
	}
	if err := e.db.Save(&rec).Error; err != nil {
		log.Printf("[federation] persist swarm %s failed: %v", s.ID, err)
	}
}

func (e *Engine) persistHandshake(h *Handshake) {
	if e.db == nil {
		return
	}
	rec := model.FederationHandshake{
		HandshakeID: h.ID,
		FromSwarm:   h.FromSwarm,
		ToSwarm:     h.ToSwarm,
		Status:      h.Status,
		Challenge:   h.Challenge,
		Response:    h.Response,
		CompletedAt: h.CompletedAt,
	}
	if err := e.db.Save(&rec).Error; err != nil {
		log.Printf("[federation] persist handshake %s failed: %v", h.ID, err)
	}
}

func (e *Engine) persistTaskRoute(r *TaskRoute) {
	if e.db == nil {
		return
	}
	params, _ := json.Marshal(r.Params)
	result, _ := json.Marshal(r.Result)
	rec := model.FederationTaskRoute{
		RouteID:     r.ID,
		SourceSwarm: r.SourceSwarm,
		TargetSwarm: r.TargetSwarm,
		TaskType:    r.TaskType,
		Description: r.Description,
		Params:      string(params),
		Priority:    r.Priority,
		Status:      string(r.Status),
		Bid:         r.Bid,
		Result:      string(result),
		CompletedAt: r.CompletedAt,
	}
	if err := e.db.Save(&rec).Error; err != nil {
		log.Printf("[federation] persist route %s failed: %v", r.ID, err)
	}
}

func (e *Engine) persistTrustEvent(te *TrustEvent) {
	if e.db == nil {
		return
	}
	rec := model.FederationTrustEvent{
		EventID: te.ID,
		SwarmID: te.SwarmID,
		Type:    te.Type,
		Delta:   te.Delta,
		Details: te.Details,
	}
	if err := e.db.Create(&rec).Error; err != nil {
		log.Printf("[federation] persist trust event %s failed: %v", te.ID, err)
	}
}

func (e *Engine) loadFromDB() {
	if e.db == nil {
		return
	}

	// Load swarms
	var swarmRecs []model.FederationSwarm
	e.db.Where("status != ?", "offline").Find(&swarmRecs)
	for _, r := range swarmRecs {
		var caps []string
		json.Unmarshal([]byte(r.Capabilities), &caps)
		var meta map[string]string
		json.Unmarshal([]byte(r.Metadata), &meta)
		e.swarms[r.SwarmID] = &SwarmNode{
			ID:           r.SwarmID,
			Name:         r.Name,
			Endpoint:     r.Endpoint,
			Region:       r.Region,
			Status:       SwarmStatus(r.Status),
			Trust:        TrustLevel(r.Trust),
			Capabilities: caps,
			NodeCount:    r.NodeCount,
			AgentCount:   r.AgentCount,
			Reputation:   r.Reputation,
			LastSeen:     r.LastSeen,
			JoinedAt:     r.JoinedAt,
			Metadata:     meta,
		}
	}

	// Load active task routes
	var routeRecs []model.FederationTaskRoute
	e.db.Where("status IN ?", []string{"proposed", "accepted", "running"}).Find(&routeRecs)
	for _, r := range routeRecs {
		var params map[string]interface{}
		json.Unmarshal([]byte(r.Params), &params)
		var result map[string]interface{}
		json.Unmarshal([]byte(r.Result), &result)
		e.routes = append(e.routes, TaskRoute{
			ID:          r.RouteID,
			SourceSwarm: r.SourceSwarm,
			TargetSwarm: r.TargetSwarm,
			TaskType:    r.TaskType,
			Description: r.Description,
			Params:      params,
			Priority:    r.Priority,
			Status:      TaskRouteStatus(r.Status),
			Bid:         r.Bid,
			Result:      result,
			CreatedAt:   r.CreatedAt,
			CompletedAt: r.CompletedAt,
		})
	}

	// Load trust events (latest 200)
	var teRecs []model.FederationTrustEvent
	e.db.Order("created_at desc").Limit(200).Find(&teRecs)
	for i := len(teRecs) - 1; i >= 0; i-- {
		r := teRecs[i]
		e.trustEvents = append(e.trustEvents, TrustEvent{
			ID:        r.EventID,
			SwarmID:   r.SwarmID,
			Type:      r.Type,
			Delta:     r.Delta,
			Details:   r.Details,
			Timestamp: r.CreatedAt,
		})
	}

	total := len(swarmRecs) + len(routeRecs) + len(teRecs)
	if total > 0 {
		log.Printf("[federation] restored from DB: %d swarms, %d routes, %d trust events",
			len(swarmRecs), len(routeRecs), len(teRecs))
	}
}

func (e *Engine) SetDB(db *gorm.DB) {
	e.db = db
}
