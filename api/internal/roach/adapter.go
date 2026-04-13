package roach

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
// Roach Adapter — Claw ↔ 轮式/履带地面机器人适配层
//
// 架构:
//   Claw Agent Runtime → Roach Adapter (Go, 本包)
//        ↓ HTTP JSON
//   Roach ROS2 Bridge (Python, rclpy + Nav2)
//        ↓ ROS2 DDS (CycloneDDS / FastDDS)
//   底盘驱动节点 → CAN/UART → 电机
//
// 核心职责:
//   1. 将 MCP Tool 调用转换为 ROS2 cmd_vel / Nav2 Goal
//   2. Safety Guard: 速度/电量/围栏/碰撞/倾斜硬限制
//   3. 状态缓存: 定期拉取里程计+传感器
//   4. SLAM/地图管理: 建图/加载/保存
//   5. 事件上报: 碰撞/低电量/导航失败 → Pheromone
// ════════════════════════════════════════════════════════════

// ── Robot Types ──

type ChassisType string

const (
	ChassisDiff     ChassisType = "differential"    // 差速 (TurtleBot, Scout)
	ChassisMecanum  ChassisType = "mecanum"          // 麦克纳姆轮 (LIMO麦轮模式)
	ChassisAckerman ChassisType = "ackermann"        // 阿克曼转向 (LIMO阿克曼模式)
	ChassisTank     ChassisType = "tank"             // 履带
	ChassisOmni     ChassisType = "omnidirectional"  // 全向轮
)

type RobotState string

const (
	StateOffline    RobotState = "offline"
	StateIdle       RobotState = "idle"
	StateMoving     RobotState = "moving"
	StateNavigating RobotState = "navigating"
	StateRecovery   RobotState = "recovery"
	StateCharging   RobotState = "charging"
	StateStopped    RobotState = "emergency_stop"
	StateMapping    RobotState = "mapping"
)

type NavStatus string

const (
	NavIdle       NavStatus = "idle"
	NavActive     NavStatus = "active"
	NavSucceeded  NavStatus = "succeeded"
	NavFailed     NavStatus = "failed"
	NavCanceled   NavStatus = "canceled"
	NavRecovering NavStatus = "recovering"
)

// ── Status Data ──

// RoachStatus represents the full state of the ground robot
type RoachStatus struct {
	State       RobotState  `json:"state"`
	Chassis     ChassisType `json:"chassis_type"`
	Position    Pose2D      `json:"position"`
	Velocity    Twist2D     `json:"velocity"`
	Battery     Battery     `json:"battery"`
	IMU         IMUData     `json:"imu"`
	NavStatus   NavStatus   `json:"nav_status"`
	NavGoal     *Pose2D     `json:"nav_goal,omitempty"`
	MapLoaded   bool        `json:"map_loaded"`
	MapName     string      `json:"map_name,omitempty"`
	LidarMin    float64     `json:"lidar_min_distance"` // nearest obstacle (m)
	Bumper      bool        `json:"bumper_pressed"`
	Timestamp   time.Time   `json:"timestamp"`
}

type Pose2D struct {
	X   float64 `json:"x"`
	Y   float64 `json:"y"`
	Yaw float64 `json:"yaw"` // radians
}

type Twist2D struct {
	Linear  float64 `json:"linear"`  // m/s
	Angular float64 `json:"angular"` // rad/s
}

type Battery struct {
	Percent     int     `json:"percent"`
	Voltage     float64 `json:"voltage"`
	Current     float64 `json:"current"`
	Temperature float64 `json:"temperature"`
	Charging    bool    `json:"charging"`
}

type IMUData struct {
	AccX  float64    `json:"acc_x"`
	AccY  float64    `json:"acc_y"`
	AccZ  float64    `json:"acc_z"`
	GyroX float64    `json:"gyro_x"`
	GyroY float64    `json:"gyro_y"`
	GyroZ float64    `json:"gyro_z"`
	Roll  float64    `json:"roll"`
	Pitch float64    `json:"pitch"`
	Yaw   float64    `json:"yaw"`
}

// ── Safety Guard ──

type SafetyConfig struct {
	MaxLinearSpeed  float64 `json:"max_linear_speed"`   // m/s
	MaxAngularSpeed float64 `json:"max_angular_speed"`  // rad/s
	MinBattery      int     `json:"min_battery"`        // % — force return to dock
	CriticalBattery int     `json:"critical_battery"`   // % — emergency stop
	GeofenceRadius  float64 `json:"geofence_radius"`    // m from origin
	GeofenceEnabled bool    `json:"geofence_enabled"`
	MinLidarDist    float64 `json:"min_lidar_distance"` // m — emergency stop
	MaxTiltAngle    float64 `json:"max_tilt_angle"`     // degrees
	CmdVelTimeout   int     `json:"cmd_vel_timeout_ms"` // ms — auto stop if no new cmd
}

func DefaultSafetyConfig() *SafetyConfig {
	return &SafetyConfig{
		MaxLinearSpeed:  2.0,
		MaxAngularSpeed: 1.5,
		MinBattery:      20,
		CriticalBattery: 10,
		GeofenceRadius:  100.0,
		GeofenceEnabled: true,
		MinLidarDist:    0.3,
		MaxTiltAngle:    30.0,
		CmdVelTimeout:   500,
	}
}

type SafetyGuard struct {
	config     *SafetyConfig
	origin     Pose2D
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

// ValidateMove checks and clamps a velocity command
func (sg *SafetyGuard) ValidateMove(linear, angular float64, status *RoachStatus) (float64, float64, error) {
	sg.mu.Lock()
	defer sg.mu.Unlock()

	// Battery check
	if status != nil && status.Battery.Percent <= sg.config.CriticalBattery {
		sg.recordViolation("critical_battery", "Battery critically low",
			float64(status.Battery.Percent), float64(sg.config.CriticalBattery), "block")
		return 0, 0, fmt.Errorf("SAFETY: critical battery %d%%", status.Battery.Percent)
	}

	// Bumper check
	if status != nil && status.Bumper {
		sg.recordViolation("bumper", "Bumper pressed — collision detected", 0, 0, "block")
		return 0, 0, fmt.Errorf("SAFETY: bumper collision detected")
	}

	// LiDAR proximity check
	if status != nil && status.LidarMin > 0 && status.LidarMin < sg.config.MinLidarDist && linear > 0 {
		sg.recordViolation("proximity", "Obstacle too close",
			status.LidarMin, sg.config.MinLidarDist, "block")
		return 0, 0, fmt.Errorf("SAFETY: obstacle at %.2fm (min %.2fm)", status.LidarMin, sg.config.MinLidarDist)
	}

	// Tilt check
	if status != nil {
		tilt := math.Max(math.Abs(status.IMU.Roll), math.Abs(status.IMU.Pitch))
		tiltDeg := tilt * 180 / math.Pi
		if tiltDeg > sg.config.MaxTiltAngle {
			sg.recordViolation("tilt", "Excessive tilt angle",
				tiltDeg, sg.config.MaxTiltAngle, "block")
			return 0, 0, fmt.Errorf("SAFETY: tilt %.1f° exceeds limit %.1f°", tiltDeg, sg.config.MaxTiltAngle)
		}
	}

	// Geofence check
	if sg.config.GeofenceEnabled && status != nil {
		dist := math.Sqrt(
			math.Pow(status.Position.X-sg.origin.X, 2) +
				math.Pow(status.Position.Y-sg.origin.Y, 2),
		)
		if dist > sg.config.GeofenceRadius {
			sg.recordViolation("geofence", "Outside geofence",
				dist, sg.config.GeofenceRadius, "block")
			return 0, 0, fmt.Errorf("SAFETY: outside geofence (%.1fm > %.1fm)", dist, sg.config.GeofenceRadius)
		}
	}

	// Speed clamping
	if math.Abs(linear) > sg.config.MaxLinearSpeed {
		sign := 1.0
		if linear < 0 {
			sign = -1.0
		}
		linear = sign * sg.config.MaxLinearSpeed
		sg.recordViolation("speed_clamp", "Linear speed clamped",
			math.Abs(linear), sg.config.MaxLinearSpeed, "clamp")
	}

	if math.Abs(angular) > sg.config.MaxAngularSpeed {
		sign := 1.0
		if angular < 0 {
			sign = -1.0
		}
		angular = sign * sg.config.MaxAngularSpeed
		sg.recordViolation("angular_clamp", "Angular speed clamped",
			math.Abs(angular), sg.config.MaxAngularSpeed, "clamp")
	}

	return linear, angular, nil
}

// ValidateNavGoal checks if a navigation goal is within safety limits
func (sg *SafetyGuard) ValidateNavGoal(x, y float64, status *RoachStatus) error {
	sg.mu.Lock()
	defer sg.mu.Unlock()

	if status != nil && status.Battery.Percent <= sg.config.MinBattery {
		sg.recordViolation("low_battery_nav", "Battery too low for navigation",
			float64(status.Battery.Percent), float64(sg.config.MinBattery), "block")
		return fmt.Errorf("SAFETY: battery %d%% too low for navigation", status.Battery.Percent)
	}

	if sg.config.GeofenceEnabled {
		dist := math.Sqrt(
			math.Pow(x-sg.origin.X, 2) + math.Pow(y-sg.origin.Y, 2),
		)
		if dist > sg.config.GeofenceRadius {
			sg.recordViolation("geofence_goal", "Nav goal outside geofence",
				dist, sg.config.GeofenceRadius, "block")
			return fmt.Errorf("SAFETY: goal (%.1f, %.1f) outside geofence", x, y)
		}
	}

	return nil
}

// CheckBatteryReturn returns true if battery is low enough to force return
func (sg *SafetyGuard) CheckBatteryReturn(percent int) bool {
	return percent <= sg.config.MinBattery
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
	log.Printf("[roach/safety] %s: %s (value=%.2f, limit=%.2f, action=%s)", vtype, msg, value, limit, action)
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

// ── Roach Adapter ──

type AdapterConfig struct {
	BridgeURL    string        `json:"bridge_url"`
	PollInterval time.Duration `json:"poll_interval"`
	Safety       *SafetyConfig `json:"safety"`
	Chassis      ChassisType   `json:"chassis_type"`
}

func DefaultAdapterConfig() *AdapterConfig {
	return &AdapterConfig{
		BridgeURL:    "http://127.0.0.1:9201",
		PollInterval: 100 * time.Millisecond,
		Safety:       DefaultSafetyConfig(),
		Chassis:      ChassisDiff,
	}
}

type Adapter struct {
	mu         sync.RWMutex
	config     *AdapterConfig
	nodeID     string
	safety     *SafetyGuard
	status     *RoachStatus
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
	NavGoalsFailed  int       `json:"nav_goals_failed"`
	Connected       bool      `json:"connected"`
	Uptime          string    `json:"uptime"`
	LastCommand     string    `json:"last_command,omitempty"`
	LastCommandAt   time.Time `json:"last_command_at,omitempty"`
}

var (
	globalAdapter *Adapter
	adapterOnce   sync.Once
)

// InitAdapter creates the global Roach adapter
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
			status: &RoachStatus{
				State:   StateOffline,
				Chassis: cfg.Chassis,
			},
		}
		log.Printf("[roach] adapter ready (bridge=%s, chassis=%s)", cfg.BridgeURL, cfg.Chassis)
	})
	return globalAdapter
}

// GetAdapter returns the global adapter
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

// Move sends a velocity command (with safety validation)
func (a *Adapter) Move(linear, angular float64, durationMs int) (map[string]interface{}, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	safeLinear, safeAngular, err := a.safety.ValidateMove(linear, angular, a.status)
	if err != nil {
		a.stats.CommandsBlocked++
		a.stats.SafetyEvents++
		return nil, err
	}

	a.stats.LastCommand = "move"
	a.stats.LastCommandAt = time.Now()

	return a.bridgeRequest("POST", "/cmd/move", map[string]interface{}{
		"linear":      safeLinear,
		"angular":     safeAngular,
		"duration_ms": durationMs,
	})
}

// Goto sends a Nav2 navigation goal
func (a *Adapter) Goto(x, y, yaw float64) (map[string]interface{}, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if err := a.safety.ValidateNavGoal(x, y, a.status); err != nil {
		a.stats.CommandsBlocked++
		return nil, err
	}

	a.stats.LastCommand = "goto"
	a.stats.LastCommandAt = time.Now()
	a.stats.NavGoalsSent++

	return a.bridgeRequest("POST", "/cmd/goto", map[string]interface{}{
		"x":   x,
		"y":   y,
		"yaw": yaw,
	})
}

// Patrol sends multiple waypoints for sequential navigation
func (a *Adapter) Patrol(waypoints []Pose2D) (map[string]interface{}, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for i, wp := range waypoints {
		if err := a.safety.ValidateNavGoal(wp.X, wp.Y, a.status); err != nil {
			a.stats.CommandsBlocked++
			return nil, fmt.Errorf("waypoint %d: %w", i, err)
		}
	}

	a.stats.LastCommand = fmt.Sprintf("patrol:%d_waypoints", len(waypoints))
	a.stats.LastCommandAt = time.Now()
	a.stats.NavGoalsSent += len(waypoints)

	return a.bridgeRequest("POST", "/cmd/patrol", map[string]interface{}{
		"waypoints": waypoints,
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

// Spin rotates in place by given degrees
func (a *Adapter) Spin(angleDeg float64) (map[string]interface{}, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.stats.LastCommand = "spin"
	a.stats.LastCommandAt = time.Now()

	return a.bridgeRequest("POST", "/cmd/spin", map[string]interface{}{
		"angle_deg": angleDeg,
	})
}

// Backup moves backward by given distance
func (a *Adapter) Backup(distanceM float64) (map[string]interface{}, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if distanceM > 3.0 {
		distanceM = 3.0 // max backup distance
	}

	a.stats.LastCommand = "backup"
	a.stats.LastCommandAt = time.Now()

	return a.bridgeRequest("POST", "/cmd/backup", map[string]interface{}{
		"distance_m": distanceM,
	})
}

// MapSave saves the current SLAM map
func (a *Adapter) MapSave(filename string) (map[string]interface{}, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.stats.LastCommand = "map_save"
	a.stats.LastCommandAt = time.Now()

	return a.bridgeRequest("POST", "/map/save", map[string]interface{}{
		"filename": filename,
	})
}

// MapLoad loads a saved map
func (a *Adapter) MapLoad(filename string) (map[string]interface{}, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.stats.LastCommand = "map_load"
	a.stats.LastCommandAt = time.Now()

	return a.bridgeRequest("POST", "/map/load", map[string]interface{}{
		"filename": filename,
	})
}

// SetPose sets the initial pose for AMCL localization
func (a *Adapter) SetPose(x, y, yaw float64) (map[string]interface{}, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.stats.LastCommand = "set_pose"
	a.stats.LastCommandAt = time.Now()

	return a.bridgeRequest("POST", "/cmd/set_pose", map[string]interface{}{
		"x":   x,
		"y":   y,
		"yaw": yaw,
	})
}

// GetCamera captures a frame from on-board camera
func (a *Adapter) GetCamera(cameraID string) (map[string]interface{}, error) {
	if cameraID == "" {
		cameraID = "front"
	}
	return a.bridgeRequest("GET", "/sensor/camera?id="+cameraID, nil)
}

// GetLidar returns latest LiDAR scan summary
func (a *Adapter) GetLidar() (map[string]interface{}, error) {
	return a.bridgeRequest("GET", "/sensor/lidar", nil)
}

// ── Status ──

func (a *Adapter) Status() *RoachStatus {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.status
}

func (a *Adapter) Connected() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.status.State != StateOffline
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
