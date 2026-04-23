package tool

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
)

// CDN upload for generated video clips → cdn.starclaw.net
//
// Env（全部齐备才会走 scp，否则 UploadLocalFileToCDN 返回 ok=false，调用方回退）：
//
//	CDN_SSH_HOST       e.g. 43.106.158.26
//	CDN_SSH_USER       e.g. root
//	CDN_SSH_KEY_PATH   container 内路径 e.g. /root/.ssh/cdn_id_ed25519
//	CDN_REMOTE_ROOT    e.g. /opt/cdn
//	CDN_PUBLIC_BASE    e.g. https://cdn.starclaw.net
//	CDN_CLAW_ID        e.g. 0x1a2b3c4d（上传者 Claw 节点地址/租户目录）
//
// Upload rules:
//
//	{CDN_REMOTE_ROOT}/{CDN_CLAW_ID}/{drama}/{asset_type}/{filename}
//	→ {CDN_PUBLIC_BASE}/{CDN_CLAW_ID}/{drama}/{asset_type}/{filename}
type cdnEnvConfig struct {
	Host, User, KeyPath, RemoteRoot, PublicBase, ClawID string
}

func loadCDNEnvConfig() (*cdnEnvConfig, bool) {
	cfg := &cdnEnvConfig{
		Host:       strings.TrimSpace(os.Getenv("CDN_SSH_HOST")),
		User:       strings.TrimSpace(os.Getenv("CDN_SSH_USER")),
		KeyPath:    strings.TrimSpace(os.Getenv("CDN_SSH_KEY_PATH")),
		RemoteRoot: strings.TrimSpace(os.Getenv("CDN_REMOTE_ROOT")),
		PublicBase: strings.TrimSpace(os.Getenv("CDN_PUBLIC_BASE")),
		ClawID:     strings.TrimSpace(os.Getenv("CDN_CLAW_ID")),
	}
	if cfg.Host == "" || cfg.User == "" || cfg.KeyPath == "" || cfg.RemoteRoot == "" || cfg.PublicBase == "" || cfg.ClawID == "" {
		return nil, false
	}
	return cfg, true
}

// sanitizeCDNPathSegment 允许 letters/digits/_-. 和常用 CJK
func sanitizeCDNPathSegment(s string) string {
	s = strings.ReplaceAll(s, "\\", "/")
	parts := strings.Split(s, "/")
	var clean []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" || p == "." || p == ".." {
			continue
		}
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

// UploadLocalFileToCDN takes a filesystem path to a local file (e.g. the MP4 that
// SaveClipLocally produced) and SCPs it to cdn.starclaw.net following the layout
//
//	{CDN_PUBLIC_BASE}/{CDN_CLAW_ID}/{drama}/{assetType}/{filename}
//
// Returns the public CDN URL on success. If CDN env is not configured or scp
// fails, returns (empty, false) so the caller can fall back to the local URL.
func UploadLocalFileToCDN(localPath, drama, assetType, filename string) (string, bool) {
	if _, err := os.Stat(localPath); err != nil {
		return "", false
	}
	cfg, ok := loadCDNEnvConfig()
	if !ok {
		return "", false
	}
	drama = sanitizeCDNPathSegment(drama)
	if drama == "" {
		drama = "default"
	}
	assetType = sanitizeCDNPathSegment(assetType)
	if assetType == "" {
		assetType = "misc"
	}
	if filename == "" {
		filename = filepath.Base(localPath)
	}
	filename = sanitizeCDNPathSegment(filename)
	if filename == "" {
		filename = filepath.Base(localPath)
	}

	remotePath := path.Join(cfg.RemoteRoot, cfg.ClawID, drama, assetType, filename)
	publicURL := strings.TrimRight(cfg.PublicBase, "/") + "/" + path.Join(cfg.ClawID, drama, assetType, filename)

	if err := scpUploadLocalFile(cfg, localPath, remotePath); err != nil {
		log.Printf("[cdn_upload] scp failed for %s → %s: %v", localPath, remotePath, err)
		return "", false
	}
	log.Printf("[cdn_upload] uploaded %s → %s", localPath, publicURL)
	return publicURL, true
}

// scpUploadLocalFile runs ssh mkdir then scp localPath -> user@host:remotePath.
func scpUploadLocalFile(cfg *cdnEnvConfig, localPath, remotePath string) error {
	remoteDir := path.Dir(remotePath)

	// 1) ssh mkdir -p remoteDir
	sshCmd := exec.Command(
		"ssh",
		"-i", cfg.KeyPath,
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

	// 2) scp
	scpCmd := exec.Command(
		"scp",
		"-i", cfg.KeyPath,
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "UserKnownHostsFile=/tmp/cdn_known_hosts",
		"-o", "ConnectTimeout=10",
		"-o", "BatchMode=yes",
		localPath,
		fmt.Sprintf("%s@%s:%s", cfg.User, cfg.Host, remotePath),
	)
	if out, err := scpCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("scp: %w (out=%s)", err, string(out))
	}
	return nil
}

// LocalPathFromVideoURL converts a "/v1/videos/clips/<name>.mp4" URL back to
// its filesystem path under VideosDir(). Returns empty string if the URL is
// not a local clip URL that maps to disk.
func LocalPathFromVideoURL(videoURL string) string {
	const prefix = "/v1/videos/clips/"
	if !strings.HasPrefix(videoURL, prefix) {
		return ""
	}
	name := strings.TrimPrefix(videoURL, prefix)
	if name == "" || strings.Contains(name, "/") || strings.Contains(name, "..") {
		return ""
	}
	return filepath.Join(VideosDir(), name)
}

// DramaFromScene derives a drama slug from a scene label like "EP05.S1" → "ep05".
// Used as the {drama} segment in the CDN path. Falls back to category if empty.
func DramaFromScene(scene, category string) string {
	s := strings.TrimSpace(scene)
	if s != "" {
		if idx := strings.IndexAny(s, ".·_ "); idx > 0 {
			s = s[:idx]
		}
		s = sanitizeCDNPathSegment(strings.ToLower(s))
		if s != "" {
			return s
		}
	}
	cat := sanitizeCDNPathSegment(strings.ToLower(strings.TrimSpace(category)))
	if cat != "" {
		return cat
	}
	return "default"
}
