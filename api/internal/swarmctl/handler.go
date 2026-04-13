package swarmctl

import (
	"encoding/json"
	"net/http"
)

// ════════════════════════════════════════════════════════════
// Physical Swarm HTTP Handlers — /v1/swarmctl/*
// ════════════════════════════════════════════════════════════

// HandleStats handles GET /swarmctl/stats
func HandleStats(w http.ResponseWriter, r *http.Request) {
	eng := GetEngine()
	if eng == nil {
		http.Error(w, `{"error":"swarmctl engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(eng.Stats())
}

// HandleRegisterUnit handles POST /swarmctl/unit
func HandleRegisterUnit(w http.ResponseWriter, r *http.Request) {
	eng := GetEngine()
	if eng == nil {
		http.Error(w, `{"error":"swarmctl engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		Name         string            `json:"name"`
		UnitType     string            `json:"unit_type"`
		Domain       string            `json:"domain"`
		Position     Position          `json:"position"`
		Capabilities []string          `json:"capabilities"`
		Battery      float64           `json:"battery"`
		Health       float64           `json:"health"`
		Payload      float64           `json:"payload_kg"`
		Speed        float64           `json:"speed_mps"`
		Metadata     map[string]string `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.Name == "" || req.UnitType == "" {
		http.Error(w, `{"error":"name and unit_type required"}`, http.StatusBadRequest)
		return
	}
	if req.Battery == 0 {
		req.Battery = 100
	}
	if req.Health == 0 {
		req.Health = 100
	}
	unit := eng.RegisterUnit(req.Name, req.UnitType, Domain(req.Domain), req.Position, req.Capabilities, req.Battery, req.Health, req.Payload, req.Speed, req.Metadata)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(unit)
}

// HandleListUnits handles GET /swarmctl/units?domain=&status=
func HandleListUnits(w http.ResponseWriter, r *http.Request) {
	eng := GetEngine()
	if eng == nil {
		http.Error(w, `{"error":"swarmctl engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	domain := r.URL.Query().Get("domain")
	status := r.URL.Query().Get("status")
	units := eng.ListUnits(domain, status)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"units": units,
		"count": len(units),
	})
}

// HandleGetUnit handles GET /swarmctl/unit?id=
func HandleGetUnit(w http.ResponseWriter, r *http.Request) {
	eng := GetEngine()
	if eng == nil {
		http.Error(w, `{"error":"swarmctl engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, `{"error":"id required"}`, http.StatusBadRequest)
		return
	}
	u := eng.GetUnit(id)
	if u == nil {
		http.Error(w, `{"error":"unit not found"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(u)
}

// HandleUpdateUnit handles POST /swarmctl/unit/status
func HandleUpdateUnit(w http.ResponseWriter, r *http.Request) {
	eng := GetEngine()
	if eng == nil {
		http.Error(w, `{"error":"swarmctl engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		UnitID   string    `json:"unit_id"`
		Status   string    `json:"status"`
		Position *Position `json:"position"`
		Battery  float64   `json:"battery"`
		Health   float64   `json:"health"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if err := eng.UpdateUnitStatus(req.UnitID, UnitStatus(req.Status), req.Position, req.Battery, req.Health); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
}

// HandleCreateFormation handles POST /swarmctl/formation
func HandleCreateFormation(w http.ResponseWriter, r *http.Request) {
	eng := GetEngine()
	if eng == nil {
		http.Error(w, `{"error":"swarmctl engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		Name     string   `json:"name"`
		Shape    string   `json:"shape"`
		UnitIDs  []string `json:"unit_ids"`
		LeaderID string   `json:"leader_id"`
		Center   Position `json:"center"`
		Spacing  float64  `json:"spacing_m"`
		Heading  float64  `json:"heading_deg"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.Spacing == 0 {
		req.Spacing = 5
	}
	f, err := eng.CreateFormation(req.Name, FormationShape(req.Shape), req.UnitIDs, req.LeaderID, req.Center, req.Spacing, req.Heading)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(f)
}

// HandleDissolveFormation handles POST /swarmctl/formation/dissolve
func HandleDissolveFormation(w http.ResponseWriter, r *http.Request) {
	eng := GetEngine()
	if eng == nil {
		http.Error(w, `{"error":"swarmctl engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		FormationID string `json:"formation_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if err := eng.DissolveFormation(req.FormationID); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "dissolved"})
}

// HandleListFormations handles GET /swarmctl/formations
func HandleListFormations(w http.ResponseWriter, r *http.Request) {
	eng := GetEngine()
	if eng == nil {
		http.Error(w, `{"error":"swarmctl engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	formations := eng.ListFormations()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"formations": formations,
		"count":      len(formations),
	})
}

// HandleCreateMission handles POST /swarmctl/mission
func HandleCreateMission(w http.ResponseWriter, r *http.Request) {
	eng := GetEngine()
	if eng == nil {
		http.Error(w, `{"error":"swarmctl engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		Name        string                 `json:"name"`
		Type        string                 `json:"type"`
		Priority    string                 `json:"priority"`
		Domains     []string               `json:"domains"`
		UnitIDs     []string               `json:"unit_ids"`
		Waypoints   []Position             `json:"waypoints"`
		Objectives  []string               `json:"objectives"`
		Constraints MissionConstraints     `json:"constraints"`
		Params      map[string]interface{} `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		http.Error(w, `{"error":"name required"}`, http.StatusBadRequest)
		return
	}
	var domains []Domain
	for _, d := range req.Domains {
		domains = append(domains, Domain(d))
	}
	m, err := eng.CreateMission(req.Name, req.Type, req.Priority, domains, req.UnitIDs, req.Waypoints, req.Objectives, req.Constraints, req.Params)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(m)
}

// HandleStartMission handles POST /swarmctl/mission/start
func HandleStartMission(w http.ResponseWriter, r *http.Request) {
	eng := GetEngine()
	if eng == nil {
		http.Error(w, `{"error":"swarmctl engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		MissionID string `json:"mission_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if err := eng.StartMission(req.MissionID); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "started"})
}

// HandleCompleteMission handles POST /swarmctl/mission/complete
func HandleCompleteMission(w http.ResponseWriter, r *http.Request) {
	eng := GetEngine()
	if eng == nil {
		http.Error(w, `{"error":"swarmctl engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		MissionID string `json:"mission_id"`
		Success   bool   `json:"success"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if err := eng.CompleteMission(req.MissionID, req.Success); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "completed"})
}

// HandleAbortMission handles POST /swarmctl/mission/abort
func HandleAbortMission(w http.ResponseWriter, r *http.Request) {
	eng := GetEngine()
	if eng == nil {
		http.Error(w, `{"error":"swarmctl engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		MissionID string `json:"mission_id"`
		Reason    string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if err := eng.AbortMission(req.MissionID, req.Reason); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "aborted"})
}

// HandleListMissions handles GET /swarmctl/missions?status=
func HandleListMissions(w http.ResponseWriter, r *http.Request) {
	eng := GetEngine()
	if eng == nil {
		http.Error(w, `{"error":"swarmctl engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	status := r.URL.Query().Get("status")
	missions := eng.ListMissions(status)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"missions": missions,
		"count":    len(missions),
	})
}

// HandleGetMission handles GET /swarmctl/mission?id=
func HandleGetMission(w http.ResponseWriter, r *http.Request) {
	eng := GetEngine()
	if eng == nil {
		http.Error(w, `{"error":"swarmctl engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, `{"error":"id required"}`, http.StatusBadRequest)
		return
	}
	m := eng.GetMission(id)
	if m == nil {
		http.Error(w, `{"error":"mission not found"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(m)
}

// HandleUpdateProgress handles POST /swarmctl/mission/progress
func HandleUpdateProgress(w http.ResponseWriter, r *http.Request) {
	eng := GetEngine()
	if eng == nil {
		http.Error(w, `{"error":"swarmctl engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		MissionID  string  `json:"mission_id"`
		Progress   float64 `json:"progress"`
		LogEvent   string  `json:"log_event"`
		LogDetails string  `json:"log_details"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if err := eng.UpdateMissionProgress(req.MissionID, req.Progress, req.LogEvent, req.LogDetails); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
}
