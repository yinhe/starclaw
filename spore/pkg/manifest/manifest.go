package manifest

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"time"
)

// Manifest describes a Spore package.
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
	Checksum    string            `json:"checksum,omitempty"`
	BuiltAt     string            `json:"built_at,omitempty"`
	BuiltBy     string            `json:"built_by,omitempty"`
}

// Platform describes the target platform.
type Platform struct {
	OS        string `json:"os"`
	Arch      string `json:"arch"`
	MinKernel string `json:"min_kernel,omitempty"`
}

// Resource describes minimum and recommended resources.
type Resource struct {
	MinMemoryMB         int `json:"min_memory_mb,omitempty"`
	MinDiskMB           int `json:"min_disk_mb,omitempty"`
	RecommendedMemoryMB int `json:"recommended_memory_mb,omitempty"`
}

// PortMapping describes a network port.
type PortMapping struct {
	Port        int    `json:"port"`
	Protocol    string `json:"protocol,omitempty"` // tcp, udp
	Description string `json:"description,omitempty"`
}

// Network describes network requirements.
type Network struct {
	Ports []PortMapping `json:"ports,omitempty"`
}

// Health describes health check configuration.
type Health struct {
	Endpoint        string `json:"endpoint,omitempty"`
	IntervalSeconds int    `json:"interval_seconds,omitempty"`
	TimeoutSeconds  int    `json:"timeout_seconds,omitempty"`
}

// Update describes update configuration.
type Update struct {
	Channel    string `json:"channel,omitempty"` // stable, beta, nightly
	AutoUpdate bool   `json:"auto_update,omitempty"`
	Delta      bool   `json:"delta_enabled,omitempty"`
}

// Load reads a manifest from a JSON file.
func Load(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

// Save writes the manifest to a JSON file.
func (m *Manifest) Save(path string) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// Validate checks required fields.
func (m *Manifest) Validate() error {
	if m.Name == "" {
		return fmt.Errorf("manifest: name is required")
	}
	if m.Version == "" {
		return fmt.Errorf("manifest: version is required")
	}
	if m.Binary == "" {
		return fmt.Errorf("manifest: binary path is required")
	}
	if m.Platform.OS == "" || m.Platform.Arch == "" {
		return fmt.Errorf("manifest: platform os and arch are required")
	}
	return nil
}

// MatchesRuntime returns true if the manifest's platform matches the current OS/Arch.
func (m *Manifest) MatchesRuntime() bool {
	return m.Platform.OS == runtime.GOOS && m.Platform.Arch == runtime.GOARCH
}

// PackageName returns the conventional package filename.
func (m *Manifest) PackageName() string {
	return fmt.Sprintf("%s-v%s-%s-%s.spore", m.Name, m.Version, m.Platform.OS, m.Platform.Arch)
}

// NewDefault creates a manifest with sensible defaults for the current platform.
func NewDefault(name, version, binary string) *Manifest {
	return &Manifest{
		Name:    name,
		Version: version,
		Binary:  binary,
		Platform: Platform{
			OS:   runtime.GOOS,
			Arch: runtime.GOARCH,
		},
		Resources: Resource{
			MinMemoryMB:         256,
			MinDiskMB:           100,
			RecommendedMemoryMB: 1024,
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
		BuiltAt: time.Now().UTC().Format(time.RFC3339),
		BuiltBy: fmt.Sprintf("hatchery/%s", runtime.Version()),
	}
}
