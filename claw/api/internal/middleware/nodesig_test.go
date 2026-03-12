package middleware

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw/internal/node"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// helper: create a signed request
func makeSignedRequest(method, path string, body []byte, identity *node.Identity) *http.Request {
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	SignRequest(req, identity, body)
	return req
}

func TestNodeSignatureAuth_ValidSignature(t *testing.T) {
	identity := node.LoadOrCreateIdentity()

	router := gin.New()
	router.POST("/test", NodeSignatureAuth(), func(c *gin.Context) {
		nodeID := c.GetString("node_id")
		pubKey := c.GetString("node_pubkey")
		if nodeID == "" || pubKey == "" {
			t.Error("expected node_id and node_pubkey in context")
		}
		if nodeID != identity.NodeID {
			t.Errorf("expected node_id=%s, got %s", identity.NodeID, nodeID)
		}
		c.JSON(200, gin.H{"ok": true})
	})

	body := []byte(`{"hello":"world"}`)
	req := makeSignedRequest("POST", "/test", body, identity)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestNodeSignatureAuth_MissingHeaders(t *testing.T) {
	router := gin.New()
	router.POST("/test", NodeSignatureAuth(), func(c *gin.Context) {
		t.Error("handler should not be called")
	})

	req := httptest.NewRequest("POST", "/test", bytes.NewReader([]byte("{}")))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestNodeSignatureAuth_WrongSignature(t *testing.T) {
	identity := node.LoadOrCreateIdentity()

	router := gin.New()
	router.POST("/test", NodeSignatureAuth(), func(c *gin.Context) {
		t.Error("handler should not be called")
	})

	body := []byte(`{"hello":"world"}`)
	req := makeSignedRequest("POST", "/test", body, identity)
	// Corrupt the signature
	req.Header.Set(HeaderNodeSignature, "deadbeef"+req.Header.Get(HeaderNodeSignature)[8:])

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestNodeSignatureAuth_ExpiredTimestamp(t *testing.T) {
	identity := node.LoadOrCreateIdentity()

	router := gin.New()
	router.POST("/test", NodeSignatureAuth(), func(c *gin.Context) {
		t.Error("handler should not be called")
	})

	body := []byte(`{}`)
	// Build request with old timestamp
	oldTs := fmt.Sprintf("%d", time.Now().Unix()-600) // 10 minutes ago
	bodyHash := sha256.Sum256(body)
	message := fmt.Sprintf("POST\n/test\n%s\n%s", oldTs, hex.EncodeToString(bodyHash[:]))
	sig := identity.Sign([]byte(message))

	req := httptest.NewRequest("POST", "/test", bytes.NewReader(body))
	req.Header.Set(HeaderNodeID, identity.NodeID)
	req.Header.Set(HeaderNodePubKey, identity.PublicKeyHex())
	req.Header.Set(HeaderNodeSignature, hex.EncodeToString(sig))
	req.Header.Set(HeaderNodeTimestamp, oldTs)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestNodeSignatureAuth_NodeIDMismatch(t *testing.T) {
	identity := node.LoadOrCreateIdentity()

	router := gin.New()
	router.POST("/test", NodeSignatureAuth(), func(c *gin.Context) {
		t.Error("handler should not be called")
	})

	body := []byte(`{}`)
	req := makeSignedRequest("POST", "/test", body, identity)
	// Replace node_id with a fake one
	req.Header.Set(HeaderNodeID, "claw:0000000000000000000000000000000000000000")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSignRequest_SetsAllHeaders(t *testing.T) {
	identity := node.LoadOrCreateIdentity()
	body := []byte(`{"test":true}`)
	req := httptest.NewRequest("POST", "/v1/inference/register", bytes.NewReader(body))
	SignRequest(req, identity, body)

	for _, h := range []string{HeaderNodeID, HeaderNodePubKey, HeaderNodeSignature, HeaderNodeTimestamp} {
		if req.Header.Get(h) == "" {
			t.Errorf("expected header %s to be set", h)
		}
	}

	if req.Header.Get(HeaderNodeID) != identity.NodeID {
		t.Errorf("expected node_id=%s", identity.NodeID)
	}
}
