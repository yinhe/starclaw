package carapace

import (
	"fmt"
	"strconv"
	"strings"
)

// Ciphertext format:
//   plaintext     → "sk-abc123..."           (no prefix, legacy/migration)
//   Phase 1-2     → "enc:{base64}"           (direct master key encryption)
//   Phase 3+      → "enc:v{N}:{base64}"      (envelope encryption with data key version N)

const (
	prefixLegacy   = "enc:"
	prefixVersioned = "enc:v"
)

type sealedFormat struct {
	Version int    // 0 = legacy (Phase 1-2), N = data key version
	Payload string // base64-encoded ciphertext
}

// parseSealed parses a stored ciphertext string into its components.
// Returns (format, isEncrypted).
func parseSealed(s string) (sealedFormat, bool) {
	if !strings.HasPrefix(s, "enc:") {
		return sealedFormat{}, false // plaintext
	}

	// Try versioned format: enc:v{N}:{base64}
	if strings.HasPrefix(s, prefixVersioned) {
		rest := s[len(prefixVersioned):]
		idx := strings.Index(rest, ":")
		if idx > 0 {
			if v, err := strconv.Atoi(rest[:idx]); err == nil {
				return sealedFormat{Version: v, Payload: rest[idx+1:]}, true
			}
		}
	}

	// Legacy format: enc:{base64}
	return sealedFormat{Version: 0, Payload: s[len(prefixLegacy):]}, true
}

// formatSealed creates the stored ciphertext string.
func formatSealed(version int, base64Payload string) string {
	if version == 0 {
		return prefixLegacy + base64Payload
	}
	return fmt.Sprintf("enc:v%d:%s", version, base64Payload)
}
