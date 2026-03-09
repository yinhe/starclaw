package v1

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/spf13/viper"
	"github.com/yinhe/starclaw/internal/config"
	"github.com/yinhe/starclaw/internal/model"
	"github.com/yinhe/starclaw/internal/molt"
	"gorm.io/gorm"
)

// PeerHandler manages node identity and peer-to-peer networking
type PeerHandler struct {
	db     *gorm.DB
	cfg    *config.Config
	nodeID string
	httpC  *http.Client
}

func NewPeerHandler(db *gorm.DB, cfg *config.Config) *PeerHandler {
	h := &PeerHandler{
		db:  db,
		cfg: cfg,
		httpC: &http.Client{Timeout: 10 * time.Second},
	}
	h.nodeID = h.loadOrCreateNodeID()
	return h
}

// NodeID returns this node's unique identifier
func (h *PeerHandler) NodeID() string {
	return h.nodeID
}

// --- Node Identity ---

// GetNodeInfo returns this Claw's identity and capabilities
func (h *PeerHandler) GetNodeInfo(c *gin.Context) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	hostname, _ := os.Hostname()
	name := h.cfg.Node.Name
	if name == "" {
		name = hostname
	}

	// Count peers
	var peerCount int64
	h.db.Model(&model.Peer{}).Count(&peerCount)

	// Count online peers
	var onlineCount int64
	h.db.Model(&model.Peer{}).Where("status = ?", "online").Count(&onlineCount)

	c.JSON(http.StatusOK, gin.H{
		"node_id":      h.nodeID,
		"name":         name,
		"hostname":     hostname,
		"address":      h.cfg.Node.Address,
		"region":       h.cfg.Node.Region,
		"version":      molt.Version,
		"go_version":   runtime.Version(),
		"os":           runtime.GOOS,
		"arch":         runtime.GOARCH,
		"memory_mb":    memStats.Alloc / 1024 / 1024,
		"goroutines":   runtime.NumGoroutine(),
		"peer_count":   peerCount,
		"online_peers": onlineCount,
	})
}

// UpdateNodeConfig updates this node's address/name/region
func (h *PeerHandler) UpdateNodeConfig(c *gin.Context) {
	var req struct {
		Address string `json:"address"`
		Name    string `json:"name"`
		Region  string `json:"region"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Address != "" {
		h.cfg.Node.Address = req.Address
		viper.Set("node.address", req.Address)
	}
	if req.Name != "" {
		h.cfg.Node.Name = req.Name
		viper.Set("node.name", req.Name)
	}
	if req.Region != "" {
		h.cfg.Node.Region = req.Region
		viper.Set("node.region", req.Region)
	}
	_ = viper.WriteConfig()

	log.Printf("[node] config updated: address=%s name=%s region=%s", h.cfg.Node.Address, h.cfg.Node.Name, h.cfg.Node.Region)
	c.JSON(http.StatusOK, gin.H{"message": "节点配置已更新"})
}

// --- Peer Management ---

// ListPeers returns all known peers
func (h *PeerHandler) ListPeers(c *gin.Context) {
	var peers []model.Peer
	h.db.Order("created_at DESC").Find(&peers)
	c.JSON(http.StatusOK, peers)
}

// AddPeer adds a remote Claw as a peer via handshake
func (h *PeerHandler) AddPeer(c *gin.Context) {
	var req struct {
		Address string `json:"address" binding:"required"`
		Token   string `json:"token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	addr := strings.TrimRight(req.Address, "/")

	// Generate a shared token if not provided
	token := req.Token
	if token == "" {
		b := make([]byte, 16)
		rand.Read(b)
		token = hex.EncodeToString(b)
	}

	// Handshake: probe the remote node
	remoteInfo, err := h.probeRemoteNode(addr, token)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("无法连接到远程节点: %v", err)})
		return
	}

	remoteNodeID, _ := remoteInfo["node_id"].(string)
	if remoteNodeID == "" {
		c.JSON(http.StatusBadGateway, gin.H{"error": "远程节点未返回 node_id"})
		return
	}

	// Check if already exists
	var existing model.Peer
	if h.db.Where("node_id = ?", remoteNodeID).First(&existing).Error == nil {
		// Update existing
		existing.Address = addr
		existing.Status = "online"
		existing.LastSeen = time.Now()
		existing.Token = token
		if name, ok := remoteInfo["name"].(string); ok {
			existing.Name = name
		}
		if ver, ok := remoteInfo["version"].(string); ok {
			existing.Version = ver
		}
		if region, ok := remoteInfo["region"].(string); ok {
			existing.Region = region
		}
		h.db.Save(&existing)
		c.JSON(http.StatusOK, existing)
		return
	}

	peer := model.Peer{
		ID:       uuid.New().String(),
		NodeID:   remoteNodeID,
		Address:  addr,
		Token:    token,
		Status:   "online",
		LastSeen: time.Now(),
	}
	if name, ok := remoteInfo["name"].(string); ok {
		peer.Name = name
	}
	if ver, ok := remoteInfo["version"].(string); ok {
		peer.Version = ver
	}
	if region, ok := remoteInfo["region"].(string); ok {
		peer.Region = region
	}

	if err := h.db.Create(&peer).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存节点失败"})
		return
	}

	// Register ourselves on the remote node (bidirectional)
	go h.registerSelfOnRemote(addr, token)

	log.Printf("[peer] added peer %s (%s) at %s", peer.Name, peer.NodeID[:8], addr)
	c.JSON(http.StatusCreated, peer)
}

// RemovePeer removes a peer
func (h *PeerHandler) RemovePeer(c *gin.Context) {
	id := c.Param("id")
	if err := h.db.Where("id = ?", id).Delete(&model.Peer{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "节点已移除"})
}

// PingPeer checks if a peer is reachable
func (h *PeerHandler) PingPeer(c *gin.Context) {
	id := c.Param("id")
	var peer model.Peer
	if err := h.db.First(&peer, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "节点不存在"})
		return
	}

	info, err := h.probeRemoteNode(peer.Address, peer.Token)
	if err != nil {
		peer.Status = "offline"
		h.db.Save(&peer)
		c.JSON(http.StatusOK, gin.H{"status": "offline", "error": err.Error()})
		return
	}

	peer.Status = "online"
	peer.LastSeen = time.Now()
	if ver, ok := info["version"].(string); ok {
		peer.Version = ver
	}
	h.db.Save(&peer)

	c.JSON(http.StatusOK, gin.H{"status": "online", "remote": info})
}

// --- Inter-node API (called by remote peers) ---

// HandleHandshake is called by a remote peer to probe this node
func (h *PeerHandler) HandleHandshake(c *gin.Context) {
	hostname, _ := os.Hostname()
	name := h.cfg.Node.Name
	if name == "" {
		name = hostname
	}

	c.JSON(http.StatusOK, gin.H{
		"node_id": h.nodeID,
		"name":    name,
		"version": molt.Version,
		"region":  h.cfg.Node.Region,
		"address": h.cfg.Node.Address,
	})
}

// HandlePeerRegister is called by a remote peer to register itself here
func (h *PeerHandler) HandlePeerRegister(c *gin.Context) {
	var req struct {
		NodeID  string `json:"node_id" binding:"required"`
		Name    string `json:"name"`
		Address string `json:"address" binding:"required"`
		Version string `json:"version"`
		Region  string `json:"region"`
		Token   string `json:"token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Upsert peer
	var peer model.Peer
	if h.db.Where("node_id = ?", req.NodeID).First(&peer).Error != nil {
		peer = model.Peer{
			ID:     uuid.New().String(),
			NodeID: req.NodeID,
		}
	}
	peer.Name = req.Name
	peer.Address = req.Address
	peer.Version = req.Version
	peer.Region = req.Region
	peer.Token = req.Token
	peer.Status = "online"
	peer.LastSeen = time.Now()

	h.db.Save(&peer)
	log.Printf("[peer] remote peer registered: %s (%s) at %s", peer.Name, peer.NodeID[:8], peer.Address)
	c.JSON(http.StatusOK, gin.H{"message": "registered", "node_id": h.nodeID})
}

// HandleRelayTask receives a task delegation from a remote peer
func (h *PeerHandler) HandleRelayTask(c *gin.Context) {
	var req struct {
		FromNodeID string `json:"from_node_id"`
		TaskType   string `json:"task_type"`
		Payload    string `json:"payload"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	log.Printf("[peer] received relay task from %s: type=%s", req.FromNodeID, req.TaskType)

	// TODO: Route to appropriate handler based on task_type
	c.JSON(http.StatusOK, gin.H{
		"message":    "task received",
		"node_id":    h.nodeID,
		"task_type":  req.TaskType,
		"status":     "queued",
	})
}

// --- Helpers ---

func (h *PeerHandler) loadOrCreateNodeID() string {
	// Try to load from file
	data, err := os.ReadFile(".node_id")
	if err == nil {
		nid := strings.TrimSpace(string(data))
		if nid != "" {
			return nid
		}
	}

	// Generate new node ID
	nid := uuid.New().String()
	os.WriteFile(".node_id", []byte(nid), 0600)
	log.Printf("[node] generated new node ID: %s", nid)
	return nid
}

func (h *PeerHandler) probeRemoteNode(address, token string) (map[string]interface{}, error) {
	url := address + "/v1/peer/handshake"
	req, _ := http.NewRequest("GET", url, nil)
	if token != "" {
		req.Header.Set("X-Peer-Token", token)
	}

	resp, err := h.httpC.Do(req)
	if err != nil {
		return nil, fmt.Errorf("连接失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	return result, nil
}

func (h *PeerHandler) registerSelfOnRemote(remoteAddr, token string) {
	hostname, _ := os.Hostname()
	name := h.cfg.Node.Name
	if name == "" {
		name = hostname
	}

	body := map[string]string{
		"node_id": h.nodeID,
		"name":    name,
		"address": h.cfg.Node.Address,
		"version": molt.Version,
		"region":  h.cfg.Node.Region,
		"token":   token,
	}
	data, _ := json.Marshal(body)

	url := remoteAddr + "/v1/peer/register"
	req, _ := http.NewRequest("POST", url, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("X-Peer-Token", token)
	}

	resp, err := h.httpC.Do(req)
	if err != nil {
		log.Printf("[peer] failed to register on remote %s: %v", remoteAddr, err)
		return
	}
	defer resp.Body.Close()
	log.Printf("[peer] registered self on remote %s (HTTP %d)", remoteAddr, resp.StatusCode)
}

// RelayTaskToPeer sends a task to a specific peer
func (h *PeerHandler) RelayTaskToPeer(peerID string, taskType string, payload string) (map[string]interface{}, error) {
	var peer model.Peer
	if err := h.db.First(&peer, "id = ?", peerID).Error; err != nil {
		return nil, fmt.Errorf("peer not found")
	}

	body := map[string]string{
		"from_node_id": h.nodeID,
		"task_type":    taskType,
		"payload":      payload,
	}
	data, _ := json.Marshal(body)

	url := peer.Address + "/v1/peer/relay"
	req, _ := http.NewRequest("POST", url, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	if peer.Token != "" {
		req.Header.Set("X-Peer-Token", peer.Token)
	}

	resp, err := h.httpC.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	return result, nil
}
