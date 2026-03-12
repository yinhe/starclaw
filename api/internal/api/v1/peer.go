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
	db          *gorm.DB
	cfg         *config.Config
	identity    *node.Identity
	gossip      *node.GossipEngine
	httpC       *http.Client
	swarmClient swarmAddressSetter
}

// swarmAddressSetter is satisfied by swarm.Client (avoid import cycle)
type swarmAddressSetter interface {
	SetAddress(addr string)
}

func NewPeerHandler(db *gorm.DB, cfg *config.Config, opts ...interface{}) *PeerHandler {
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

	// Accept optional swarm client for address sync
	for _, opt := range opts {
		if sc, ok := opt.(swarmAddressSetter); ok {
			h.swarmClient = sc
		}
	}

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

	// Auto-detect host IPs for display
	publicIP, privateIPs := detectHostIPs()

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
		"public_ip":    publicIP,
		"private_ips":  privateIPs,
	})
}

// AutoSetupNode auto-detects address and region, saves config (one-click setup)
func (h *PeerHandler) AutoSetupNode(c *gin.Context) {
	var req struct {
		UsePublicIP bool   `json:"use_public_ip"` // true=public, false=first private
		Port        string `json:"port"`          // default "8080"
		Name        string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	publicIP, privateIPs := detectHostIPs()

	// Choose IP
	chosenIP := ""
	if req.UsePublicIP && publicIP != "" {
		chosenIP = publicIP
	} else if len(privateIPs) > 0 {
		chosenIP = privateIPs[0]
	} else if publicIP != "" {
		chosenIP = publicIP
	}

	if chosenIP == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法检测到任何可用 IP 地址"})
		return
	}

	port := req.Port
	if port == "" {
		port = "8080"
	}

	// Build address
	address := fmt.Sprintf("http://%s:%s", chosenIP, port)

	// Auto-detect region
	region := detectRegionFromAddress(address)

	// Save config
	h.cfg.Node.Address = address
	viper.Set("node.address", address)
	h.gossip.SetAddress(address)
	if h.swarmClient != nil {
		h.swarmClient.SetAddress(address)
	}

	if region != "" {
		h.cfg.Node.Region = region
		viper.Set("node.region", region)
	}

	if req.Name != "" {
		h.cfg.Node.Name = req.Name
		viper.Set("node.name", req.Name)
	}

	_ = viper.WriteConfig()

	log.Printf("[node] auto-setup: address=%s region=%s", address, region)
	c.JSON(http.StatusOK, gin.H{
		"message": "节点已自动配置",
		"address": address,
		"region":  region,
		"ip":      chosenIP,
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

// --- Middleware-aware endpoints (use NodeSignatureAuth middleware) ---

// HandleGossipSigned receives gossip from a peer (identity verified by middleware).
// The middleware sets "node_id" and "node_pubkey" in the Gin context.
func (h *PeerHandler) HandleGossipSigned(c *gin.Context) {
	nodeID := c.GetString("node_id")
	pubKey := c.GetString("node_pubkey")
	if nodeID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing node identity"})
		return
	}

	var req struct {
		Peers []node.PeerInfo `json:"peers"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify node_id matches public key (double-check)
	derivedID, err := node.DeriveNodeIDFromPubKey(pubKey)
	if err != nil || derivedID != nodeID {
		c.JSON(http.StatusForbidden, gin.H{"error": "node_id does not match public key"})
		return
	}

	// Merge remote peers
	changed := false
	for _, p := range req.Peers {
		if p.NodeID == h.identity.NodeID {
			continue
		}
		if h.gossip.AddPeer(p) {
			changed = true
		}
	}
	if changed {
		h.syncGossipToDB(h.gossip.GetPeers())
	}

	c.JSON(http.StatusOK, gin.H{"peers": h.gossip.GetPeers()})
}

// HandleRelayTaskSigned receives a task delegation (identity verified by middleware).
func (h *PeerHandler) HandleRelayTaskSigned(c *gin.Context) {
	nodeID := c.GetString("node_id")
	if nodeID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing node identity"})
		return
	}

	var req struct {
		TaskType string `json:"task_type"`
		Payload  string `json:"payload"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	log.Printf("[peer] signed relay task from %s: type=%s", nodeID[:16], req.TaskType)

	c.JSON(http.StatusOK, gin.H{
		"message":   "task received",
		"node_id":   h.identity.NodeID,
		"task_type": req.TaskType,
		"status":    "queued",
	})
}

// ResolveNode resolves a claw: node_id to a network address (protected, for frontend)
// Resolution chain: Local DB → Gossip P2P → Overlord (Brood) → Queen (Swarm)
func (h *PeerHandler) ResolveNode(c *gin.Context) {
	nodeID := c.Query("node_id")
	if nodeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "node_id required"})
		return
	}

	// 1. Check local DB (Nydus direct peers)
	var peer model.Peer
	if h.db.Where("node_id = ?", nodeID).First(&peer).Error == nil {
		c.JSON(http.StatusOK, gin.H{"found": true, "source": "nydus", "address": peer.Address, "peer": peer})
		return
	}

	// 2. Ask gossip network (P2P query to known peers)
	info, found := h.gossip.Resolve(nodeID)
	if found && info.Address != "" {
		c.JSON(http.StatusOK, gin.H{"found": true, "source": "gossip", "address": info.Address, "peer": info})
		return
	}

	// 3. Ask Overlord / Brood (enterprise registry)
	if h.cfg.Overlord.Enabled && h.cfg.Overlord.OverlordURL != "" {
		addr, ok := h.resolveViaHTTP(h.cfg.Overlord.OverlordURL+"/brood/resolve", nodeID)
		if ok {
			c.JSON(http.StatusOK, gin.H{"found": true, "source": "brood", "address": addr})
			return
		}
	}

	// 4. Ask Queen / Swarm (public ecosystem registry)
	if h.cfg.Swarm.Enabled && h.cfg.Swarm.QueenURL != "" {
		addr, ok := h.resolveViaHTTP(h.cfg.Swarm.QueenURL+"/swarm/resolve", nodeID)
		if ok {
			c.JSON(http.StatusOK, gin.H{"found": true, "source": "swarm", "address": addr})
			return
		}
	}

	// Build helpful message
	hints := []string{}
	if !h.cfg.Swarm.Enabled {
		hints = append(hints, "加入虫群网络可获得全网节点发现能力")
	}
	if !h.cfg.Overlord.Enabled {
		hints = append(hints, "加入虫巢可发现企业内部节点")
	}
	msg := "无法解析该 Claw 地址 — 该节点不在已知网络中。"
	if len(hints) > 0 {
		msg += "\n提示: " + strings.Join(hints, "; ")
	}

	c.JSON(http.StatusOK, gin.H{"found": false, "message": msg})
}

// resolveViaHTTP queries an external resolve endpoint (Swarm or Brood)
func (h *PeerHandler) resolveViaHTTP(baseURL, nodeID string) (string, bool) {
	reqURL := fmt.Sprintf("%s?claw_id=%s", baseURL, nodeID)
	resp, err := h.httpC.Get(reqURL)
	if err != nil {
		log.Printf("[resolve] query %s failed: %v", baseURL, err)
		return "", false
	}
	defer resp.Body.Close()

	var result struct {
		Found   bool   `json:"found"`
		Address string `json:"address"`
	}
	if json.NewDecoder(resp.Body).Decode(&result) != nil {
		return "", false
	}
	if result.Found && result.Address != "" {
		log.Printf("[resolve] found %s via %s → %s", nodeID, baseURL, result.Address)
		return result.Address, true
	}
	return "", false
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

// detectHostIPs returns the public IP (via external service) and all private IPs.
// Works inside Docker by trying host.docker.internal and filtering Docker bridge IPs.
func detectHostIPs() (publicIP string, privateIPs []string) {
	// Get public IP via ip-api.com (works from inside Docker)
	client := &http.Client{Timeout: 3 * time.Second}
	if resp, err := client.Get("http://ip-api.com/json/?fields=query"); err == nil {
		defer resp.Body.Close()
		var result struct {
			Query string `json:"query"`
		}
		if json.NewDecoder(resp.Body).Decode(&result) == nil && result.Query != "" {
			publicIP = result.Query
		}
	}

	// Try host.docker.internal first (Docker Desktop / extra_hosts: host-gateway)
	if ips, err := net.LookupIP("host.docker.internal"); err == nil {
		for _, ip := range ips {
			if ip4 := ip.To4(); ip4 != nil && ip.IsPrivate() {
				privateIPs = append(privateIPs, ip4.String())
			}
		}
	}

	// Also try HOST_IP env var (user can set in docker-compose)
	if hostIP := os.Getenv("HOST_IP"); hostIP != "" {
		found := false
		for _, p := range privateIPs {
			if p == hostIP {
				found = true
				break
			}
		}
		if !found {
			privateIPs = append([]string{hostIP}, privateIPs...)
		}
	}

	// Get private IPs from network interfaces (filter Docker bridge 172.x)
	ifaces, err := net.Interfaces()
	if err == nil {
		for _, iface := range ifaces {
			if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
				continue
			}
			addrs, _ := iface.Addrs()
			for _, addr := range addrs {
				var ip net.IP
				switch v := addr.(type) {
				case *net.IPNet:
					ip = v.IP
				case *net.IPAddr:
					ip = v.IP
				}
				if ip == nil || ip.IsLoopback() || ip.To4() == nil {
					continue
				}
				// Skip Docker bridge IPs (172.16-31.x.x)
				if ip.IsPrivate() {
					b := ip.To4()
					if b[0] == 172 && b[1] >= 16 && b[1] <= 31 {
						continue // Docker bridge network, skip
					}
					// Check not already added
					s := ip.String()
					found := false
					for _, p := range privateIPs {
						if p == s {
							found = true
							break
						}
					}
					if !found {
						privateIPs = append(privateIPs, s)
					}
				}
			}
		}
	}
	return
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

	// Build city suffix (lowercase, ascii only)
	city := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(geo.City), " ", ""))

	// Map to our region codes: big-region-city
	var base string
	switch geo.CountryCode {
	case "CN":
		region := strings.ToLower(geo.RegionName)
		switch {
		case strings.Contains(region, "shanghai") || strings.Contains(region, "zhejiang") ||
			strings.Contains(region, "jiangsu") || strings.Contains(region, "anhui") ||
			strings.Contains(region, "fujian") || strings.Contains(region, "jiangxi"):
			base = "cn-east"
		case strings.Contains(region, "guangdong") || strings.Contains(region, "guangxi") ||
			strings.Contains(region, "hainan"):
			base = "cn-south"
		case strings.Contains(region, "beijing") || strings.Contains(region, "tianjin") ||
			strings.Contains(region, "hebei") || strings.Contains(region, "shandong") ||
			strings.Contains(region, "liaoning") || strings.Contains(region, "jilin") ||
			strings.Contains(region, "heilongjiang") || strings.Contains(region, "inner mongolia"):
			base = "cn-north"
		case strings.Contains(region, "hubei") || strings.Contains(region, "hunan") ||
			strings.Contains(region, "henan"):
			base = "cn-central"
		case strings.Contains(region, "sichuan") || strings.Contains(region, "chongqing") ||
			strings.Contains(region, "yunnan") || strings.Contains(region, "guizhou") ||
			strings.Contains(region, "tibet"):
			base = "cn-southwest"
		default:
			base = "cn-east"
		}
	case "HK", "MO":
		base = "hk"
	case "TW":
		base = "hk"
	case "JP":
		base = "jp"
	case "US":
		region := strings.ToLower(geo.RegionName)
		if strings.Contains(region, "california") || strings.Contains(region, "oregon") ||
			strings.Contains(region, "washington") || strings.Contains(region, "nevada") {
			base = "us-west"
		} else {
			base = "us-east"
		}
	case "DE", "FR", "GB", "NL", "IE", "IT", "ES", "SE", "NO", "FI", "DK", "CH", "AT", "BE", "PL":
		base = "eu-west"
	case "SG", "MY", "TH", "VN", "PH", "ID":
		base = "ap-southeast"
	default:
		base = strings.ToLower(geo.CountryCode)
	}

	if city != "" {
		return base + "-" + city
	}
	return base
}
