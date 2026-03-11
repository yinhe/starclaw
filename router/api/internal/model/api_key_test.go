package model

import (
	"strings"
	"testing"
)

func TestGenerateAPIKey(t *testing.T) {
	key := GenerateAPIKey()

	if !strings.HasPrefix(key, "sk-star-") {
		t.Errorf("expected key to start with 'sk-star-', got %q", key)
	}

	// sk-star- (8 chars) + 48 hex chars (24 bytes) = 56 chars
	if len(key) != 56 {
		t.Errorf("expected key length 56, got %d (%q)", len(key), key)
	}

	// Two keys should be different
	key2 := GenerateAPIKey()
	if key == key2 {
		t.Error("two generated keys should not be identical")
	}
}
