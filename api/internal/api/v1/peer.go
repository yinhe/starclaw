package v1

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
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
	"github.com/yinhe/starclaw/internal/node"
	"gorm.io/gorm"
)

// PeerHandler manages node identity and peer-to-peer networking.
// Uses Ed25519 crypto identity: Node ID = "claw:" + SHA256(publicKey)[:40] (160-bit, Bitcoin-level)
type PeerHandler struct {
	db       *gorm.DB
	cfg      *config.Config
	identity *node.Identity
	gossip   *node.GossipEngine
	httpC    *http.Client
}

func NewPeerHandler(db *gorm.DB, cfg *config.Config) *PeerHandler {
	identity := node.LoadOrCreateIdentity()

	h := &PeerHandler{
		db:       db,
		cfg:      cfg,
		identity: identity,
		httpC:    &http.Client{Timeout: 10 * time.Second},
	}

	// Initialize gossip engine
	h.gossip = node.NewGossipEngine(identity, cfg.Node.Address, func(peers []node.PeerInfo) {
		h.syncGossipToDB(peers)
	})

	// Seed gossip with existing DB peers
	var dbPeers []model.Peer
	db.Find(&dbPeers)
	for _, p := range dbPeers {
		h.gossip.AddPeer(node.PeerInfo{
			NodeID:    p.NodeID,
			Address:   p.Address,
			Name:      p.Name,
			Version:   p.Version,
			Region:    p.Region,
			PublicKey: p.PublicKey,
			LastSeen:  p.LastSeen.Unix(),
		})
	}

	// Start gossip loop (every 30s)
	h.gossip.Start(30 * time.Second)

	return h
}

// NodeID returns this node's crypto-derived ID
func (h *PeerHandler) NodeID() string {
	return h.identity.NodeID
}

// --- Node Identity ---

// GetNodeInfo returns this Claw's cryptographic identity and capabilities
func (h *PeerHandler) GetNodeInfo(c *gin.Context) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	hostname, _ := os.Hostname()
	name := h.cfg.Node.Name
	if name == "" {
		name = hostname
	}

	var peerCount int64
	h.db.Model(&model.Peer{}).Count(&peerCount)
	var onlineCount int64
	h.db.Model(&model.Peer{}).Where("status = ?", "online").Count(&onlineCount)

	c.JSON(http.StatusOK, gin.H{
		"node_id":      h.identity.NodeID,
		"public_key":   h.identity.PublicKeyHex(),
		"fingerprint":  h.identity.Fingerprint(),
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
		h.gossip.SetAddress(req.Address)
	}
	if req.Name != "" {
		h.cfg.Node.Name = req.Name
		viper.Set("node.name", req.Name)
	}
	if req.Region != "" {
		h.cfg.Node.Region = req.Region
		viper.Set("node.region", req.Region)
	}

	// Auto-detect region from address if not set
	detectedRegion := ""
	if h.cfg.Node.Region == "" && h.cfg.Node.Address != "" {
		detectedRegion = detectRegionFromAddress(h.cfg.Node.Address)
		if detectedRegion != "" {
			h.cfg.Node.Region = detectedRegion
			viper.Set("node.region", detectedRegion)
			log.Printf("[node] auto-detected region: %s", detectedRegion)
		}
	}

	_ = viper.WriteConfig()

	log.Printf("[node] config updated: address=%s name=%s region=%s", h.cfg.Node.Address, h.cfg.Node.Name, h.cfg.Node.Region)
	resp := gin.H{"message": "节点配置已更新"}
	if detectedRegion != "" {
		resp["detected_region"] = detectedRegion
	}
	c.JSON(http.StatusOK, resp)
}

// --- Peer Management ---

// ListPeers returns all known peers
func (h *PeerHandler) ListPeers(c *gin.Context) {
	var peers []model.Peer
	h.db.Order("created_at DESC").Find(&peers)
	c.JSON(http.StatusOK, peers)
}

// AddPeer adds a remote Claw via signed handshake
func (h *PeerHandler) AddPeer(c *gin.Context) {
	var req struct {
		Address string `json:"address" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	addr := strings.TrimRight(req.Address, "/")

	// Signed handshake: send our identity, verify theirs
	remoteInfo, err := h.signedHandshake(addr)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("握手失败: %v", err)})
		return
	}

	remoteNodeID, _ := remoteInfo["node_id"].(string)
	remotePubKey, _ := remoteInfo["public_key"].(string)
	if remoteNodeID == "" || remotePubKey == "" {
		c.JSON(http.StatusBadGateway, gin.H{"error": "远程节点未返回有效身份"})
		return
	}

	// Verify: node_id must match public key hash
	derivedID, err := node.DeriveNodeIDFromPubKey(remotePubKey)
	if err != nil || derivedID != remoteNodeID {
		c.JSON(http.StatusBadGateway, gin.H{"error": "远程节点身份验证失败: node_id 与公钥不匹配"})
		return
	}

	// Upsert peer
	var peer model.Peer
	if h.db.Where("node_id = ?", remoteNodeID).First(&peer).Error != nil {
		peer = model.Peer{ID: uuid.New().String(), NodeID: remoteNodeID}
	}
	peer.Address = addr
	peer.PublicKey = remotePubKey
	peer.Status = "online"
	peer.LastSeen = time.Now()
	if name, ok := remoteInfo["name"].(string); ok {
		peer.Name = name
	}
	if ver, ok := remoteInfo["version"].(string); ok {
		peer.Version = ver
	}
	if region, ok := remoteInfo["region"].(string); ok {
		peer.Region = region
	}

	h.db.Save(&peer)

	// Add to gossip engine
	h.gossip.AddPeer(node.PeerInfo{
		NodeID:    remoteNodeID,
		Address:   addr,
		Name:      peer.Name,
		Version:   peer.Version,
		Region:    peer.Region,
		PublicKey: remotePubKey,
		LastSeen:  time.Now().Unix(),
	})

	log.Printf("[peer] added verified peer %s (%s) at %s", peer.Name, remoteNodeID[:8], addr)
	c.JSON(http.StatusCreated, peer)
}

// RemovePeer removes a peer
func (h *PeerHandler) RemovePeer(c *gin.Context) {
	id := c.Param("id")
	var peer model.Peer
	if h.db.First(&peer, "id = ?", id).Error == nil {
		h.gossip.RemovePeer(peer.NodeID)
	}
	h.db.Where("id = ?", id).Delete(&model.Peer{})
	c.JSON(http.StatusOK, gin.H{"message": "节点已移除"})
}

// PingPeer checks if a peer is reachable with signature verification
func (h *PeerHandler) PingPeer(c *gin.Context) {
	id := c.Param("id")
	var peer model.Peer
	if err := h.db.First(&peer, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "节点不存在"})
		return
	}

	info, err := h.signedHandshake(peer.Address)
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

// --- Inter-node API (called by remote peers, public) ---

// HandleHandshake returns this node's signed identity
func (h *PeerHandler) HandleHandshake(c *gin.Context) {
	hostname, _ := os.Hostname()
	name := h.cfg.Node.Name
	if name == "" {
		name = hostname
	}

	challenge, signature := h.identity.SignChallenge()

	c.JSON(http.StatusOK, gin.H{
		"node_id":    h.identity.NodeID,
		"public_key": h.identity.PublicKeyHex(),
		"name":       name,
		"version":    molt.Version,
		"region":     h.cfg.Node.Region,
		"address":    h.cfg.Node.Address,
		"challenge":  challenge,
		"signature":  signature,
	})
}

// HandleGossip receives gossip from a remote peer (signature verified)
func (h *PeerHandler) HandleGossip(c *gin.Context) {
	var req struct {
		FromNodeID string          `json:"from_node_id"`
		PublicKey  string          `json:"public_key"`
		Challenge  string          `json:"challenge"`
		Signature  string          `json:"signature"`
		Peers      []node.PeerInfo `json:"peers"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	myPeers, err := h.gossip.HandleGossip(req.FromNodeID, req.PublicKey, req.Challenge, req.Signature, req.Peers)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"peers": myPeers})
}

// HandleRelayTask receives a signed task delegation from a remote peer
func (h *PeerHandler) HandleRelayTask(c *gin.Context) {
	var req struct {
		FromNodeID string `json:"from_node_id"`
		PublicKey  string `json:"public_key"`
		Challenge  string `json:"challenge"`
		Signature  string `json:"signature"`
		TaskType   string `json:"task_type"`
		Payload    string `json:"payload"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify signature
	if !node.VerifySignature(req.PublicKey, []byte(req.Challenge), req.Signature) {
		c.JSON(http.StatusForbidden, gin.H{"error": "签名验证失败"})
		return
	}

	log.Printf("[peer] verified relay task from %s: type=%s", req.FromNodeID, req.TaskType)

	c.JSON(http.StatusOK, gin.H{
		"message":   "task received",
		"node_id":   h.identity.NodeID,
		"task_type": req.TaskType,
		"status":    "queued",
	})
}

// ResolveNode resolves a claw: node_id to a network address (protected, for frontend)
func (h *PeerHandler) ResolveNode(c *gin.Context) {
	nodeID := c.Query("node_id")
	if nodeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "node_id required"})
		return
	}

	// 1. Check local DB
	var peer model.Peer
	if h.db.Where("node_id = ?", nodeID).First(&peer).Error == nil {
		c.JSON(http.StatusOK, gin.H{"found": true, "address": peer.Address, "peer": peer})
		return
	}

	// 2. Ask gossip network
	info, found := h.gossip.Resolve(nodeID)
	if found && info.Address != "" {
		c.JSON(http.StatusOK, gin.H{"found": true, "address": info.Address, "peer": info})
		return
	}

	c.JSON(http.StatusOK, gin.H{"found": false, "message": "该节点不在已知网络中，请使用 IP 或域名连接"})
}

// HandleResolve is the public endpoint for other nodes to query "do you know this node?"
func (h *PeerHandler) HandleResolve(c *gin.Context) {
	nodeID := c.Query("node_id")
	if nodeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"found": false})
		return
	}

	// Check if it's us
	if nodeID == h.identity.NodeID {
		c.JSON(http.StatusOK, gin.H{
			"found": true,
			"peer": node.PeerInfo{
				NodeID:    h.identity.NodeID,
				Address:   h.cfg.Node.Address,
				Name:      h.cfg.Node.Name,
				PublicKey: h.identity.PublicKeyHex(),
				LastSeen:  time.Now().Unix(),
			},
		})
		return
	}

	// Check local DB
	var peer model.Peer
	if h.db.Where("node_id = ?", nodeID).First(&peer).Error == nil {
		c.JSON(http.StatusOK, gin.H{
			"found": true,
			"peer": node.PeerInfo{
				NodeID:    peer.NodeID,
				Address:   peer.Address,
				Name:      peer.Name,
				Version:   peer.Version,
				Region:    peer.Region,
				PublicKey: peer.PublicKey,
				LastSeen:  peer.LastSeen.Unix(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"found": false})
}

// --- Helpers ---

func (h *PeerHandler) signedHandshake(address string) (map[string]interface{}, error) {
	url := address + "/v1/peer/handshake"
	resp, err := h.httpC.Get(url)
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

	// Verify remote node's signature
	pubKey, _ := result["public_key"].(string)
	challenge, _ := result["challenge"].(string)
	signature, _ := result["signature"].(string)
	if pubKey == "" || challenge == "" || signature == "" {
		return nil, fmt.Errorf("远程节点未提供签名")
	}

	if !node.VerifySignature(pubKey, []byte(challenge), signature) {
		return nil, fmt.Errorf("远程节点签名验证失败")
	}

	// Verify node_id matches public key
	claimedID, _ := result["node_id"].(string)
	derivedID, _ := node.DeriveNodeIDFromPubKey(pubKey)
	if claimedID != derivedID {
		return nil, fmt.Errorf("node_id 与公钥不匹配 (claimed=%s derived=%s)", claimedID, derivedID)
	}

	return result, nil
}

// syncGossipToDB syncs gossip-discovered peers into the database
func (h *PeerHandler) syncGossipToDB(peers []node.PeerInfo) {
	for _, p := range peers {
		if p.NodeID == h.identity.NodeID {
			continue
		}
		var dbPeer model.Peer
		if h.db.Where("node_id = ?", p.NodeID).First(&dbPeer).Error != nil {
			dbPeer = model.Peer{ID: uuid.New().String(), NodeID: p.NodeID}
		}
		dbPeer.Address = p.Address
		dbPeer.Name = p.Name
		dbPeer.Version = p.Version
		dbPeer.Region = p.Region
		dbPeer.PublicKey = p.PublicKey
		dbPeer.Status = "online"
		dbPeer.LastSeen = time.Unix(p.LastSeen, 0)
		h.db.Save(&dbPeer)
	}
}

// detectRegionFromAddress extracts IP from address URL and detects region.
// Private IPs → "local", public IPs → query ip-api.com, domains → resolve then query.
func detectRegionFromAddress(address string) string {
	// Parse URL to extract host
	u, err := url.Parse(address)
	if err != nil {
		return ""
	}
	host := u.Hostname()
	if host == "" {
		return ""
	}

	// Resolve domain to IP if needed
	ip := net.ParseIP(host)
	if ip == nil {
		// It's a domain, try to resolve
		ips, err := net.LookupIP(host)
		if err != nil || len(ips) == 0 {
			return ""
		}
		ip = ips[0]
	}

	// Check for private/local IPs
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() {
		return "local"
	}

	// Query ip-api.com for geolocation (free, no key needed, 45 req/min)
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://ip-api.com/json/%s?fields=status,countryCode,regionName,city", ip.String()))
	if err != nil {
		log.Printf("[node] ip-api.com query failed: %v", err)
		return ""
	}
	defer resp.Body.Close()

	var geo struct {
		Status      string `json:"status"`
		CountryCode string `json:"countryCode"`
		RegionName  string `json:"regionName"`
		City        string `json:"city"`
	}
	if json.NewDecoder(resp.Body).Decode(&geo) != nil || geo.Status != "success" {
		return ""
	}

	// Map to our region codes
	switch geo.CountryCode {
	case "CN":
		region := strings.ToLower(geo.RegionName)
		switch {
		case strings.Contains(region, "shanghai") || strings.Contains(region, "zhejiang") ||
			strings.Contains(region, "jiangsu") || strings.Contains(region, "anhui") ||
			strings.Contains(region, "fujian") || strings.Contains(region, "jiangxi"):
			return "cn-east"
		case strings.Contains(region, "guangdong") || strings.Contains(region, "guangxi") ||
			strings.Contains(region, "hainan"):
			return "cn-south"
		case strings.Contains(region, "beijing") || strings.Contains(region, "tianjin") ||
			strings.Contains(region, "hebei") || strings.Contains(region, "shandong") ||
			strings.Contains(region, "liaoning") || strings.Contains(region, "jilin") ||
			strings.Contains(region, "heilongjiang") || strings.Contains(region, "inner mongolia"):
			return "cn-north"
		case strings.Contains(region, "hubei") || strings.Contains(region, "hunan") ||
			strings.Contains(region, "henan"):
			return "cn-central"
		case strings.Contains(region, "sichuan") || strings.Contains(region, "chongqing") ||
			strings.Contains(region, "yunnan") || strings.Contains(region, "guizhou") ||
			strings.Contains(region, "tibet"):
			return "cn-southwest"
		default:
			return "cn-east" // fallback for China
		}
	case "HK", "MO":
		return "hk"
	case "TW":
		return "hk" // closest region
	case "JP":
		return "jp"
	case "US":
		region := strings.ToLower(geo.RegionName)
		if strings.Contains(region, "california") || strings.Contains(region, "oregon") ||
			strings.Contains(region, "washington") || strings.Contains(region, "nevada") {
			return "us-west"
		}
		return "us-east"
	case "DE", "FR", "GB", "NL", "IE", "IT", "ES", "SE", "NO", "FI", "DK", "CH", "AT", "BE", "PL":
		return "eu-west"
	case "SG", "MY", "TH", "VN", "PH", "ID":
		return "ap-southeast"
	default:
		return strings.ToLower(geo.CountryCode)
	}
}
