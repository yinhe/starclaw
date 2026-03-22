package service

import (
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/yinhe/starclaw/hive/api/config"
	"github.com/yinhe/starclaw/hive/api/model"
)

// DockerService manages Claw containers via Docker CLI
type DockerService struct {
	cfg *config.Config
}

func NewDockerService(cfg *config.Config) (*DockerService, error) {
	// Verify docker CLI is available
	if _, err := exec.LookPath("docker"); err != nil {
		return nil, fmt.Errorf("docker CLI not found: %w", err)
	}
	return &DockerService{cfg: cfg}, nil
}

// CreateContainer creates and starts a Claw API container for the given instance
func (d *DockerService) CreateContainer(inst *model.ClawInstance) (string, error) {
	containerName := fmt.Sprintf("claw-%s-api", inst.Slug)
	dataDir := fmt.Sprintf("%s/instances/%s", d.cfg.DataDir, inst.Slug)

	args := []string{
		"run", "-d",
		"--name", containerName,
		"--network", d.cfg.NetworkName,
		"--restart", "unless-stopped",
		"-p", fmt.Sprintf("127.0.0.1:%d:8080", inst.Port),
		// Resource limits
		"--cpus", fmt.Sprintf("%.1f", inst.CPULimit),
		"--memory", fmt.Sprintf("%dM", inst.MemoryLimit/(1024*1024)),
		// Volumes
		"-v", fmt.Sprintf("%s/identity:/app/data/identity", dataDir),
		"-v", fmt.Sprintf("%s/uploads:/app/uploads", dataDir),
		"-v", fmt.Sprintf("%s/workspaces:/app/workspaces", dataDir),
		"-v", fmt.Sprintf("%s/images:/app/images", dataDir),
		// Environment
		"-e", "STARCLAW_SERVER_MODE=release",
		"-e", "STARCLAW_SERVER_DEPLOY_MODE=hosted",
		"-e", fmt.Sprintf("STARCLAW_DATABASE_HOST=%s", d.cfg.MySQLHost),
		"-e", fmt.Sprintf("STARCLAW_DATABASE_PORT=%d", d.cfg.MySQLPort),
		"-e", fmt.Sprintf("STARCLAW_DATABASE_USER=%s", inst.DBUser),
		"-e", fmt.Sprintf("STARCLAW_DATABASE_PASSWORD=%s", inst.DBPassword),
		"-e", fmt.Sprintf("STARCLAW_DATABASE_DBNAME=%s", inst.DBName),
		"-e", fmt.Sprintf("STARCLAW_REDIS_HOST=%s", d.cfg.RedisHost),
		"-e", fmt.Sprintf("STARCLAW_REDIS_PORT=%d", d.cfg.RedisPort),
		"-e", fmt.Sprintf("STARCLAW_REDIS_PASSWORD=%s", d.cfg.RedisPassword),
		"-e", fmt.Sprintf("STARCLAW_REDIS_DB=%d", inst.Port-d.cfg.PortRangeStart),
		"-e", fmt.Sprintf("STARCLAW_JWT_SECRET=%s", inst.JWTSecret),
		"-e", fmt.Sprintf("STARCLAW_NODE_ADDRESS=https://%s.%s", inst.Slug, d.cfg.Domain),
		"-e", "STARCLAW_OVERLORD_ENABLED=true",
		"-e", fmt.Sprintf("STARCLAW_OVERLORD_OVERLORD_URL=%s", d.cfg.OverlordURL),
		"-e", fmt.Sprintf("STARCLAW_OVERLORD_NODE_NAME=claw-%s", inst.Slug),
		"-e", "STARCLAW_OVERLORD_REGION=cn-east",
		"-e", "NODE_KEY_PATH=/app/data/identity/.node_key",
		// Image
		d.cfg.ClawImage,
	}

	out, err := exec.Command("docker", args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("docker run: %s: %w", strings.TrimSpace(string(out)), err)
	}

	containerID := strings.TrimSpace(string(out))
	log.Printf("[hive] container %s started on port %d for %s (id: %s)", containerName, inst.Port, inst.Slug, containerID[:12])
	return containerID, nil
}

// CreateLiteContainer creates and starts a Claw Lite container (Spark tier, SQLite, no MySQL/Redis)
func (d *DockerService) CreateLiteContainer(inst *model.ClawInstance) (string, error) {
	containerName := fmt.Sprintf("claw-%s-lite", inst.Slug)
	dataDir := fmt.Sprintf("%s/instances/%s", d.cfg.DataDir, inst.Slug)

	args := []string{
		"run", "-d",
		"--name", containerName,
		"--network", d.cfg.NetworkName,
		"--restart", "unless-stopped",
		"-p", fmt.Sprintf("127.0.0.1:%d:8080", inst.Port),
		// Resource limits (Spark: lightweight)
		"--cpus", fmt.Sprintf("%.2f", inst.CPULimit),
		"--memory", fmt.Sprintf("%dM", inst.MemoryLimit/(1024*1024)),
		// Volumes (data + identity only, no uploads/workspaces for lite)
		"-v", fmt.Sprintf("%s/data:/opt/claw/data", dataDir),
		"-v", fmt.Sprintf("%s/identity:/app/data/identity", dataDir),
		// Environment (SQLite mode, no MySQL/Redis)
		"-e", "STARCLAW_DATABASE_DRIVER=sqlite",
		"-e", "STARCLAW_DATABASE_SQLITE_PATH=/opt/claw/data/claw.db",
		"-e", "STARCLAW_REDIS_ENABLED=false",
		"-e", "STARCLAW_SERVER_MODE=release",
		"-e", "STARCLAW_SERVER_DEPLOY_MODE=hosted",
		"-e", fmt.Sprintf("STARCLAW_JWT_SECRET=%s", inst.JWTSecret),
		"-e", fmt.Sprintf("STARCLAW_NODE_ADDRESS=https://%s.%s", inst.Slug, d.cfg.Domain),
		"-e", "STARCLAW_OVERLORD_ENABLED=true",
		"-e", fmt.Sprintf("STARCLAW_OVERLORD_OVERLORD_URL=%s", d.cfg.OverlordURL),
		"-e", fmt.Sprintf("STARCLAW_OVERLORD_NODE_NAME=claw-%s", inst.Slug),
		"-e", "STARCLAW_OVERLORD_REGION=cn-east",
		"-e", "NODE_KEY_PATH=/app/data/identity/.node_key",
		"-e", "GIN_MODE=release",
		// Image (lite)
		d.cfg.ClawLiteImage,
	}

	out, err := exec.Command("docker", args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("docker run (lite): %s: %w", strings.TrimSpace(string(out)), err)
	}

	containerID := strings.TrimSpace(string(out))
	log.Printf("[hive] lite container %s started on port %d for %s (id: %s)", containerName, inst.Port, inst.Slug, containerID[:12])
	return containerID, nil
}

// StopContainer stops a running container
func (d *DockerService) StopContainer(containerID string) error {
	out, err := exec.Command("docker", "stop", "-t", "30", containerID).CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker stop: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// StartContainer starts a stopped container
func (d *DockerService) StartContainer(containerID string) error {
	out, err := exec.Command("docker", "start", containerID).CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker start: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// RestartContainer restarts a container
func (d *DockerService) RestartContainer(containerID string) error {
	out, err := exec.Command("docker", "restart", "-t", "30", containerID).CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker restart: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// RemoveContainer stops and removes a container
func (d *DockerService) RemoveContainer(containerID string) error {
	out, err := exec.Command("docker", "rm", "-f", containerID).CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker rm: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// ContainerLogs returns the last N lines of container logs
func (d *DockerService) ContainerLogs(containerID string, tail int) (string, error) {
	out, err := exec.Command("docker", "logs", "--tail", fmt.Sprintf("%d", tail), containerID).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("docker logs: %w", err)
	}
	return string(out), nil
}

// WaitHealthy polls the health endpoint until it responds OK or timeout
func (d *DockerService) WaitHealthy(port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	url := fmt.Sprintf("http://127.0.0.1:%d/health", port)

	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("health check timeout after %s", timeout)
}

// WaitHealthyByName polls the health endpoint via container name on the Docker network
func (d *DockerService) WaitHealthyByName(containerName string, internalPort int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	url := fmt.Sprintf("http://%s:%d/health", containerName, internalPort)

	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("health check timeout after %s for %s", timeout, containerName)
}

// EnsureNetwork creates the hive Docker network if it doesn't exist
func (d *DockerService) EnsureNetwork() error {
	// Check if network exists
	out, err := exec.Command("docker", "network", "ls", "--filter", fmt.Sprintf("name=^%s$", d.cfg.NetworkName), "--format", "{{.Name}}").CombinedOutput()
	if err == nil && strings.TrimSpace(string(out)) == d.cfg.NetworkName {
		return nil
	}
	// Create network
	out, err = exec.Command("docker", "network", "create", d.cfg.NetworkName).CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker network create: %s: %w", strings.TrimSpace(string(out)), err)
	}
	log.Printf("[hive] created Docker network: %s", d.cfg.NetworkName)
	return nil
}
