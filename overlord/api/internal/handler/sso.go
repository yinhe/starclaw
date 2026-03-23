package handler

import (
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"starclaw.net/overlord/api/internal/middleware"
	"starclaw.net/overlord/api/internal/model"
	"gorm.io/gorm"
)

type SSOHandler struct {
	db *gorm.DB
}

func NewSSOHandler(db *gorm.DB) *SSOHandler {
	return &SSOHandler{db: db}
}

// ==================== Provider CRUD ====================

// ListProviders GET /brood/sso/providers
func (h *SSOHandler) ListProviders(c *gin.Context) {
	var providers []model.SSOProvider
	q := h.db.Order("created_at DESC")
	if team := c.Query("team_id"); team != "" {
		q = q.Where("team_id = ?", team)
	}
	q.Find(&providers)
	c.JSON(http.StatusOK, gin.H{"providers": providers, "total": len(providers)})
}

// GetProvider GET /brood/sso/providers/:id
func (h *SSOHandler) GetProvider(c *gin.Context) {
	var provider model.SSOProvider
	if err := h.db.First(&provider, "id = ?", c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "provider not found"})
		return
	}
	c.JSON(http.StatusOK, provider)
}

// CreateProvider POST /brood/sso/providers
func (h *SSOHandler) CreateProvider(c *gin.Context) {
	var provider model.SSOProvider
	if err := c.ShouldBindJSON(&provider); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if provider.Type == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "type is required (oauth2, ldap, oidc)"})
		return
	}
	if provider.DefaultRole == "" {
		provider.DefaultRole = "viewer"
	}
	if err := h.db.Create(&provider).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	audit(h.db, c, "sso_provider.create", provider.ID, fmt.Sprintf("name=%s type=%s provider=%s", provider.Name, provider.Type, provider.Provider))
	c.JSON(http.StatusCreated, provider)
}

// UpdateProvider PUT /brood/sso/providers/:id
func (h *SSOHandler) UpdateProvider(c *gin.Context) {
	var provider model.SSOProvider
	if err := h.db.First(&provider, "id = ?", c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "provider not found"})
		return
	}
	var req model.SSOProvider
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := map[string]interface{}{
		"name":         req.Name,
		"type":         req.Type,
		"provider":     req.Provider,
		"enabled":      req.Enabled,
		"default_role": req.DefaultRole,
		"team_id":      req.TeamID,
		// OAuth2
		"client_id":    req.ClientID,
		"auth_url":     req.AuthURL,
		"token_url":    req.TokenURL,
		"userinfo_url": req.UserInfoURL,
		"redirect_url": req.RedirectURL,
		"scopes":       req.Scopes,
		// LDAP
		"ldap_host":        req.LDAPHost,
		"ldap_base_dn":     req.LDAPBaseDN,
		"ldap_bind_dn":     req.LDAPBindDN,
		"ldap_user_filter": req.LDAPUserFilter,
		"ldap_use_tls":     req.LDAPUseTLS,
		"ldap_attr_email":  req.LDAPAttrEmail,
		"ldap_attr_name":   req.LDAPAttrName,
	}
	// Only update secrets if provided
	if req.ClientSecret != "" {
		updates["client_secret"] = req.ClientSecret
	}
	if req.LDAPBindPass != "" {
		updates["ldap_bind_pass"] = req.LDAPBindPass
	}
	h.db.Model(&provider).Updates(updates)
	h.db.First(&provider, "id = ?", c.Param("id"))
	audit(h.db, c, "sso_provider.update", provider.ID, provider.Name)
	c.JSON(http.StatusOK, provider)
}

// DeleteProvider DELETE /brood/sso/providers/:id
func (h *SSOHandler) DeleteProvider(c *gin.Context) {
	id := c.Param("id")
	if err := h.db.Where("id = ?", id).Delete(&model.SSOProvider{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete provider"})
		return
	}
	audit(h.db, c, "sso_provider.delete", id, "")
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

// ==================== OAuth2 Flow ====================

// OAuth2Authorize GET /brood/sso/oauth2/authorize?provider_id=xxx
// Redirects the user to the external IdP's authorization page
func (h *SSOHandler) OAuth2Authorize(c *gin.Context) {
	providerID := c.Query("provider_id")
	if providerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "provider_id is required"})
		return
	}

	var provider model.SSOProvider
	if err := h.db.First(&provider, "id = ? AND enabled = ?", providerID, true).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "SSO provider not found or disabled"})
		return
	}
	if provider.Type != "oauth2" && provider.Type != "oidc" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "provider is not OAuth2/OIDC type"})
		return
	}

	// Generate state parameter
	state := generateRandomString(32)
	session := model.SSOSession{
		ProviderID: provider.ID,
		State:      state,
		Status:     "pending",
		IPAddress:  c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
		ExpiresAt:  time.Now().Add(10 * time.Minute),
	}
	h.db.Create(&session)

	// Build authorization URL
	authURL := provider.AuthURL
	scopes := provider.Scopes
	if scopes == "" {
		scopes = "openid,profile,email"
	}

	params := url.Values{
		"client_id":     {provider.ClientID},
		"redirect_uri":  {provider.RedirectURL},
		"response_type": {"code"},
		"scope":         {strings.ReplaceAll(scopes, ",", " ")},
		"state":         {state},
	}

	redirectTo := authURL + "?" + params.Encode()

	c.JSON(http.StatusOK, gin.H{
		"redirect_url": redirectTo,
		"state":        state,
		"expires_in":   600,
	})
}

// OAuth2Callback POST /brood/sso/oauth2/callback
// Exchanges authorization code for token, fetches user info, provisions user
func (h *SSOHandler) OAuth2Callback(c *gin.Context) {
	var req struct {
		Code  string `json:"code" binding:"required"`
		State string `json:"state" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate state
	var session model.SSOSession
	if err := h.db.Where("state = ? AND status = ?", req.State, "pending").First(&session).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid or expired state"})
		return
	}
	if time.Now().After(session.ExpiresAt) {
		h.db.Model(&session).Update("status", "expired")
		c.JSON(http.StatusBadRequest, gin.H{"error": "SSO session expired"})
		return
	}

	// Get provider
	var provider model.SSOProvider
	if err := h.db.First(&provider, "id = ?", session.ProviderID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "provider not found"})
		return
	}

	// Exchange code for token
	tokenResp, err := h.exchangeOAuth2Code(provider, req.Code)
	if err != nil {
		h.db.Model(&session).Update("status", "failed")
		c.JSON(http.StatusBadGateway, gin.H{"error": "token exchange failed: " + err.Error()})
		return
	}

	// Fetch user info
	userInfo, err := h.fetchOAuth2UserInfo(provider, tokenResp.AccessToken)
	if err != nil {
		h.db.Model(&session).Update("status", "failed")
		c.JSON(http.StatusBadGateway, gin.H{"error": "userinfo fetch failed: " + err.Error()})
		return
	}

	// Auto-provision or link user
	adminUser, token, err := h.provisionSSOUser(provider, userInfo)
	if err != nil {
		h.db.Model(&session).Update("status", "failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "user provisioning failed: " + err.Error()})
		return
	}

	// Mark session as completed
	now := time.Now()
	h.db.Model(&session).Updates(map[string]interface{}{
		"status":         "completed",
		"external_id":    userInfo.ID,
		"external_name":  userInfo.Name,
		"external_email": userInfo.Email,
		"admin_user_id":  adminUser.ID,
		"completed_at":   &now,
	})

	audit(h.db, c, "sso.oauth2_login", adminUser.ID, fmt.Sprintf("provider=%s external=%s email=%s", provider.Name, userInfo.ID, userInfo.Email))

	c.JSON(http.StatusOK, gin.H{
		"token":   token,
		"user":    adminUser,
		"message": "SSO login successful",
		"sso": gin.H{
			"provider":      provider.Name,
			"external_id":   userInfo.ID,
			"external_name": userInfo.Name,
		},
	})
}

// ==================== LDAP Authentication ====================

// LDAPLogin POST /brood/sso/ldap/login
func (h *SSOHandler) LDAPLogin(c *gin.Context) {
	var req struct {
		ProviderID string `json:"provider_id" binding:"required"`
		Username   string `json:"username" binding:"required"`
		Password   string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var provider model.SSOProvider
	if err := h.db.First(&provider, "id = ? AND enabled = ? AND type = ?", req.ProviderID, true, "ldap").Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "LDAP provider not found or disabled"})
		return
	}

	// Perform LDAP authentication
	ldapUser, err := h.ldapAuthenticate(provider, req.Username, req.Password)
	if err != nil {
		// Record failed session
		h.db.Create(&model.SSOSession{
			ProviderID:   provider.ID,
			State:        generateRandomString(16),
			ExternalName: req.Username,
			Status:       "failed",
			IPAddress:    c.ClientIP(),
			UserAgent:    c.GetHeader("User-Agent"),
			ExpiresAt:    time.Now(),
		})
		c.JSON(http.StatusUnauthorized, gin.H{"error": "LDAP authentication failed: " + err.Error()})
		return
	}

	// Auto-provision or link user
	adminUser, token, err := h.provisionSSOUser(provider, ldapUser)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "user provisioning failed: " + err.Error()})
		return
	}

	// Record successful session
	now := time.Now()
	h.db.Create(&model.SSOSession{
		ProviderID:    provider.ID,
		State:         generateRandomString(16),
		ExternalID:    ldapUser.ID,
		ExternalName:  ldapUser.Name,
		ExternalEmail: ldapUser.Email,
		AdminUserID:   adminUser.ID,
		Status:        "completed",
		IPAddress:     c.ClientIP(),
		UserAgent:     c.GetHeader("User-Agent"),
		ExpiresAt:     time.Now().Add(24 * time.Hour),
		CompletedAt:   &now,
	})

	audit(h.db, c, "sso.ldap_login", adminUser.ID, fmt.Sprintf("provider=%s ldap_user=%s email=%s", provider.Name, ldapUser.Name, ldapUser.Email))

	c.JSON(http.StatusOK, gin.H{
		"token":   token,
		"user":    adminUser,
		"message": "LDAP login successful",
		"sso": gin.H{
			"provider":      provider.Name,
			"external_id":   ldapUser.ID,
			"external_name": ldapUser.Name,
		},
	})
}

// ==================== SSO Sessions ====================

// ListSessions GET /brood/sso/sessions
func (h *SSOHandler) ListSessions(c *gin.Context) {
	var sessions []model.SSOSession
	q := h.db.Order("created_at DESC").Limit(200)
	if pid := c.Query("provider_id"); pid != "" {
		q = q.Where("provider_id = ?", pid)
	}
	if status := c.Query("status"); status != "" {
		q = q.Where("status = ?", status)
	}
	q.Find(&sessions)
	c.JSON(http.StatusOK, gin.H{"sessions": sessions, "total": len(sessions)})
}

// TestProvider POST /brood/sso/providers/:id/test
func (h *SSOHandler) TestProvider(c *gin.Context) {
	var provider model.SSOProvider
	if err := h.db.First(&provider, "id = ?", c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "provider not found"})
		return
	}

	switch provider.Type {
	case "oauth2", "oidc":
		// Test OAuth2: verify endpoints are reachable
		result := h.testOAuth2Provider(provider)
		c.JSON(http.StatusOK, gin.H{"type": provider.Type, "test": result})
	case "ldap":
		// Test LDAP: try bind with service account
		result := h.testLDAPProvider(provider)
		c.JSON(http.StatusOK, gin.H{"type": "ldap", "test": result})
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported provider type: " + provider.Type})
	}
}

// ==================== Internal Helpers ====================

// ssoUserInfo represents normalized user info from any SSO provider
type ssoUserInfo struct {
	ID    string
	Name  string
	Email string
}

// OAuth2 token response
type oauth2TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

// exchangeOAuth2Code exchanges an authorization code for an access token
func (h *SSOHandler) exchangeOAuth2Code(provider model.SSOProvider, code string) (*oauth2TokenResponse, error) {
	data := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"client_id":     {provider.ClientID},
		"client_secret": {provider.ClientSecret},
		"redirect_uri":  {provider.RedirectURL},
	}

	resp, err := http.Post(provider.TokenURL, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read token response failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp oauth2TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("parse token response failed: %w", err)
	}

	if tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("empty access_token in response")
	}
	return &tokenResp, nil
}

// fetchOAuth2UserInfo fetches user info from the IdP using the access token
func (h *SSOHandler) fetchOAuth2UserInfo(provider model.SSOProvider, accessToken string) (*ssoUserInfo, error) {
	if provider.UserInfoURL == "" {
		return nil, fmt.Errorf("userinfo_url not configured")
	}

	req, err := http.NewRequest("GET", provider.UserInfoURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("userinfo request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read userinfo response failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("userinfo endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	// Parse generic JSON — different providers use different field names
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse userinfo failed: %w", err)
	}

	info := &ssoUserInfo{
		ID:    extractStringField(raw, "sub", "id", "user_id", "userid", "openid"),
		Name:  extractStringField(raw, "name", "login", "username", "nickname"),
		Email: extractStringField(raw, "email", "mail"),
	}

	if info.ID == "" && info.Email != "" {
		info.ID = info.Email
	}
	if info.ID == "" {
		return nil, fmt.Errorf("could not extract user ID from userinfo response")
	}

	return info, nil
}

// ldapAuthenticate performs LDAP bind authentication
func (h *SSOHandler) ldapAuthenticate(provider model.SSOProvider, username, password string) (*ssoUserInfo, error) {
	if provider.LDAPHost == "" {
		return nil, fmt.Errorf("LDAP host not configured")
	}

	// Connect to LDAP
	var conn ldapConn
	var err error

	scheme := "ldap"
	host := provider.LDAPHost
	if provider.LDAPUseTLS {
		scheme = "ldaps"
	}

	conn, err = dialLDAP(scheme, host, provider.LDAPUseTLS)
	if err != nil {
		return nil, fmt.Errorf("LDAP connect failed: %w", err)
	}
	defer conn.Close()

	// Bind with service account to search for user
	if provider.LDAPBindDN != "" {
		if err := conn.Bind(provider.LDAPBindDN, provider.LDAPBindPass); err != nil {
			return nil, fmt.Errorf("LDAP service bind failed: %w", err)
		}
	}

	// Search for user
	userFilter := provider.LDAPUserFilter
	if userFilter == "" {
		userFilter = "(uid=%s)"
	}
	filter := fmt.Sprintf(userFilter, escapeLDAPFilter(username))

	entries, err := conn.Search(provider.LDAPBaseDN, filter)
	if err != nil {
		return nil, fmt.Errorf("LDAP search failed: %w", err)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("user not found in LDAP directory")
	}
	if len(entries) > 1 {
		return nil, fmt.Errorf("multiple users matched in LDAP (ambiguous)")
	}

	entry := entries[0]

	// Bind as the user to verify password
	if err := conn.Bind(entry.DN, password); err != nil {
		return nil, fmt.Errorf("invalid LDAP credentials")
	}

	// Extract attributes
	attrEmail := provider.LDAPAttrEmail
	if attrEmail == "" {
		attrEmail = "mail"
	}
	attrName := provider.LDAPAttrName
	if attrName == "" {
		attrName = "cn"
	}

	info := &ssoUserInfo{
		ID:    entry.DN,
		Name:  entry.GetAttr(attrName),
		Email: entry.GetAttr(attrEmail),
	}

	if info.Name == "" {
		info.Name = username
	}

	return info, nil
}

// provisionSSOUser auto-provisions or links an SSO user to a local AdminUser
func (h *SSOHandler) provisionSSOUser(provider model.SSOProvider, userInfo *ssoUserInfo) (*model.AdminUser, string, error) {
	// Try to find existing user by email or external ID pattern
	ssoUsername := fmt.Sprintf("sso_%s_%s", provider.Provider, userInfo.ID)
	if userInfo.Email != "" {
		ssoUsername = userInfo.Email
	}

	var existingUser model.AdminUser
	err := h.db.Where("username = ? OR email = ?", ssoUsername, userInfo.Email).First(&existingUser).Error

	if err == nil {
		// Existing user — generate new token and return
		token := generateToken(32)
		h.db.Model(&existingUser).Updates(map[string]interface{}{
			"password_hash": middleware.HashTokenExported(token),
			"last_login_at": time.Now(),
		})
		return &existingUser, token, nil
	}

	// Create new user (auto-provisioning)
	token := generateToken(32)
	newUser := model.AdminUser{
		Username:     ssoUsername,
		PasswordHash: middleware.HashTokenExported(token),
		Role:         provider.DefaultRole,
		TeamID:       provider.TeamID,
		Email:        userInfo.Email,
	}
	if err := h.db.Create(&newUser).Error; err != nil {
		return nil, "", fmt.Errorf("failed to create user: %w", err)
	}

	return &newUser, token, nil
}

// testOAuth2Provider tests OAuth2 provider connectivity
func (h *SSOHandler) testOAuth2Provider(provider model.SSOProvider) map[string]interface{} {
	result := map[string]interface{}{
		"client_id_set": provider.ClientID != "",
		"secret_set":    provider.ClientSecret != "",
	}

	// Test auth URL
	if provider.AuthURL != "" {
		resp, err := http.Head(provider.AuthURL)
		if err != nil {
			result["auth_url"] = "unreachable: " + err.Error()
		} else {
			result["auth_url"] = fmt.Sprintf("reachable (HTTP %d)", resp.StatusCode)
			resp.Body.Close()
		}
	} else {
		result["auth_url"] = "not configured"
	}

	// Test token URL
	if provider.TokenURL != "" {
		resp, err := http.Head(provider.TokenURL)
		if err != nil {
			result["token_url"] = "unreachable: " + err.Error()
		} else {
			result["token_url"] = fmt.Sprintf("reachable (HTTP %d)", resp.StatusCode)
			resp.Body.Close()
		}
	} else {
		result["token_url"] = "not configured"
	}

	return result
}

// testLDAPProvider tests LDAP provider connectivity
func (h *SSOHandler) testLDAPProvider(provider model.SSOProvider) map[string]interface{} {
	result := map[string]interface{}{
		"host":    provider.LDAPHost,
		"base_dn": provider.LDAPBaseDN,
		"use_tls": provider.LDAPUseTLS,
	}

	if provider.LDAPHost == "" {
		result["status"] = "error: host not configured"
		return result
	}

	scheme := "ldap"
	if provider.LDAPUseTLS {
		scheme = "ldaps"
	}

	conn, err := dialLDAP(scheme, provider.LDAPHost, provider.LDAPUseTLS)
	if err != nil {
		result["status"] = "connection failed: " + err.Error()
		return result
	}
	defer conn.Close()

	result["connection"] = "ok"

	// Try bind
	if provider.LDAPBindDN != "" {
		if err := conn.Bind(provider.LDAPBindDN, provider.LDAPBindPass); err != nil {
			result["bind"] = "failed: " + err.Error()
		} else {
			result["bind"] = "ok"
		}
	}

	result["status"] = "ok"
	return result
}

// ==================== Lightweight LDAP Client ====================
// Minimal LDAP implementation to avoid heavy third-party dependencies.
// Supports simple bind and basic search via net/tcp + BER encoding.

// ldapConn abstracts an LDAP connection
type ldapConn interface {
	Bind(dn, password string) error
	Search(baseDN, filter string) ([]ldapEntry, error)
	Close()
}

type ldapEntry struct {
	DN    string
	Attrs map[string][]string
}

func (e *ldapEntry) GetAttr(name string) string {
	if vals, ok := e.Attrs[name]; ok && len(vals) > 0 {
		return vals[0]
	}
	return ""
}

// simpleLDAPConn is a minimal LDAP client using raw TCP + BER
type simpleLDAPConn struct {
	conn  io.ReadWriteCloser
	msgID int
}

func dialLDAP(scheme, host string, useTLS bool) (ldapConn, error) {
	addr := host
	if !strings.Contains(addr, ":") {
		if useTLS {
			addr += ":636"
		} else {
			addr += ":389"
		}
	}

	var rwc io.ReadWriteCloser
	var err error

	if useTLS {
		rwc, err = tls.Dial("tcp", addr, &tls.Config{InsecureSkipVerify: true})
	} else {
		rwc, err = dialTCP(addr)
	}
	if err != nil {
		return nil, err
	}

	return &simpleLDAPConn{conn: rwc, msgID: 0}, nil
}

func dialTCP(addr string) (io.ReadWriteCloser, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

// Bind performs LDAP simple bind
func (c *simpleLDAPConn) Bind(dn, password string) error {
	c.msgID++
	// Build BindRequest BER packet
	bindReq := berSequence(
		berInteger(c.msgID),
		berApp(0, // BindRequest
			berInteger(3), // LDAP v3
			berOctetString(dn),
			berContextString(0, password), // simple auth
		),
	)
	if _, err := c.conn.Write(berWrapMessage(bindReq)); err != nil {
		return fmt.Errorf("write bind request: %w", err)
	}

	// Read BindResponse
	data, err := readBERMessage(c.conn)
	if err != nil {
		return fmt.Errorf("read bind response: %w", err)
	}

	resultCode := extractLDAPResultCode(data)
	if resultCode != 0 {
		return fmt.Errorf("LDAP bind failed (result code: %d)", resultCode)
	}
	return nil
}

// Search performs LDAP search
func (c *simpleLDAPConn) Search(baseDN, filter string) ([]ldapEntry, error) {
	c.msgID++
	// Build SearchRequest BER packet
	searchReq := berSequence(
		berInteger(c.msgID),
		berApp(3, // SearchRequest
			berOctetString(baseDN), // baseObject
			berEnum(2),             // scope: wholeSubtree
			berEnum(0),             // derefAliases: neverDerefAliases
			berInteger(0),          // sizeLimit
			berInteger(30),         // timeLimit (seconds)
			berBoolean(false),      // typesOnly
			berLDAPFilter(filter),  // filter
			berSequence(),          // attributes (all)
		),
	)
	if _, err := c.conn.Write(berWrapMessage(searchReq)); err != nil {
		return nil, fmt.Errorf("write search request: %w", err)
	}

	var entries []ldapEntry
	for {
		data, err := readBERMessage(c.conn)
		if err != nil {
			return nil, fmt.Errorf("read search response: %w", err)
		}

		tag, content := parseBERResponse(data)

		if tag == 0x65 { // SearchResultDone (APPLICATION 5)
			resultCode := extractLDAPResultCode(content)
			if resultCode != 0 && resultCode != 4 { // 4 = sizeLimitExceeded (ok)
				return nil, fmt.Errorf("LDAP search failed (result code: %d)", resultCode)
			}
			break
		}

		if tag == 0x64 { // SearchResultEntry (APPLICATION 4)
			entry := parseLDAPSearchEntry(content)
			entries = append(entries, entry)
		}
	}

	return entries, nil
}

func (c *simpleLDAPConn) Close() {
	c.conn.Close()
}

// ==================== BER Encoding Helpers ====================

func berSequence(items ...[]byte) []byte {
	content := concatBytes(items...)
	return berTLV(0x30, content)
}

func berInteger(val int) []byte {
	if val == 0 {
		return berTLV(0x02, []byte{0})
	}
	var buf []byte
	v := val
	for v > 0 || (len(buf) > 0 && buf[0]&0x80 != 0) {
		buf = append([]byte{byte(v & 0xff)}, buf...)
		v >>= 8
		if v == 0 && buf[0]&0x80 != 0 {
			buf = append([]byte{0}, buf...)
			break
		}
	}
	if len(buf) == 0 {
		buf = []byte{0}
	}
	return berTLV(0x02, buf)
}

func berEnum(val int) []byte {
	return berTLV(0x0a, []byte{byte(val)})
}

func berBoolean(val bool) []byte {
	if val {
		return berTLV(0x01, []byte{0xff})
	}
	return berTLV(0x01, []byte{0x00})
}

func berOctetString(s string) []byte {
	return berTLV(0x04, []byte(s))
}

func berContextString(tag int, s string) []byte {
	return berTLV(byte(0x80|tag), []byte(s))
}

func berApp(tag int, items ...[]byte) []byte {
	content := concatBytes(items...)
	return berTLV(byte(0x60|tag), content)
}

func berTLV(tag byte, value []byte) []byte {
	length := len(value)
	var result []byte
	result = append(result, tag)
	if length < 128 {
		result = append(result, byte(length))
	} else if length < 256 {
		result = append(result, 0x81, byte(length))
	} else {
		result = append(result, 0x82, byte(length>>8), byte(length&0xff))
	}
	result = append(result, value...)
	return result
}

func berWrapMessage(content []byte) []byte {
	return content // Already wrapped as sequence
}

func berLDAPFilter(filter string) []byte {
	// Simple filter parser: supports (attr=value) equality only
	// For complex filters, this returns the raw string as a filter extension
	filter = strings.TrimSpace(filter)
	if strings.HasPrefix(filter, "(") && strings.HasSuffix(filter, ")") {
		filter = filter[1 : len(filter)-1]
	}
	if idx := strings.Index(filter, "="); idx > 0 {
		attr := filter[:idx]
		val := filter[idx+1:]
		// equalityMatch: context tag 3
		return berTLV(0xa3, append(berOctetString(attr), berOctetString(val)...))
	}
	// Fallback: present filter
	return berTLV(0x87, []byte(filter))
}

func concatBytes(items ...[]byte) []byte {
	var result []byte
	for _, item := range items {
		result = append(result, item...)
	}
	return result
}

// ==================== BER Decoding Helpers ====================

func readBERMessage(r io.Reader) ([]byte, error) {
	// Read tag
	tagBuf := make([]byte, 1)
	if _, err := io.ReadFull(r, tagBuf); err != nil {
		return nil, err
	}

	// Read length
	lenBuf := make([]byte, 1)
	if _, err := io.ReadFull(r, lenBuf); err != nil {
		return nil, err
	}

	var length int
	if lenBuf[0] < 128 {
		length = int(lenBuf[0])
	} else {
		numBytes := int(lenBuf[0] & 0x7f)
		lBytes := make([]byte, numBytes)
		if _, err := io.ReadFull(r, lBytes); err != nil {
			return nil, err
		}
		for _, b := range lBytes {
			length = (length << 8) | int(b)
		}
	}

	// Read value
	value := make([]byte, length)
	if length > 0 {
		if _, err := io.ReadFull(r, value); err != nil {
			return nil, err
		}
	}

	// Return full TLV
	result := append(tagBuf, lenBuf...)
	result = append(result, value...)
	return result, nil
}

func parseBERResponse(data []byte) (byte, []byte) {
	if len(data) < 2 {
		return 0, nil
	}
	// Parse outer SEQUENCE
	if data[0] != 0x30 {
		return 0, nil
	}
	_, content := parseBERTLV(data)

	// Skip message ID
	_, remaining := parseBERTLV(content)

	// Get application response
	if len(remaining) < 2 {
		return 0, nil
	}
	tag := remaining[0]
	_, responseContent := parseBERTLV(remaining)
	return tag, responseContent
}

func parseBERTLV(data []byte) ([]byte, []byte) {
	if len(data) < 2 {
		return nil, nil
	}
	pos := 1
	var length int
	if data[pos] < 128 {
		length = int(data[pos])
		pos++
	} else {
		numBytes := int(data[pos] & 0x7f)
		pos++
		for i := 0; i < numBytes && pos < len(data); i++ {
			length = (length << 8) | int(data[pos])
			pos++
		}
	}
	end := pos + length
	if end > len(data) {
		end = len(data)
	}
	return data[pos:end], data[end:]
}

func extractLDAPResultCode(content []byte) int {
	if len(content) < 3 {
		return -1
	}
	// First element should be an ENUMERATED (result code)
	if content[0] == 0x0a && content[1] == 0x01 {
		return int(content[2])
	}
	return -1
}

func parseLDAPSearchEntry(content []byte) ldapEntry {
	entry := ldapEntry{Attrs: make(map[string][]string)}

	// First element: objectName (OCTET STRING)
	if len(content) < 2 {
		return entry
	}
	dnValue, remaining := parseBERTLV(content)
	entry.DN = string(dnValue)

	// Second element: attributes (SEQUENCE OF)
	if len(remaining) < 2 {
		return entry
	}
	attrsContent, _ := parseBERTLV(remaining)

	// Parse each attribute
	for len(attrsContent) > 2 {
		attrSeq, rest := parseBERTLV(attrsContent)
		attrsContent = rest

		if len(attrSeq) < 2 {
			continue
		}
		// Attribute type (OCTET STRING)
		attrType, valuesData := parseBERTLV(attrSeq)
		attrName := string(attrType)

		// Attribute values (SET OF OCTET STRING)
		if len(valuesData) < 2 {
			continue
		}
		valuesContent, _ := parseBERTLV(valuesData)
		for len(valuesContent) > 2 {
			val, rest := parseBERTLV(valuesContent)
			valuesContent = rest
			entry.Attrs[attrName] = append(entry.Attrs[attrName], string(val))
		}
	}

	return entry
}

// ==================== Utility Functions ====================

func extractStringField(raw map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if val, ok := raw[key]; ok {
			switch v := val.(type) {
			case string:
				return v
			case float64:
				return fmt.Sprintf("%.0f", v)
			}
		}
	}
	return ""
}

func generateRandomString(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)[:n]
}

func escapeLDAPFilter(s string) string {
	replacer := strings.NewReplacer(
		`\`, `\5c`,
		`*`, `\2a`,
		`(`, `\28`,
		`)`, `\29`,
		"\x00", `\00`,
	)
	return replacer.Replace(s)
}
