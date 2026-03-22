package carapace

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
)

// aesEncrypt encrypts plaintext using AES-256-GCM with the given 32-byte key.
// Returns raw bytes: nonce || ciphertext || tag
func aesEncrypt(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// aesDecrypt decrypts AES-256-GCM ciphertext (nonce || ciphertext || tag).
func aesDecrypt(key, data []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, errors.New("carapace: ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

// aesEncryptBase64 encrypts and returns base64-encoded result.
func aesEncryptBase64(key []byte, plaintext string) (string, error) {
	raw, err := aesEncrypt(key, []byte(plaintext))
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(raw), nil
}

// aesDecryptBase64 decrypts a base64-encoded ciphertext.
func aesDecryptBase64(key []byte, encoded string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	plain, err := aesDecrypt(key, data)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
