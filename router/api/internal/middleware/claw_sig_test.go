package middleware

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestClawSignatureAuth_ValidSignature(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Generate test keypair
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	pubHex := hex.EncodeToString(pub)
	hash := sha256.Sum256(pub)
	clawID := "claw:" + hex.EncodeToString(hash[:])[:40]

	// Build signed request
	body := `{"model":"gpt-4o","messages":[{"role":"user","content":"hello"}]}`
	ts := fmt.Sprintf("%d", time.Now().Unix())
	bodyHash := sha256.Sum256([]byte(body))
	message := fmt.Sprintf("%s\n%s\n%s\n%s", "POST", "/v1/chat/completions", ts, hex.EncodeToString(bodyHash[:]))
	sig := ed25519.Sign(priv, []byte(message))

	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("X-Claw-ID", clawID)
	req.Header.Set("X-Claw-PubKey", pubHex)
	req.Header.Set("X-Claw-Signature", hex.EncodeToString(sig))
	req.Header.Set("X-Claw-Timestamp", ts)

	w := httptest.NewRecorder()
	c, r := gin.CreateTestContext(w)
	c.Request = req

	var gotClawID, gotAuthType string
	r.POST("/v1/chat/completions", ClawSignatureAuth(), func(c *gin.Context) {
		gotClawID = c.GetString("claw_id")
		gotAuthType = c.GetString("auth_type")
		c.JSON(200, gin.H{"ok": true})
	})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if gotClawID != clawID {
		t.Fatalf("expected claw_id=%s, got %s", clawID, gotClawID)
	}
	if gotAuthType != "claw" {
		t.Fatalf("expected auth_type=claw, got %s", gotAuthType)
	}
}

func TestClawSignatureAuth_InvalidSignature(t *testing.T) {
	gin.SetMode(gin.TestMode)

	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	pubHex := hex.EncodeToString(pub)
	hash := sha256.Sum256(pub)
	clawID := "claw:" + hex.EncodeToString(hash[:])[:40]

	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader("{}"))
	req.Header.Set("X-Claw-ID", clawID)
	req.Header.Set("X-Claw-PubKey", pubHex)
	req.Header.Set("X-Claw-Signature", hex.EncodeToString(make([]byte, ed25519.SignatureSize))) // zeroed sig
	req.Header.Set("X-Claw-Timestamp", fmt.Sprintf("%d", time.Now().Unix()))

	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)
	r.POST("/v1/chat/completions", ClawSignatureAuth(), func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for invalid sig, got %d: %s", w.Code, w.Body.String())
	}
}

func TestClawSignatureAuth_MissingHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader("{}"))
	// No claw headers

	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)
	r.POST("/v1/chat/completions", ClawSignatureAuth(), func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for missing headers, got %d", w.Code)
	}
}

func TestClawSignatureAuth_TimestampDrift(t *testing.T) {
	gin.SetMode(gin.TestMode)

	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	pubHex := hex.EncodeToString(pub)
	hash := sha256.Sum256(pub)
	clawID := "claw:" + hex.EncodeToString(hash[:])[:40]

	body := `{}`
	// Use timestamp 10 minutes ago (exceeds 5-minute drift)
	ts := fmt.Sprintf("%d", time.Now().Unix()-600)
	bodyHash := sha256.Sum256([]byte(body))
	message := fmt.Sprintf("%s\n%s\n%s\n%s", "POST", "/v1/chat/completions", ts, hex.EncodeToString(bodyHash[:]))
	sig := ed25519.Sign(priv, []byte(message))

	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("X-Claw-ID", clawID)
	req.Header.Set("X-Claw-PubKey", pubHex)
	req.Header.Set("X-Claw-Signature", hex.EncodeToString(sig))
	req.Header.Set("X-Claw-Timestamp", ts)

	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)
	r.POST("/v1/chat/completions", ClawSignatureAuth(), func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for timestamp drift, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeriveClawID(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	pubHex := hex.EncodeToString(pub)

	id, err := deriveClawID(pubHex)
	if err != nil {
		t.Fatalf("deriveClawID error: %v", err)
	}
	if !strings.HasPrefix(id, "claw:") {
		t.Fatalf("expected claw: prefix, got %s", id)
	}
	if len(id) != 5+40 { // "claw:" + 40 hex chars
		t.Fatalf("expected length 45, got %d: %s", len(id), id)
	}
}
