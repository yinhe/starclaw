package media

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw/internal/tool"
)

// ── Video Archive ──────────────────────────────────────────────
//
// Why this exists:
//   Seedance / Volcengine video URLs expire after 24h. When a user runs an
//   entire episode production pass, we want each successful take to be
//   written to the local drama repo so that:
//     1. The raw mp4 survives beyond the TOS URL expiry.
//     2. The directory mirrors the "authoritative" clip register that
//        editors/git can follow (e.g. docs/swarm-universe/production/ep05/clips_v2/S1_t1.mp4).
//     3. A JSON ledger (_generated_urls.json) captures prompt, refs, task_id,
//        tos_url, etc. so regenerations/audits are reproducible.
//
// Contract:
//   POST /v1/videos/archive
//   body: {
//     video_url:   "<http/https url OR /v1/videos/...>"    (required)
//     project:     "swarm-universe"                        (required)
//     episode:     "ep05"                                  (required)
//     scene:       "S1"                                    (required)
//     take_id:     "t1"                                    (required)
//     prompt?:     "...",
//     ref_images?: ["/v1/projects/...", "https://..."],
//     task_id?:    "cgt-...",
//     model?:      "doubao-seedance-2-0-...",
//     overwrite?:  true                                    (default true; false → 409 if exists)
//   }
//   resp: {
//     local_path: "/production/ep05/clips_v2/S1_t1.mp4",
//     local_url:  "/v1/projects/swarm-universe/production/ep05/clips_v2/S1_t1.mp4",
//     size:       12345678,
//     ledger_entries: N,
//     note:       "..."
//   }

// archiveLedgerMu 保护 _generated_urls.json 的并发读-改-写。
// 按 <project>/<episode> 粒度共享一把锁——同一集并行归档多 take 时
// 只会串行追写 JSON，避免丢条目。
var (
	archiveLedgerMu   sync.Mutex
	archiveValidIDRe  = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.\-]*$`)
	archiveValidEpRe  = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.\-]{0,63}$`)
	archiveMaxVideoMB = int64(512) // hard cap to avoid runaway downloads
)

type videoArchiveReq struct {
	VideoURL  string   `json:"video_url" binding:"required"`
	Project   string   `json:"project" binding:"required"`
	Episode   string   `json:"episode" binding:"required"`
	Scene     string   `json:"scene" binding:"required"`
	TakeID    string   `json:"take_id" binding:"required"`
	Prompt    string   `json:"prompt,omitempty"`
	RefImages []string `json:"ref_images,omitempty"`
	TaskID    string   `json:"task_id,omitempty"`
	Model     string   `json:"model,omitempty"`
	Overwrite *bool    `json:"overwrite,omitempty"`
}

// Archive downloads a generated video to the project's production/<episode>/clips_v2/
// directory and appends a ledger entry to _generated_urls.json.
func (h *VideoHandler) Archive(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req videoArchiveReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "detail": err.Error()})
		return
	}

	// Sanitize path components — no traversal, no slashes in "leaf" fields.
	if !archiveValidEpRe.MatchString(req.Project) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project id"})
		return
	}
	if !archiveValidEpRe.MatchString(req.Episode) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid episode id"})
		return
	}
	if !archiveValidIDRe.MatchString(req.Scene) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid scene id"})
		return
	}
	if !archiveValidIDRe.MatchString(req.TakeID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid take_id"})
		return
	}

	docsDir := os.Getenv("DOCS_DIR")
	if docsDir == "" {
		docsDir = "/app/docs"
	}
	destDir := filepath.Join(docsDir, req.Project, "production", req.Episode, "clips_v2")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "create dest dir failed", "detail": err.Error()})
		return
	}

	filename := fmt.Sprintf("%s_%s.mp4", req.Scene, req.TakeID)
	destPath := filepath.Join(destDir, filename)

	overwrite := true
	if req.Overwrite != nil {
		overwrite = *req.Overwrite
	}
	if !overwrite {
		if _, err := os.Stat(destPath); err == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "file already exists", "local_path": toPublicPath(destPath, docsDir, req.Project)})
			return
		}
	}

	// Download or copy the video bytes to destPath.
	size, err := archiveFetchToFile(c.Request.Context(), req.VideoURL, destPath)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "fetch video failed", "detail": err.Error()})
		return
	}

	// Append to _generated_urls.json (serialized via package-level mutex).
	ledgerPath := filepath.Join(destDir, "_generated_urls.json")
	entriesN, ledgerErr := appendArchiveLedger(ledgerPath, archiveLedgerEntry{
		Scene:      req.Scene,
		TakeID:     req.TakeID,
		Filename:   filename,
		LocalPath:  toPublicPath(destPath, docsDir, req.Project),
		TOSUrl:     normalizeVideoURL(req.VideoURL),
		Prompt:     req.Prompt,
		RefImages:  req.RefImages,
		TaskID:     req.TaskID,
		Model:      req.Model,
		SizeBytes:  size,
		ArchivedAt: time.Now().UTC().Format(time.RFC3339),
		ArchivedBy: userID,
	})
	if ledgerErr != nil {
		// The mp4 is already on disk; surface the ledger failure but don't fail the request.
		c.JSON(http.StatusOK, gin.H{
			"local_path":     toPublicPath(destPath, docsDir, req.Project),
			"local_url":      "/v1/projects/" + req.Project + toPublicPath(destPath, docsDir, req.Project),
			"size":           size,
			"ledger_entries": 0,
			"ledger_error":   ledgerErr.Error(),
			"note":           fmt.Sprintf("Video archived to %s; ledger write FAILED — entry not recorded.", filename),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"local_path":     toPublicPath(destPath, docsDir, req.Project),
		"local_url":      "/v1/projects/" + req.Project + toPublicPath(destPath, docsDir, req.Project),
		"size":           size,
		"ledger_entries": entriesN,
		"note":           fmt.Sprintf("Archived %s_%s.mp4 (%.2f MB) into %s/clips_v2/", req.Scene, req.TakeID, float64(size)/1024/1024, req.Episode),
	})
}

// archiveFetchToFile writes the remote (or local /v1/videos/) video to dest.
// Returns bytes written.
func archiveFetchToFile(ctx context.Context, src, dest string) (int64, error) {
	// Local path under /v1/videos/clips|merged — copy from disk.
	if strings.HasPrefix(src, "/v1/videos/clips/") || strings.HasPrefix(src, "/v1/videos/merged/") {
		var localSrc string
		if strings.HasPrefix(src, "/v1/videos/clips/") {
			localSrc = filepath.Join(tool.VideosDir(), strings.TrimPrefix(src, "/v1/videos/clips/"))
		} else {
			localSrc = filepath.Join(tool.MergedVideosDir(), strings.TrimPrefix(src, "/v1/videos/merged/"))
		}
		if _, err := os.Stat(localSrc); err != nil {
			return 0, fmt.Errorf("local source not found: %s", localSrc)
		}
		return copyFile(localSrc, dest)
	}

	// Remote — HTTP GET with cap.
	if !(strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://")) {
		return 0, fmt.Errorf("unsupported video_url scheme: %s", src)
	}
	httpCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	reqH, err := http.NewRequestWithContext(httpCtx, "GET", src, nil)
	if err != nil {
		return 0, fmt.Errorf("build request: %v", err)
	}
	resp, err := http.DefaultClient.Do(reqH)
	if err != nil {
		return 0, fmt.Errorf("GET %s: %v", src, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return 0, fmt.Errorf("GET %s: HTTP %d", src, resp.StatusCode)
	}

	tmp := dest + ".partial"
	out, err := os.Create(tmp)
	if err != nil {
		return 0, fmt.Errorf("create tmp: %v", err)
	}
	written, err := io.Copy(out, io.LimitReader(resp.Body, archiveMaxVideoMB*1024*1024))
	closeErr := out.Close()
	if err != nil {
		_ = os.Remove(tmp)
		return 0, fmt.Errorf("copy body: %v", err)
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return 0, fmt.Errorf("close tmp: %v", closeErr)
	}
	if written == archiveMaxVideoMB*1024*1024 {
		_ = os.Remove(tmp)
		return 0, fmt.Errorf("video exceeds %d MB cap", archiveMaxVideoMB)
	}
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return 0, fmt.Errorf("rename: %v", err)
	}
	return written, nil
}

// copyFile copies src → dest, returns bytes written.
func copyFile(src, dest string) (int64, error) {
	in, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer in.Close()
	out, err := os.Create(dest + ".partial")
	if err != nil {
		return 0, err
	}
	n, err := io.Copy(out, in)
	closeErr := out.Close()
	if err != nil {
		_ = os.Remove(dest + ".partial")
		return 0, err
	}
	if closeErr != nil {
		_ = os.Remove(dest + ".partial")
		return 0, closeErr
	}
	if err := os.Rename(dest+".partial", dest); err != nil {
		_ = os.Remove(dest + ".partial")
		return 0, err
	}
	return n, nil
}

// toPublicPath strips the docsDir + project prefix and returns the path users
// can serve via /v1/projects/<project>/<returned>. E.g. returns
// "/production/ep05/clips_v2/S1_t1.mp4".
func toPublicPath(absPath, docsDir, project string) string {
	prefix := filepath.Join(docsDir, project)
	rel := strings.TrimPrefix(filepath.ToSlash(absPath), filepath.ToSlash(prefix))
	if !strings.HasPrefix(rel, "/") {
		rel = "/" + rel
	}
	return rel
}

// normalizeVideoURL strips the pre-signed query string from a TOS URL (to
// keep the ledger stable across refreshes). Non-http URLs pass through.
func normalizeVideoURL(raw string) string {
	// Keep full URL as-is; the query params carry the signing metadata that
	// users may want to resign later. Store verbatim.
	return raw
}

// ── ledger ──

type archiveLedgerEntry struct {
	Scene      string   `json:"scene"`
	TakeID     string   `json:"take_id"`
	Filename   string   `json:"filename"`
	LocalPath  string   `json:"local_path"`
	TOSUrl     string   `json:"tos_url,omitempty"`
	Prompt     string   `json:"prompt,omitempty"`
	RefImages  []string `json:"ref_images,omitempty"`
	TaskID     string   `json:"task_id,omitempty"`
	Model      string   `json:"model,omitempty"`
	SizeBytes  int64    `json:"size_bytes"`
	ArchivedAt string   `json:"archived_at"`
	ArchivedBy string   `json:"archived_by,omitempty"`
}

type archiveLedger struct {
	SchemaVersion int                  `json:"schema_version"`
	UpdatedAt     string               `json:"updated_at"`
	Takes         []archiveLedgerEntry `json:"takes"`
}

// appendArchiveLedger reads the existing _generated_urls.json (if any),
// replaces any existing <scene>/<take_id> entry, appends the new one, and
// writes atomically. Entries are sorted by scene, then take_id.
func appendArchiveLedger(path string, entry archiveLedgerEntry) (int, error) {
	archiveLedgerMu.Lock()
	defer archiveLedgerMu.Unlock()

	var led archiveLedger
	if raw, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(raw, &led); err != nil {
			// Corrupt file — back it up so we don't lose history, then start fresh.
			_ = os.Rename(path, path+".corrupt-"+time.Now().UTC().Format("20060102-150405"))
			led = archiveLedger{}
		}
	}
	if led.SchemaVersion == 0 {
		led.SchemaVersion = 1
	}

	// Remove any prior entry for the same scene+take_id (we replace in place).
	filtered := make([]archiveLedgerEntry, 0, len(led.Takes)+1)
	for _, e := range led.Takes {
		if e.Scene == entry.Scene && e.TakeID == entry.TakeID {
			continue
		}
		filtered = append(filtered, e)
	}
	filtered = append(filtered, entry)
	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].Scene != filtered[j].Scene {
			return filtered[i].Scene < filtered[j].Scene
		}
		return filtered[i].TakeID < filtered[j].TakeID
	})
	led.Takes = filtered
	led.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	data, err := json.MarshalIndent(led, "", "  ")
	if err != nil {
		return 0, err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return 0, err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return 0, err
	}
	return len(led.Takes), nil
}
