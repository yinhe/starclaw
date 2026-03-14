package v1

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw/internal/config"
	"github.com/yinhe/starclaw/internal/node"
	"github.com/yinhe/starclaw/internal/swarm"
)

// QueenAccountHandler handles linking the local Claw owner to a Queen platform account.
// This enables cross-Claw identity, bounty settlement, marketplace, and community features.
type QueenAccountHandler struct {
	cfg         *config.Config
	swarmClient *swarm.Client
	identity    *node.Identity
	httpC       *http.Client
	mu          sync.RWMutex
	// Cached credentials (loaded from .queen_credentials)
	queenUserID string
	queenToken  string
	queenEmail  string
}

func NewQueenAccountHandler(cfg *config.Config, sc *swarm.Client, identity *node.Identity) *QueenAccountHandler {
	h := &QueenAccountHandler{
		cfg:      cfg,
		identity: identity,
		httpC:    &http.Client{Timeout: 15 * time.Second},
	}
	if sc != nil {
		h.swarmClient = sc
	}
	// Load persisted credentials
	h.loadCredentials()
	return h
}

// GetStatus returns whether this Claw is linked to a Queen account.
func (h *QueenAccountHandler) GetStatus(c *gin.Context) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	linked := h.queenUserID != "" && h.queenToken != ""
	resp := gin.H{
		"linked":        linked,
		"queen_user_id": h.queenUserID,
		"email":         h.queenEmail,
		"queen_api_url": h.getQueenAPIURL(),
	}

	// If linked, verify token is still valid by calling Queen /v1/user/profile
	if linked {
		profile, err := h.fetchProfile()
		if err != nil {
			resp["token_valid"] = false
			resp["error"] = "Queen token 已过期，请重新登录"
		} else {
			resp["token_valid"] = true
			if email, ok := profile["email"].(string); ok {
				resp["email"] = email
			}
			if username, ok := profile["username"].(string); ok {
				resp["username"] = username
			}
		}
	}

	c.JSON(http.StatusOK, resp)
}

// Link authenticates with Queen and binds this Claw node to the Queen account.
func (h *QueenAccountHandler) Link(c *gin.Context) {
	var req struct {
		Email    string `json:"email"`
		Phone    string `json:"phone"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入账号和密码"})
		return
	}
	if req.Email == "" && req.Phone == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入邮箱或手机号"})
		return
	}

	queenAPI := h.getQueenAPIURL()
	if queenAPI == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "未配置 Queen 地址，请先加入虫群"})
		return
	}

	// Step 1: Authenticate with Queen
	loginBody := map[string]string{"password": req.Password}
	if req.Email != "" {
		loginBody["email"] = req.Email
	} else {
		loginBody["phone"] = req.Phone
	}

	loginData, _ := json.Marshal(loginBody)
	loginResp, err := h.httpC.Post(queenAPI+"/v1/auth/login", "application/json", bytes.NewReader(loginData))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("无法连接 Queen: %v", err)})
		return
	}
	defer loginResp.Body.Close()

	var loginResult map[string]interface{}
	if err := json.NewDecoder(loginResp.Body).Decode(&loginResult); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Queen 响应解析失败"})
		return
	}

	if loginResp.StatusCode != 200 {
		errMsg := "登录失败"
		if data, ok := loginResult["data"].(map[string]interface{}); ok {
			if e, ok := data["error"].(string); ok {
				errMsg = e
			}
		} else if e, ok := loginResult["error"].(string); ok {
			errMsg = e
		} else if msg, ok := loginResult["message"].(string); ok {
			errMsg = msg
		}
		c.JSON(http.StatusUnauthorized, gin.H{"error": errMsg})
		return
	}

	// Extract token and user info from Queen response
	data := loginResult
	if d, ok := loginResult["data"].(map[string]interface{}); ok {
		data = d
	}
	queenToken, _ := data["token"].(string)
	if queenToken == "" {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Queen 未返回 token"})
		return
	}

	var queenUserID string
	if user, ok := data["user"].(map[string]interface{}); ok {
		if id, ok := user["id"].(string); ok {
			queenUserID = id
		}
	}
	if queenUserID == "" {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Queen 未返回 user_id"})
		return
	}

	queenEmail := req.Email
	if queenEmail == "" {
		queenEmail = req.Phone
	}

	// Step 2: Bind this Claw node to the Queen account
	clawID := ""
	if h.identity != nil {
		clawID = h.identity.NodeID
	}

	localUserID := c.GetString("user_id")

	bindBody := map[string]string{
		"queen_user_id": queenUserID,
		"node_id":       clawID,
		"local_user_id": localUserID,
		"node_name":     h.cfg.Swarm.NodeName,
		"node_addr":     h.cfg.Node.Address,
		"node_region":   h.cfg.Swarm.Region,
	}

	// Call Queen's internal bind API (uses JWT secret as X-Node-Token)
	bindData, _ := json.Marshal(bindBody)
	bindReq, _ := http.NewRequest("POST", queenAPI+"/internal/user/bind", bytes.NewReader(bindData))
	bindReq.Header.Set("Content-Type", "application/json")
	bindReq.Header.Set("X-Node-Token", h.cfg.JWT.Secret)

	bindResp, err := h.httpC.Do(bindReq)
	if err != nil {
		log.Printf("[queen] bind failed: %v (continuing with link anyway)", err)
		// Don't fail — the link is still useful even without the bind call
	} else {
		bindResp.Body.Close()
		if bindResp.StatusCode >= 400 {
			log.Printf("[queen] bind returned %d (continuing)", bindResp.StatusCode)
		}
	}

	// Step 3: Persist credentials locally
	h.mu.Lock()
	h.queenUserID = queenUserID
	h.queenToken = queenToken
	h.queenEmail = queenEmail
	h.mu.Unlock()
	h.saveCredentials()

	log.Printf("[queen] linked to Queen account: user=%s email=%s claw=%s", queenUserID, queenEmail, clawID)
	c.JSON(http.StatusOK, gin.H{
		"message":       "已关联 Queen 账号",
		"queen_user_id": queenUserID,
		"email":         queenEmail,
	})
}

// LinkWithClaw authenticates with Queen using this Claw node's Ed25519 identity.
// No email/password needed — one-click association.
func (h *QueenAccountHandler) LinkWithClaw(c *gin.Context) {
	if h.identity == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "节点身份未初始化"})
		return
	}

	queenAPI := h.getQueenAPIURL()
	if queenAPI == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "未配置 Queen 地址，请先加入虫群"})
		return
	}

	clawID := h.identity.NodeID
	pubKeyHex := h.identity.PublicKeyHex()

	// Step 1: Get challenge from Queen
	challengeResp, err := h.httpC.Post(queenAPI+"/v1/auth/claw/challenge", "application/json",
		bytes.NewReader([]byte(fmt.Sprintf(`{"node_id":"%s"}`, clawID))))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("无法连接 Queen: %v", err)})
		return
	}
	defer challengeResp.Body.Close()

	var challengeResult map[string]interface{}
	json.NewDecoder(challengeResp.Body).Decode(&challengeResult)
	challenge, _ := challengeResult["challenge"].(string)
	if challenge == "" {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Queen 未返回 challenge"})
		return
	}

	// Step 2: Sign the challenge
	signature := h.identity.Sign([]byte(challenge))
	sigHex := fmt.Sprintf("%x", signature)

	// Step 3: Verify with Queen → get token
	verifyBody := map[string]string{
		"node_id":    clawID,
		"challenge":  challenge,
		"signature":  sigHex,
		"public_key": pubKeyHex,
		"node_url":   h.cfg.Node.Address,
	}
	verifyData, _ := json.Marshal(verifyBody)
	verifyResp, err := h.httpC.Post(queenAPI+"/v1/auth/claw/verify", "application/json", bytes.NewReader(verifyData))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("验证失败: %v", err)})
		return
	}
	defer verifyResp.Body.Close()

	var verifyResult map[string]interface{}
	json.NewDecoder(verifyResp.Body).Decode(&verifyResult)

	if verifyResp.StatusCode != 200 {
		errMsg := "Claw 签名验证失败"
		if e, ok := verifyResult["error"].(string); ok {
			errMsg = e
		}
		c.JSON(http.StatusBadGateway, gin.H{"error": errMsg})
		return
	}

	queenToken, _ := verifyResult["token"].(string)
	if queenToken == "" {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Queen 未返回 token"})
		return
	}

	var queenUserID, queenNickname string
	if user, ok := verifyResult["user"].(map[string]interface{}); ok {
		queenUserID, _ = user["id"].(string)
		queenNickname, _ = user["nickname"].(string)
	}
	if queenUserID == "" {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Queen 未返回 user_id"})
		return
	}

	// Step 4: Save credentials
	h.mu.Lock()
	h.queenUserID = queenUserID
	h.queenToken = queenToken
	h.queenEmail = queenNickname // reuse field for display name
	h.mu.Unlock()
	h.saveCredentials()

	log.Printf("[queen] linked via Claw signature: user=%s claw=%s", queenUserID, clawID)
	c.JSON(http.StatusOK, gin.H{
		"message":       "已通过 Claw 身份关联 Queen 账号",
		"queen_user_id": queenUserID,
		"nickname":      queenNickname,
	})
}

// Unlink removes the Queen account association from this Claw.
func (h *QueenAccountHandler) Unlink(c *gin.Context) {
	h.mu.Lock()
	h.queenUserID = ""
	h.queenToken = ""
	h.queenEmail = ""
	h.mu.Unlock()

	os.Remove(".queen_credentials")
	log.Println("[queen] unlinked from Queen account")
	c.JSON(http.StatusOK, gin.H{"message": "已解除 Queen 账号关联"})
}

// --- Internal helpers ---

func (h *QueenAccountHandler) getQueenAPIURL() string {
	queenURL := h.cfg.Swarm.QueenURL
	if queenURL == "" {
		return ""
	}
	// Derive queen-api URL from swarm URL:
	// https://swarm.starclaw.net → https://api.starclaw.net
	queenURL = strings.TrimSuffix(queenURL, "/")
	if strings.Contains(queenURL, "swarm.") {
		return strings.Replace(queenURL, "swarm.", "api.", 1)
	}
	// Fallback: if queen_url doesn't contain "swarm.", treat it as the base domain
	// and append /api prefix
	return queenURL
}

func (h *QueenAccountHandler) fetchProfile() (map[string]interface{}, error) {
	h.mu.RLock()
	token := h.queenToken
	h.mu.RUnlock()

	apiURL := h.getQueenAPIURL()
	req, _ := http.NewRequest("GET", apiURL+"/v1/user/profile", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := h.httpC.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if data, ok := result["data"].(map[string]interface{}); ok {
		return data, nil
	}
	return result, nil
}

func (h *QueenAccountHandler) saveCredentials() {
	h.mu.RLock()
	defer h.mu.RUnlock()
	data := fmt.Sprintf("%s\n%s\n%s\n", h.queenUserID, h.queenToken, h.queenEmail)
	os.WriteFile(".queen_credentials", []byte(data), 0600)
}

func (h *QueenAccountHandler) loadCredentials() {
	data, err := os.ReadFile(".queen_credentials")
	if err != nil {
		return
	}
	parts := strings.Split(strings.TrimSpace(string(data)), "\n")
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(parts) >= 1 {
		h.queenUserID = parts[0]
	}
	if len(parts) >= 2 {
		h.queenToken = parts[1]
	}
	if len(parts) >= 3 {
		h.queenEmail = parts[2]
	}
}

// QueenUserID returns the linked Queen user ID (for use by other handlers).
func (h *QueenAccountHandler) QueenUserID() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.queenUserID
}

// QueenToken returns the linked Queen JWT token.
func (h *QueenAccountHandler) QueenToken() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.queenToken
}
