package mutalisk

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"sync"
	"time"
)

// ════════════════════════════════════════════════════════════
// Mutalisk Adapter — Claw ↔ DJI 无人机适配层
//
// 架构:
//   Claw Agent Runtime → Mutalisk Adapter (Go, 本包)
//        ↓ MQTT (DJI Cloud API v2)
//   DJI Cloud / MQTT Broker
//        ↓ OcuSync / 4G
//   DJI M30T / Mavic 3E / Dock 2 (无人机)
//
// 核心职责:
//   1. 将 MCP Tool 调用转换为 DJI Cloud API MQTT 指令
//   2. Safety Guard: 高度/距离/电量/风速硬限制
//   3. 遥测缓存: 订阅 OSD topic, 缓存飞机状态
//   4. 舰队管理: 多机 SN 注册 + 状态追踪
//   5. 事件上报: 低电量/围栏越界/异常 → Pheromone
// ════════════════════════════════════════════════════════════

// ── Drone Types ──

type DroneModel string

const (
	ModelM350    DroneModel = "matrice_350_rtk"
	ModelM30T    DroneModel = "matrice_30t"
	ModelMavic3E DroneModel = "mavic_3_enterprise"
	ModelMavic3T DroneModel = "mavic_3_thermal"
	ModelDock2   DroneModel = "dock_2"
)

type DroneState string

const (
	StateOffline     DroneState = "offline"
	StateOnline      DroneState = "online"
	StateIdle        DroneState = "idle"
	StateTakingOff   DroneState = "taking_off"
	StateFlying      DroneState = "flying"
	StateLanding     DroneState = "landing"
	StateReturning   DroneState = "returning"
	StateHovering    DroneState = "hovering"
	StateDocked      DroneState = "docked"
	StateEmergencyDn DroneState = "emergency"
)

type FlightMode int

const (
	ModeManual    FlightMode = 0
	ModeGPS       FlightMode = 6
	ModeWaypoint  FlightMode = 14
	ModeDRC       FlightMode = 17 // Drone Remote Control (virtual stick)
	ModeRTH       FlightMode = 11
	ModeLanding   FlightMode = 9
	ModeTakeoff   FlightMode = 1
)

// ── Telemetry Data ──

// DroneTelemetry represents the full OSD state of a connected drone
type DroneTelemetry struct {
	SN            string     `json:"sn"`
	Model         DroneModel `json:"model"`
	State         DroneState `json:"state"`
	Latitude      float64    `json:"latitude"`
	Longitude     float64    `json:"longitude"`
	Height        float64    `json:"height"`          // relative height (m)
	Altitude      float64    `json:"altitude"`         // sea-level altitude (m)
	SpeedX        float64    `json:"speed_x"`          // forward m/s
	SpeedY        float64    `json:"speed_y"`          // lateral m/s
	SpeedZ        float64    `json:"speed_z"`          // vertical m/s
	HSpeed        float64    `json:"horizontal_speed"`
	VSpeed        float64    `json:"vertical_speed"`
	Heading       float64    `json:"heading"`           // degrees
	Pitch         float64    `json:"pitch"`
	Roll          float64    `json:"roll"`
	Battery       DroneBattery `json:"battery"`
	WindSpeed     float64    `json:"wind_speed"`        // m/s
	WindDirection float64    `json:"wind_direction"`    // degrees
	HomeDistance  float64    `json:"home_distance"`     // m
	HomeLat       float64    `json:"home_latitude"`
	HomeLng       float64    `json:"home_longitude"`
	GimbalPitch   float64    `json:"gimbal_pitch"`
	GimbalYaw     float64    `json:"gimbal_yaw"`
	ModeCode      FlightMode `json:"mode_code"`
	GPS           GPSInfo    `json:"gps"`
	ObstacleAvoid bool       `json:"obstacle_avoidance"`
	IsFlying      bool       `json:"is_flying"`
	Timestamp     time.Time  `json:"timestamp"`
}

type DroneBattery struct {
	Percent     int     `json:"percent"`       // 0-100
	Voltage     float64 `json:"voltage"`       // mV
	Temperature float64 `json:"temperature"`   // °C
	RemainFlightTime int `json:"remain_flight_time"` // seconds
}

type GPSInfo struct {
	SatCount int     `json:"satellite_count"`
	Quality  int     `json:"quality"` // 0-5 (5=RTK fixed)
	HDOP     float64 `json:"hdop"`
}

// ── Safety Guard ──

// SafetyConfig defines hard limits for drone operations
type SafetyConfig struct {
	MaxHeight       float64 `json:"max_height"`        // m — China regulation: 120m
	MaxDistance      float64 `json:"max_distance"`      // m from home
	MinBattery       int     `json:"min_battery"`       // % — force RTH
	CriticalBattery  int     `json:"critical_battery"`  // % — emergency land
	MaxWindSpeed     float64 `json:"max_wind_speed"`    // m/s
	MaxFlightTime    int     `json:"max_flight_time"`   // seconds per mission
	MaxHSpeed        float64 `json:"max_h_speed"`       // m/s horizontal
	MaxVSpeed        float64 `json:"max_v_speed"`       // m/s vertical
	MaxYawRate       float64 `json:"max_yaw_rate"`      // °/s
	GeofenceEnabled  bool    `json:"geofence_enabled"`
	GeofenceLat      float64 `json:"geofence_lat"`      // center latitude
	GeofenceLng      float64 `json:"geofence_lng"`      // center longitude
	GeofenceRadius   float64 `json:"geofence_radius"`   // m
}

// DefaultSafetyConfig returns conservative defaults
func DefaultSafetyConfig() *SafetyConfig {
	return &SafetyConfig{
		MaxHeight:       120.0,
		MaxDistance:      3000.0,
		MinBattery:      30,
		CriticalBattery: 15,
		MaxWindSpeed:    10.0,
		MaxFlightTime:   1800, // 30 minutes
		MaxHSpeed:       15.0,
		MaxVSpeed:       5.0,
		MaxYawRate:      60.0,
		GeofenceEnabled: true,
		GeofenceRadius:  3000.0,
	}
}

// SafetyGuard enforces flight safety limits
type SafetyGuard struct {
	config     *SafetyConfig
	violations []SafetyViolation
	mu         sync.RWMutex
}

type SafetyViolation struct {
	Type      string    `json:"type"`
	Message   string    `json:"message"`
	Value     float64   `json:"value,omitempty"`
	Limit     float64   `json:"limit,omitempty"`
	Action    string    `json:"action"` // clamp, block, emergency_land, rth
	DeviceSN  string    `json:"device_sn,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

func NewSafetyGuard(cfg *SafetyConfig) *SafetyGuard {
	if cfg == nil {
		cfg = DefaultSafetyConfig()
	}
	return &SafetyGuard{
		config:     cfg,
		violations: make([]SafetyViolation, 0),
	}
}

// ValidateTakeoff checks if conditions allow takeoff
func (sg *SafetyGuard) ValidateTakeoff(tel *DroneTelemetry) error {
	sg.mu.Lock()
	defer sg.mu.Unlock()

	if tel.Battery.Percent <= sg.config.MinBattery {
		sg.recordViolation("low_battery_takeoff", "Battery too low for takeoff",
			float64(tel.Battery.Percent), float64(sg.config.MinBattery), "block", tel.SN)
		return fmt.Errorf("SAFETY: battery %d%% too low (min %d%%)", tel.Battery.Percent, sg.config.MinBattery)
	}
	if tel.WindSpeed > sg.config.MaxWindSpeed {
		sg.recordViolation("wind_takeoff", "Wind too strong for takeoff",
			tel.WindSpeed, sg.config.MaxWindSpeed, "block", tel.SN)
		return fmt.Errorf("SAFETY: wind %.1fm/s exceeds limit %.1fm/s", tel.WindSpeed, sg.config.MaxWindSpeed)
	}
	return nil
}

// ValidateGoto checks target position against safety limits
func (sg *SafetyGuard) ValidateGoto(lat, lng, alt float64, tel *DroneTelemetry) error {
	sg.mu.Lock()
	defer sg.mu.Unlock()

	// Height check
	if alt > sg.config.MaxHeight {
		sg.recordViolation("max_height", "Target altitude exceeds limit",
			alt, sg.config.MaxHeight, "block", tel.SN)
		return fmt.Errorf("SAFETY: target alt %.1fm exceeds max %.1fm", alt, sg.config.MaxHeight)
	}

	// Distance check from home
	dist := haversineDistance(tel.HomeLat, tel.HomeLng, lat, lng)
	if dist > sg.config.MaxDistance {
		sg.recordViolation("max_distance", "Target exceeds max distance",
			dist, sg.config.MaxDistance, "block", tel.SN)
		return fmt.Errorf("SAFETY: target %.0fm from home exceeds max %.0fm", dist, sg.config.MaxDistance)
	}

	// Geofence check
	if sg.config.GeofenceEnabled && sg.config.GeofenceLat != 0 {
		gDist := haversineDistance(sg.config.GeofenceLat, sg.config.GeofenceLng, lat, lng)
		if gDist > sg.config.GeofenceRadius {
			sg.recordViolation("geofence", "Target outside geofence",
				gDist, sg.config.GeofenceRadius, "block", tel.SN)
			return fmt.Errorf("SAFETY: target outside geofence (%.0fm > %.0fm)", gDist, sg.config.GeofenceRadius)
		}
	}

	return nil
}

// ValidateDRC checks DRC velocity command
func (sg *SafetyGuard) ValidateDRC(x, y, h, w float64, tel *DroneTelemetry) (float64, float64, float64, float64, error) {
	sg.mu.Lock()
	defer sg.mu.Unlock()

	// Battery check
	if tel.Battery.Percent <= sg.config.CriticalBattery {
		sg.recordViolation("critical_battery", "Battery critical — movement blocked",
			float64(tel.Battery.Percent), float64(sg.config.CriticalBattery), "block", tel.SN)
		return 0, 0, 0, 0, fmt.Errorf("SAFETY: critical battery %d%%", tel.Battery.Percent)
	}

	// Wind check
	if tel.WindSpeed > sg.config.MaxWindSpeed {
		sg.recordViolation("wind_flight", "Wind too strong",
			tel.WindSpeed, sg.config.MaxWindSpeed, "block", tel.SN)
		return 0, 0, 0, 0, fmt.Errorf("SAFETY: wind %.1fm/s exceeds limit", tel.WindSpeed)
	}

	// Height check
	if tel.Height >= sg.config.MaxHeight && h > 0 {
		h = 0
		sg.recordViolation("height_clamp", "Ascending clamped at max height",
			tel.Height, sg.config.MaxHeight, "clamp", tel.SN)
	}

	// Horizontal speed clamp
	hSpeed := math.Sqrt(x*x + y*y)
	if hSpeed > sg.config.MaxHSpeed {
		scale := sg.config.MaxHSpeed / hSpeed
		x *= scale
		y *= scale
		sg.recordViolation("hspeed_clamp", "Horizontal speed clamped",
			hSpeed, sg.config.MaxHSpeed, "clamp", tel.SN)
	}

	// Vertical speed clamp
	if math.Abs(h) > sg.config.MaxVSpeed {
		sign := 1.0
		if h < 0 {
			sign = -1.0
		}
		h = sign * sg.config.MaxVSpeed
		sg.recordViolation("vspeed_clamp", "Vertical speed clamped",
			math.Abs(h), sg.config.MaxVSpeed, "clamp", tel.SN)
	}

	// Yaw rate clamp
	if math.Abs(w) > sg.config.MaxYawRate {
		sign := 1.0
		if w < 0 {
			sign = -1.0
		}
		w = sign * sg.config.MaxYawRate
		sg.recordViolation("yaw_clamp", "Yaw rate clamped",
			math.Abs(w), sg.config.MaxYawRate, "clamp", tel.SN)
	}

	return x, y, h, w, nil
}

// CheckBatteryRTH returns true if battery is low enough to force RTH
func (sg *SafetyGuard) CheckBatteryRTH(percent int) bool {
	return percent <= sg.config.MinBattery
}

func (sg *SafetyGuard) recordViolation(vtype, msg string, value, limit float64, action, sn string) {
	v := SafetyViolation{
		Type:      vtype,
		Message:   msg,
		Value:     value,
		Limit:     limit,
		Action:    action,
		DeviceSN:  sn,
		Timestamp: time.Now(),
	}
	sg.violations = append(sg.violations, v)
	if len(sg.violations) > 500 {
		sg.violations = sg.violations[1:]
	}
	log.Printf("[mutalisk/safety] %s: %s (value=%.2f, limit=%.2f, action=%s, sn=%s)", vtype, msg, value, limit, action, sn)
}

// Violations returns recent safety violations
func (sg *SafetyGuard) Violations(limit int) []SafetyViolation {
	sg.mu.RLock()
	defer sg.mu.RUnlock()
	if limit <= 0 || limit > len(sg.violations) {
		limit = len(sg.violations)
	}
	if limit == 0 {
		return nil
	}
	start := len(sg.violations) - limit
	result := make([]SafetyViolation, limit)
	copy(result, sg.violations[start:])
	return result
}

// ── Fleet Manager ──

// DroneEntry represents one drone in the fleet
type DroneEntry struct {
	SN        string          `json:"sn"`
	Model     DroneModel      `json:"model"`
	Name      string          `json:"name"`
	Telemetry *DroneTelemetry `json:"telemetry"`
	Online    bool            `json:"online"`
	JoinedAt  time.Time       `json:"joined_at"`
}

// ── Mutalisk Adapter ──

// AdapterConfig holds MQTT and safety settings
type AdapterConfig struct {
	MQTTBroker   string        `json:"mqtt_broker"`
	MQTTUsername string        `json:"mqtt_username"`
	MQTTPassword string        `json:"mqtt_password"`
	AppID        string        `json:"app_id"`
	AppKey       string        `json:"app_key"`
	Safety       *SafetyConfig `json:"safety"`
}

// DefaultAdapterConfig returns development defaults
func DefaultAdapterConfig() *AdapterConfig {
	return &AdapterConfig{
		MQTTBroker: "tcp://127.0.0.1:1883",
		Safety:     DefaultSafetyConfig(),
	}
}

// Adapter bridges Claw to DJI drones via Cloud API MQTT
type Adapter struct {
	mu       sync.RWMutex
	config   *AdapterConfig
	nodeID   string
	safety   *SafetyGuard
	fleet    map[string]*DroneEntry // SN → DroneEntry
	stats    AdapterStats
	startAt  time.Time
}

// AdapterStats tracks adapter metrics
type AdapterStats struct {
	CommandsSent    int    `json:"commands_sent"`
	CommandsFailed  int    `json:"commands_failed"`
	CommandsBlocked int    `json:"commands_blocked"`
	TelemetryRx     int    `json:"telemetry_received"`
	EventsRx        int    `json:"events_received"`
	SafetyEvents    int    `json:"safety_events"`
	FleetSize       int    `json:"fleet_size"`
	OnlineCount     int    `json:"online_count"`
	Uptime          string `json:"uptime"`
	LastCommand     string `json:"last_command,omitempty"`
	LastCommandAt   time.Time `json:"last_command_at,omitempty"`
}

var (
	globalAdapter *Adapter
	adapterOnce   sync.Once
)

// InitAdapter creates the global Mutalisk adapter
func InitAdapter(nodeID string, cfg *AdapterConfig) *Adapter {
	if cfg == nil {
		cfg = DefaultAdapterConfig()
	}
	adapterOnce.Do(func() {
		globalAdapter = &Adapter{
			config:  cfg,
			nodeID:  nodeID,
			safety:  NewSafetyGuard(cfg.Safety),
			fleet:   make(map[string]*DroneEntry),
			startAt: time.Now(),
		}
		log.Printf("[mutalisk] adapter ready (mqtt=%s, fleet_size=0)", cfg.MQTTBroker)
	})
	return globalAdapter
}

// GetAdapter returns the global adapter
func GetAdapter() *Adapter {
	return globalAdapter
}

// ── Fleet Operations ──

// RegisterDrone adds a drone to the fleet
func (a *Adapter) RegisterDrone(sn string, model DroneModel, name string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.fleet[sn] = &DroneEntry{
		SN:       sn,
		Model:    model,
		Name:     name,
		Online:   false,
		JoinedAt: time.Now(),
		Telemetry: &DroneTelemetry{
			SN:    sn,
			Model: model,
			State: StateOffline,
		},
	}
	log.Printf("[mutalisk] drone registered: %s (%s) %s", sn, model, name)
}

// Fleet returns all drones
func (a *Adapter) Fleet() []*DroneEntry {
	a.mu.RLock()
	defer a.mu.RUnlock()
	result := make([]*DroneEntry, 0, len(a.fleet))
	for _, d := range a.fleet {
		result = append(result, d)
	}
	return result
}

// getDrone returns a drone by SN (or the first online one if sn is empty)
func (a *Adapter) getDrone(sn string) (*DroneEntry, error) {
	if sn == "" {
		// Find first online drone
		for _, d := range a.fleet {
			if d.Online {
				return d, nil
			}
		}
		if len(a.fleet) > 0 {
			for _, d := range a.fleet {
				return d, nil
			}
		}
		return nil, fmt.Errorf("no drones registered")
	}
	d, ok := a.fleet[sn]
	if !ok {
		return nil, fmt.Errorf("drone %s not found", sn)
	}
	return d, nil
}

// UpdateTelemetry updates a drone's telemetry (called from MQTT handler)
func (a *Adapter) UpdateTelemetry(sn string, data json.RawMessage) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	d, ok := a.fleet[sn]
	if !ok {
		// Auto-register unknown drone
		d = &DroneEntry{
			SN:        sn,
			Online:    true,
			JoinedAt:  time.Now(),
			Telemetry: &DroneTelemetry{SN: sn, State: StateOnline},
		}
		a.fleet[sn] = d
	}

	if err := json.Unmarshal(data, d.Telemetry); err != nil {
		return err
	}
	d.Telemetry.SN = sn
	d.Telemetry.Timestamp = time.Now()
	d.Online = true

	// Determine state from telemetry
	if d.Telemetry.IsFlying {
		hSpd := math.Sqrt(d.Telemetry.SpeedX*d.Telemetry.SpeedX + d.Telemetry.SpeedY*d.Telemetry.SpeedY)
		if hSpd > 0.3 {
			d.Telemetry.State = StateFlying
		} else {
			d.Telemetry.State = StateHovering
		}
		if d.Telemetry.ModeCode == ModeRTH {
			d.Telemetry.State = StateReturning
		}
	} else {
		d.Telemetry.State = StateIdle
	}

	a.stats.TelemetryRx++

	// Safety: auto-RTH check
	if a.safety.CheckBatteryRTH(d.Telemetry.Battery.Percent) && d.Telemetry.IsFlying {
		a.safety.recordViolation("auto_rth", "Low battery — auto RTH triggered",
			float64(d.Telemetry.Battery.Percent), float64(a.safety.config.MinBattery), "rth", sn)
		a.stats.SafetyEvents++
	}

	return nil
}

// ── Flight Commands ──

// buildMQTTCommand creates a DJI Cloud API MQTT service command structure
func buildMQTTCommand(method string, params map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"tid":       fmt.Sprintf("tid-%d", time.Now().UnixMilli()),
		"bid":       fmt.Sprintf("bid-%d", time.Now().UnixMilli()),
		"timestamp": time.Now().UnixMilli(),
		"method":    method,
		"data":      params,
	}
}

// Takeoff commands the drone to take off to a specified height
func (a *Adapter) Takeoff(sn string, height float64) (map[string]interface{}, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	d, err := a.getDrone(sn)
	if err != nil {
		return nil, err
	}

	// Safety check
	if err := a.safety.ValidateTakeoff(d.Telemetry); err != nil {
		a.stats.CommandsBlocked++
		return nil, err
	}

	// Height check
	if height > a.safety.config.MaxHeight {
		height = a.safety.config.MaxHeight
	}
	if height < 1.5 {
		height = 1.5
	}

	a.stats.LastCommand = "takeoff"
	a.stats.LastCommandAt = time.Now()
	a.stats.CommandsSent++

	cmd := buildMQTTCommand("takeoff", map[string]interface{}{
		"target_height": height,
	})
	// In production: publish to MQTT topic thing/product/{sn}/services
	log.Printf("[mutalisk] CMD takeoff: sn=%s height=%.1f", d.SN, height)
	return cmd, nil
}

// Land commands the drone to land at current position
func (a *Adapter) Land(sn string) (map[string]interface{}, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	d, err := a.getDrone(sn)
	if err != nil {
		return nil, err
	}

	a.stats.LastCommand = "land"
	a.stats.LastCommandAt = time.Now()
	a.stats.CommandsSent++

	cmd := buildMQTTCommand("land", nil)
	log.Printf("[mutalisk] CMD land: sn=%s", d.SN)
	return cmd, nil
}

// ReturnHome commands the drone to RTH at specified height
func (a *Adapter) ReturnHome(sn string, height float64) (map[string]interface{}, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	d, err := a.getDrone(sn)
	if err != nil {
		return nil, err
	}

	if height > a.safety.config.MaxHeight {
		height = a.safety.config.MaxHeight
	}
	if height < 20 {
		height = 20
	}

	a.stats.LastCommand = "rth"
	a.stats.LastCommandAt = time.Now()
	a.stats.CommandsSent++

	cmd := buildMQTTCommand("return_home", map[string]interface{}{
		"height": height,
	})
	log.Printf("[mutalisk] CMD rth: sn=%s height=%.1f", d.SN, height)
	return cmd, nil
}

// Goto commands the drone to fly to a GPS position
func (a *Adapter) Goto(sn string, lat, lng, alt float64) (map[string]interface{}, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	d, err := a.getDrone(sn)
	if err != nil {
		return nil, err
	}

	// Safety checks
	if err := a.safety.ValidateGoto(lat, lng, alt, d.Telemetry); err != nil {
		a.stats.CommandsBlocked++
		return nil, err
	}

	a.stats.LastCommand = "goto"
	a.stats.LastCommandAt = time.Now()
	a.stats.CommandsSent++

	cmd := buildMQTTCommand("fly_to_point", map[string]interface{}{
		"latitude":  lat,
		"longitude": lng,
		"altitude":  alt,
	})
	log.Printf("[mutalisk] CMD goto: sn=%s → (%.6f, %.6f, %.1f)", d.SN, lat, lng, alt)
	return cmd, nil
}

// DRCControl sends virtual stick commands (DRC mode)
func (a *Adapter) DRCControl(sn string, x, y, h, w float64) (map[string]interface{}, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	d, err := a.getDrone(sn)
	if err != nil {
		return nil, err
	}

	// Safety check + clamp
	safeX, safeY, safeH, safeW, err := a.safety.ValidateDRC(x, y, h, w, d.Telemetry)
	if err != nil {
		a.stats.CommandsBlocked++
		return nil, err
	}

	a.stats.LastCommand = "drc"
	a.stats.LastCommandAt = time.Now()
	a.stats.CommandsSent++

	cmd := buildMQTTCommand("drc_drone_control", map[string]interface{}{
		"x": safeX,
		"y": safeY,
		"h": safeH,
		"w": safeW,
	})
	return cmd, nil
}

// Stop sends hover/brake command
func (a *Adapter) Stop(sn string) (map[string]interface{}, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	d, err := a.getDrone(sn)
	if err != nil {
		return nil, err
	}

	a.stats.LastCommand = "stop"
	a.stats.LastCommandAt = time.Now()
	a.stats.CommandsSent++

	cmd := buildMQTTCommand("drc_drone_control", map[string]interface{}{
		"x": 0, "y": 0, "h": 0, "w": 0,
	})
	log.Printf("[mutalisk] CMD stop/hover: sn=%s", d.SN)
	return cmd, nil
}

// Photo takes a camera photo
func (a *Adapter) Photo(sn string, cameraID string) (map[string]interface{}, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	d, err := a.getDrone(sn)
	if err != nil {
		return nil, err
	}

	if cameraID == "" {
		cameraID = "wide"
	}

	a.stats.LastCommand = "photo"
	a.stats.LastCommandAt = time.Now()
	a.stats.CommandsSent++

	cmd := buildMQTTCommand("camera_photo_take", map[string]interface{}{
		"payload_index": cameraID,
	})
	log.Printf("[mutalisk] CMD photo: sn=%s camera=%s", d.SN, cameraID)
	return cmd, nil
}

// VideoStart starts recording
func (a *Adapter) VideoStart(sn string, cameraID string) (map[string]interface{}, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	d, err := a.getDrone(sn)
	if err != nil {
		return nil, err
	}
	if cameraID == "" {
		cameraID = "wide"
	}

	a.stats.LastCommand = "video_start"
	a.stats.LastCommandAt = time.Now()
	a.stats.CommandsSent++

	cmd := buildMQTTCommand("camera_recording_start", map[string]interface{}{
		"payload_index": cameraID,
	})
	log.Printf("[mutalisk] CMD video_start: sn=%s camera=%s", d.SN, cameraID)
	return cmd, nil
}

// VideoStop stops recording
func (a *Adapter) VideoStop(sn string, cameraID string) (map[string]interface{}, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	d, err := a.getDrone(sn)
	if err != nil {
		return nil, err
	}
	if cameraID == "" {
		cameraID = "wide"
	}

	a.stats.LastCommand = "video_stop"
	a.stats.LastCommandAt = time.Now()
	a.stats.CommandsSent++

	cmd := buildMQTTCommand("camera_recording_stop", map[string]interface{}{
		"payload_index": cameraID,
	})
	log.Printf("[mutalisk] CMD video_stop: sn=%s camera=%s", d.SN, cameraID)
	return cmd, nil
}

// GimbalRotate sets gimbal angle
func (a *Adapter) GimbalRotate(sn string, pitch, yaw float64) (map[string]interface{}, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	d, err := a.getDrone(sn)
	if err != nil {
		return nil, err
	}

	// Clamp gimbal pitch: -90 to 30 degrees
	if pitch < -90 {
		pitch = -90
	}
	if pitch > 30 {
		pitch = 30
	}

	a.stats.LastCommand = "gimbal"
	a.stats.LastCommandAt = time.Now()
	a.stats.CommandsSent++

	cmd := buildMQTTCommand("gimbal_rotate", map[string]interface{}{
		"pitch": pitch,
		"yaw":   yaw,
	})
	log.Printf("[mutalisk] CMD gimbal: sn=%s pitch=%.1f yaw=%.1f", d.SN, pitch, yaw)
	return cmd, nil
}

// LiveStart starts RTMP push streaming
func (a *Adapter) LiveStart(sn string, url string) (map[string]interface{}, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	d, err := a.getDrone(sn)
	if err != nil {
		return nil, err
	}

	a.stats.LastCommand = "live_start"
	a.stats.LastCommandAt = time.Now()
	a.stats.CommandsSent++

	cmd := buildMQTTCommand("live_start_push", map[string]interface{}{
		"url_type": 0, // RTMP
		"url":      url,
	})
	log.Printf("[mutalisk] CMD live_start: sn=%s → %s", d.SN, url)
	return cmd, nil
}

// Wayline executes a waypoint mission
func (a *Adapter) Wayline(sn string, waypoints []Waypoint) (map[string]interface{}, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	d, err := a.getDrone(sn)
	if err != nil {
		return nil, err
	}

	// Validate all waypoints
	for i, wp := range waypoints {
		if err := a.safety.ValidateGoto(wp.Latitude, wp.Longitude, wp.Altitude, d.Telemetry); err != nil {
			a.stats.CommandsBlocked++
			return nil, fmt.Errorf("waypoint %d: %w", i, err)
		}
	}

	a.stats.LastCommand = fmt.Sprintf("wayline:%d_waypoints", len(waypoints))
	a.stats.LastCommandAt = time.Now()
	a.stats.CommandsSent++

	cmd := buildMQTTCommand("flighttask_create", map[string]interface{}{
		"waypoints": waypoints,
		"auto_start": true,
	})
	log.Printf("[mutalisk] CMD wayline: sn=%s waypoints=%d", d.SN, len(waypoints))
	return cmd, nil
}

// ── Status ──

// Status returns telemetry for a specific drone (or first online)
func (a *Adapter) Status(sn string) (*DroneTelemetry, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	d, err := a.getDrone(sn)
	if err != nil {
		return nil, err
	}
	return d.Telemetry, nil
}

// Stats returns adapter metrics
func (a *Adapter) Stats() *AdapterStats {
	a.mu.RLock()
	defer a.mu.RUnlock()
	s := a.stats
	s.FleetSize = len(a.fleet)
	online := 0
	for _, d := range a.fleet {
		if d.Online {
			online++
		}
	}
	s.OnlineCount = online
	s.Uptime = time.Since(a.startAt).Round(time.Second).String()
	return &s
}

// SafetyConfig returns current safety config
func (a *Adapter) SafetyConfig() *SafetyConfig {
	return a.safety.config
}

// SafetyViolations returns recent violations
func (a *Adapter) SafetyViolations(limit int) []SafetyViolation {
	return a.safety.Violations(limit)
}

// ── Waypoint Model ──

type Waypoint struct {
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	Altitude    float64 `json:"altitude"`    // relative height
	Speed       float64 `json:"speed"`       // m/s
	GimbalPitch float64 `json:"gimbal_pitch"`
	TakePhoto   bool    `json:"take_photo"`
}

// ── Geo Helpers ──

// haversineDistance computes distance in meters between two GPS coordinates
func haversineDistance(lat1, lng1, lat2, lng2 float64) float64 {
	const R = 6371000 // earth radius meters
	dLat := (lat2 - lat1) * math.Pi / 180
	dLng := (lng2 - lng1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLng/2)*math.Sin(dLng/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return R * c
}
