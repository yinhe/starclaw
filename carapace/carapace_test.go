package carapace

import (
	"strings"
	"testing"
)

// mockBackend for testing
type mockBackend struct {
	key []byte
}

func (m *mockBackend) LoadMasterKey() ([]byte, error) {
	return m.key, nil
}
func (m *mockBackend) Name() string { return "mock" }

func newTestVault(t *testing.T) *Vault {
	t.Helper()
	v, err := New(Config{
		Backend: &mockBackend{key: make([]byte, 32)}, // zero key for testing
		Service: "test",
	})
	if err != nil {
		t.Fatalf("new vault: %v", err)
	}
	return v
}

func TestSealUnsealRoundtrip(t *testing.T) {
	v := newTestVault(t)

	original := "sk-abc123-secret-api-key"
	sealed, err := v.Seal("api_key", original)
	if err != nil {
		t.Fatalf("seal: %v", err)
	}

	if !strings.HasPrefix(sealed, "enc:v1:") {
		t.Errorf("expected enc:v1: prefix, got %s", sealed[:20])
	}

	plain, err := v.Unseal("api_key", sealed)
	if err != nil {
		t.Fatalf("unseal: %v", err)
	}

	if plain != original {
		t.Errorf("roundtrip failed: got %q, want %q", plain, original)
	}
}

func TestUnsealPlaintext(t *testing.T) {
	v := newTestVault(t)

	// Plaintext should pass through unchanged (migration support)
	plain, err := v.Unseal("api_key", "sk-plaintext-key")
	if err != nil {
		t.Fatalf("unseal plaintext: %v", err)
	}
	if plain != "sk-plaintext-key" {
		t.Errorf("plaintext passthrough failed: got %q", plain)
	}
}

func TestSealEmpty(t *testing.T) {
	v := newTestVault(t)

	sealed, err := v.Seal("api_key", "")
	if err != nil {
		t.Fatalf("seal empty: %v", err)
	}
	if sealed != "" {
		t.Errorf("expected empty, got %q", sealed)
	}
}

func TestUnsealEmpty(t *testing.T) {
	v := newTestVault(t)

	plain, err := v.Unseal("api_key", "")
	if err != nil {
		t.Fatalf("unseal empty: %v", err)
	}
	if plain != "" {
		t.Errorf("expected empty, got %q", plain)
	}
}

func TestPurposeIsolation(t *testing.T) {
	v := newTestVault(t)

	sealed, _ := v.Seal("api_key", "my-secret")

	// Same purpose should work
	plain, err := v.Unseal("api_key", sealed)
	if err != nil || plain != "my-secret" {
		t.Fatalf("same purpose failed: %v", err)
	}

	// Different purpose should fail
	_, err = v.Unseal("db_password", sealed)
	if err == nil {
		t.Error("expected error for wrong purpose, got nil")
	}
}

func TestKeyRotation(t *testing.T) {
	v := newTestVault(t)

	// Seal with v1
	sealed1, _ := v.Seal("api_key", "secret-v1")

	// Rotate to v2
	ver, _, err := v.RotateDataKey()
	if err != nil {
		t.Fatalf("rotate: %v", err)
	}
	if ver != 2 {
		t.Errorf("expected v2, got v%d", ver)
	}

	// Seal with v2
	sealed2, _ := v.Seal("api_key", "secret-v2")
	if !strings.HasPrefix(sealed2, "enc:v2:") {
		t.Errorf("expected enc:v2: prefix, got %s", sealed2[:20])
	}

	// Can still unseal v1
	plain1, err := v.Unseal("api_key", sealed1)
	if err != nil || plain1 != "secret-v1" {
		t.Fatalf("unseal v1 after rotation: %v", err)
	}

	// Can unseal v2
	plain2, err := v.Unseal("api_key", sealed2)
	if err != nil || plain2 != "secret-v2" {
		t.Fatalf("unseal v2: %v", err)
	}
}

func TestReEncrypt(t *testing.T) {
	v := newTestVault(t)

	sealed1, _ := v.Seal("api_key", "my-key")
	if !strings.HasPrefix(sealed1, "enc:v1:") {
		t.Fatal("expected v1")
	}

	// Rotate
	v.RotateDataKey()

	// Re-encrypt should produce v2
	sealed2, err := v.ReEncrypt("api_key", sealed1)
	if err != nil {
		t.Fatalf("re-encrypt: %v", err)
	}
	if !strings.HasPrefix(sealed2, "enc:v2:") {
		t.Errorf("expected enc:v2: prefix, got %s", sealed2[:20])
	}

	// Should still decrypt to same value
	plain, _ := v.Unseal("api_key", sealed2)
	if plain != "my-key" {
		t.Errorf("re-encrypted value mismatch: got %q", plain)
	}
}

func TestExportLoadDataKeys(t *testing.T) {
	v := newTestVault(t)
	v.Seal("test", "hello")
	v.RotateDataKey()

	// Export keys
	exported := v.ExportDataKeys()
	if len(exported) != 2 {
		t.Fatalf("expected 2 exported keys, got %d", len(exported))
	}

	// Create new vault with same master key and load exported keys
	v2, _ := New(Config{
		Backend: &mockBackend{key: make([]byte, 32)},
		Service: "test2",
	})

	for _, dk := range exported {
		if err := v2.LoadDataKey(dk.Version, dk.EncryptedKey, dk.Status); err != nil {
			t.Fatalf("load data key v%d: %v", dk.Version, err)
		}
	}

	// Seal with v1 vault, unseal with v2 vault
	sealed, _ := v.Seal("api_key", "cross-vault-secret")
	plain, err := v2.Unseal("api_key", sealed)
	if err != nil || plain != "cross-vault-secret" {
		t.Fatalf("cross-vault unseal: %v (got %q)", err, plain)
	}
}

func TestVaultInfo(t *testing.T) {
	v := newTestVault(t)

	info := v.Info()
	if info.Backend != "mock" {
		t.Errorf("backend: got %q", info.Backend)
	}
	if info.CurrentKeyVersion != 1 {
		t.Errorf("key version: got %d", info.CurrentKeyVersion)
	}
	if !info.Initialized {
		t.Error("expected initialized")
	}
	if info.MasterFingerprint == "" {
		t.Error("expected fingerprint")
	}
}

func TestAuditLog(t *testing.T) {
	v := newTestVault(t)

	v.Seal("api_key", "secret")
	v.Unseal("api_key", "plaintext")

	entries := v.Audit().Recent(10)
	if len(entries) < 2 {
		t.Fatalf("expected at least 2 audit entries, got %d", len(entries))
	}

	stats := v.Audit().Stats()
	if stats["seal"] < 1 || stats["unseal"] < 1 {
		t.Errorf("unexpected stats: %v", stats)
	}
}

func TestFormatParsing(t *testing.T) {
	tests := []struct {
		input     string
		encrypted bool
		version   int
	}{
		{"sk-abc123", false, 0},
		{"enc:base64data", true, 0},
		{"enc:v1:base64data", true, 1},
		{"enc:v42:base64data", true, 42},
		{"", false, 0},
	}

	for _, tt := range tests {
		sf, enc := parseSealed(tt.input)
		if enc != tt.encrypted {
			t.Errorf("parseSealed(%q): encrypted=%v, want %v", tt.input, enc, tt.encrypted)
		}
		if enc && sf.Version != tt.version {
			t.Errorf("parseSealed(%q): version=%d, want %d", tt.input, sf.Version, tt.version)
		}
	}
}
