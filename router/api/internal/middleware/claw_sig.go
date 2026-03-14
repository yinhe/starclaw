package middleware

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	// Claw signature headers (same protocol as Claw's inter-node auth)
	HeaderClawID        = "X-Claw-ID"
	HeaderClawPubKey    = "X-Claw-PubKey"
	HeaderClawSignature = "X-Claw-Signature"
	HeaderClawTimestamp = "X-Claw-Timestamp"

	// Maximum allowed clock skew
	clawMaxTimestampDrift = 300 // 5 minutes
)

// HasClawSignature returns true if the request contains Claw signature headers.
func HasClawSignature(c *gin.Context) bool {
	return c.GetHeader(HeaderClawID) != "" && c.GetHeader(HeaderClawSignature) != ""
}

// ClawSignatureAuth verifies Ed25519 signatures from Claw nodes.
//
// Protocol:
//
//	Signature = Ed25519.Sign(privateKey, "METHOD\nPATH\nTIMESTAMP\nBODY_SHA256")
//	Headers:  X-Claw-ID, X-Claw-PubKey, X-Claw-Signature, X-Claw-Timestamp
//
// After verification, sets "claw_id", "claw_pubkey", and "auth_type"="claw" in the Gin context.
func ClawSignatureAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		clawID := c.GetHeader(HeaderClawID)
		pubKeyHex := c.GetHeader(HeaderClawPubKey)
		signature := c.GetHeader(HeaderClawSignature)
		tsStr := c.GetHeader(HeaderClawTimestamp)

		if clawID == "" || pubKeyHex == "" || signature == "" || tsStr == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"message": "missing Claw signature headers (X-Claw-ID, X-Claw-PubKey, X-Claw-Signature, X-Claw-Timestamp)",
					"type":    "authentication_error",
				},
			})
			return
		}

		// 1. Parse and validate timestamp (anti-replay)
		ts, err := strconv.ParseInt(tsStr, 10, 64)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{"message": "invalid timestamp", "type": "authentication_error"},
			})
			return
		}
		drift := math.Abs(float64(time.Now().Unix() - ts))
		if drift > clawMaxTimestampDrift {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"message": fmt.Sprintf("timestamp drift too large: %.0fs (max %ds)", drift, clawMaxTimestampDrift),
					"type":    "authentication_error",
				},
			})
			return
		}

		// 2. Verify claw_id matches public key: claw_id = "claw:" + SHA256(pubkey)[:40]
		derivedID, err := deriveClawID(pubKeyHex)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{"message": "invalid public key", "type": "authentication_error"},
			})
			return
		}
		if derivedID != clawID {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{"message": "claw_id does not match public key", "type": "authentication_error"},
			})
			return
		}

		// 3. Decode public key and signature bytes
		pubKey, err := hex.DecodeString(pubKeyHex)
		if err != nil || len(pubKey) != ed25519.PublicKeySize {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{"message": "malformed public key", "type": "authentication_error"},
			})
			return
		}
		sigBytes, err := hex.DecodeString(signature)
		if err != nil || len(sigBytes) != ed25519.SignatureSize {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{"message": "malformed signature", "type": "authentication_error"},
			})
			return
		}

		// 4. Reconstruct signed message: "METHOD\nPATH\nTIMESTAMP\nBODY_SHA256"
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error": gin.H{"message": "failed to read body", "type": "invalid_request_error"},
			})
			return
		}
		// Restore body for downstream handlers
		c.Request.Body = io.NopCloser(strings.NewReader(string(bodyBytes)))

		bodyHash := sha256.Sum256(bodyBytes)
		message := fmt.Sprintf("%s\n%s\n%s\n%s",
			c.Request.Method,
			c.Request.URL.Path,
			tsStr,
			hex.EncodeToString(bodyHash[:]),
		)

		// 5. Verify Ed25519 signature
		if !ed25519.Verify(pubKey, []byte(message), sigBytes) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{"message": "signature verification failed", "type": "authentication_error"},
			})
			return
		}

		// 6. Set verified Claw identity in context
		c.Set("claw_id", clawID)
		c.Set("claw_pubkey", pubKeyHex)
		c.Set("auth_type", "claw")
		c.Next()
	}
}

// deriveClawID derives "claw:" + first 40 hex chars of SHA-256(publicKey)
func deriveClawID(pubKeyHex string) (string, error) {
	pubKey, err := hex.DecodeString(pubKeyHex)
	if err != nil || len(pubKey) != ed25519.PublicKeySize {
		return "", fmt.Errorf("invalid public key")
	}
	hash := sha256.Sum256(pubKey)
	return "claw:" + hex.EncodeToString(hash[:])[:40], nil
}
