package provider

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/yinhe/starclaw/internal/node"
)

// SignedTransport is an http.RoundTripper that adds Ed25519 signature headers
// to outgoing requests, enabling Claw nodes to authenticate with the Router
// using their cryptographic identity instead of API keys.
//
// Headers added:
//   - X-Claw-ID: node address (claw:xxxx)
//   - X-Claw-PubKey: hex-encoded Ed25519 public key
//   - X-Claw-Signature: hex-encoded Ed25519 signature
//   - X-Claw-Timestamp: Unix timestamp (seconds)
//
// Signed message format: "METHOD\nPATH\nTIMESTAMP\nBODY_SHA256"
type SignedTransport struct {
	Identity  *node.Identity
	Transport http.RoundTripper // underlying transport (nil = http.DefaultTransport)
}

func (t *SignedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Read body for signing
	var bodyBytes []byte
	if req.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read body for signing: %w", err)
		}
		// Restore body for the actual request
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}

	ts := fmt.Sprintf("%d", time.Now().Unix())
	bodyHash := sha256.Sum256(bodyBytes)
	message := fmt.Sprintf("%s\n%s\n%s\n%s",
		req.Method,
		req.URL.Path,
		ts,
		hex.EncodeToString(bodyHash[:]),
	)

	sig := t.Identity.Sign([]byte(message))

	req.Header.Set("X-Claw-ID", t.Identity.NodeID)
	req.Header.Set("X-Claw-PubKey", t.Identity.PublicKeyHex())
	req.Header.Set("X-Claw-Signature", hex.EncodeToString(sig))
	req.Header.Set("X-Claw-Timestamp", ts)

	// Remove the API key Authorization header if present (signature replaces it)
	if auth := req.Header.Get("Authorization"); auth != "" {
		if strings.HasPrefix(auth, "Bearer ") && strings.TrimPrefix(auth, "Bearer ") == "" {
			req.Header.Del("Authorization")
		}
	}

	transport := t.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	return transport.RoundTrip(req)
}
