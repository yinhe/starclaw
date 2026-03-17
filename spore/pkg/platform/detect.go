package platform

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// Info holds detected platform information.
type Info struct {
	OS            string `json:"os"`
	Arch          string `json:"arch"`
	Kernel        string `json:"kernel,omitempty"`
	InitSystem    string `json:"init_system"`    // systemd, openrc, procd, launchd, windows, manual
	Hostname      string `json:"hostname,omitempty"`
	IsRoot        bool   `json:"is_root"`
	HomeDir       string `json:"home_dir"`
	SporeHome     string `json:"spore_home"`     // resolved SPORE_HOME
}

// Detect gathers current platform information.
func Detect() *Info {
	info := &Info{
		OS:   runtime.GOOS,
		Arch: runtime.GOARCH,
	}

	info.Hostname, _ = os.Hostname()
	info.HomeDir, _ = os.UserHomeDir()
	info.Kernel = detectKernel()
	info.InitSystem = detectInitSystem()
	info.IsRoot = detectRoot()
	info.SporeHome = resolveSporeHome(info.HomeDir)

	return info
}

// String returns a human-readable summary.
func (i *Info) String() string {
	return fmt.Sprintf("%s/%s kernel=%s init=%s root=%v home=%s",
		i.OS, i.Arch, i.Kernel, i.InitSystem, i.IsRoot, i.SporeHome)
}

func detectKernel() string {
	switch runtime.GOOS {
	case "linux", "darwin":
		out, err := exec.Command("uname", "-r").Output()
		if err == nil {
			return strings.TrimSpace(string(out))
		}
	case "windows":
		out, err := exec.Command("cmd", "/c", "ver").Output()
		if err == nil {
			return strings.TrimSpace(string(out))
		}
	}
	return "unknown"
}

func detectInitSystem() string {
	switch runtime.GOOS {
	case "darwin":
		return "launchd"
	case "windows":
		return "windows"
	case "linux":
		// Check for systemd
		if _, err := os.Stat("/run/systemd/system"); err == nil {
			return "systemd"
		}
		// Check for openrc
		if _, err := exec.LookPath("rc-service"); err == nil {
			return "openrc"
		}
		// Check for procd (OpenWrt)
		if _, err := os.Stat("/sbin/procd"); err == nil {
			return "procd"
		}
		return "manual"
	default:
		return "manual"
	}
}

func detectRoot() bool {
	switch runtime.GOOS {
	case "windows":
		// Check for admin privileges (simplified)
		_, err := os.Open("\\\\.\\PHYSICALDRIVE0")
		return err == nil
	default:
		return os.Getuid() == 0
	}
}

func resolveSporeHome(homeDir string) string {
	if env := os.Getenv("SPORE_HOME"); env != "" {
		return env
	}
	if homeDir != "" {
		return homeDir + string(os.PathSeparator) + ".spore"
	}
	return "/opt/spore"
}
