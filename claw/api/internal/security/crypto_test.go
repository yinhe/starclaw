package security

import (
	"strings"
	"testing"
)

func TestKeyManager_NewRandom(t *testing.T) {
	km, err := NewKeyManager()
	if err != nil {
		t.Fatalf("NewKeyManager: %v", err)
	}
	if len(km.masterKey) != 32 {
		t.Fatalf("expected 32-byte key, got %d", len(km.masterKey))
	}
}

func TestKeyManager_Fingerprint(t *testing.T) {
	km, err := NewKeyManager()
	if err != nil {
		t.Fatal(err)
	}
	fp := km.MasterKeyFingerprint()
	if len(fp) != 16 { // 8 bytes = 16 hex chars
		t.Errorf("expected 16-char fingerprint, got %d: %s", len(fp), fp)
	}
}

func TestKeyManager_DeriveKey(t *testing.T) {
	km, err := NewKeyManager()
	if err != nil {
		t.Fatal(err)
	}

	k1 := km.DeriveKey("purpose-a")
	k2 := km.DeriveKey("purpose-b")
	k3 := km.DeriveKey("purpose-a")

	if len(k1) != 32 {
		t.Fatalf("expected 32-byte derived key, got %d", len(k1))
	}

	// Same purpose → same key
	if string(k1) != string(k3) {
		t.Error("same purpose should produce same key")
	}

	// Different purpose → different key
	if string(k1) == string(k2) {
		t.Error("different purpose should produce different key")
	}
}

func TestKeyManager_EncryptDecrypt(t *testing.T) {
	km, err := NewKeyManager()
	if err != nil {
		t.Fatal(err)
	}

	plaintext := "Hello, 星爪! 🦞"
	purpose := "test"

	encrypted, err := km.Encrypt([]byte(plaintext), purpose)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	if encrypted == plaintext {
		t.Error("encrypted should differ from plaintext")
	}

	decrypted, err := km.Decrypt(encrypted, purpose)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}

	if string(decrypted) != plaintext {
		t.Errorf("expected %q, got %q", plaintext, string(decrypted))
	}
}

func TestKeyManager_EncryptDecryptString(t *testing.T) {
	km, err := NewKeyManager()
	if err != nil {
		t.Fatal(err)
	}

	original := "sensitive data with unicode: 数据加密"
	purpose := "user_data"

	enc, err := km.EncryptString(original, purpose)
	if err != nil {
		t.Fatalf("EncryptString: %v", err)
	}

	dec, err := km.DecryptString(enc, purpose)
	if err != nil {
		t.Fatalf("DecryptString: %v", err)
	}

	if dec != original {
		t.Errorf("expected %q, got %q", original, dec)
	}
}

func TestKeyManager_WrongPurpose(t *testing.T) {
	km, err := NewKeyManager()
	if err != nil {
		t.Fatal(err)
	}

	enc, err := km.Encrypt([]byte("secret"), "purpose-a")
	if err != nil {
		t.Fatal(err)
	}

	// Decrypt with wrong purpose should fail
	_, err = km.Decrypt(enc, "purpose-b")
	if err == nil {
		t.Error("expected error when decrypting with wrong purpose")
	}
}

func TestKeyManager_TamperedCiphertext(t *testing.T) {
	km, err := NewKeyManager()
	if err != nil {
		t.Fatal(err)
	}

	enc, err := km.EncryptString("secret data", "test")
	if err != nil {
		t.Fatal(err)
	}

	// Tamper with the ciphertext
	tampered := enc[:len(enc)-4] + "XXXX"
	_, err = km.DecryptString(tampered, "test")
	if err == nil {
		t.Error("expected error when decrypting tampered ciphertext")
	}
}

func TestKeyManager_EmptyData(t *testing.T) {
	km, err := NewKeyManager()
	if err != nil {
		t.Fatal(err)
	}

	enc, err := km.Encrypt([]byte(""), "test")
	if err != nil {
		t.Fatalf("Encrypt empty: %v", err)
	}

	dec, err := km.Decrypt(enc, "test")
	if err != nil {
		t.Fatalf("Decrypt empty: %v", err)
	}

	if string(dec) != "" {
		t.Errorf("expected empty string, got %q", string(dec))
	}
}

func TestKeyManager_DifferentKeysProduceDifferentCiphertext(t *testing.T) {
	km1, _ := NewKeyManager()
	km2, _ := NewKeyManager()

	plain := "same plaintext"
	enc1, _ := km1.EncryptString(plain, "p")
	enc2, _ := km2.EncryptString(plain, "p")

	if enc1 == enc2 {
		t.Error("different keys should produce different ciphertext")
	}
}

func TestKeyManager_CiphertextTooShort(t *testing.T) {
	km, _ := NewKeyManager()
	_, err := km.Decrypt("YWJj", "test") // "abc" in base64 — too short for nonce
	if err == nil {
		t.Error("expected error for short ciphertext")
	}
}

func TestKeyManager_InvalidBase64(t *testing.T) {
	km, _ := NewKeyManager()
	_, err := km.Decrypt("not-valid-base64!!!", "test")
	if err == nil {
		t.Error("expected error for invalid base64")
	}
}

func TestKeyManager_LargeData(t *testing.T) {
	km, _ := NewKeyManager()
	large := strings.Repeat("A", 100_000) // 100KB

	enc, err := km.EncryptString(large, "bulk")
	if err != nil {
		t.Fatalf("Encrypt large: %v", err)
	}

	dec, err := km.DecryptString(enc, "bulk")
	if err != nil {
		t.Fatalf("Decrypt large: %v", err)
	}

	if dec != large {
		t.Error("decrypted large data mismatch")
	}
}
