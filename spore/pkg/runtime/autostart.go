package runtime

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// EnableAutostart configures the named spore instance to start on system boot.
func (m *Manager) EnableAutostart(name string) error {
	inst, err := m.Get(name)
	if err != nil {
		return err
	}

	sporeBin := m.sporeBinPath()
	if sporeBin == "" {
		return fmt.Errorf("cannot locate spore binary")
	}

	switch runtime.GOOS {
	case "windows":
		return enableAutostartWindows(sporeBin, name)
	case "linux":
		return enableAutostartLinux(sporeBin, name, inst)
	case "darwin":
		return enableAutostartDarwin(sporeBin, name, inst)
	default:
		return fmt.Errorf("autostart not supported on %s", runtime.GOOS)
	}
}

// DisableAutostart removes the autostart configuration for a spore instance.
func (m *Manager) DisableAutostart(name string) error {
	switch runtime.GOOS {
	case "windows":
		return disableAutostartWindows(name)
	case "linux":
		return disableAutostartLinux(name)
	case "darwin":
		return disableAutostartDarwin(name)
	default:
		return fmt.Errorf("autostart not supported on %s", runtime.GOOS)
	}
}

// IsAutostartEnabled checks if autostart is configured for a spore instance.
func (m *Manager) IsAutostartEnabled(name string) bool {
	switch runtime.GOOS {
	case "windows":
		return isAutostartEnabledWindows(name)
	case "linux":
		return isAutostartEnabledLinux(name)
	case "darwin":
		return isAutostartEnabledDarwin(name)
	default:
		return false
	}
}

// sporeBinPath finds the spore binary path.
func (m *Manager) sporeBinPath() string {
	// Check if current executable IS spore
	exe, err := os.Executable()
	if err == nil {
		base := strings.ToLower(filepath.Base(exe))
		if strings.HasPrefix(base, "spore") {
			return exe
		}
	}
	// Look in SPORE_HOME/bin or PATH
	binName := "spore"
	if runtime.GOOS == "windows" {
		binName = "spore.exe"
	}
	candidate := filepath.Join(m.sporeHome, "bin", binName)
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	if p, err := exec.LookPath(binName); err == nil {
		return p
	}
	return ""
}

// --- Windows: Registry Run key ---

const winRunKey = `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`

func enableAutostartWindows(sporeBin, name string) error {
	valName := "StarClaw-" + name
	startCmd := fmt.Sprintf(`"%s" start %s`, sporeBin, name)
	return exec.Command("reg", "add", winRunKey, "/v", valName, "/t", "REG_SZ", "/d", startCmd, "/f").Run()
}

func disableAutostartWindows(name string) error {
	valName := "StarClaw-" + name
	err := exec.Command("reg", "delete", winRunKey, "/v", valName, "/f").Run()
	if err != nil {
		// Value may not exist, that's fine
		return nil
	}
	return nil
}

func isAutostartEnabledWindows(name string) bool {
	valName := "StarClaw-" + name
	err := exec.Command("reg", "query", winRunKey, "/v", valName).Run()
	return err == nil
}

// --- Linux: systemd user service ---

func enableAutostartLinux(sporeBin, name string, inst *SporeInstance) error {
	serviceDir := filepath.Join(os.Getenv("HOME"), ".config", "systemd", "user")
	if err := os.MkdirAll(serviceDir, 0755); err != nil {
		return err
	}

	serviceName := fmt.Sprintf("spore-%s.service", name)
	unit := fmt.Sprintf(`[Unit]
Description=StarClaw %s (Spore)
After=network.target

[Service]
Type=simple
ExecStart=%s start %s
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
`, name, sporeBin, name)

	servicePath := filepath.Join(serviceDir, serviceName)
	if err := os.WriteFile(servicePath, []byte(unit), 0644); err != nil {
		return err
	}

	// Enable the user service
	exec.Command("systemctl", "--user", "daemon-reload").Run()
	return exec.Command("systemctl", "--user", "enable", serviceName).Run()
}

func disableAutostartLinux(name string) error {
	serviceName := fmt.Sprintf("spore-%s.service", name)
	exec.Command("systemctl", "--user", "disable", serviceName).Run()

	serviceDir := filepath.Join(os.Getenv("HOME"), ".config", "systemd", "user")
	servicePath := filepath.Join(serviceDir, serviceName)
	if err := os.Remove(servicePath); err != nil && !os.IsNotExist(err) {
		return err
	}
	exec.Command("systemctl", "--user", "daemon-reload").Run()
	return nil
}

func isAutostartEnabledLinux(name string) bool {
	serviceName := fmt.Sprintf("spore-%s.service", name)
	out, err := exec.Command("systemctl", "--user", "is-enabled", serviceName).Output()
	return err == nil && strings.TrimSpace(string(out)) == "enabled"
}

// --- macOS: launchd plist ---

func enableAutostartDarwin(sporeBin, name string, inst *SporeInstance) error {
	launchAgentsDir := filepath.Join(os.Getenv("HOME"), "Library", "LaunchAgents")
	if err := os.MkdirAll(launchAgentsDir, 0755); err != nil {
		return err
	}

	label := fmt.Sprintf("net.starclaw.spore.%s", name)
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>%s</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
        <string>start</string>
        <string>%s</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <false/>
</dict>
</plist>
`, label, sporeBin, name)

	plistPath := filepath.Join(launchAgentsDir, label+".plist")
	return os.WriteFile(plistPath, []byte(plist), 0644)
}

func disableAutostartDarwin(name string) error {
	label := fmt.Sprintf("net.starclaw.spore.%s", name)
	launchAgentsDir := filepath.Join(os.Getenv("HOME"), "Library", "LaunchAgents")
	plistPath := filepath.Join(launchAgentsDir, label+".plist")

	// Unload if loaded
	exec.Command("launchctl", "unload", plistPath).Run()

	if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func isAutostartEnabledDarwin(name string) bool {
	label := fmt.Sprintf("net.starclaw.spore.%s", name)
	launchAgentsDir := filepath.Join(os.Getenv("HOME"), "Library", "LaunchAgents")
	plistPath := filepath.Join(launchAgentsDir, label+".plist")
	_, err := os.Stat(plistPath)
	return err == nil
}
