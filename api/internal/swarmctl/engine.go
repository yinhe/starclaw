package swarmctl

import (
	"fmt"
	"log"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/yinhe/starclaw/internal/nerve"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════
// Physical Swarm Controller — 物理虫群协调 (Phase 5D)
//
// 三域协同:
//   - Ground: Zergling(机器狗) + Roach(地面机器人) + Hydralisk(车辆)
//   - Air: Mutalisk(无人机/UAV)
//   - Digital: Claw(数字Agent)
//
// 子系统:
//   1. Tri-Domain Coordinator — 三域单位注册/状态/指派
//   2. Formation Engine — 混合编队/动态阵型
//   3. Mission Planner — 自适应任务规划/多目标优化
// ════════════════════════════════════════════════════════════

// ── Types ──

type Domain string
type UnitStatus string
type MissionStatus string
type FormationShape string

const (
	DomainGround  Domain = "ground"
	DomainAir     Domain = "air"
	DomainDigital Domain = "digital"

	UnitReady   UnitStatus = "ready"
	UnitBusy    UnitStatus = "busy"
	UnitOffline UnitStatus = "offline"
	UnitDamaged UnitStatus = "damaged"

	MissionPlanned  MissionStatus = "planned"
	MissionActive   MissionStatus = "active"
	MissionComplete MissionStatus = "complete"
	MissionAborted  MissionStatus = "aborted"
	MissionFailed   MissionStatus = "failed"

	FormationLine      FormationShape = "line"
	FormationWedge     FormationShape = "wedge"
	FormationCircle    FormationShape = "circle"
	FormationScatter   FormationShape = "scatter"
	FormationColumn    FormationShape = "column"
	FormationOverwatch FormationShape = "overwatch" // air high, ground spread
)

// ── Data Structures ──

type PhysicalUnit struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	UnitType     string            `json:"unit_type"` // zergling, roach, hydralisk, mutalisk, claw
	Domain       Domain            `json:"domain"`
	Status       UnitStatus        `json:"status"`
	Position     Position          `json:"position"`
	Battery      float64           `json:"battery"`               // 0-100%
	Health       float64           `json:"health"`                // 0-100%
	Capabilities []string          `json:"capabilities"`          // e.g. "lidar", "camera", "gps", "arm", "cargo"
	Payload      float64           `json:"payload_kg"`            // max cargo
	Speed        float64           `json:"speed_mps"`             // max speed m/s
	AssignedTo   string            `json:"assigned_to,omitempty"` // mission ID
	LastSeen     time.Time         `json:"last_seen"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

type Position struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
	Alt float64 `json:"alt"` // meters, 0 for ground
}

type Formation struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Shape     FormationShape `json:"shape"`
	Units     []string       `json:"unit_ids"`
	Leader    string         `json:"leader_id"`
	Center    Position       `json:"center"`
	Spacing   float64        `json:"spacing_m"`   // distance between units
	Heading   float64        `json:"heading_deg"` // 0=north, clockwise
	Status    string         `json:"status"`      // forming, formed, moving, dissolved
	CreatedAt time.Time      `json:"created_at"`
}

type Mission struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Type        string                 `json:"type"` // patrol, escort, survey, delivery, search_rescue
	Status      MissionStatus          `json:"status"`
	Priority    string                 `json:"priority"` // P0-P3
	Domains     []Domain               `json:"domains"`  // which domains involved
	Units       []string               `json:"unit_ids"`
	Formation   string                 `json:"formation_id,omitempty"`
	Waypoints   []Position             `json:"waypoints"`
	Objectives  []string               `json:"objectives"`
	Constraints MissionConstraints     `json:"constraints"`
	Progress    float64                `json:"progress"` // 0-100%
	Params      map[string]interface{} `json:"params,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	StartedAt   *time.Time             `json:"started_at,omitempty"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	Log         []MissionLogEntry      `json:"log"`
}

type MissionConstraints struct {
	MaxAltitude float64       `json:"max_altitude_m"`
	MaxSpeed    float64       `json:"max_speed_mps"`
	MinBattery  float64       `json:"min_battery_pct"`
	Geofence    *Geofence     `json:"geofence,omitempty"`
	TimeLimit   time.Duration `json:"time_limit"`
	WeatherOK   bool          `json:"weather_ok"`
}

type Geofence struct {
	Center Position `json:"center"`
	Radius float64  `json:"radius_m"`
}

type MissionLogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	UnitID    string    `json:"unit_id,omitempty"`
	Event     string    `json:"event"`
	Details   string    `json:"details,omitempty"`
}

// ── Engine ──

type Engine struct {
	mu         sync.RWMutex
	db         *gorm.DB
	nodeID     string
	units      map[string]*PhysicalUnit
	formations map[string]*Formation
	missions   map[string]*Mission
	stats      EngineStats //nolint:unused // reserved for future stats collection
	startAt    time.Time
	nextID     int
}

type EngineStats struct {
	UnitsTotal       int    `json:"units_total"`
	UnitsGround      int    `json:"units_ground"`
	UnitsAir         int    `json:"units_air"`
	UnitsDigital     int    `json:"units_digital"`
	UnitsReady       int    `json:"units_ready"`
	FormationsActive int    `json:"formations_active"`
	MissionsTotal    int    `json:"missions_total"`
	MissionsActive   int    `json:"missions_active"`
	MissionsComplete int    `json:"missions_complete"`
	Uptime           string `json:"uptime"`
}

var (
	globalEngine *Engine
	engineOnce   sync.Once
)

func InitEngine(nodeID string, db *gorm.DB) *Engine {
	engineOnce.Do(func() {
		globalEngine = &Engine{
			db:         db,
			nodeID:     nodeID,
			units:      make(map[string]*PhysicalUnit),
			formations: make(map[string]*Formation),
			missions:   make(map[string]*Mission),
			startAt:    time.Now(),
		}
		globalEngine.loadFromDB()
		log.Printf("[swarmctl] physical swarm controller initialized (node=%s, db=%v)", nodeID, db != nil)
	})
	return globalEngine
}

func GetEngine() *Engine {
	return globalEngine
}

func (e *Engine) genID(prefix string) string {
	e.nextID++
	return fmt.Sprintf("%s-%d-%d", prefix, time.Now().UnixMilli(), e.nextID)
}

// ══════════════ Unit Management ══════════════

func (e *Engine) RegisterUnit(name, unitType string, domain Domain, pos Position, capabilities []string, battery, health, payload, speed float64, meta map[string]string) *PhysicalUnit {
	e.mu.Lock()
	defer e.mu.Unlock()

	unit := &PhysicalUnit{
		ID:           e.genID("unit"),
		Name:         name,
		UnitType:     unitType,
		Domain:       domain,
		Status:       UnitReady,
		Position:     pos,
		Battery:      battery,
		Health:       health,
		Capabilities: capabilities,
		Payload:      payload,
		Speed:        speed,
		LastSeen:     time.Now(),
		Metadata:     meta,
	}
	e.units[unit.ID] = unit
	go e.persistUnit(unit)

	if bus := nerve.GetBus(); bus != nil {
		bus.Publish("swarmctl.unit.registered", "swarmctl", map[string]interface{}{
			"unit_id":   unit.ID,
			"unit_type": unitType,
			"domain":    string(domain),
		})
	}
	log.Printf("[swarmctl] unit registered: %s (%s/%s)", name, unitType, domain)
	return unit
}

func (e *Engine) UpdateUnitStatus(unitID string, status UnitStatus, pos *Position, battery, health float64) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	u, ok := e.units[unitID]
	if !ok {
		return fmt.Errorf("unit %s not found", unitID)
	}
	u.Status = status
	if pos != nil {
		u.Position = *pos
	}
	if battery >= 0 {
		u.Battery = battery
	}
	if health >= 0 {
		u.Health = health
	}
	u.LastSeen = time.Now()
	go e.persistUnit(u)
	return nil
}

func (e *Engine) ListUnits(domain string, statusFilter string) []*PhysicalUnit {
	e.mu.RLock()
	defer e.mu.RUnlock()
	var result []*PhysicalUnit
	for _, u := range e.units {
		if domain != "" && string(u.Domain) != domain {
			continue
		}
		if statusFilter != "" && string(u.Status) != statusFilter {
			continue
		}
		result = append(result, u)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

func (e *Engine) GetUnit(id string) *PhysicalUnit {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.units[id]
}

// ══════════════ Formation ══════════════

func (e *Engine) CreateFormation(name string, shape FormationShape, unitIDs []string, leaderID string, center Position, spacing, heading float64) (*Formation, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Validate units exist and are ready
	for _, uid := range unitIDs {
		u, ok := e.units[uid]
		if !ok {
			return nil, fmt.Errorf("unit %s not found", uid)
		}
		if u.Status != UnitReady {
			return nil, fmt.Errorf("unit %s is not ready (status=%s)", uid, u.Status)
		}
	}

	f := &Formation{
		ID:        e.genID("form"),
		Name:      name,
		Shape:     shape,
		Units:     unitIDs,
		Leader:    leaderID,
		Center:    center,
		Spacing:   spacing,
		Heading:   heading,
		Status:    "forming",
		CreatedAt: time.Now(),
	}
	e.formations[f.ID] = f
	go e.persistFormation(f)

	// Mark units as busy
	for _, uid := range unitIDs {
		if u, ok := e.units[uid]; ok {
			u.Status = UnitBusy
			go e.persistUnit(u)
		}
	}

	if bus := nerve.GetBus(); bus != nil {
		bus.Publish("swarmctl.formation.created", "swarmctl", map[string]interface{}{
			"formation_id": f.ID,
			"shape":        string(shape),
			"unit_count":   len(unitIDs),
		})
	}
	return f, nil
}

func (e *Engine) DissolveFormation(formationID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	f, ok := e.formations[formationID]
	if !ok {
		return fmt.Errorf("formation %s not found", formationID)
	}
	f.Status = "dissolved"
	go e.persistFormation(f)
	for _, uid := range f.Units {
		if u, ok := e.units[uid]; ok && u.Status == UnitBusy {
			u.Status = UnitReady
			go e.persistUnit(u)
		}
	}
	return nil
}

func (e *Engine) ListFormations() []*Formation {
	e.mu.RLock()
	defer e.mu.RUnlock()
	var result []*Formation
	for _, f := range e.formations {
		if f.Status != "dissolved" {
			result = append(result, f)
		}
	}
	return result
}

// ══════════════ Mission ══════════════

func (e *Engine) CreateMission(name, mtype, priority string, domains []Domain, unitIDs []string, waypoints []Position, objectives []string, constraints MissionConstraints, params map[string]interface{}) (*Mission, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Validate units
	for _, uid := range unitIDs {
		if _, ok := e.units[uid]; !ok {
			return nil, fmt.Errorf("unit %s not found", uid)
		}
	}

	// Check geofence constraints on waypoints
	if constraints.Geofence != nil {
		for i, wp := range waypoints {
			dist := haversine(constraints.Geofence.Center.Lat, constraints.Geofence.Center.Lon, wp.Lat, wp.Lon)
			if dist > constraints.Geofence.Radius {
				return nil, fmt.Errorf("waypoint %d is %.0fm outside geofence (radius=%.0fm)", i, dist-constraints.Geofence.Radius, constraints.Geofence.Radius)
			}
		}
	}

	m := &Mission{
		ID:          e.genID("mission"),
		Name:        name,
		Type:        mtype,
		Status:      MissionPlanned,
		Priority:    priority,
		Domains:     domains,
		Units:       unitIDs,
		Waypoints:   waypoints,
		Objectives:  objectives,
		Constraints: constraints,
		Params:      params,
		CreatedAt:   time.Now(),
		Log:         []MissionLogEntry{{Timestamp: time.Now(), Event: "created", Details: fmt.Sprintf("Mission %s created with %d units", name, len(unitIDs))}},
	}
	e.missions[m.ID] = m
	go e.persistMission(m)

	if bus := nerve.GetBus(); bus != nil {
		bus.Publish("swarmctl.mission.created", "swarmctl", map[string]interface{}{
			"mission_id": m.ID,
			"type":       mtype,
			"units":      len(unitIDs),
			"domains":    domains,
		})
	}
	return m, nil
}

func (e *Engine) StartMission(missionID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	m, ok := e.missions[missionID]
	if !ok {
		return fmt.Errorf("mission %s not found", missionID)
	}
	if m.Status != MissionPlanned {
		return fmt.Errorf("mission %s is not planned (status=%s)", missionID, m.Status)
	}

	// Pre-flight: check battery
	for _, uid := range m.Units {
		if u, ok := e.units[uid]; ok {
			if u.Battery < m.Constraints.MinBattery {
				return fmt.Errorf("unit %s battery %.0f%% below minimum %.0f%%", uid, u.Battery, m.Constraints.MinBattery)
			}
			u.Status = UnitBusy
			u.AssignedTo = missionID
		}
	}

	now := time.Now()
	m.Status = MissionActive
	m.StartedAt = &now
	entry := MissionLogEntry{Timestamp: now, Event: "started", Details: "Mission is now active"}
	m.Log = append(m.Log, entry)
	go e.persistMission(m)
	go e.persistMissionLog(missionID, &entry)
	return nil
}

func (e *Engine) UpdateMissionProgress(missionID string, progress float64, logEvent, logDetails string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	m, ok := e.missions[missionID]
	if !ok {
		return fmt.Errorf("mission %s not found", missionID)
	}
	if progress >= 0 {
		m.Progress = progress
	}
	if logEvent != "" {
		entry := MissionLogEntry{
			Timestamp: time.Now(),
			Event:     logEvent,
			Details:   logDetails,
		}
		m.Log = append(m.Log, entry)
		go e.persistMissionLog(missionID, &entry)
	}
	go e.persistMission(m)
	return nil
}

func (e *Engine) CompleteMission(missionID string, success bool) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	m, ok := e.missions[missionID]
	if !ok {
		return fmt.Errorf("mission %s not found", missionID)
	}
	now := time.Now()
	m.CompletedAt = &now
	m.Progress = 100

	var entry MissionLogEntry
	if success {
		m.Status = MissionComplete
		entry = MissionLogEntry{Timestamp: now, Event: "completed", Details: "Mission completed successfully"}
	} else {
		m.Status = MissionFailed
		entry = MissionLogEntry{Timestamp: now, Event: "failed", Details: "Mission failed"}
	}
	m.Log = append(m.Log, entry)
	go e.persistMission(m)
	go e.persistMissionLog(missionID, &entry)

	// Release units
	for _, uid := range m.Units {
		if u, ok := e.units[uid]; ok {
			u.Status = UnitReady
			u.AssignedTo = ""
			go e.persistUnit(u)
		}
	}

	if bus := nerve.GetBus(); bus != nil {
		bus.Publish("swarmctl.mission.completed", "swarmctl", map[string]interface{}{
			"mission_id": m.ID,
			"success":    success,
		})
	}
	return nil
}

func (e *Engine) AbortMission(missionID, reason string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	m, ok := e.missions[missionID]
	if !ok {
		return fmt.Errorf("mission %s not found", missionID)
	}
	now := time.Now()
	m.Status = MissionAborted
	m.CompletedAt = &now
	entry := MissionLogEntry{Timestamp: now, Event: "aborted", Details: reason}
	m.Log = append(m.Log, entry)
	go e.persistMission(m)
	go e.persistMissionLog(missionID, &entry)

	for _, uid := range m.Units {
		if u, ok := e.units[uid]; ok {
			u.Status = UnitReady
			u.AssignedTo = ""
			go e.persistUnit(u)
		}
	}
	return nil
}

func (e *Engine) ListMissions(status string) []*Mission {
	e.mu.RLock()
	defer e.mu.RUnlock()
	var result []*Mission
	for _, m := range e.missions {
		if status != "" && string(m.Status) != status {
			continue
		}
		result = append(result, m)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.After(result[j].CreatedAt) })
	return result
}

func (e *Engine) GetMission(id string) *Mission {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.missions[id]
}

// ── Stats ──

func (e *Engine) Stats() *EngineStats {
	e.mu.RLock()
	defer e.mu.RUnlock()
	s := EngineStats{Uptime: time.Since(e.startAt).Round(time.Second).String()}
	for _, u := range e.units {
		s.UnitsTotal++
		switch u.Domain {
		case DomainGround:
			s.UnitsGround++
		case DomainAir:
			s.UnitsAir++
		case DomainDigital:
			s.UnitsDigital++
		}
		if u.Status == UnitReady {
			s.UnitsReady++
		}
	}
	for _, f := range e.formations {
		if f.Status != "dissolved" {
			s.FormationsActive++
		}
	}
	for _, m := range e.missions {
		s.MissionsTotal++
		if m.Status == MissionActive {
			s.MissionsActive++
		}
		if m.Status == MissionComplete {
			s.MissionsComplete++
		}
	}
	return &s
}

// ── Helpers ──

func haversine(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371000 // earth radius meters
	dLat := (lat2 - lat1) * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return R * c
}
