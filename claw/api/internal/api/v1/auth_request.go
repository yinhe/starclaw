package v1

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw/internal/node"
)

// AuthRequest represents a pending login authorization from Queen portal
type AuthRequest struct {
	ID        string `json:"id"`
	Challenge string `json:"challenge"`
	Origin    string `json:"origin"` // e.g. "starclaw.net"
	Status    string `json:"status"` // pending / approved / rejected / expired
	CreatedAt int64  `json:"created_at"`
	// Filled after approval
	NodeID    string `json:"node_id,omitempty"`
	PublicKey string `json:"public_key,omitempty"`
	Signature string `json:"signature,omitempty"`
	Username  string `json:"username,omitempty"`
}

type AuthRequestHandler struct {
	identity *node.Identity
	mu       sync.RWMutex
	requests map[string]*AuthRequest // id → request
}

func NewAuthRequestHandler(identity *node.Identity) *AuthRequestHandler {
	h := &AuthRequestHandler{
		identity: identity,
		requests: make(map[string]*AuthRequest),
	}
	// Clean up expired requests every 60s
	go func() {
		for {
			time.Sleep(60 * time.Second)
			h.mu.Lock()
			now := time.Now().Unix()
			for id, req := range h.requests {
				if now-req.CreatedAt > 300 { // 5 min expiry
					delete(h.requests, id)
				}
			}
			h.mu.Unlock()
		}
	}()
	return h
}

// Create creates a new pending auth request (PUBLIC — called by Queen portal)
func (h *AuthRequestHandler) Create(c *gin.Context) {
	var req struct {
		Challenge string `json:"challenge" binding:"required"`
		Origin    string `json:"origin"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "challenge required"})
		return
	}

	// Generate request ID
	idBytes := make([]byte, 16)
	rand.Read(idBytes)
	id := hex.EncodeToString(idBytes)

	authReq := &AuthRequest{
		ID:        id,
		Challenge: req.Challenge,
		Origin:    req.Origin,
		Status:    "pending",
		CreatedAt: time.Now().Unix(),
	}

	h.mu.Lock()
	h.requests[id] = authReq
	h.mu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"id":      id,
		"status":  "pending",
		"node_id": h.identity.NodeID,
	})
}

// GetStatus returns the status of an auth request (PUBLIC — polled by Queen portal)
func (h *AuthRequestHandler) GetStatus(c *gin.Context) {
	id := c.Param("id")

	h.mu.RLock()
	req, ok := h.requests[id]
	h.mu.RUnlock()

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "request not found or expired"})
		return
	}

	// Check expiry
	if time.Now().Unix()-req.CreatedAt > 300 {
		c.JSON(http.StatusGone, gin.H{"error": "request expired", "status": "expired"})
		return
	}

	resp := gin.H{
		"id":     req.ID,
		"status": req.Status,
	}
	if req.Status == "approved" {
		resp["node_id"] = req.NodeID
		resp["public_key"] = req.PublicKey
		resp["signature"] = req.Signature
		resp["challenge"] = req.Challenge
		if req.Username != "" {
			resp["username"] = req.Username
		}
	}
	c.JSON(http.StatusOK, resp)
}

// List returns all pending auth requests (PROTECTED — shown in Claw UI)
func (h *AuthRequestHandler) List(c *gin.Context) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	now := time.Now().Unix()
	pending := []AuthRequest{}
	for _, req := range h.requests {
		if req.Status == "pending" && now-req.CreatedAt <= 300 {
			pending = append(pending, *req)
		}
	}

	c.JSON(http.StatusOK, gin.H{"requests": pending})
}

// Approve signs the challenge and marks the request as approved (PROTECTED)
func (h *AuthRequestHandler) Approve(c *gin.Context) {
	id := c.Param("id")

	h.mu.Lock()
	defer h.mu.Unlock()

	req, ok := h.requests[id]
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "request not found"})
		return
	}
	if req.Status != "pending" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "request already " + req.Status})
		return
	}
	if time.Now().Unix()-req.CreatedAt > 300 {
		c.JSON(http.StatusGone, gin.H{"error": "request expired"})
		return
	}

	// Sign the challenge
	sig := h.identity.Sign([]byte(req.Challenge))
	req.Status = "approved"
	req.NodeID = h.identity.NodeID
	req.PublicKey = h.identity.PublicKeyHex()
	req.Signature = fmt.Sprintf("%x", sig)
	req.Username = c.GetString("username")

	c.JSON(http.StatusOK, gin.H{"status": "approved"})
}

// Reject marks the request as rejected (PROTECTED)
func (h *AuthRequestHandler) Reject(c *gin.Context) {
	id := c.Param("id")

	h.mu.Lock()
	defer h.mu.Unlock()

	req, ok := h.requests[id]
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "request not found"})
		return
	}
	if req.Status != "pending" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "request already " + req.Status})
		return
	}

	req.Status = "rejected"
	c.JSON(http.StatusOK, gin.H{"status": "rejected"})
}
