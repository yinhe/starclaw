package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"starclaw.net/spore/pkg/archive"
)

type Manifest struct {
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Description string            `json:"description,omitempty"`
	Platform    Platform          `json:"platform"`
	Binary      string            `json:"binary"`
	Args        []string          `json:"args,omitempty"`
	Resources   Resource          `json:"resources,omitempty"`
	Network     Network           `json:"network,omitempty"`
	Health      Health            `json:"health,omitempty"`
	Update      Update            `json:"update,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	BuiltAt     string            `json:"built_at,omitempty"`
	BuiltBy     string            `json:"built_by,omitempty"`
}

type Platform struct {
	OS   string `json:"os"`
	Arch string `json:"arch"`
}
type Resource struct {
	MinMemoryMB         int `json:"min_memory_mb,omitempty"`
	MinDiskMB           int `json:"min_disk_mb,omitempty"`
	RecommendedMemoryMB int `json:"recommended_memory_mb,omitempty"`
}
type PortMapping struct {
	Port        int    `json:"port"`
	Protocol    string `json:"protocol,omitempty"`
	Description string `json:"description,omitempty"`
}
type Network struct {
	Ports []PortMapping `json:"ports,omitempty"`
}
type Health struct {
	Endpoint        string `json:"endpoint,omitempty"`
	IntervalSeconds int    `json:"interval_seconds,omitempty"`
	TimeoutSeconds  int    `json:"timeout_seconds,omitempty"`
}
type Update struct {
	Channel    string `json:"channel,omitempty"`
	AutoUpdate bool   `json:"auto_update,omitempty"`
	Delta      bool   `json:"delta_enabled,omitempty"`
}

func main() {
	if len(os.Args) < 4 {
		fmt.Println("usage: repack <binary> <os> <arch> [output.spore]")
		os.Exit(1)
	}
	binPath := os.Args[1]
	goos := os.Args[2]
	goarch := os.Args[3]

	binName := "claw"
	if goos == "windows" {
		binName = "claw.exe"
	}

	m := Manifest{
		Name:        "claw",
		Version:     "1.0.0",
		Description: "StarClaw AI Agent Platform",
		Platform:    Platform{OS: goos, Arch: goarch},
		Binary:      binName,
		Args:        []string{"serve"},
		Resources: Resource{
			MinMemoryMB:         256,
			MinDiskMB:           100,
			RecommendedMemoryMB: 1024,
		},
		Network: Network{
			Ports: []PortMapping{{Port: 8080, Protocol: "tcp", Description: "API Server"}},
		},
		Health: Health{
			Endpoint:        "http://localhost:8080/health",
			IntervalSeconds: 30,
			TimeoutSeconds:  5,
		},
		Update: Update{
			Channel:    "stable",
			AutoUpdate: false,
			Delta:      true,
		},
		Env: map[string]string{
			"CLAW_DATA_DIR":  "./data",
			"CLAW_LOG_LEVEL": "info",
		},
		BuiltAt: time.Now().UTC().Format(time.RFC3339),
		BuiltBy: "repack-tool",
	}

	// Create staging dir
	tmpDir, err := os.MkdirTemp("", "spore-repack-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "mktemp: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	// Write manifest
	mdata, _ := json.MarshalIndent(m, "", "  ")
	os.WriteFile(filepath.Join(tmpDir, "manifest.json"), mdata, 0644)

	// Copy binary
	dst := filepath.Join(tmpDir, binName)
	copyFile(binPath, dst)
	os.Chmod(dst, 0755)

	// Determine output path
	out := fmt.Sprintf("claw-v1.0.0-%s-%s.spore", goos, goarch)
	if len(os.Args) > 4 {
		out = os.Args[4]
	}

	checksum, err := archive.Pack(tmpDir, out)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pack: %v\n", err)
		os.Exit(1)
	}
	fi, _ := os.Stat(out)
	fmt.Printf("✓ %s (%.1f MB, sha256=%s)\n", out, float64(fi.Size())/(1024*1024), checksum[:16]+"...")
}

func copyFile(src, dst string) {
	in, err := os.Open(src)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open %s: %v\n", src, err)
		os.Exit(1)
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create %s: %v\n", dst, err)
		os.Exit(1)
	}
	defer out.Close()
	n, err := io.Copy(out, in)
	if err != nil {
		fmt.Fprintf(os.Stderr, "copy: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  copied %s → %s (%d bytes)\n", src, dst, n)
}
