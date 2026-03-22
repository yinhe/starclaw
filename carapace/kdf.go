package carapace

import (
	"crypto/sha256"
	"io"

	"golang.org/x/crypto/hkdf"
)

// deriveKey derives a 32-byte purpose-specific key using HKDF-SHA256.
// This ensures that the same data key produces different encryption keys
// for different purposes (e.g. "api_key" vs "db_password").
func deriveKey(secret []byte, purpose string) []byte {
	r := hkdf.New(sha256.New, secret, []byte("carapace-v1"), []byte(purpose))
	key := make([]byte, 32)
	io.ReadFull(r, key)
	return key
}

// deriveKeySimple derives a key using SHA-256 (compatible with Claw's security/crypto.go).
// Used for backward compatibility with Phase 1-2 "enc:" format.
func deriveKeySimple(master []byte, purpose string) []byte {
	h := sha256.New()
	h.Write(master)
	h.Write([]byte(purpose))
	return h.Sum(nil)
}
