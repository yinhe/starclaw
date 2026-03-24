package sandbox

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"starclaw.net/spore/pkg/manifest"
)

func newPlatformRunner() Runner {
	r := &linuxRunner{}
	if r.Available() {
		return r
	}
	return nil
}

func platformInstallHint() string {
	return "Install bubblewrap: apt install bubblewrap"
}

// linuxRunner uses bubblewrap (bwrap) for lightweight sandboxing on Linux.
// bwrap provides: mount namespace, PID namespace, filesystem isolation.
// No root required (user namespace).
type linuxRunner struct{}

func (r *linuxRunner) Name() string { return "bwrap" }

func (r *linuxRunner) Available() bool {
	_, err := exec.LookPath("bwrap")
	return err == nil
}

// Wrap replaces the cmd with a bwrap-wrapped version.
// The original binary runs inside a mount namespace with:
//   - Read-only bind of system dirs (/usr, /lib, /lib64, /etc, /bin, /sbin)
//   - Writable bind of dataDir and logDir
//   - Writable tmpfs at /tmp
//   - Optional PID namespace isolation
func (r *linuxRunner) Wrap(cmd *exec.Cmd, binPath, dataDir, logDir string, cfg manifest.Sandbox) error {
	args := []string{
		// Minimal read-only system mounts
		"--ro-bind", "/usr", "/usr",
		"--ro-bind", "/etc", "/etc",
		"--symlink", "/usr/lib", "/lib",
		"--symlink", "/usr/lib64", "/lib64",
		"--symlink", "/usr/bin", "/bin",
		"--symlink", "/usr/sbin", "/sbin",
		// /proc is needed for most Go programs
		"--proc", "/proc",
		// /dev minimal
		"--dev", "/dev",
		// Writable tmp
		"--tmpfs", "/tmp",
	}

	// Bind the binary's directory as read-only (contains the executable + web assets)
	binDir := filepath.Dir(binPath)
	appDir := "/app"
	args = append(args, "--ro-bind", binDir, appDir)

	// Writable data and log directories
	os.MkdirAll(dataDir, 0755)
	os.MkdirAll(logDir, 0755)
	args = append(args, "--bind", dataDir, appDir+"/data")
	args = append(args, "--bind", logDir, appDir+"/logs")

	// Additional writable paths from config
	for _, p := range cfg.AllowPaths {
		if _, err := os.Stat(p); err == nil {
			args = append(args, "--bind", p, p)
		}
	}

	// PID namespace isolation
	if cfg.IsolatePID {
		args = append(args, "--unshare-pid")
	}

	// Die with parent — if spore stops, sandbox dies too
	args = append(args, "--die-with-parent")

	// Network: default allow. If AllowNet is explicitly false, unshare network.
	if !cfg.AllowNet {
		args = append(args, "--unshare-net")
	}

	// The actual command to run inside the sandbox
	innerBin := filepath.Join(appDir, filepath.Base(binPath))
	args = append(args, "--", innerBin)
	args = append(args, cmd.Args[1:]...) // original arguments

	// Replace the cmd
	cmd.Path, _ = exec.LookPath("bwrap")
	cmd.Args = append([]string{"bwrap"}, args...)
	cmd.Dir = appDir

	// Resource limits via cgroups v2 (if available) — use systemd-run wrapper
	if cfg.MaxMemoryMB > 0 || cfg.MaxCPU > 0 {
		if _, err := exec.LookPath("systemd-run"); err == nil {
			sysArgs := []string{
				"systemd-run", "--user", "--scope", "--quiet",
			}
			if cfg.MaxMemoryMB > 0 {
				sysArgs = append(sysArgs, fmt.Sprintf("-p MemoryMax=%dM", cfg.MaxMemoryMB))
			}
			if cfg.MaxCPU > 0 {
				pct := int(cfg.MaxCPU * 100)
				sysArgs = append(sysArgs, fmt.Sprintf("-p CPUQuota=%d%%", pct))
			}
			// Prepend systemd-run before bwrap
			sysArgs = append(sysArgs, "--")
			sysArgs = append(sysArgs, cmd.Args...)
			cmd.Path, _ = exec.LookPath("systemd-run")
			cmd.Args = sysArgs
		}
	}

	return nil
}

// cgroupLimit applies resource limits via cgroup v2 directly (fallback if systemd-run unavailable).
func cgroupLimit(pid int, memoryMB int, cpuFrac float64) error {
	cgDir := fmt.Sprintf("/sys/fs/cgroup/user.slice/spore-%d", pid)
	if err := os.MkdirAll(cgDir, 0755); err != nil {
		return err // cgroup v2 not available or no permission
	}

	if memoryMB > 0 {
		os.WriteFile(filepath.Join(cgDir, "memory.max"),
			[]byte(strconv.Itoa(memoryMB*1024*1024)), 0644)
	}
	if cpuFrac > 0 {
		// cpu.max format: "quota period" in microseconds
		period := 100000
		quota := int(cpuFrac * float64(period))
		os.WriteFile(filepath.Join(cgDir, "cpu.max"),
			[]byte(fmt.Sprintf("%d %d", quota, period)), 0644)
	}

	// Move process into cgroup
	return os.WriteFile(filepath.Join(cgDir, "cgroup.procs"),
		[]byte(strconv.Itoa(pid)), 0644)
}
