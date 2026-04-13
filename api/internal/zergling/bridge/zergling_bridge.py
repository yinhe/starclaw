#!/usr/bin/env python3
"""
Zergling DDS Bridge — Python ↔ Unitree SDK2 ↔ DDS ↔ Robot

This FastAPI server bridges the Go Zergling Adapter to actual Unitree hardware
via unitree_sdk2_python's DDS communication layer.

Architecture:
    Claw Go Adapter → HTTP JSON → THIS BRIDGE → DDS → Go2/B2/G1

Endpoints:
    GET  /health          — bridge health + robot connection status
    GET  /status          — full robot state (position, battery, IMU, etc.)
    POST /cmd/move        — velocity control (vx, vy, vyaw, duration_ms)
    POST /cmd/goto        — point-to-point navigation
    POST /cmd/action      — predefined actions (stand, sit, hello, dance...)
    POST /cmd/stop        — emergency stop
    POST /cmd/gait        — switch gait type
    POST /cmd/obstacle    — toggle obstacle avoidance
    POST /cmd/patrol      — autonomous waypoint patrol
    GET  /sensor/camera   — front camera frame (jpeg base64)

Usage:
    pip install unitree_sdk2py fastapi uvicorn
    python zergling_bridge.py --interface eth0 --port 9200

    # Or WiFi mode (auto-discovery):
    python zergling_bridge.py --port 9200
"""

import argparse
import asyncio
import base64
import json
import logging
import math
import os
import sys
import time
import threading
from typing import Optional, List, Dict, Any

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
import uvicorn

# ── Logging ──
logging.basicConfig(
    level=logging.INFO,
    format="[%(asctime)s] %(name)s %(levelname)s: %(message)s",
    datefmt="%H:%M:%S",
)
log = logging.getLogger("zergling-bridge")

# ── Try importing Unitree SDK ──
SDK_AVAILABLE = False
try:
    from unitree_sdk2py.core.channel import ChannelFactory, ChannelSubscriber, ChannelPublisher
    from unitree_sdk2py.go2.sport.sport_client import SportClient
    from unitree_sdk2py.go2.video.video_client import VideoClient
    from unitree_sdk2py.idl.go2.SportModeState_ import SportModeState_
    SDK_AVAILABLE = True
    log.info("unitree_sdk2py loaded successfully")
except ImportError:
    log.warning("unitree_sdk2py not available — running in SIMULATION mode")


# ════════════════════════════════════════════════════════════
# Data Models
# ════════════════════════════════════════════════════════════

class MoveCmd(BaseModel):
    vx: float = 0.0
    vy: float = 0.0
    vyaw: float = 0.0
    duration_ms: int = 1000

class GotoCmd(BaseModel):
    x: float
    y: float
    yaw: float = 0.0

class ActionCmd(BaseModel):
    name: str

class GaitCmd(BaseModel):
    type: str  # idle, trot, trot_running, climb_stair, trot_obstacle

class ObstacleCmd(BaseModel):
    enable: bool

class PatrolCmd(BaseModel):
    waypoints: List[Dict[str, float]]  # [{x, y, z?}]


# ════════════════════════════════════════════════════════════
# Robot State Cache
# ════════════════════════════════════════════════════════════

class RobotStateCache:
    """Thread-safe cache for robot state received via DDS subscription."""

    def __init__(self):
        self._lock = threading.Lock()
        self.connected = False
        self.position = {"x": 0.0, "y": 0.0, "z": 0.0}
        self.velocity = {"x": 0.0, "y": 0.0, "z": 0.0}
        self.orientation = {"roll": 0.0, "pitch": 0.0, "yaw": 0.0}
        self.body_height = 0.3
        self.gait_type = "idle"
        self.mode = 0
        self.battery = {"soc": 100, "voltage": 0.0, "current": 0.0, "temperature": 0.0}
        self.foot_force = [0.0, 0.0, 0.0, 0.0]
        self.imu = {
            "acc_x": 0.0, "acc_y": 0.0, "acc_z": 9.81,
            "gyro_x": 0.0, "gyro_y": 0.0, "gyro_z": 0.0,
            "quaternion": [1.0, 0.0, 0.0, 0.0],
        }
        self.obstacle_avoidance = False
        self.terrain_adapt = False
        self.last_update = 0.0

    def update_from_sport_state(self, msg):
        """Update cache from SportModeState_ DDS message."""
        with self._lock:
            self.connected = True
            self.last_update = time.time()

            if hasattr(msg, "position") and len(msg.position) >= 3:
                self.position = {"x": msg.position[0], "y": msg.position[1], "z": msg.position[2]}
            if hasattr(msg, "velocity") and len(msg.velocity) >= 3:
                self.velocity = {"x": msg.velocity[0], "y": msg.velocity[1], "z": msg.velocity[2]}
            if hasattr(msg, "imu_state"):
                imu = msg.imu_state
                if hasattr(imu, "accelerometer") and len(imu.accelerometer) >= 3:
                    self.imu["acc_x"] = imu.accelerometer[0]
                    self.imu["acc_y"] = imu.accelerometer[1]
                    self.imu["acc_z"] = imu.accelerometer[2]
                if hasattr(imu, "gyroscope") and len(imu.gyroscope) >= 3:
                    self.imu["gyro_x"] = imu.gyroscope[0]
                    self.imu["gyro_y"] = imu.gyroscope[1]
                    self.imu["gyro_z"] = imu.gyroscope[2]
                if hasattr(imu, "quaternion") and len(imu.quaternion) >= 4:
                    self.imu["quaternion"] = list(imu.quaternion[:4])
                    # Convert quaternion to euler
                    q = imu.quaternion
                    self.orientation = self._quat_to_euler(q[0], q[1], q[2], q[3])
            if hasattr(msg, "foot_force") and len(msg.foot_force) >= 4:
                self.foot_force = list(msg.foot_force[:4])
            if hasattr(msg, "body_height"):
                self.body_height = msg.body_height
            if hasattr(msg, "gait_type"):
                gait_map = {0: "idle", 1: "trot", 2: "trot_running", 3: "climb_stair", 4: "trot_obstacle"}
                self.gait_type = gait_map.get(msg.gait_type, str(msg.gait_type))
            if hasattr(msg, "mode"):
                self.mode = msg.mode
            if hasattr(msg, "battery_state"):
                bat = msg.battery_state
                if hasattr(bat, "soc"):
                    self.battery["soc"] = int(bat.soc)
                if hasattr(bat, "voltage"):
                    self.battery["voltage"] = bat.voltage
                if hasattr(bat, "current"):
                    self.battery["current"] = bat.current

    def to_dict(self) -> dict:
        with self._lock:
            state = "disconnected"
            if self.connected:
                speed = math.sqrt(self.velocity["x"]**2 + self.velocity["y"]**2)
                if speed > 0.05:
                    state = "moving"
                else:
                    state = "standing"
            return {
                "model": "go2_edu",
                "state": state,
                "position": self.position,
                "velocity": self.velocity,
                "orientation": self.orientation,
                "body_height": self.body_height,
                "gait_type": self.gait_type,
                "battery": self.battery,
                "foot_force": self.foot_force,
                "imu": self.imu,
                "obstacle_avoidance": self.obstacle_avoidance,
                "terrain_adapt": self.terrain_adapt,
                "timestamp": self.last_update,
            }

    @staticmethod
    def _quat_to_euler(w, x, y, z):
        """Convert quaternion to euler angles (roll, pitch, yaw)."""
        sinr_cosp = 2 * (w * x + y * z)
        cosr_cosp = 1 - 2 * (x * x + y * y)
        roll = math.atan2(sinr_cosp, cosr_cosp)

        sinp = 2 * (w * y - z * x)
        sinp = max(-1.0, min(1.0, sinp))
        pitch = math.asin(sinp)

        siny_cosp = 2 * (w * z + x * y)
        cosy_cosp = 1 - 2 * (y * y + z * z)
        yaw = math.atan2(siny_cosp, cosy_cosp)

        return {"roll": roll, "pitch": pitch, "yaw": yaw}


# ════════════════════════════════════════════════════════════
# Simulation Backend (when SDK not available)
# ════════════════════════════════════════════════════════════

class SimulatedRobot:
    """Fake robot for development without hardware."""

    def __init__(self):
        self.state = RobotStateCache()
        self.state.connected = True
        self.state.battery["soc"] = 85
        self.state.last_update = time.time()
        self._patrol_task = None
        log.info("SimulatedRobot initialized (no hardware)")

    def move(self, vx, vy, vyaw, duration_ms):
        self.state.velocity = {"x": vx, "y": vy, "z": 0.0}
        dt = duration_ms / 1000.0
        self.state.position["x"] += vx * dt
        self.state.position["y"] += vy * dt
        self.state.orientation["yaw"] += vyaw * dt
        self.state.last_update = time.time()
        return {"ok": True, "simulated": True}

    def goto(self, x, y, yaw):
        self.state.position = {"x": x, "y": y, "z": 0.0}
        self.state.orientation["yaw"] = yaw
        self.state.last_update = time.time()
        return {"ok": True, "simulated": True}

    def action(self, name):
        log.info(f"[sim] action: {name}")
        self.state.last_update = time.time()
        return {"ok": True, "action": name, "simulated": True}

    def stop(self):
        self.state.velocity = {"x": 0, "y": 0, "z": 0}
        self.state.last_update = time.time()
        return {"ok": True, "simulated": True}

    def set_gait(self, gait_type):
        self.state.gait_type = gait_type
        self.state.last_update = time.time()
        return {"ok": True, "gait": gait_type, "simulated": True}

    def set_obstacle(self, enable):
        self.state.obstacle_avoidance = enable
        return {"ok": True, "obstacle_avoidance": enable, "simulated": True}

    def patrol(self, waypoints):
        log.info(f"[sim] patrol with {len(waypoints)} waypoints")
        return {"ok": True, "waypoints": len(waypoints), "simulated": True}

    def camera(self, fmt):
        # Return a tiny 1x1 red pixel JPEG as placeholder
        pixel = base64.b64encode(b'\xff\xd8\xff\xe0\x00\x10JFIF').decode()
        return {"ok": True, "format": fmt, "data": pixel, "simulated": True}


# ════════════════════════════════════════════════════════════
# Real Robot Backend (Unitree SDK2)
# ════════════════════════════════════════════════════════════

class UnitreeRobot:
    """Real robot control via unitree_sdk2_python DDS."""

    def __init__(self, interface: Optional[str] = None):
        self.state = RobotStateCache()
        self._sport: Optional[SportClient] = None
        self._video: Optional[VideoClient] = None
        self._interface = interface

        # Initialize DDS
        if interface:
            ChannelFactory.Instance().Init(0, interface)
            log.info(f"DDS initialized on interface: {interface}")
        else:
            ChannelFactory.Instance().Init(0)
            log.info("DDS initialized (default multicast)")

        # Sport client
        self._sport = SportClient()
        self._sport.SetTimeout(5.0)
        self._sport.Init()
        log.info("SportClient initialized")

        # Video client
        try:
            self._video = VideoClient()
            self._video.SetTimeout(3.0)
            self._video.Init()
            log.info("VideoClient initialized")
        except Exception as e:
            log.warning(f"VideoClient failed: {e}")
            self._video = None

        # Subscribe to sport mode state
        self._state_sub = ChannelSubscriber("rt/sportmodestate", SportModeState_)
        self._state_sub.Init(self._on_state, 10)
        log.info("Subscribed to rt/sportmodestate")

    def _on_state(self, msg: SportModeState_):
        self.state.update_from_sport_state(msg)

    # ── Gait type map ──
    _GAIT_MAP = {
        "idle": 0, "trot": 1, "trot_running": 2,
        "climb_stair": 3, "trot_obstacle": 4,
    }

    # ── Action map ──
    _ACTION_MAP = {
        "stand_up": "StandUp", "stand_down": "StandDown",
        "balance_stand": "BalanceStand", "recovery_stand": "RecoveryStand",
        "sit": "Sit", "rise_sit": "RiseSit",
        "hello": "Hello", "stretch": "Stretch",
        "wiggle_hips": "WiggleHips", "heart": "Heart",
        "dance1": "Dance1", "dance2": "Dance2",
        "front_jump": "FrontJump", "front_flip": "FrontFlip",
        "back_flip": "BackFlip",
    }

    def move(self, vx, vy, vyaw, duration_ms):
        self._sport.Move(vx, vy, vyaw)
        if duration_ms > 0:
            threading.Timer(duration_ms / 1000.0, lambda: self._sport.StopMove()).start()
        return {"ok": True}

    def goto(self, x, y, yaw):
        # Use Move to approximate point-to-point (SDK TrajectoryFollow for real nav)
        # Simple approach: compute direction and move
        cur = self.state.position
        dx = x - cur["x"]
        dy = y - cur["y"]
        dist = math.sqrt(dx * dx + dy * dy)
        if dist < 0.1:
            return {"ok": True, "message": "already at target"}
        # Normalize to 0.5 m/s
        speed = 0.5
        vx = (dx / dist) * speed
        vy = (dy / dist) * speed
        duration = dist / speed
        self._sport.Move(vx, vy, 0)
        threading.Timer(duration, lambda: self._sport.StopMove()).start()
        return {"ok": True, "estimated_time_s": round(duration, 1)}

    def action(self, name):
        method_name = self._ACTION_MAP.get(name)
        if not method_name:
            return {"ok": False, "error": f"unknown action: {name}"}
        method = getattr(self._sport, method_name, None)
        if method is None:
            return {"ok": False, "error": f"SDK method not found: {method_name}"}
        method()
        return {"ok": True, "action": name}

    def stop(self):
        self._sport.StopMove()
        return {"ok": True}

    def set_gait(self, gait_type):
        gait_id = self._GAIT_MAP.get(gait_type, 0)
        self._sport.SwitchGait(gait_id)
        return {"ok": True, "gait": gait_type}

    def set_obstacle(self, enable):
        self._sport.SwitchObstacleAvoid(enable)
        self.state.obstacle_avoidance = enable
        return {"ok": True, "obstacle_avoidance": enable}

    def patrol(self, waypoints):
        # Sequential goto for each waypoint
        def _run_patrol():
            for i, wp in enumerate(waypoints):
                log.info(f"[patrol] waypoint {i+1}/{len(waypoints)}: ({wp.get('x',0)}, {wp.get('y',0)})")
                self.goto(wp.get("x", 0), wp.get("y", 0), wp.get("yaw", 0))
                # Wait for estimated arrival
                cur = self.state.position
                dist = math.sqrt((wp.get("x",0)-cur["x"])**2 + (wp.get("y",0)-cur["y"])**2)
                time.sleep(max(1, dist / 0.5))
            log.info("[patrol] complete")
        threading.Thread(target=_run_patrol, daemon=True).start()
        return {"ok": True, "waypoints": len(waypoints), "async": True}

    def camera(self, fmt):
        if self._video is None:
            return {"ok": False, "error": "video client not available"}
        code, data = self._video.GetImageSample()
        if code != 0:
            return {"ok": False, "error": f"camera error code: {code}"}
        encoded = base64.b64encode(data).decode()
        return {"ok": True, "format": "h264", "data": encoded, "size": len(data)}


# ════════════════════════════════════════════════════════════
# FastAPI Application
# ════════════════════════════════════════════════════════════

app = FastAPI(title="Zergling DDS Bridge", version="1.0.0")
robot = None  # Set during startup


@app.get("/health")
def health():
    return {
        "status": "ok",
        "sdk_available": SDK_AVAILABLE,
        "simulated": not SDK_AVAILABLE or isinstance(robot, SimulatedRobot),
        "connected": robot.state.connected if robot else False,
        "last_update": robot.state.last_update if robot else 0,
    }


@app.get("/status")
def status():
    if robot is None:
        raise HTTPException(503, "robot not initialized")
    return robot.state.to_dict()


@app.post("/cmd/move")
def cmd_move(cmd: MoveCmd):
    if robot is None:
        raise HTTPException(503, "robot not initialized")
    return robot.move(cmd.vx, cmd.vy, cmd.vyaw, cmd.duration_ms)


@app.post("/cmd/goto")
def cmd_goto(cmd: GotoCmd):
    if robot is None:
        raise HTTPException(503, "robot not initialized")
    return robot.goto(cmd.x, cmd.y, cmd.yaw)


@app.post("/cmd/action")
def cmd_action(cmd: ActionCmd):
    if robot is None:
        raise HTTPException(503, "robot not initialized")
    return robot.action(cmd.name)


@app.post("/cmd/stop")
def cmd_stop():
    if robot is None:
        raise HTTPException(503, "robot not initialized")
    return robot.stop()


@app.post("/cmd/gait")
def cmd_gait(cmd: GaitCmd):
    if robot is None:
        raise HTTPException(503, "robot not initialized")
    return robot.set_gait(cmd.type)


@app.post("/cmd/obstacle")
def cmd_obstacle(cmd: ObstacleCmd):
    if robot is None:
        raise HTTPException(503, "robot not initialized")
    return robot.set_obstacle(cmd.enable)


@app.post("/cmd/patrol")
def cmd_patrol(cmd: PatrolCmd):
    if robot is None:
        raise HTTPException(503, "robot not initialized")
    return robot.patrol(cmd.waypoints)


@app.get("/sensor/camera")
def sensor_camera(format: str = "jpeg"):
    if robot is None:
        raise HTTPException(503, "robot not initialized")
    return robot.camera(format)


# ════════════════════════════════════════════════════════════
# Main
# ════════════════════════════════════════════════════════════

def main():
    global robot

    parser = argparse.ArgumentParser(description="Zergling DDS Bridge")
    parser.add_argument("--port", type=int, default=9200, help="HTTP port (default: 9200)")
    parser.add_argument("--interface", type=str, default=None,
                        help="Network interface for DDS (e.g. eth0). If not set, uses WiFi multicast.")
    parser.add_argument("--simulate", action="store_true",
                        help="Force simulation mode even if SDK is available")
    args = parser.parse_args()

    if SDK_AVAILABLE and not args.simulate:
        log.info("Initializing REAL Unitree robot connection...")
        try:
            robot = UnitreeRobot(interface=args.interface)
        except Exception as e:
            log.error(f"Failed to initialize real robot: {e}")
            log.info("Falling back to simulation mode")
            robot = SimulatedRobot()
    else:
        robot = SimulatedRobot()

    log.info(f"Zergling DDS Bridge starting on :{args.port}")
    uvicorn.run(app, host="0.0.0.0", port=args.port, log_level="info")


if __name__ == "__main__":
    main()
