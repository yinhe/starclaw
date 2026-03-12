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
	"github.com/yinhe/starclaw/internal/node"
)

const (
	// HeaderNodeID is the claw: address of the requesting node
	HeaderNodeID = "X-Node-ID"
	// HeaderNodePubKey is the hex-encoded Ed25519 public key
	HeaderNodePubKey = "X-Node-PubKey"
	// HeaderNodeSignature is the hex-encoded Ed25519 signature
	HeaderNodeSignature = "X-Node-Signature"
	// HeaderNodeTimestamp is the Unix timestamp (seconds) to prevent replay
	HeaderNodeTimestamp = "X-Node-Timestamp"

	// maxTimestampDrift is the maximum allowed clock skew between nodes
	maxTimestampDrift = 300 // 5 minutes
)

// NodeSignatureClaims holds the verified identity of the requesting node.
type NodeSignatureClaims struct {
	NodeID    string
	PublicKey string
}

// NodeSignatureAuth returns a Gin middleware that verifies Ed25519 node signatures.
//
// Protocol:
//
//	Signature = Ed25519.Sign(privateKey, "METHOD\nPATH\nTIMESTAMP\nBODY_SHA256")
//	Headers:  X-Node-ID, X-Node-PubKey, X-Node-Signature, X-Node-Timestamp
//
// After verification, sets "node_id" and "node_pubkey" in the Gin context.
func NodeSignatureAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		nodeID := c.GetHeader(HeaderNodeID)
		pubKeyHex := c.GetHeader(HeaderNodePubKey)
		signature := c.GetHeader(HeaderNodeSignature)
		tsStr := c.GetHeader(HeaderNodeTimestamp)

		if nodeID == "" || pubKeyHex == "" || signature == "" || tsStr == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "missing node signature headers (X-Node-ID, X-Node-PubKey, X-Node-Signature, X-Node-Timestamp)",
			})
			c.Abort()
			return
		}

		// 1. Parse and validate timestamp (anti-replay)
		ts, err := strconv.ParseInt(tsStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid timestamp"})
			c.Abort()
			return
		}
		drift := math.Abs(float64(time.Now().Unix() - ts))
		if drift > maxTimestampDrift {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": fmt.Sprintf("timestamp drift too large: %.0fs (max %ds)", drift, maxTimestampDrift),
			})
			c.Abort()
			return
		}

		// 2. Verify node_id matches public key
		derivedID, err := node.DeriveNodeIDFromPubKey(pubKeyHex)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid public key"})
			c.Abort()
			return
		}
		if derivedID != nodeID {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "node_id does not match public key"})
			c.Abort()
			return
		}

		// 3. Decode public key and signature
		pubKey, err := hex.DecodeString(pubKeyHex)
		if err != nil || len(pubKey) != ed25519.PublicKeySize {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "malformed public key"})
			c.Abort()
			return
		}
		sigBytes, err := hex.DecodeString(signature)
		if err != nil || len(sigBytes) != ed25519.SignatureSize {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "malformed signature"})
			c.Abort()
			return
		}

		// 4. Reconstruct the signed message: "METHOD\nPATH\nTIMESTAMP\nBODY_SHA256"
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
			c.Abort()
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
			c.JSON(http.StatusUnauthorized, gin.H{"error": "signature verification failed"})
			c.Abort()
			return
		}

		// 6. Set verified node identity in context
		c.Set("node_id", nodeID)
		c.Set("node_pubkey", pubKeyHex)
		c.Next()
	}
}

// SignRequest signs an outgoing HTTP request with the node's Ed25519 key.
// This is the client-side counterpart to NodeSignatureAuth middleware.
func SignRequest(req *http.Request, identity *node.Identity, body []byte) {
	ts := fmt.Sprintf("%d", time.Now().Unix())
	bodyHash := sha256.Sum256(body)
	message := fmt.Sprintf("%s\n%s\n%s\n%s",
		req.Method,
		req.URL.Path,
		ts,
		hex.EncodeToString(bodyHash[:]),
	)

	sig := identity.Sign([]byte(message))

	req.Header.Set(HeaderNodeID, identity.NodeID)
	req.Header.Set(HeaderNodePubKey, identity.PublicKeyHex())
	req.Header.Set(HeaderNodeSignature, hex.EncodeToString(sig))
	req.Header.Set(HeaderNodeTimestamp, ts)
}
