package swarmctl

import (
	"encoding/json"
	"log"

	"github.com/yinhe/starclaw/internal/model"
	"gorm.io/gorm"
)

func (e *Engine) persistUnit(u *PhysicalUnit) {
	if e.db == nil {
		return
	}
	caps, _ := json.Marshal(u.Capabilities)
	meta, _ := json.Marshal(u.Metadata)
	rec := model.SwarmCtlUnit{
		UnitID:       u.ID,
		NodeID:       e.nodeID,
		Name:         u.Name,
		UnitType:     u.UnitType,
		Domain:       string(u.Domain),
		Status:       string(u.Status),
		Lat:          u.Position.Lat,
		Lon:          u.Position.Lon,
		Alt:          u.Position.Alt,
		Battery:      u.Battery,
		Health:       u.Health,
		Capabilities: string(caps),
		PayloadKg:    u.Payload,
		SpeedMps:     u.Speed,
		AssignedTo:   u.AssignedTo,
		LastSeen:     u.LastSeen,
		Metadata:     string(meta),
	}
	if err := e.db.Save(&rec).Error; err != nil {
		log.Printf("[swarmctl] persist unit %s failed: %v", u.ID, err)
	}
}

func (e *Engine) persistFormation(f *Formation) {
	if e.db == nil {
		return
	}
	units, _ := json.Marshal(f.Units)
	rec := model.SwarmCtlFormation{
		FormationID: f.ID,
		NodeID:      e.nodeID,
		Name:        f.Name,
		Shape:       string(f.Shape),
		UnitIDs:     string(units),
		LeaderID:    f.Leader,
		CenterLat:   f.Center.Lat,
		CenterLon:   f.Center.Lon,
		CenterAlt:   f.Center.Alt,
		Spacing:     f.Spacing,
		Heading:     f.Heading,
		Status:      f.Status,
	}
	if err := e.db.Save(&rec).Error; err != nil {
		log.Printf("[swarmctl] persist formation %s failed: %v", f.ID, err)
	}
}

func (e *Engine) persistMission(m *Mission) {
	if e.db == nil {
		return
	}
	domains, _ := json.Marshal(m.Domains)
	units, _ := json.Marshal(m.Units)
	waypoints, _ := json.Marshal(m.Waypoints)
	objectives, _ := json.Marshal(m.Objectives)
	constraints, _ := json.Marshal(m.Constraints)
	params, _ := json.Marshal(m.Params)
	rec := model.SwarmCtlMission{
		MissionID:   m.ID,
		NodeID:      e.nodeID,
		Name:        m.Name,
		Type:        m.Type,
		Status:      string(m.Status),
		Priority:    m.Priority,
		Domains:     string(domains),
		UnitIDs:     string(units),
		FormationID: m.Formation,
		Waypoints:   string(waypoints),
		Objectives:  string(objectives),
		Constraints: string(constraints),
		Params:      string(params),
		Progress:    m.Progress,
		StartedAt:   m.StartedAt,
		CompletedAt: m.CompletedAt,
	}
	if err := e.db.Save(&rec).Error; err != nil {
		log.Printf("[swarmctl] persist mission %s failed: %v", m.ID, err)
	}
}

func (e *Engine) persistMissionLog(missionID string, entry *MissionLogEntry) {
	if e.db == nil {
		return
	}
	rec := model.SwarmCtlMissionLog{
		MissionID: missionID,
		UnitID:    entry.UnitID,
		Event:     entry.Event,
		Details:   entry.Details,
		EventTime: entry.Timestamp,
	}
	if err := e.db.Create(&rec).Error; err != nil {
		log.Printf("[swarmctl] persist mission log failed: %v", err)
	}
}

func (e *Engine) loadFromDB() {
	if e.db == nil {
		return
	}

	// Load units
	var unitRecs []model.SwarmCtlUnit
	e.db.Where("node_id = ? AND status != ?", e.nodeID, "offline").Find(&unitRecs)
	for _, r := range unitRecs {
		var caps []string
		json.Unmarshal([]byte(r.Capabilities), &caps)
		var meta map[string]string
		json.Unmarshal([]byte(r.Metadata), &meta)
		e.units[r.UnitID] = &PhysicalUnit{
			ID:           r.UnitID,
			Name:         r.Name,
			UnitType:     r.UnitType,
			Domain:       Domain(r.Domain),
			Status:       UnitStatus(r.Status),
			Position:     Position{Lat: r.Lat, Lon: r.Lon, Alt: r.Alt},
			Battery:      r.Battery,
			Health:       r.Health,
			Capabilities: caps,
			Payload:      r.PayloadKg,
			Speed:        r.SpeedMps,
			AssignedTo:   r.AssignedTo,
			LastSeen:     r.LastSeen,
			Metadata:     meta,
		}
	}

	// Load active formations
	var fmtRecs []model.SwarmCtlFormation
	e.db.Where("node_id = ? AND status IN ?", e.nodeID, []string{"forming", "formed", "moving"}).Find(&fmtRecs)
	for _, r := range fmtRecs {
		var unitIDs []string
		json.Unmarshal([]byte(r.UnitIDs), &unitIDs)
		e.formations[r.FormationID] = &Formation{
			ID:        r.FormationID,
			Name:      r.Name,
			Shape:     FormationShape(r.Shape),
			Units:     unitIDs,
			Leader:    r.LeaderID,
			Center:    Position{Lat: r.CenterLat, Lon: r.CenterLon, Alt: r.CenterAlt},
			Spacing:   r.Spacing,
			Heading:   r.Heading,
			Status:    r.Status,
			CreatedAt: r.CreatedAt,
		}
	}

	// Load active missions
	var msnRecs []model.SwarmCtlMission
	e.db.Where("node_id = ? AND status IN ?", e.nodeID, []string{"planned", "active"}).Find(&msnRecs)
	for _, r := range msnRecs {
		var domains []Domain
		json.Unmarshal([]byte(r.Domains), &domains)
		var unitIDs []string
		json.Unmarshal([]byte(r.UnitIDs), &unitIDs)
		var waypoints []Position
		json.Unmarshal([]byte(r.Waypoints), &waypoints)
		var objectives []string
		json.Unmarshal([]byte(r.Objectives), &objectives)
		var constraints MissionConstraints
		json.Unmarshal([]byte(r.Constraints), &constraints)
		var params map[string]interface{}
		json.Unmarshal([]byte(r.Params), &params)

		mission := &Mission{
			ID:          r.MissionID,
			Name:        r.Name,
			Type:        r.Type,
			Status:      MissionStatus(r.Status),
			Priority:    r.Priority,
			Domains:     domains,
			Units:       unitIDs,
			Formation:   r.FormationID,
			Waypoints:   waypoints,
			Objectives:  objectives,
			Constraints: constraints,
			Progress:    r.Progress,
			Params:      params,
			CreatedAt:   r.CreatedAt,
			StartedAt:   r.StartedAt,
			CompletedAt: r.CompletedAt,
			Log:         make([]MissionLogEntry, 0),
		}

		// Load mission logs
		var logRecs []model.SwarmCtlMissionLog
		e.db.Where("mission_id = ?", r.MissionID).Order("event_time asc").Find(&logRecs)
		for _, l := range logRecs {
			mission.Log = append(mission.Log, MissionLogEntry{
				Timestamp: l.EventTime,
				UnitID:    l.UnitID,
				Event:     l.Event,
				Details:   l.Details,
			})
		}
		e.missions[r.MissionID] = mission
	}

	total := len(unitRecs) + len(fmtRecs) + len(msnRecs)
	if total > 0 {
		log.Printf("[swarmctl] restored from DB: %d units, %d formations, %d missions",
			len(unitRecs), len(fmtRecs), len(msnRecs))
	}
}

func (e *Engine) SetDB(db *gorm.DB) {
	e.db = db
}
