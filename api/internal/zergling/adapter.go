package zergling

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
// Zergling Adapter — Claw ↔ Unitree 四足机器人适配层
//
// 架构:
//   Claw Agent Runtime → Zergling Adapter (Go, 本包)
//        ↓ HTTP JSON-RPC
//   Zergling DDS Bridge (Python, unitree_sdk2_python)
//        ↓ DDS (CycloneDDS)
//   Unitree Go2 / B2 / G1 (物理机器人)
//
// 核心职责:
//   1. 将 MCP Tool 调用转换为 Unitree SDK 指令
//   2. Safety Guard: 速度/电量/围栏/碰撞硬限制
//   3. 状态缓存: 定期拉取机器人状态, 减少延迟
//   4. 事件上报: 跌倒/低电量/围栏越界 → Pheromone
// ════════════════════════════════════════════════════════════

// ── Robot Types ──

type RobotModel string

const (
	ModelGo2  RobotModel = "go2"
	ModelGo2E RobotModel = "go2_edu"
	ModelB2   RobotModel = "b2"
	ModelG1   RobotModel = "g1"
	ModelH1   RobotModel = "h1"
)

type RobotState string

const (
	StateDisconnected RobotState = "disconnected"
	StateConnected    RobotState = "connected"
	StateStanding     RobotState = "standing"
	StateMoving       RobotState = "moving"
	StateSitting      RobotState = "sitting"
	StateLying        RobotState = "lying"
	StateFallen       RobotState = "fallen"
	StateCharging     RobotState = "charging"
	StateEmergency    RobotState = "emergency"
)

type GaitType string

const (
	GaitIdle     GaitType = "idle"
	GaitTrot     GaitType = "trot"
	GaitRun      GaitType = "trot_running"
	GaitClimb    GaitType = "climb_stair"
	GaitObstacle GaitType = "trot_obstacle"
)

// ── Status Data ──

// RobotStatus represents the full state of a connected robot
type RobotStatus struct {
	Model       RobotModel `json:"model"`
	State       RobotState `json:"state"`
	Position    Vec3       `json:"position"`
	Velocity    Vec3       `json:"velocity"`
	Orientation Euler      `json:"orientation"`
	BodyHeight  float64    `json:"body_height"`
	GaitType    GaitType   `json:"gait_type"`
	Battery     Battery    `json:"battery"`
	FootForce   [4]float64 `json:"foot_force"` // FR, FL, RR, RL
	IMU         IMUData    `json:"imu"`
	Obstacle    bool       `json:"obstacle_avoidance"`
	Terrain     bool       `json:"terrain_adapt"`
	Timestamp   time.Time  `json:"timestamp"`
}

type Vec3 struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

type Euler struct {
	Roll  float64 `json:"roll"`
	Pitch float64 `json:"pitch"`
	Yaw   float64 `json:"yaw"`
}

type Battery struct {
	SOC         int     `json:"soc"` // 0-100%
	Voltage     float64 `json:"voltage"`
	Current     float64 `json:"current"`
	Temperature float64 `json:"temperature"`
}

type IMUData struct {
	AccX  float64    `json:"acc_x"`
	AccY  float64    `json:"acc_y"`
	AccZ  float64    `json:"acc_z"`
	GyroX float64    `json:"gyro_x"`
	GyroY float64    `json:"gyro_y"`
	GyroZ float64    `json:"gyro_z"`
	Quat  [4]float64 `json:"quaternion"` // w, x, y, z
}

// ── Safety Guard ──

// SafetyConfig defines hard limits that cannot be overridden by agents
type SafetyConfig struct {
	MaxLinearSpeed  float64 `json:"max_linear_speed"`  // m/s
	MaxAngularSpeed float64 `json:"max_angular_speed"` // rad/s
	MinBattery      int     `json:"min_battery"`       // % — force return below this
	CriticalBattery int     `json:"critical_battery"`  // % — emergency stop
	GeofenceRadius  float64 `json:"geofence_radius"`   // m from origin
	GeofenceEnabled bool    `json:"geofence_enabled"`
	CliffThreshold  float64 `json:"cliff_threshold"` // m — edge detection
	MaxTiltAngle    float64 `json:"max_tilt_angle"`  // degrees — rollover protection
	CollisionForce  float64 `json:"collision_force"` // N — emergency stop threshold
}

// DefaultSafetyConfig returns conservative safety defaults
func DefaultSafetyConfig() *SafetyConfig {
	return &SafetyConfig{
		MaxLinearSpeed:  1.5,
		MaxAngularSpeed: 2.0,
		MinBattery:      15,
		CriticalBattery: 5,
		GeofenceRadius:  50.0,
		GeofenceEnabled: true,
		CliffThreshold:  0.3,
		MaxTiltAngle:    45.0,
		CollisionForce:  50.0,
	}
}

// SafetyGuard enforces hard limits on all robot commands
type SafetyGuard struct {
	config     *SafetyConfig
	origin     Vec3 // geofence center
	violations []SafetyViolation
	mu         sync.RWMutex
}

type SafetyViolation struct {
	Type      string    `json:"type"`
	Message   string    `json:"message"`
	Value     float64   `json:"value,omitempty"`
	Limit     float64   `json:"limit,omitempty"`
	Action    string    `json:"action"` // clamp, block, emergency_stop
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

// ValidateMove checks and clamps a velocity command
func (sg *SafetyGuard) ValidateMove(vx, vy, vyaw float64, status *RobotStatus) (float64, float64, float64, error) {
	sg.mu.Lock()
	defer sg.mu.Unlock()

	// Battery check
	if status != nil && status.Battery.SOC <= sg.config.CriticalBattery {
		sg.recordViolation("critical_battery", "Battery critically low", float64(status.Battery.SOC), float64(sg.config.CriticalBattery), "block")
		return 0, 0, 0, fmt.Errorf("SAFETY: critical battery %d%% — all movement blocked", status.Battery.SOC)
	}

	// Geofence check
	if sg.config.GeofenceEnabled && status != nil {
		dist := math.Sqrt(
			math.Pow(status.Position.X-sg.origin.X, 2) +
				math.Pow(status.Position.Y-sg.origin.Y, 2),
		)
		if dist > sg.config.GeofenceRadius {
			sg.recordViolation("geofence", "Outside geofence", dist, sg.config.GeofenceRadius, "block")
			return 0, 0, 0, fmt.Errorf("SAFETY: outside geofence (%.1fm > %.1fm)", dist, sg.config.GeofenceRadius)
		}
	}

	// Tilt check (rollover protection)
	if status != nil {
		tilt := math.Max(math.Abs(status.Orientation.Roll), math.Abs(status.Orientation.Pitch))
		tiltDeg := tilt * 180 / math.Pi
		if tiltDeg > sg.config.MaxTiltAngle {
			sg.recordViolation("tilt", "Excessive tilt angle", tiltDeg, sg.config.MaxTiltAngle, "block")
			return 0, 0, 0, fmt.Errorf("SAFETY: excessive tilt %.1f° — movement blocked", tiltDeg)
		}
	}

	// Speed clamping
	linearSpeed := math.Sqrt(vx*vx + vy*vy)
	if linearSpeed > sg.config.MaxLinearSpeed {
		scale := sg.config.MaxLinearSpeed / linearSpeed
		vx *= scale
		vy *= scale
		sg.recordViolation("speed_clamp", "Linear speed clamped", linearSpeed, sg.config.MaxLinearSpeed, "clamp")
	}

	if math.Abs(vyaw) > sg.config.MaxAngularSpeed {
		sign := 1.0
		if vyaw < 0 {
			sign = -1.0
		}
		vyaw = sign * sg.config.MaxAngularSpeed
		sg.recordViolation("angular_clamp", "Angular speed clamped", math.Abs(vyaw), sg.config.MaxAngularSpeed, "clamp")
	}

	return vx, vy, vyaw, nil
}

// CheckBatteryReturn returns true if battery is low enough to force return
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
	log.Printf("[zergling/safety] %s: %s (value=%.2f, limit=%.2f, action=%s)", vtype, msg, value, limit, action)
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

// ── Zergling Adapter ──

// AdapterConfig holds connection settings
type AdapterConfig struct {
	BridgeURL    string        `json:"bridge_url"`    // Python DDS bridge URL
	PollInterval time.Duration `json:"poll_interval"` // status poll interval
	Safety       *SafetyConfig `json:"safety"`
	Model        RobotModel    `json:"model"`
}

// DefaultAdapterConfig returns development defaults
func DefaultAdapterConfig() *AdapterConfig {
	return &AdapterConfig{
		BridgeURL:    "http://127.0.0.1:9200",
		PollInterval: 200 * time.Millisecond,
		Safety:       DefaultSafetyConfig(),
		Model:        ModelGo2E,
	}
}

// Adapter bridges Claw to Unitree robots via the Python DDS bridge
type Adapter struct {
	mu         sync.RWMutex
	config     *AdapterConfig
	nodeID     string
	safety     *SafetyGuard
	status     *RobotStatus
	connected  bool
	httpClient *http.Client
	stopCh     chan struct{}
	stats      AdapterStats
}

// AdapterStats tracks adapter metrics
type AdapterStats struct {
	CommandsSent    int       `json:"commands_sent"`
	CommandsFailed  int       `json:"commands_failed"`
	CommandsBlocked int       `json:"commands_blocked"` // by safety
	StatusPolls     int       `json:"status_polls"`
	PollErrors      int       `json:"poll_errors"`
	SafetyEvents    int       `json:"safety_events"`
	Connected       bool      `json:"connected"`
	Uptime          string    `json:"uptime,omitempty"`
	LastCommand     string    `json:"last_command,omitempty"`
	LastCommandAt   time.Time `json:"last_command_at,omitempty"`
}

var (
	globalAdapter *Adapter
	adapterOnce   sync.Once
)

// InitAdapter creates the global Zergling adapter
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
			stopCh:     make(chan struct{}),
			status: &RobotStatus{
				Model: cfg.Model,
				State: StateDisconnected,
			},
		}
		log.Printf("[zergling] adapter ready (bridge=%s, model=%s)", cfg.BridgeURL, cfg.Model)
	})
	return globalAdapter
}

// GetAdapter returns the global adapter
func GetAdapter() *Adapter {
	return globalAdapter
}

// ── Bridge Communication ──

// bridgeRequest sends a command to the Python DDS bridge
func (a *Adapter) bridgeRequest(method, path string, body interface{}) (map[string]interface{}, error) {
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		req, err := http.NewRequest(method, a.config.BridgeURL+path, jsonReader(bodyBytes))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := a.httpClient.Do(req)
		if err != nil {
			a.stats.CommandsFailed++
			return nil, fmt.Errorf("bridge unreachable: %w", err)
		}
		defer resp.Body.Close()

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, err
		}
		a.stats.CommandsSent++
		return result, nil
	}

	// GET request (no body)
	req, err := http.NewRequest(method, a.config.BridgeURL+path, nil)
	if err != nil {
		return nil, err
	}
	resp, err := a.httpClient.Do(req)
	if err != nil {
		a.stats.PollErrors++
		return nil, fmt.Errorf("bridge unreachable: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}

// ── High-Level Commands ──

// Move sends a velocity command (with safety validation)
func (a *Adapter) Move(vx, vy, vyaw float64, durationMs int) (map[string]interface{}, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Safety check
	safeVx, safeVy, safeVyaw, err := a.safety.ValidateMove(vx, vy, vyaw, a.status)
	if err != nil {
		a.stats.CommandsBlocked++
		a.stats.SafetyEvents++
		return nil, err
	}

	a.stats.LastCommand = "move"
	a.stats.LastCommandAt = time.Now()

	return a.bridgeRequest("POST", "/cmd/move", map[string]interface{}{
		"vx":          safeVx,
		"vy":          safeVy,
		"vyaw":        safeVyaw,
		"duration_ms": durationMs,
	})
}

// Goto navigates to a point (x, y, yaw)
func (a *Adapter) Goto(x, y, yaw float64) (map[string]interface{}, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Check geofence for target
	if a.config.Safety.GeofenceEnabled {
		dist := math.Sqrt(x*x + y*y)
		if dist > a.config.Safety.GeofenceRadius {
			a.stats.CommandsBlocked++
			return nil, fmt.Errorf("SAFETY: target (%.1f, %.1f) outside geofence", x, y)
		}
	}

	a.stats.LastCommand = "goto"
	a.stats.LastCommandAt = time.Now()

	return a.bridgeRequest("POST", "/cmd/goto", map[string]interface{}{
		"x":   x,
		"y":   y,
		"yaw": yaw,
	})
}

// Action executes a predefined action
func (a *Adapter) Action(name string) (map[string]interface{}, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Validate action name
	validActions := map[string]bool{
		"stand_up": true, "stand_down": true, "balance_stand": true,
		"recovery_stand": true, "sit": true, "rise_sit": true,
		"hello": true, "stretch": true, "wiggle_hips": true,
		"heart": true, "dance1": true, "dance2": true,
		"front_jump": true, "front_flip": true, "back_flip": true,
	}
	if !validActions[name] {
		return nil, fmt.Errorf("unknown action: %s", name)
	}

	a.stats.LastCommand = "action:" + name
	a.stats.LastCommandAt = time.Now()

	return a.bridgeRequest("POST", "/cmd/action", map[string]interface{}{
		"name": name,
	})
}

// Stop sends emergency stop
func (a *Adapter) Stop() (map[string]interface{}, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.stats.LastCommand = "stop"
	a.stats.LastCommandAt = time.Now()

	return a.bridgeRequest("POST", "/cmd/stop", nil)
}

// SetGait switches gait type
func (a *Adapter) SetGait(gait GaitType) (map[string]interface{}, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.stats.LastCommand = "gait:" + string(gait)
	a.stats.LastCommandAt = time.Now()

	return a.bridgeRequest("POST", "/cmd/gait", map[string]interface{}{
		"type": gait,
	})
}

// SetObstacleAvoidance toggles obstacle avoidance
func (a *Adapter) SetObstacleAvoidance(enable bool) (map[string]interface{}, error) {
	return a.bridgeRequest("POST", "/cmd/obstacle", map[string]interface{}{
		"enable": enable,
	})
}

// Patrol sends a series of waypoints for autonomous patrol
func (a *Adapter) Patrol(waypoints []Vec3) (map[string]interface{}, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Validate all waypoints within geofence
	if a.config.Safety.GeofenceEnabled {
		for i, wp := range waypoints {
			dist := math.Sqrt(wp.X*wp.X + wp.Y*wp.Y)
			if dist > a.config.Safety.GeofenceRadius {
				return nil, fmt.Errorf("SAFETY: waypoint %d (%.1f, %.1f) outside geofence", i, wp.X, wp.Y)
			}
		}
	}

	a.stats.LastCommand = fmt.Sprintf("patrol:%d_waypoints", len(waypoints))
	a.stats.LastCommandAt = time.Now()

	return a.bridgeRequest("POST", "/cmd/patrol", map[string]interface{}{
		"waypoints": waypoints,
	})
}

// GetCamera captures a frame from front camera
func (a *Adapter) GetCamera(format string) (map[string]interface{}, error) {
	if format == "" {
		format = "jpeg"
	}
	return a.bridgeRequest("GET", "/sensor/camera?format="+format, nil)
}

// ── Status ──

// Status returns cached robot status
func (a *Adapter) Status() *RobotStatus {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.status
}

// Connected returns whether the bridge is reachable
func (a *Adapter) Connected() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.connected
}

// Stats returns adapter metrics
func (a *Adapter) Stats() *AdapterStats {
	a.mu.RLock()
	defer a.mu.RUnlock()
	s := a.stats
	s.Connected = a.connected
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

func jsonReader(data []byte) io.Reader {
	return &byteReader{data: data}
}
