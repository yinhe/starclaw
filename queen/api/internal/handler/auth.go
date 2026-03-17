package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/yinhe/starclaw-queen/api/internal/config"
	"github.com/yinhe/starclaw-queen/api/internal/database"
	"github.com/yinhe/starclaw-queen/api/internal/middleware"
	"github.com/yinhe/starclaw-queen/api/internal/model"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct{}

// POST /auth/register  — email or phone + password, no SMS
func (h *AuthHandler) Register(c *gin.Context) {
	var req struct {
		Email    string `json:"email"`
		Phone    string `json:"phone"`
		Nickname string `json:"nickname" binding:"required"`
		Password string `json:"password" binding:"required,min=6"`
		RefCode  string `json:"ref_code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请填写昵称和密码（至少 6 位）"})
		return
	}
	if req.Email == "" && req.Phone == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供邮箱或手机号"})
		return
	}

	// Check duplicate
	var exists model.User
	if req.Email != "" {
		if err := database.DB.Where("email = ?", req.Email).First(&exists).Error; err == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "该邮箱已注册"})
			return
		}
	}
	if req.Phone != "" {
		if err := database.DB.Where("phone = ?", req.Phone).First(&exists).Error; err == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "该手机号已注册"})
			return
		}
	}

	hashed, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	user := model.User{
		ID:       uuid.New().String(),
		Email:    req.Email,
		Phone:    req.Phone,
		Nickname: req.Nickname,
		Password: string(hashed),
		Role:     "user",
		Status:   "active",
	}
	if err := database.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "注册失败"})
		return
	}

	// Referral attribution: link user to city partner
	if req.RefCode != "" {
		var partner model.CityPartner
		if err := database.DB.Where("ref_code = ? AND status = ?", req.RefCode, "approved").First(&partner).Error; err == nil {
			client := model.CityClient{
				ID:          uuid.New().String(),
				PartnerID:   partner.ID,
				ClientName:  req.Nickname,
				ContactInfo: req.Email + " " + req.Phone,
				Status:      "lead",
				RefSource:   req.RefCode,
			}
			database.DB.Create(&client)
			database.DB.Model(&partner).UpdateColumn("total_clients", partner.TotalClients+1)
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "注册成功", "user_id": user.ID})
}

// POST /auth/login  — email/phone + password
func (h *AuthHandler) Login(c *gin.Context) {
	var req struct {
		Email    string `json:"email"`
		Phone    string `json:"phone"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数不完整"})
		return
	}
	if req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入密码"})
		return
	}
	if req.Email == "" && req.Phone == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供邮箱或手机号"})
		return
	}

	var user model.User
	if req.Email != "" {
		if err := database.DB.Where("email = ?", req.Email).First(&user).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "账号不存在"})
			return
		}
	} else {
		if err := database.DB.Where("phone = ?", req.Phone).First(&user).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "账号不存在"})
			return
		}
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "密码错误"})
		return
	}
	if user.Status == "banned" {
		c.JSON(http.StatusForbidden, gin.H{"error": "账号已被封禁"})
		return
	}

	respondWithToken(c, &user)
}

// POST /auth/oauth/google
func (h *AuthHandler) OAuthGoogle(c *gin.Context) {
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少授权码"})
		return
	}

	cfg := config.GetConfig()
	clientID := cfg.GetString("oauth.google.client_id")
	clientSecret := cfg.GetString("oauth.google.client_secret")
	redirectURL := cfg.GetString("oauth.google.redirect_url")
	if clientID == "" || clientSecret == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Google 登录尚未配置"})
		return
	}

	// Exchange code for tokens
	resp, err := http.PostForm("https://oauth2.googleapis.com/token", url.Values{
		"code":          {req.Code},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"redirect_uri":  {redirectURL},
		"grant_type":    {"authorization_code"},
	})
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Google 授权失败"})
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var tokenResp struct {
		AccessToken string `json:"access_token"`
		IDToken     string `json:"id_token"`
		Error       string `json:"error"`
	}
	json.Unmarshal(body, &tokenResp)
	if tokenResp.AccessToken == "" {
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("Google 授权失败: %s", tokenResp.Error)})
		return
	}

	// Get user info
	infoReq, _ := http.NewRequest("GET", "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	infoReq.Header.Set("Authorization", "Bearer "+tokenResp.AccessToken)
	infoResp, err := http.DefaultClient.Do(infoReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "获取 Google 用户信息失败"})
		return
	}
	defer infoResp.Body.Close()
	infoBody, _ := io.ReadAll(infoResp.Body)
	var gUser struct {
		ID      string `json:"id"`
		Email   string `json:"email"`
		Name    string `json:"name"`
		Picture string `json:"picture"`
	}
	json.Unmarshal(infoBody, &gUser)
	if gUser.ID == "" {
		c.JSON(http.StatusBadGateway, gin.H{"error": "获取 Google 用户信息失败"})
		return
	}

	user := findOrCreateOAuthUser("google", gUser.ID, gUser.Email, gUser.Name, gUser.Picture)
	if user == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建用户失败"})
		return
	}
	respondWithToken(c, user)
}

// POST /auth/oauth/github
func (h *AuthHandler) OAuthGitHub(c *gin.Context) {
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少授权码"})
		return
	}

	cfg := config.GetConfig()
	clientID := cfg.GetString("oauth.github.client_id")
	clientSecret := cfg.GetString("oauth.github.client_secret")
	if clientID == "" || clientSecret == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "GitHub 登录尚未配置"})
		return
	}

	// Exchange code for access token
	tokenReq, _ := http.NewRequest("POST", "https://github.com/login/oauth/access_token", strings.NewReader(fmt.Sprintf(
		"client_id=%s&client_secret=%s&code=%s", clientID, clientSecret, req.Code)))
	tokenReq.Header.Set("Accept", "application/json")
	tokenReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	tokenResp, err := http.DefaultClient.Do(tokenReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "GitHub 授权失败"})
		return
	}
	defer tokenResp.Body.Close()
	tokenBody, _ := io.ReadAll(tokenResp.Body)
	var ghToken struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	json.Unmarshal(tokenBody, &ghToken)
	if ghToken.AccessToken == "" {
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("GitHub 授权失败: %s", ghToken.Error)})
		return
	}

	// Get user info
	userReq, _ := http.NewRequest("GET", "https://api.github.com/user", nil)
	userReq.Header.Set("Authorization", "Bearer "+ghToken.AccessToken)
	userResp, err := http.DefaultClient.Do(userReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "获取 GitHub 用户信息失败"})
		return
	}
	defer userResp.Body.Close()
	userBody, _ := io.ReadAll(userResp.Body)
	var ghUser struct {
		ID        int    `json:"id"`
		Login     string `json:"login"`
		Name      string `json:"name"`
		Email     string `json:"email"`
		AvatarURL string `json:"avatar_url"`
	}
	json.Unmarshal(userBody, &ghUser)
	if ghUser.ID == 0 {
		c.JSON(http.StatusBadGateway, gin.H{"error": "获取 GitHub 用户信息失败"})
		return
	}

	// If email not public, fetch from emails API
	email := ghUser.Email
	if email == "" {
		emailReq, _ := http.NewRequest("GET", "https://api.github.com/user/emails", nil)
		emailReq.Header.Set("Authorization", "Bearer "+ghToken.AccessToken)
		emailResp, err := http.DefaultClient.Do(emailReq)
		if err == nil {
			defer emailResp.Body.Close()
			emailBody, _ := io.ReadAll(emailResp.Body)
			var emails []struct {
				Email    string `json:"email"`
				Primary  bool   `json:"primary"`
				Verified bool   `json:"verified"`
			}
			json.Unmarshal(emailBody, &emails)
			for _, e := range emails {
				if e.Primary && e.Verified {
					email = e.Email
					break
				}
			}
		}
	}

	name := ghUser.Name
	if name == "" {
		name = ghUser.Login
	}
	oauthID := fmt.Sprintf("%d", ghUser.ID)

	user := findOrCreateOAuthUser("github", oauthID, email, name, ghUser.AvatarURL)
	if user == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建用户失败"})
		return
	}
	respondWithToken(c, user)
}

// ---- helpers ----

func findOrCreateOAuthUser(provider, oauthID, email, name, avatar string) *model.User {
	var user model.User
	// Find by OAuth provider + id
	if err := database.DB.Where("oauth_provider = ? AND oauth_id = ?", provider, oauthID).First(&user).Error; err == nil {
		return &user
	}
	// Find by email and link
	if email != "" {
		if err := database.DB.Where("email = ?", email).First(&user).Error; err == nil {
			user.OAuthProvider = provider
			user.OAuthID = oauthID
			if user.Avatar == "" && avatar != "" {
				user.Avatar = avatar
			}
			database.DB.Save(&user)
			return &user
		}
	}
	// Create new user
	user = model.User{
		ID:            uuid.New().String(),
		Email:         email,
		Nickname:      name,
		Avatar:        avatar,
		Role:          "user",
		Status:        "active",
		OAuthProvider: provider,
		OAuthID:       oauthID,
	}
	if err := database.DB.Create(&user).Error; err != nil {
		return nil
	}
	return &user
}

func respondWithToken(c *gin.Context, user *model.User) {
	token, err := middleware.GenerateToken(user.ID, user.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成令牌失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"user": gin.H{
			"id":             user.ID,
			"email":          user.Email,
			"phone":          user.Phone,
			"nickname":       user.Nickname,
			"avatar":         user.Avatar,
			"role":           user.Role,
			"oauth_provider": user.OAuthProvider,
		},
	})
}
