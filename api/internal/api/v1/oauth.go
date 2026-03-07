package v1

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/yinhe/starclaw/internal/config"
	"github.com/yinhe/starclaw/internal/model"
	"gorm.io/gorm"
)

type OAuthHandler struct {
	db  *gorm.DB
	cfg *config.Config
}

func NewOAuthHandler(db *gorm.DB, cfg *config.Config) *OAuthHandler {
	return &OAuthHandler{db: db, cfg: cfg}
}

// OAuthCallbackRequest is sent by the frontend after receiving the OAuth code
type OAuthCallbackRequest struct {
	Code string `json:"code" binding:"required"`
}

// ---- GitHub OAuth ----

func (h *OAuthHandler) GitHubCallback(c *gin.Context) {
	var req OAuthCallbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ghCfg := h.cfg.OAuth.GitHub
	if ghCfg.ClientID == "" || ghCfg.ClientSecret == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "GitHub OAuth not configured"})
		return
	}

	// Exchange code for access token
	accessToken, err := h.exchangeGitHubCode(req.Code, ghCfg)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "failed to exchange code: " + err.Error()})
		return
	}

	// Get user info from GitHub
	ghUser, err := h.getGitHubUser(accessToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "failed to get user info: " + err.Error()})
		return
	}

	// Find or create user
	user, err := h.findOrCreateOAuthUser("github", ghUser.ID, ghUser.Login, ghUser.Email, ghUser.AvatarURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user: " + err.Error()})
		return
	}

	token, err := h.generateToken(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, AuthResponse{Token: token, User: *user})
}

func (h *OAuthHandler) exchangeGitHubCode(code string, cfg config.OAuthProviderConfig) (string, error) {
	data := url.Values{
		"client_id":     {cfg.ClientID},
		"client_secret": {cfg.ClientSecret},
		"code":          {code},
	}
	if cfg.RedirectURL != "" {
		data.Set("redirect_uri", cfg.RedirectURL)
	}

	req, _ := http.NewRequest("POST", "https://github.com/login/oauth/access_token", strings.NewReader(data.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if result.Error != "" {
		return "", fmt.Errorf("%s: %s", result.Error, result.ErrorDesc)
	}
	return result.AccessToken, nil
}

type githubUser struct {
	ID        string `json:"id"`
	Login     string `json:"login"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}

func (h *OAuthHandler) getGitHubUser(accessToken string) (*githubUser, error) {
	req, _ := http.NewRequest("GET", "https://api.github.com/user", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// Parse manually since ID is numeric
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}

	user := &githubUser{}
	if v, ok := raw["id"].(float64); ok {
		user.ID = fmt.Sprintf("%d", int64(v))
	}
	if v, ok := raw["login"].(string); ok {
		user.Login = v
	}
	if v, ok := raw["email"].(string); ok {
		user.Email = v
	}
	if v, ok := raw["avatar_url"].(string); ok {
		user.AvatarURL = v
	}

	if user.ID == "" {
		return nil, fmt.Errorf("invalid GitHub user response")
	}
	return user, nil
}

// ---- Google OAuth ----

func (h *OAuthHandler) GoogleCallback(c *gin.Context) {
	var req OAuthCallbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	gCfg := h.cfg.OAuth.Google
	if gCfg.ClientID == "" || gCfg.ClientSecret == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Google OAuth not configured"})
		return
	}

	// Exchange code for access token
	accessToken, err := h.exchangeGoogleCode(req.Code, gCfg)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "failed to exchange code: " + err.Error()})
		return
	}

	// Get user info from Google
	gUser, err := h.getGoogleUser(accessToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "failed to get user info: " + err.Error()})
		return
	}

	// Find or create user
	user, err := h.findOrCreateOAuthUser("google", gUser.ID, gUser.Name, gUser.Email, gUser.Picture)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user: " + err.Error()})
		return
	}

	token, err := h.generateToken(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, AuthResponse{Token: token, User: *user})
}

func (h *OAuthHandler) exchangeGoogleCode(code string, cfg config.OAuthProviderConfig) (string, error) {
	data := url.Values{
		"client_id":     {cfg.ClientID},
		"client_secret": {cfg.ClientSecret},
		"code":          {code},
		"grant_type":    {"authorization_code"},
	}
	if cfg.RedirectURL != "" {
		data.Set("redirect_uri", cfg.RedirectURL)
	}

	resp, err := http.PostForm("https://oauth2.googleapis.com/token", data)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if result.Error != "" {
		return "", fmt.Errorf("%s: %s", result.Error, result.ErrorDesc)
	}
	return result.AccessToken, nil
}

type googleUser struct {
	ID      string `json:"id"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
}

func (h *OAuthHandler) getGoogleUser(accessToken string) (*googleUser, error) {
	req, _ := http.NewRequest("GET", "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var user googleUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, err
	}
	if user.ID == "" {
		return nil, fmt.Errorf("invalid Google user response")
	}
	return &user, nil
}

// ---- Shared ----

// GetOAuthConfig returns which OAuth providers are enabled (public endpoint)
func (h *OAuthHandler) GetOAuthConfig(c *gin.Context) {
	providers := []gin.H{}
	if h.cfg.OAuth.GitHub.ClientID != "" {
		providers = append(providers, gin.H{
			"name":      "github",
			"client_id": h.cfg.OAuth.GitHub.ClientID,
		})
	}
	if h.cfg.OAuth.Google.ClientID != "" {
		providers = append(providers, gin.H{
			"name":      "google",
			"client_id": h.cfg.OAuth.Google.ClientID,
		})
	}
	c.JSON(http.StatusOK, gin.H{"providers": providers})
}

func (h *OAuthHandler) findOrCreateOAuthUser(provider, oauthID, username, email, avatar string) (*model.User, error) {
	var user model.User

	// Try to find by OAuth provider + ID
	err := h.db.Where("oauth_provider = ? AND oauth_id = ?", provider, oauthID).First(&user).Error
	if err == nil {
		// Update avatar if changed
		if avatar != "" && user.Avatar != avatar {
			h.db.Model(&user).Update("avatar", avatar)
			user.Avatar = avatar
		}
		return &user, nil
	}

	// Try to find by email (link accounts)
	if email != "" {
		err = h.db.Where("email = ?", email).First(&user).Error
		if err == nil {
			// Link OAuth to existing account
			h.db.Model(&user).Updates(map[string]interface{}{
				"oauth_provider": provider,
				"oauth_id":       oauthID,
				"avatar":         avatar,
			})
			user.OAuthProvider = provider
			user.OAuthID = oauthID
			user.Avatar = avatar
			return &user, nil
		}
	}

	// Ensure unique username
	finalUsername := username
	var count int64
	h.db.Model(&model.User{}).Where("username = ?", finalUsername).Count(&count)
	if count > 0 {
		finalUsername = fmt.Sprintf("%s_%s", username, uuid.New().String()[:6])
	}

	// Create new user (OAuth users have a random password since they don't use it)
	user = model.User{
		Email:         email,
		Username:      finalUsername,
		Password:      uuid.New().String(), // random, not used for OAuth login
		Avatar:        avatar,
		OAuthProvider: provider,
		OAuthID:       oauthID,
	}

	if err := h.db.Create(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (h *OAuthHandler) generateToken(user *model.User) (string, error) {
	role := user.Role
	if role == "" {
		role = "user"
	}
	claims := jwt.MapClaims{
		"sub":      user.ID,
		"username": user.Username,
		"role":     role,
		"exp":      time.Now().Add(time.Duration(h.cfg.JWT.ExpireHour) * time.Hour).Unix(),
		"iat":      time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.cfg.JWT.Secret))
}
