package backend

import (
	"encoding/hex"
	"fmt"
	"os"
)

// EnvBackend loads the master key from an environment variable.
type EnvBackend struct {
	envVar string
}

// NewEnvBackend creates a backend that reads master key from the given env var.
// The env var should contain a 64-character hex string (32 bytes).
func NewEnvBackend(envVar string) *EnvBackend {
	return &EnvBackend{envVar: envVar}
}

func (b *EnvBackend) LoadMasterKey() ([]byte, error) {
	raw := os.Getenv(b.envVar)
	if raw == "" {
		return nil, fmt.Errorf("carapace: env var %s not set", b.envVar)
	}
	key, err := hex.DecodeString(raw)
	if err != nil || len(key) != 32 {
		return nil, fmt.Errorf("carapace: %s must be 64 hex chars (32 bytes)", b.envVar)
	}
	return key, nil
}

func (b *EnvBackend) Name() string {
	return "env:" + b.envVar
}
