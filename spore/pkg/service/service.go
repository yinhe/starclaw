package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
)

// Config describes a system service to register.
type Config struct {
	Name        string
	Description string
	BinaryPath  string
	Args        []string
	WorkDir     string
	EnvVars     map[string]string
	User        string // linux only
	AutoRestart bool
}

// Register installs the service into the platform's init system.
func Register(cfg *Config) error {
	switch detectInitSystem() {
	case "systemd":
		return registerSystemd(cfg)
	case "launchd":
		return registerLaunchd(cfg)
	case "openrc":
		return registerOpenRC(cfg)
	case "procd":
		return registerProcd(cfg)
	case "windows":
		return registerWindows(cfg)
	default:
		return fmt.Errorf("no supported init system detected; use 'spore run' for manual mode")
	}
}

// Unregister removes the service from the platform's init system.
func Unregister(name string) error {
	switch detectInitSystem() {
	case "systemd":
		return unregisterSystemd(name)
	case "launchd":
		return unregisterLaunchd(name)
	case "openrc":
		return unregisterOpenRC(name)
	case "procd":
		return unregisterProcd(name)
	case "windows":
		return unregisterWindows(name)
	default:
		return nil
	}
}

// Enable sets the service to start on boot.
func Enable(name string) error {
	switch detectInitSystem() {
	case "systemd":
		return run("systemctl", "enable", "spore-"+name)
	case "launchd":
		return run("launchctl", "load", "-w", launchdPlistPath(name))
	case "openrc":
		return run("rc-update", "add", "spore-"+name, "default")
	default:
		return nil
	}
}

// Disable prevents the service from starting on boot.
func Disable(name string) error {
	switch detectInitSystem() {
	case "systemd":
		return run("systemctl", "disable", "spore-"+name)
	case "launchd":
		return run("launchctl", "unload", "-w", launchdPlistPath(name))
	case "openrc":
		return run("rc-update", "del", "spore-"+name)
	default:
		return nil
	}
}

func detectInitSystem() string {
	switch runtime.GOOS {
	case "darwin":
		return "launchd"
	case "windows":
		return "windows"
	case "linux":
		if _, err := os.Stat("/run/systemd/system"); err == nil {
			return "systemd"
		}
		if _, err := exec.LookPath("rc-service"); err == nil {
			return "openrc"
		}
		if _, err := os.Stat("/sbin/procd"); err == nil {
			return "procd"
		}
		return "manual"
	default:
		return "manual"
	}
}

// ── systemd ──

const systemdTemplate = `[Unit]
Description={{.Description}}
After=network.target

[Service]
Type=simple
ExecStart={{.BinaryPath}} {{.ArgsStr}}
WorkingDirectory={{.WorkDir}}
Restart={{if .AutoRestart}}on-failure{{else}}no{{end}}
RestartSec=5
{{range $k, $v := .EnvVars}}Environment="{{$k}}={{$v}}"
{{end}}
{{if .User}}User={{.User}}{{end}}

[Install]
WantedBy=multi-user.target
`

func registerSystemd(cfg *Config) error {
	data := struct {
		*Config
		ArgsStr string
	}{cfg, strings.Join(cfg.Args, " ")}

	path := fmt.Sprintf("/etc/systemd/system/spore-%s.service", cfg.Name)
	if err := renderTemplate(path, systemdTemplate, data); err != nil {
		return err
	}
	run("systemctl", "daemon-reload")
	return nil
}

func unregisterSystemd(name string) error {
	svc := "spore-" + name
	run("systemctl", "stop", svc)
	run("systemctl", "disable", svc)
	path := fmt.Sprintf("/etc/systemd/system/%s.service", svc)
	os.Remove(path)
	run("systemctl", "daemon-reload")
	return nil
}

// ── launchd (macOS) ──

const launchdTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>me.starclaw.spore.{{.Name}}</string>
    <key>ProgramArguments</key>
    <array>
        <string>{{.BinaryPath}}</string>
{{range .Args}}        <string>{{.}}</string>
{{end}}    </array>
    <key>WorkingDirectory</key>
    <string>{{.WorkDir}}</string>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <{{if .AutoRestart}}true{{else}}false{{end}}/>
    <key>StandardOutPath</key>
    <string>{{.WorkDir}}/logs/{{.Name}}.log</string>
    <key>StandardErrorPath</key>
    <string>{{.WorkDir}}/logs/{{.Name}}.err</string>
{{if .EnvVars}}    <key>EnvironmentVariables</key>
    <dict>
{{range $k, $v := .EnvVars}}        <key>{{$k}}</key>
        <string>{{$v}}</string>
{{end}}    </dict>
{{end}}</dict>
</plist>
`

func launchdPlistPath(name string) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", fmt.Sprintf("me.starclaw.spore.%s.plist", name))
}

func registerLaunchd(cfg *Config) error {
	path := launchdPlistPath(cfg.Name)
	os.MkdirAll(filepath.Dir(path), 0755)
	return renderTemplate(path, launchdTemplate, cfg)
}

func unregisterLaunchd(name string) error {
	path := launchdPlistPath(name)
	run("launchctl", "unload", path)
	os.Remove(path)
	return nil
}

// ── OpenRC ──

const openrcTemplate = `#!/sbin/openrc-run

name="spore-{{.Name}}"
description="{{.Description}}"
command="{{.BinaryPath}}"
command_args="{{.ArgsStr}}"
command_background="yes"
pidfile="/run/spore-{{.Name}}.pid"
directory="{{.WorkDir}}"

depend() {
    need net
}
`

func registerOpenRC(cfg *Config) error {
	data := struct {
		*Config
		ArgsStr string
	}{cfg, strings.Join(cfg.Args, " ")}

	path := fmt.Sprintf("/etc/init.d/spore-%s", cfg.Name)
	if err := renderTemplate(path, openrcTemplate, data); err != nil {
		return err
	}
	return os.Chmod(path, 0755)
}

func unregisterOpenRC(name string) error {
	svc := "spore-" + name
	run("rc-service", svc, "stop")
	run("rc-update", "del", svc)
	os.Remove("/etc/init.d/" + svc)
	return nil
}

// ── procd (OpenWrt) ──

const procdTemplate = `#!/bin/sh /etc/rc.common

START=99
STOP=10
USE_PROCD=1

start_service() {
    procd_open_instance
    procd_set_param command {{.BinaryPath}} {{.ArgsStr}}
    procd_set_param respawn
    procd_set_param stdout 1
    procd_set_param stderr 1
    procd_close_instance
}
`

func registerProcd(cfg *Config) error {
	data := struct {
		*Config
		ArgsStr string
	}{cfg, strings.Join(cfg.Args, " ")}

	path := fmt.Sprintf("/etc/init.d/spore-%s", cfg.Name)
	if err := renderTemplate(path, procdTemplate, data); err != nil {
		return err
	}
	os.Chmod(path, 0755)
	return run("/etc/init.d/spore-"+cfg.Name, "enable")
}

func unregisterProcd(name string) error {
	svc := "spore-" + name
	run("/etc/init.d/"+svc, "stop")
	run("/etc/init.d/"+svc, "disable")
	os.Remove("/etc/init.d/" + svc)
	return nil
}

// ── Windows Service ──

func registerWindows(cfg *Config) error {
	args := strings.Join(cfg.Args, " ")
	binpath := fmt.Sprintf("%s %s", cfg.BinaryPath, args)
	return run("sc", "create", "spore-"+cfg.Name,
		"binPath=", binpath,
		"DisplayName=", "Spore: "+cfg.Name,
		"start=", "auto")
}

func unregisterWindows(name string) error {
	svc := "spore-" + name
	run("sc", "stop", svc)
	return run("sc", "delete", svc)
}

// ── Helpers ──

func renderTemplate(path, tmpl string, data interface{}) error {
	t, err := template.New("").Parse(tmpl)
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer f.Close()
	return t.Execute(f, data)
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
