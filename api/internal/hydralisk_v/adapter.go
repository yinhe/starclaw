package hydralisk_v

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"sync"
	"time"
)

// ════════════════════════════════════════════════════════════
// Hydralisk Vehicle Adapter — Claw ↔ 移动载具适配层
//
// 架构:
//   Claw Agent Runtime → Hydralisk Adapter (Go, 本包)
//        ↓ HTTP JSON
//   Vehicle Bridge (Python/C++, Autoware/CAN)
//        ↓ ROS2 DDS / CAN Bus
//   线控底盘 (DBW) → EPS/油门/制动/档位
//
// 核心职责:
//   1. 将 MCP Tool 调用转换为 Ackermann 控制指令
//   2. Safety Guard: 4层防护 (Agent信任→速度围栏→AD栈→底盘)
//   3. 车辆状态: 速度/转向/电量/定位/故障码
//   4. 导航: GPS航点/路线跟踪
//   5. 辅助: 档位/车灯/喇叭/货舱
// ════════════════════════════════════════════════════════════

// ── Vehicle Types ──

type VehicleType string

const (
	VehiclePatrol   VehicleType = "patrol"    // 巡逻车
	VehicleDelivery VehicleType = "delivery"  // 配送车
	VehicleSweeper  VehicleType = "sweeper"   // 清扫车
	VehicleMining   VehicleType = "mining"    // 矿车
	VehicleFarming  VehicleType = "farming"   // 农机
	VehicleCustom   VehicleType = "custom"    // 改装车
)

type GearPosition string

const (
	GearPark    GearPosition = "P"
	GearReverse GearPosition = "R"
	GearNeutral GearPosition = "N"
	GearDrive   GearPosition = "D"
)

type VehicleState string

const (
	StateOffline    VehicleState = "offline"
	StateParked     VehicleState = "parked"
	StateIdle       VehicleState = "idle"        // engine on, not moving
	StateDriving    VehicleState = "driving"
	StateNavigating VehicleState = "navigating"
	StateBraking    VehicleState = "braking"
	StateEstop      VehicleState = "emergency_stop"
	StateCharging   VehicleState = "charging"
	StateFault      VehicleState = "fault"
)

type NavStatus string

const (
	NavIdle      NavStatus = "idle"
	NavActive    NavStatus = "active"
	NavArrived   NavStatus = "arrived"
	NavFailed    NavStatus = "failed"
	NavCanceled  NavStatus = "canceled"
	NavRerouting NavStatus = "rerouting"
)

// ── Telemetry ──

type VehicleStatus struct {
	State       VehicleState `json:"state"`
	Type        VehicleType  `json:"vehicle_type"`
	Speed       float64      `json:"speed_mps"`       // m/s
	SpeedKmh    float64      `json:"speed_kmh"`       // km/h (convenience)
	Steering    float64      `json:"steering_deg"`    // degrees
	Gear        GearPosition `json:"gear"`
	Throttle    float64      `json:"throttle_pct"`    // 0-100%
	Brake       float64      `json:"brake_pct"`       // 0-100%
	Battery     BatteryInfo  `json:"battery"`
	Position    GPSPosition  `json:"position"`
	Heading     float64      `json:"heading_deg"`     // 0-360°
	IMU         IMUData      `json:"imu"`
	NavStatus   NavStatus    `json:"nav_status"`
	NavGoal     *GPSPosition `json:"nav_goal,omitempty"`
	Lights      LightState   `json:"lights"`
	FaultCodes  []string     `json:"fault_codes,omitempty"`
	Odometer    float64      `json:"odometer_km"`
	Timestamp   time.Time    `json:"timestamp"`
}

type GPSPosition struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Altitude  float64 `json:"altitude"`
	Accuracy  float64 `json:"accuracy_m"`   // horizontal accuracy
	RTKFixed  bool    `json:"rtk_fixed"`
}

type BatteryInfo struct {
	SOC         int     `json:"soc"`          // state of charge %
	Voltage     float64 `json:"voltage"`
	Current     float64 `json:"current"`
	Temperature float64 `json:"temperature"`
	Charging    bool    `json:"charging"`
	Range       float64 `json:"range_km"`    // estimated remaining range
}

type IMUData struct {
	AccX  float64 `json:"acc_x"`
	AccY  float64 `json:"acc_y"`
	AccZ  float64 `json:"acc_z"`
	Roll  float64 `json:"roll_deg"`
	Pitch float64 `json:"pitch_deg"`
	Yaw   float64 `json:"yaw_deg"`
}

type LightState struct {
	Headlight   bool   `json:"headlight"`
	TailLight   bool   `json:"tail_light"`
	LeftSignal  bool   `json:"left_signal"`
	RightSignal bool   `json:"right_signal"`
	Hazard      bool   `json:"hazard"`
	Brake       bool   `json:"brake_light"`
}

// ── Safety Guard ──

type SafetyConfig struct {
	MaxSpeed        float64 `json:"max_speed_mps"`       // m/s
	MaxSteering     float64 `json:"max_steering_deg"`    // degrees
	MaxAcceleration float64 `json:"max_acceleration"`    // m/s²
	MaxDeceleration float64 `json:"max_deceleration"`    // m/s²
	MinBattery      int     `json:"min_battery_soc"`     // % — force return
	CriticalBattery int     `json:"critical_battery_soc"` // % — emergency stop
	GeofenceEnabled bool    `json:"geofence_enabled"`
	GeofenceLat     float64 `json:"geofence_center_lat"`
	GeofenceLng     float64 `json:"geofence_center_lng"`
	GeofenceRadius  float64 `json:"geofence_radius_m"`
	MaxGradient     float64 `json:"max_gradient_deg"`    // max slope
	ControlFreqHz   int     `json:"control_freq_hz"`
}

func DefaultSafetyConfig() *SafetyConfig {
	return &SafetyConfig{
		MaxSpeed:        8.33,   // 30 km/h
		MaxSteering:     30.0,
		MaxAcceleration: 2.0,
		MaxDeceleration: 4.0,
		MinBattery:      20,
		CriticalBattery: 10,
		GeofenceEnabled: true,
		GeofenceLat:     31.2304,  // default: Shanghai
		GeofenceLng:     121.4737,
		GeofenceRadius:  5000,     // 5km
		MaxGradient:     20.0,
		ControlFreqHz:   50,
	}
}

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
	Action    string    `json:"action"`
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

// ValidateDrive checks and clamps a drive command
func (sg *SafetyGuard) ValidateDrive(speed, steering float64, status *VehicleStatus) (float64, float64, error) {
	sg.mu.Lock()
	defer sg.mu.Unlock()

	// Fault check
	if status != nil && status.State == StateFault {
		sg.recordViolation("fault", "Vehicle in fault state", 0, 0, "block")
		return 0, 0, fmt.Errorf("SAFETY: vehicle in fault state, codes: %v", status.FaultCodes)
	}

	// E-stop check
	if status != nil && status.State == StateEstop {
		sg.recordViolation("estop", "Vehicle in emergency stop", 0, 0, "block")
		return 0, 0, fmt.Errorf("SAFETY: vehicle in emergency stop — manual reset required")
	}

	// Critical battery
	if status != nil && status.Battery.SOC <= sg.config.CriticalBattery {
		sg.recordViolation("critical_battery", "Battery critically low",
			float64(status.Battery.SOC), float64(sg.config.CriticalBattery), "block")
		return 0, 0, fmt.Errorf("SAFETY: critical battery SOC %d%%", status.Battery.SOC)
	}

	// Gradient/slope check
	if status != nil {
		gradient := math.Max(math.Abs(status.IMU.Roll), math.Abs(status.IMU.Pitch))
		if gradient > sg.config.MaxGradient {
			sg.recordViolation("gradient", "Excessive slope",
				gradient, sg.config.MaxGradient, "block")
			return 0, 0, fmt.Errorf("SAFETY: slope %.1f° exceeds limit %.1f°", gradient, sg.config.MaxGradient)
		}
	}

	// Geofence check (GPS)
	if sg.config.GeofenceEnabled && status != nil && status.Position.Latitude != 0 {
		dist := haversineDistance(
			status.Position.Latitude, status.Position.Longitude,
			sg.config.GeofenceLat, sg.config.GeofenceLng,
		)
		if dist > sg.config.GeofenceRadius {
			sg.recordViolation("geofence", "Outside geofence",
				dist, sg.config.GeofenceRadius, "block")
			return 0, 0, fmt.Errorf("SAFETY: outside geofence (%.0fm > %.0fm)", dist, sg.config.GeofenceRadius)
		}
	}

	// Speed clamping
	if math.Abs(speed) > sg.config.MaxSpeed {
		sign := 1.0
		if speed < 0 {
			sign = -1.0
		}
		speed = sign * sg.config.MaxSpeed
		sg.recordViolation("speed_clamp", "Speed clamped",
			math.Abs(speed), sg.config.MaxSpeed, "clamp")
	}

	// Steering clamping
	if math.Abs(steering) > sg.config.MaxSteering {
		sign := 1.0
		if steering < 0 {
			sign = -1.0
		}
		steering = sign * sg.config.MaxSteering
		sg.recordViolation("steering_clamp", "Steering clamped",
			math.Abs(steering), sg.config.MaxSteering, "clamp")
	}

	return speed, steering, nil
}

// ValidateNavGoal checks if a navigation goal is within safety limits
func (sg *SafetyGuard) ValidateNavGoal(lat, lng float64, status *VehicleStatus) error {
	sg.mu.Lock()
	defer sg.mu.Unlock()

	if status != nil && status.Battery.SOC <= sg.config.MinBattery {
		sg.recordViolation("low_battery_nav", "Battery too low for navigation",
			float64(status.Battery.SOC), float64(sg.config.MinBattery), "block")
		return fmt.Errorf("SAFETY: SOC %d%% too low for navigation", status.Battery.SOC)
	}

	if sg.config.GeofenceEnabled {
		dist := haversineDistance(lat, lng, sg.config.GeofenceLat, sg.config.GeofenceLng)
		if dist > sg.config.GeofenceRadius {
			sg.recordViolation("geofence_goal", "Nav goal outside geofence",
				dist, sg.config.GeofenceRadius, "block")
			return fmt.Errorf("SAFETY: goal outside geofence (%.0fm)", dist)
		}
	}

	return nil
}

func (sg *SafetyGuard) CheckBatteryReturn(soc int) bool {
	return soc <= sg.config.MinBattery
}

func (sg *SafetyGuard) recordViolation(vtype, msg string, value, limit float64, action string) {
	v := SafetyViolation{
		Type:      vtype,
		Message:   msg,
		Value:     value,
		Limit:     limit,
		Action:    action,
		Timestamp: time.Now(),
	}
	sg.violations = append(sg.violations, v)
	if len(sg.violations) > 500 {
		sg.violations = sg.violations[1:]
	}
	log.Printf("[hydralisk/safety] %s: %s (value=%.2f, limit=%.2f, action=%s)", vtype, msg, value, limit, action)
}

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

// ── Haversine ──

func haversineDistance(lat1, lng1, lat2, lng2 float64) float64 {
	const R = 6371000 // Earth radius in meters
	dLat := (lat2 - lat1) * math.Pi / 180
	dLng := (lng2 - lng1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLng/2)*math.Sin(dLng/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return R * c
}

// ── Adapter ──

type AdapterConfig struct {
	BridgeURL    string        `json:"bridge_url"`
	VehicleType  VehicleType   `json:"vehicle_type"`
	PollInterval time.Duration `json:"poll_interval"`
	Safety       *SafetyConfig `json:"safety"`
}

func DefaultAdapterConfig() *AdapterConfig {
	return &AdapterConfig{
		BridgeURL:    "http://127.0.0.1:9202",
		VehicleType:  VehiclePatrol,
		PollInterval: 50 * time.Millisecond,
		Safety:       DefaultSafetyConfig(),
	}
}

type Adapter struct {
	mu         sync.RWMutex
	config     *AdapterConfig
	nodeID     string
	safety     *SafetyGuard
	status     *VehicleStatus
	httpClient *http.Client
	startAt    time.Time
	stats      AdapterStats
}

type AdapterStats struct {
	CommandsSent    int       `json:"commands_sent"`
	CommandsFailed  int       `json:"commands_failed"`
	CommandsBlocked int       `json:"commands_blocked"`
	StatusPolls     int       `json:"status_polls"`
	PollErrors      int       `json:"poll_errors"`
	SafetyEvents    int       `json:"safety_events"`
	NavGoalsSent    int       `json:"nav_goals_sent"`
	NavGoalsReached int       `json:"nav_goals_reached"`
	Connected       bool      `json:"connected"`
	Uptime          string    `json:"uptime"`
	LastCommand     string    `json:"last_command,omitempty"`
	LastCommandAt   time.Time `json:"last_command_at,omitempty"`
}

var (
	globalAdapter *Adapter
	adapterOnce   sync.Once
)

// InitAdapter creates the global Hydralisk vehicle adapter
func InitAdapter(nodeID string, cfg *AdapterConfig) *Adapter {
	if cfg == nil {
		cfg = DefaultAdapterConfig()
	}
	adapterOnce.Do(func() {
		globalAdapter = &Adapter{
			config:     cfg,
			nodeID:     nodeID,
			safety:     NewSafetyGuard(cfg.Safety),
			httpClient: &http.Client{Timeout: 3 * time.Second},
			startAt:    time.Now(),
			status: &VehicleStatus{
				State: StateOffline,
				Type:  cfg.VehicleType,
				Gear:  GearPark,
			},
		}
		log.Printf("[hydralisk] vehicle adapter ready (bridge=%s, type=%s)", cfg.BridgeURL, cfg.VehicleType)
	})
	return globalAdapter
}

func GetAdapter() *Adapter {
	return globalAdapter
}

// ── Bridge Communication ──

func (a *Adapter) bridgeRequest(method, path string, body interface{}) (map[string]interface{}, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = &byteReader{data: data}
	}

	req, err := http.NewRequest(method, a.config.BridgeURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		if method != "GET" {
			a.stats.CommandsFailed++
		} else {
			a.stats.PollErrors++
		}
		return nil, fmt.Errorf("bridge unreachable: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if method != "GET" {
		a.stats.CommandsSent++
	}
	return result, nil
}

// ── Commands ──

// Drive sends a speed+steering command (with safety validation)
func (a *Adapter) Drive(speed, steering float64) (map[string]interface{}, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	safeSpeed, safeSteering, err := a.safety.ValidateDrive(speed, steering, a.status)
	if err != nil {
		a.stats.CommandsBlocked++
		a.stats.SafetyEvents++
		return nil, err
	}

	a.stats.LastCommand = "drive"
	a.stats.LastCommandAt = time.Now()

	return a.bridgeRequest("POST", "/cmd/drive", map[string]interface{}{
		"speed_mps":    safeSpeed,
		"steering_deg": safeSteering,
	})
}

// Goto navigates to a GPS coordinate
func (a *Adapter) Goto(lat, lng, speedLimit float64) (map[string]interface{}, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if err := a.safety.ValidateNavGoal(lat, lng, a.status); err != nil {
		a.stats.CommandsBlocked++
		return nil, err
	}

	if speedLimit <= 0 || speedLimit > a.safety.config.MaxSpeed {
		speedLimit = a.safety.config.MaxSpeed
	}

	a.stats.LastCommand = "goto"
	a.stats.LastCommandAt = time.Now()
	a.stats.NavGoalsSent++

	return a.bridgeRequest("POST", "/cmd/goto", map[string]interface{}{
		"latitude":    lat,
		"longitude":   lng,
		"speed_limit": speedLimit,
	})
}

// Route sends a multi-waypoint route
func (a *Adapter) Route(waypoints []GPSPosition, speedLimit float64) (map[string]interface{}, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for i, wp := range waypoints {
		if err := a.safety.ValidateNavGoal(wp.Latitude, wp.Longitude, a.status); err != nil {
			a.stats.CommandsBlocked++
			return nil, fmt.Errorf("waypoint %d: %w", i, err)
		}
	}

	if speedLimit <= 0 || speedLimit > a.safety.config.MaxSpeed {
		speedLimit = a.safety.config.MaxSpeed
	}

	a.stats.LastCommand = fmt.Sprintf("route:%d_waypoints", len(waypoints))
	a.stats.LastCommandAt = time.Now()
	a.stats.NavGoalsSent += len(waypoints)

	return a.bridgeRequest("POST", "/cmd/route", map[string]interface{}{
		"waypoints":   waypoints,
		"speed_limit": speedLimit,
	})
}

// Stop sends emergency stop (maximum braking)
func (a *Adapter) Stop() (map[string]interface{}, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.stats.LastCommand = "stop"
	a.stats.LastCommandAt = time.Now()

	return a.bridgeRequest("POST", "/cmd/stop", nil)
}

// Park pulls over and shifts to P
func (a *Adapter) Park() (map[string]interface{}, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.stats.LastCommand = "park"
	a.stats.LastCommandAt = time.Now()

	return a.bridgeRequest("POST", "/cmd/park", nil)
}

// SetGear changes gear position
func (a *Adapter) SetGear(gear GearPosition) (map[string]interface{}, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Safety: only allow gear change when nearly stopped
	if a.status != nil && math.Abs(a.status.Speed) > 0.5 && gear != GearDrive {
		a.stats.CommandsBlocked++
		return nil, fmt.Errorf("SAFETY: cannot change gear to %s while moving (%.1f m/s)", gear, a.status.Speed)
	}

	a.stats.LastCommand = "gear:" + string(gear)
	a.stats.LastCommandAt = time.Now()

	return a.bridgeRequest("POST", "/cmd/gear", map[string]interface{}{
		"gear": gear,
	})
}

// SetLights controls vehicle lights
func (a *Adapter) SetLights(headlight, leftSignal, rightSignal, hazard bool) (map[string]interface{}, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.stats.LastCommand = "lights"
	a.stats.LastCommandAt = time.Now()

	return a.bridgeRequest("POST", "/cmd/lights", map[string]interface{}{
		"headlight":    headlight,
		"left_signal":  leftSignal,
		"right_signal": rightSignal,
		"hazard":       hazard,
	})
}

// Horn sounds the horn
func (a *Adapter) Horn(durationMs int) (map[string]interface{}, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if durationMs <= 0 {
		durationMs = 500
	}
	if durationMs > 3000 {
		durationMs = 3000 // max 3 seconds
	}

	a.stats.LastCommand = "horn"
	a.stats.LastCommandAt = time.Now()

	return a.bridgeRequest("POST", "/cmd/horn", map[string]interface{}{
		"duration_ms": durationMs,
	})
}

// Cargo controls the cargo bay
func (a *Adapter) Cargo(action string) (map[string]interface{}, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	switch action {
	case "open", "close", "lock", "unlock":
	default:
		return nil, fmt.Errorf("invalid cargo action: %s (must be open/close/lock/unlock)", action)
	}

	a.stats.LastCommand = "cargo:" + action
	a.stats.LastCommandAt = time.Now()

	return a.bridgeRequest("POST", "/cmd/cargo", map[string]interface{}{
		"action": action,
	})
}

// GetCamera captures from vehicle camera
func (a *Adapter) GetCamera(cameraID string) (map[string]interface{}, error) {
	if cameraID == "" {
		cameraID = "front"
	}
	return a.bridgeRequest("GET", "/sensor/camera?id="+cameraID, nil)
}

// SetSpeedLimit updates the maximum allowed speed
func (a *Adapter) SetSpeedLimit(maxSpeedMps float64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if maxSpeedMps > 0 && maxSpeedMps <= 20.0 { // absolute max 72 km/h
		a.safety.config.MaxSpeed = maxSpeedMps
		log.Printf("[hydralisk] speed limit updated to %.1f m/s (%.0f km/h)", maxSpeedMps, maxSpeedMps*3.6)
	}
}

// ── Status ──

func (a *Adapter) Status() *VehicleStatus {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.status
}

func (a *Adapter) Stats() *AdapterStats {
	a.mu.RLock()
	defer a.mu.RUnlock()
	s := a.stats
	s.Connected = a.status.State != StateOffline
	s.Uptime = time.Since(a.startAt).Round(time.Second).String()
	return &s
}

func (a *Adapter) SafetyConfig() *SafetyConfig {
	return a.safety.config
}

func (a *Adapter) SafetyViolations(limit int) []SafetyViolation {
	return a.safety.Violations(limit)
}

// ── Helpers ──

type byteReader struct {
	data []byte
	pos  int
}

func (r *byteReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}
