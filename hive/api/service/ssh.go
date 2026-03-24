package service

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"starclaw.net/hive/api/config"
)

// SSHService manages remote command execution on ECS instances via SSH CLI.
type SSHService struct {
	cfg *config.Config
}

func NewSSHService(cfg *config.Config) *SSHService {
	return &SSHService{cfg: cfg}
}

// RunCommand executes a command on a remote ECS instance via SSH.
// Returns combined stdout+stderr output and any error.
func (s *SSHService) RunCommand(ip string, command string) (string, error) {
	args := []string{
		"-o", "StrictHostKeyChecking=no",
		"-o", "ConnectTimeout=15",
		"-o", "BatchMode=yes",
	}
	if s.cfg.SSHKeyPath != "" {
		args = append(args, "-i", s.cfg.SSHKeyPath)
	}
	if s.cfg.SSHUser != "" {
		args = append(args, fmt.Sprintf("%s@%s", s.cfg.SSHUser, ip))
	} else {
		args = append(args, fmt.Sprintf("root@%s", ip))
	}
	args = append(args, command)

	cmd := exec.Command("ssh", args...)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// UpgradeECS performs a rolling update on a remote ECS Claw instance.
// Steps: git pull → docker compose build → docker compose up -d
func (s *SSHService) UpgradeECS(ip, slug string) error {
	log.Printf("[hive] SSH upgrade %s (%s)...", slug, ip)

	script := `set -e
cd /opt/starclaw
echo "[upgrade] pulling latest code..."
git fetch origin master
git reset --hard origin/master
echo "[upgrade] building api image..."
docker compose -f docker-compose.prod.yml build api --no-cache
echo "[upgrade] restarting api..."
docker compose -f docker-compose.prod.yml up -d api
echo "[upgrade] waiting for health..."
for i in $(seq 1 30); do
  if curl -sf http://127.0.0.1:8080/health > /dev/null 2>&1; then
    echo "[upgrade] health OK"
    exit 0
  fi
  sleep 2
done
echo "[upgrade] health check timeout"
exit 1`

	out, err := s.RunCommand(ip, script)
	if err != nil {
		log.Printf("[hive] SSH upgrade failed for %s (%s): %v\n%s", slug, ip, err, out)
		return fmt.Errorf("ssh upgrade: %w", err)
	}

	log.Printf("[hive] ✅ ECS %s upgraded via SSH", slug)
	return nil
}

// CheckHealth verifies the Claw API is responding on the remote instance.
func (s *SSHService) CheckHealth(ip string) error {
	out, err := s.RunCommand(ip, "curl -sf http://127.0.0.1:8080/health")
	if err != nil {
		return fmt.Errorf("health check failed: %s", out)
	}
	return nil
}

// WaitHealthy polls the remote Claw until it responds or timeout.
func (s *SSHService) WaitHealthy(ip string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if err := s.CheckHealth(ip); err == nil {
			return nil
		}
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("health check timeout after %s", timeout)
}
