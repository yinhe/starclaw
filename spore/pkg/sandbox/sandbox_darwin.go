package sandbox

import (
	"fmt"
	"os/exec"
	"strings"

	"starclaw.net/spore/pkg/manifest"
)

func newPlatformRunner() Runner {
	r := &darwinRunner{}
	if r.Available() {
		return r
	}
	return nil
}

func platformInstallHint() string {
	return "sandbox-exec should be pre-installed on macOS"
}

// darwinRunner uses macOS sandbox-exec for lightweight filesystem sandboxing.
// sandbox-exec is deprecated by Apple but still functional and pre-installed.
type darwinRunner struct{}

func (r *darwinRunner) Name() string { return "sandbox-exec" }

func (r *darwinRunner) Available() bool {
	_, err := exec.LookPath("sandbox-exec")
	return err == nil
}

// Wrap applies a sandbox-exec profile to the command.
// macOS sandbox profiles use Scheme-like syntax to define rules.
func (r *darwinRunner) Wrap(cmd *exec.Cmd, binPath, dataDir, logDir string, cfg manifest.Sandbox) error {
	profile := r.buildProfile(binPath, dataDir, logDir, cfg)

	origArgs := cmd.Args
	cmd.Path, _ = exec.LookPath("sandbox-exec")
	cmd.Args = append([]string{"sandbox-exec", "-p", profile, "--"}, origArgs...)

	return nil
}

func (r *darwinRunner) buildProfile(binPath, dataDir, logDir string, cfg manifest.Sandbox) string {
	var rules []string
	rules = append(rules, "(version 1)")
	rules = append(rules, "(deny default)")

	// Allow process execution
	rules = append(rules, "(allow process-exec)")
	rules = append(rules, "(allow process-fork)")
	rules = append(rules, "(allow signal)")

	// Allow read of system libraries and frameworks
	rules = append(rules, `(allow file-read* (subpath "/usr"))`)
	rules = append(rules, `(allow file-read* (subpath "/System"))`)
	rules = append(rules, `(allow file-read* (subpath "/Library"))`)
	rules = append(rules, `(allow file-read* (subpath "/private/var"))`)
	rules = append(rules, `(allow file-read* (subpath "/dev"))`)
	rules = append(rules, `(allow file-read* (subpath "/etc"))`)
	rules = append(rules, `(allow file-read* (subpath "/tmp"))`)

	// Allow read of the binary directory
	rules = append(rules, fmt.Sprintf(`(allow file-read* (subpath "%s"))`, escapePath(binPath)))

	// Allow read/write of data and log directories
	rules = append(rules, fmt.Sprintf(`(allow file* (subpath "%s"))`, escapePath(dataDir)))
	rules = append(rules, fmt.Sprintf(`(allow file* (subpath "%s"))`, escapePath(logDir)))
	rules = append(rules, `(allow file* (subpath "/tmp"))`)

	// Additional writable paths
	for _, p := range cfg.AllowPaths {
		rules = append(rules, fmt.Sprintf(`(allow file* (subpath "%s"))`, escapePath(p)))
	}

	// Network access
	if cfg.AllowNet {
		rules = append(rules, "(allow network*)")
	} else {
		rules = append(rules, "(allow network-bind (local ip))")
		rules = append(rules, "(allow network-inbound)")
	}

	// Mach and sysctl (needed for basic Go runtime)
	rules = append(rules, "(allow mach-lookup)")
	rules = append(rules, "(allow sysctl-read)")

	return strings.Join(rules, "\n")
}

func escapePath(p string) string {
	return strings.ReplaceAll(p, `"`, `\"`)
}
