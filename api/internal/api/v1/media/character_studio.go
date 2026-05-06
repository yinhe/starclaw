package media

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/yinhe/starclaw/internal/config"
	"github.com/yinhe/starclaw/internal/model"
	"github.com/yinhe/starclaw/internal/node"
	"github.com/yinhe/starclaw/internal/provider"
	"github.com/yinhe/starclaw/internal/sandbox"
	"gorm.io/gorm"
)

// CharacterStudioHandler 提供角色工坊向导需要的后端辅助能力：
//   - POST /v1/characters/generate-appearance  → AI 一键生成外观卡
//   - POST /v1/cdn/upload                      → 上传参考图到 cdn.starclaw.net（env 配置 scp，未配置 fallback 到 /v1/uploads）
type CharacterStudioHandler struct {
	cfg       *config.Config
	db        *gorm.DB
	providers *provider.Registry

	// Lazy ensure state for the owned TOS bucket. Guarded by ensureBucketMu;
	// once ensureBucketOK flips to true we stop re-running HEAD/PUT on the bucket.
	ensureBucketMu    sync.Mutex
	ensureBucketOK    bool
	ensureBucketLabel string
}

func NewCharacterStudioHandler(cfg *config.Config, db *gorm.DB, providers *provider.Registry) *CharacterStudioHandler {
	return &CharacterStudioHandler{cfg: cfg, db: db, providers: providers}
}

// ── Appearance Card AI 生成 ──────────────────────────────────────────

type genAppearanceReq struct {
	Name         string `json:"name" binding:"required"`
	Role         string `json:"role"`
	Notes        string `json:"notes"`
	ReferenceURL string `json:"reference_url"`
}

const appearanceSystemPrompt = `你是短剧导演 Agent 的"角色设计师"。你负责为用户设计一张一字不差的 Appearance Card（外观卡）。

规则：
1. 只输出一段 1-3 句话、总长 40-90 字的中文外观描述，不要有任何解释、标题、引号或 Markdown。
2. 必须**具体可拍**：服装、发型、体型、配色、标志性配饰、肤色、年龄段，按这顺序写。
3. 严禁抽象词：不能出现"美丽""优雅""迷人""帅气""神秘"等形容词。
4. 如果用户给了零散片段（notes），一字不改保留已有具体信息，只补足缺失字段。
5. 同一张外观卡会在短剧全集的 [图N] 占位符中复用，所以必须稳定。
6. 风格适配真人短剧（EP01-EP04 验证），不要写幻想风的夸张元素。`

func (h *CharacterStudioHandler) GenerateAppearance(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req genAppearanceReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "detail": err.Error()})
		return
	}

	cfg, err := h.resolveModelConfig(userID)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	p := provider.CreateFromConfig(h.providers, cfg)
	if p == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "LLM provider not available"})
		return
	}

	userMsg := fmt.Sprintf(
		"角色名：%s\n定位：%s\n已有外观片段（可为空）：%s\n参考图 URL（可为空，仅作参考，不要真的读）：%s\n\n请按规则只输出一段外观卡原文。",
		strings.TrimSpace(req.Name),
		strings.TrimSpace(req.Role),
		strings.TrimSpace(req.Notes),
		strings.TrimSpace(req.ReferenceURL),
	)

	ctx, cancel := context.WithTimeout(c.Request.Context(), 45*time.Second)
	defer cancel()
	chunk, err := p.ChatSync(ctx, &provider.ChatRequest{
		Model: cfg.ModelName,
		Messages: []provider.ChatMessage{
			{Role: "system", Content: appearanceSystemPrompt},
			{Role: "user", Content: userMsg},
		},
	})
	if err != nil {
		log.Printf("[character_studio] generate-appearance err: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	text := strings.TrimSpace(chunk.Content)
	// 去掉模型可能返回的引号 / 首尾多余符号
	text = strings.Trim(text, "\"“”「」 \n\r\t")
	c.JSON(http.StatusOK, gin.H{
		"appearance_card": text,
		"model":           cfg.ModelName,
		"provider":        cfg.Provider,
	})
}

// resolveModelConfig picks the best model config by priority (mirrors chat.go logic,
// but without agent/conversation context).
func (h *CharacterStudioHandler) resolveModelConfig(userID string) (model.ModelConfig, error) {
	var cfg model.ModelConfig
	// 1. User's star-ai config (preferred in hosted/swarm mode — no API key needed)
	if err := h.db.Where("user_id = ? AND provider = ? AND is_enabled = ?", userID, "star-ai", true).First(&cfg).Error; err == nil {
		return cfg, nil
	}
	// 2. User's first enabled model with a non-empty API key
	if err := h.db.Where("user_id = ? AND is_enabled = ? AND api_key != ''", userID, true).Order("created_at ASC").First(&cfg).Error; err == nil {
		return cfg, nil
	}
	// 3. Any user-enabled model (ollama etc.)
	if err := h.db.Where("user_id = ? AND is_enabled = ?", userID, true).Order("created_at ASC").First(&cfg).Error; err == nil {
		return cfg, nil
	}
	// 4. Platform model
	if err := h.db.Where("user_id = 'platform' AND is_enabled = ?", true).Order("created_at ASC").First(&cfg).Error; err == nil {
		return cfg, nil
	}
	return cfg, errors.New("请先在「模型管理」中启用至少一个模型")
}

// ── CDN Upload（cdn.starclaw.net） ─────────────────────────────────────
//
// Env（所有可选；全部齐备才会真的走 scp，否则 fallback 到 /v1/uploads）：
//   CDN_SSH_HOST       e.g. your-cdn-server-ip
//   CDN_SSH_USER       e.g. root
//   CDN_SSH_KEY_PATH   container 内路径 e.g. /root/.ssh/cdn_id_ed25519
//   CDN_REMOTE_ROOT    e.g. /opt/cdn
//   CDN_PUBLIC_BASE    e.g. https://cdn.starclaw.net
//   CDN_CLAW_ID        e.g. 0x1a2b3c4d（上传者 Claw 节点地址/租户目录）
//
// Upload rules（见 SHORT_DRAMA_AGENT_V3_ARCHITECTURE.md § Ⅱ·补）：
//   {CDN_REMOTE_ROOT}/{CDN_CLAW_ID}/{drama}/{asset_type}/{filename}
//   → {CDN_PUBLIC_BASE}/{CDN_CLAW_ID}/{drama}/{asset_type}/{filename}

type cdnConfig struct {
	Host, User, KeyPath, RemoteRoot, PublicBase, ClawID string
}

func loadCDNConfig() (*cdnConfig, bool) {
	cfg := &cdnConfig{
		Host:       strings.TrimSpace(os.Getenv("CDN_SSH_HOST")),
		User:       strings.TrimSpace(os.Getenv("CDN_SSH_USER")),
		KeyPath:    strings.TrimSpace(os.Getenv("CDN_SSH_KEY_PATH")),
		RemoteRoot: strings.TrimSpace(os.Getenv("CDN_REMOTE_ROOT")),
		PublicBase: strings.TrimSpace(os.Getenv("CDN_PUBLIC_BASE")),
		ClawID:     strings.TrimSpace(os.Getenv("CDN_CLAW_ID")),
	}
	// Auto-derive ClawID from node identity if not explicitly set
	if cfg.ClawID == "" {
		cfg.ClawID = node.ReadNodeIDHex()
	}
	if cfg.Host == "" || cfg.User == "" || cfg.KeyPath == "" || cfg.RemoteRoot == "" || cfg.PublicBase == "" || cfg.ClawID == "" {
		return nil, false
	}
	return cfg, true
}

// sanitizeFilenamePath 允许多层目录 a/b/c.ext 但过滤 "..", 反斜杠和非法字符
func sanitizeFilenamePath(fn string) string {
	fn = strings.ReplaceAll(fn, "\\", "/")
	parts := strings.Split(fn, "/")
	var clean []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" || p == "." || p == ".." {
			continue
		}
		// allow letters, digits, _, -, dot, and common CJK
		safe := make([]rune, 0, len(p))
		for _, r := range p {
			switch {
			case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
				safe = append(safe, r)
			case r == '_', r == '-', r == '.':
				safe = append(safe, r)
			case r >= 0x4e00 && r <= 0x9fff:
				safe = append(safe, r)
			}
		}
		if len(safe) > 0 {
			clean = append(clean, string(safe))
		}
	}
	return strings.Join(clean, "/")
}

// CDNUpload 接收文件，优先 scp 到 cdn.starclaw.net；未配置或失败则回退到 /v1/uploads/<uuid>.ext
// 前端 form: file, drama, asset_type, filename（可选，支持 dir/name.ext）
// 响应: { target: "cdn" | "local", url, size, mime }
func (h *CharacterStudioHandler) CDNUpload(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no file uploaded"})
		return
	}
	defer file.Close()
	if header.Size > maxUploadSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("file too large, max %dMB", maxUploadSize/(1024*1024))})
		return
	}
	// Read file into memory once (参考图一般 < 10MB，可接受)
	buf := &bytes.Buffer{}
	if _, err := io.Copy(buf, file); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "read upload failed"})
		return
	}
	data := buf.Bytes()

	drama := strings.TrimSpace(c.PostForm("drama"))
	assetType := strings.TrimSpace(c.PostForm("asset_type"))
	filename := strings.TrimSpace(c.PostForm("filename"))
	if drama == "" {
		drama = "default"
	}
	if assetType == "" {
		assetType = "misc"
	}
	// 标准化目录名
	drama = sanitizeFilenamePath(drama)
	assetType = sanitizeFilenamePath(assetType)
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext == "" {
		ext = ".png"
	}
	if filename == "" {
		filename = uuid.New().String() + ext
	} else {
		filename = sanitizeFilenamePath(filename)
		if filepath.Ext(filename) == "" {
			filename += ext
		}
	}

	// ── Try CDN scp ──
	if cfg, ok := loadCDNConfig(); ok {
		remotePath := path.Join(cfg.RemoteRoot, cfg.ClawID, drama, assetType, filename)
		publicURL := strings.TrimRight(cfg.PublicBase, "/") + "/" + path.Join(cfg.ClawID, drama, assetType, filename)
		if err := scpUploadBytes(cfg, data, remotePath); err != nil {
			log.Printf("[cdn_upload] scp failed (%s), falling back to local: %v", remotePath, err)
		} else {
			c.JSON(http.StatusOK, gin.H{
				"target":      "cdn",
				"url":         publicURL,
				"remote_path": remotePath,
				"size":        len(data),
				"mime":        header.Header.Get("Content-Type"),
			})
			return
		}
	}

	// ── Fallback: local /v1/uploads/<uuid>.ext ──
	fileID := uuid.New().String()
	storedName := fileID + ext
	destPath := filepath.Join(sandbox.UploadsDir(), storedName)
	if err := os.WriteFile(destPath, data, 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save file"})
		return
	}
	// Build absolute URL so fal.ai/Seedance can fetch it
	origin := c.Request.Header.Get("X-Forwarded-Proto")
	host := c.Request.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = c.Request.Host
	}
	if origin == "" {
		origin = "http"
		if c.Request.TLS != nil {
			origin = "https"
		}
	}
	localURL := fmt.Sprintf("%s://%s/v1/uploads/%s", origin, host, storedName)
	c.JSON(http.StatusOK, gin.H{
		"target":         "local",
		"url":            localURL,
		"relative_url":   "/v1/uploads/" + storedName,
		"size":           len(data),
		"mime":           header.Header.Get("Content-Type"),
		"cdn_configured": false,
		"note":           "CDN env not configured; returned local stable URL. Configure CDN_SSH_HOST/USER/KEY_PATH/REMOTE_ROOT/PUBLIC_BASE/CLAW_ID to enable cdn.starclaw.net upload.",
	})
}

// ── CDN Upload From Local URL ─────────────────────────────────────
//
// POST /v1/cdn/upload-from-local
//
//	body: { image_url, drama, asset_type, filename? }
//	resp: same shape as /cdn/upload
//
// Why this exists:
//   The Workflow NodePropertyPanel stores images as string URLs (e.g.
//   "/v1/projects/swarm-universe/entities/characters/lin_jianyue/sheets/unified_sheet_v6.png"
//   or a generated /v1/uploads/<id>.png). The multipart /cdn/upload endpoint
//   is useless there — we already have the bytes on the server. This endpoint
//   takes that string, reads bytes via loadImageBytes (same helper used by
//   tos_launder), and pushes to CDN with the identical scp flow.

type uploadFromLocalReq struct {
	ImageURL  string `json:"image_url" binding:"required"`
	Drama     string `json:"drama"`
	AssetType string `json:"asset_type"`
	Filename  string `json:"filename"`
}

// UploadFromLocal reads an image by URL (http OR /v1/projects|/v1/uploads|/v1/images)
// and pushes it to cdn.starclaw.net via scp. Mirrors CDNUpload's response.
func (h *CharacterStudioHandler) UploadFromLocal(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req uploadFromLocalReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body", "detail": err.Error()})
		return
	}
	raw := strings.TrimSpace(req.ImageURL)
	if raw == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "image_url is required"})
		return
	}

	data, mime, srcLabel, err := loadImageBytes(raw)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(data) > maxUploadSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("image too large, max %dMB", maxUploadSize/(1024*1024))})
		return
	}

	drama := strings.TrimSpace(req.Drama)
	assetType := strings.TrimSpace(req.AssetType)
	filename := strings.TrimSpace(req.Filename)
	if drama == "" {
		drama = "default"
	}
	if assetType == "" {
		assetType = "misc"
	}
	drama = sanitizeFilenamePath(drama)
	assetType = sanitizeFilenamePath(assetType)

	// Infer extension from source URL / mime if caller didn't specify.
	ext := strings.ToLower(filepath.Ext(raw))
	if ext == "" || len(ext) > 6 {
		ext = ".png"
		if strings.HasPrefix(mime, "image/jpeg") {
			ext = ".jpg"
		} else if strings.HasPrefix(mime, "image/webp") {
			ext = ".webp"
		}
	}
	if filename == "" {
		// Prefer the basename of the source so users recognize it in the CDN.
		base := filepath.Base(raw)
		base = strings.Split(base, "?")[0]
		base = strings.Split(base, "#")[0]
		if base == "" || base == "/" || base == "." {
			filename = uuid.New().String() + ext
		} else {
			filename = sanitizeFilenamePath(base)
			if filepath.Ext(filename) == "" {
				filename += ext
			}
		}
	} else {
		filename = sanitizeFilenamePath(filename)
		if filepath.Ext(filename) == "" {
			filename += ext
		}
	}

	cfg, ok := loadCDNConfig()
	if !ok {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "CDN_SSH_HOST/USER/KEY_PATH/REMOTE_ROOT/PUBLIC_BASE/CLAW_ID 没配全；当前容器没法推 CDN",
			"hint":  "把图先挂到本地 /v1/uploads/ 也能喂 Seedance；或把 6 个环境变量配好再点",
		})
		return
	}
	remotePath := path.Join(cfg.RemoteRoot, cfg.ClawID, drama, assetType, filename)
	publicURL := strings.TrimRight(cfg.PublicBase, "/") + "/" + path.Join(cfg.ClawID, drama, assetType, filename)
	if err := scpUploadBytes(cfg, data, remotePath); err != nil {
		log.Printf("[cdn_upload_from_local] scp failed src=%s remote=%s: %v", srcLabel, remotePath, err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "scp to CDN failed", "detail": err.Error(), "remote_path": remotePath})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"target":      "cdn",
		"url":         publicURL,
		"remote_path": remotePath,
		"size":        len(data),
		"mime":        mime,
		"source":      srcLabel,
	})
}

// scpUploadBytes writes data to a temp file then uses scp to push to remote:path.
// mkdir -p the remote directory first via ssh.
func scpUploadBytes(cfg *cdnConfig, data []byte, remotePath string) error {
	// Write to temp file
	tmp, err := os.CreateTemp("", "cdn_upload_*")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp: %w", err)
	}
	tmp.Close()

	// Ensure SSH key has strict permissions (Docker-on-Windows mounts as 0777)
	keyPath, err := ensureKeyPerms(cfg.KeyPath)
	if err != nil {
		return fmt.Errorf("prepare ssh key: %w", err)
	}

	// 1) mkdir -p 远端目录
	remoteDir := path.Dir(remotePath)
	sshCmd := exec.Command(
		"ssh",
		"-i", keyPath,
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "UserKnownHostsFile=/tmp/cdn_known_hosts",
		"-o", "ConnectTimeout=10",
		"-o", "BatchMode=yes",
		fmt.Sprintf("%s@%s", cfg.User, cfg.Host),
		fmt.Sprintf("mkdir -p %q", remoteDir),
	)
	if out, err := sshCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ssh mkdir: %w (out=%s)", err, string(out))
	}

	// 2) scp 上传
	scpCmd := exec.Command(
		"scp",
		"-i", keyPath,
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "UserKnownHostsFile=/tmp/cdn_known_hosts",
		"-o", "ConnectTimeout=10",
		"-o", "BatchMode=yes",
		tmp.Name(),
		fmt.Sprintf("%s@%s:%s", cfg.User, cfg.Host, remotePath),
	)
	if out, err := scpCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("scp: %w (out=%s)", err, string(out))
	}

	// 3) chmod 644 so nginx (non-root) can serve the file
	chmodCmd := exec.Command(
		"ssh",
		"-i", keyPath,
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "UserKnownHostsFile=/tmp/cdn_known_hosts",
		"-o", "ConnectTimeout=10",
		"-o", "BatchMode=yes",
		fmt.Sprintf("%s@%s", cfg.User, cfg.Host),
		fmt.Sprintf("chmod 644 %q", remotePath),
	)
	if out, err := chmodCmd.CombinedOutput(); err != nil {
		log.Printf("[cdn_upload] chmod warning (non-fatal): %v (out=%s)", err, string(out))
	}
	return nil
}

// ensureKeyPerms copies the SSH key to /tmp with 0600 if the original has
// overly permissive mode (e.g. 0777 from Docker-on-Windows volume mounts).
// Returns the usable key path (original or temp copy).
func ensureKeyPerms(keyPath string) (string, error) {
	info, err := os.Stat(keyPath)
	if err != nil {
		return "", err
	}
	if info.Mode().Perm()&0077 == 0 {
		return keyPath, nil // already strict
	}
	// Copy to temp with 0600
	dst := "/tmp/cdn_key_safe"
	raw, err := os.ReadFile(keyPath)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(dst, raw, 0600); err != nil {
		return "", err
	}
	return dst, nil
}
