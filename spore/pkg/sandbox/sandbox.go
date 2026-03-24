package sandbox

import (
	"fmt"
	"os/exec"
	"runtime"

	"starclaw.net/spore/pkg/manifest"
)

// Runner wraps a process launch with optional sandbox isolation.
// Each platform provides its own implementation.
type Runner interface {
	// Wrap modifies an exec.Cmd to run inside the sandbox.
	// binPath: absolute path to the binary
	// dataDir: writable data directory
	// logDir:  writable log directory
	// cfg:     sandbox configuration from manifest
	Wrap(cmd *exec.Cmd, binPath, dataDir, logDir string, cfg manifest.Sandbox) error

	// Available returns true if the sandbox backend is usable on this system.
	Available() bool

	// Name returns the sandbox backend name (e.g. "bwrap", "job-object", "sandbox-exec").
	Name() string
}

// New returns the best available sandbox runner for the current platform.
// Returns nil if no sandbox is available (caller should fall back to bare process).
// Implemented in platform-specific files (sandbox_{linux,windows,darwin}.go).
func New() Runner {
	return newPlatformRunner()
}

// EnsureAvailable checks if sandboxing is possible and returns a descriptive error if not.
func EnsureAvailable() error {
	r := New()
	if r == nil {
		return fmt.Errorf("no sandbox backend available on %s/%s. %s", runtime.GOOS, runtime.GOARCH, installHint())
	}
	return nil
}

// installHint returns platform-specific install instructions (in platform files).
func installHint() string {
	return platformInstallHint()
}
