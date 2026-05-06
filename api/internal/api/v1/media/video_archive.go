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
	"github.com/yinhe/starclaw/internal/model"
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

	localURL := "/v1/projects/" + req.Project + toPublicPath(destPath, docsDir, req.Project)

	// 把 local_url 回写到 VideoRecord（按 task_id 匹配）。
	// /v1/videos 页面会用它做 TOS 过期退路。
	if h.db != nil && req.TaskID != "" {
		h.db.Model(&model.VideoRecord{}).
			Where("task_id = ?", req.TaskID).
			Update("local_url", localURL)
	}

	c.JSON(http.StatusOK, gin.H{
		"local_path":     toPublicPath(destPath, docsDir, req.Project),
		"local_url":      localURL,
		"size":           size,
		"ledger_entries": entriesN,
		"note":           fmt.Sprintf("Archived %s_%s.mp4 (%.2f MB) into %s/clips_v2/", req.Scene, req.TakeID, float64(size)/1024/1024, req.Episode),
	})
}

// BackfillArchivedURLs scans every docs/<project>/production/<ep>/clips_v2/_generated_urls.json
// and updates VideoRecord.local_url for each ledger entry that has a task_id but no local_url
// in DB. Idempotent — safe to run multiple times.
func (h *VideoHandler) BackfillArchivedURLs(c *gin.Context) {
	if h.db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db not configured"})
		return
	}
	docsDir := os.Getenv("DOCS_DIR")
	if docsDir == "" {
		docsDir = "/app/docs"
	}
	projects, err := os.ReadDir(docsDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "read docs dir failed", "detail": err.Error()})
		return
	}
	type result struct {
		Project  string `json:"project"`
		Episode  string `json:"episode"`
		Scene    string `json:"scene"`
		TakeID   string `json:"take_id"`
		TaskID   string `json:"task_id"`
		LocalURL string `json:"local_url"`
		Updated  bool   `json:"updated"`
		Note     string `json:"note,omitempty"`
	}
	var results []result
	updated, skipped, missing := 0, 0, 0
	for _, p := range projects {
		if !p.IsDir() {
			continue
		}
		prodDir := filepath.Join(docsDir, p.Name(), "production")
		eps, err := os.ReadDir(prodDir)
		if err != nil {
			continue
		}
		for _, ep := range eps {
			if !ep.IsDir() {
				continue
			}
			ledgerPath := filepath.Join(prodDir, ep.Name(), "clips_v2", "_generated_urls.json")
			raw, err := os.ReadFile(ledgerPath)
			if err != nil {
				continue
			}
			var led archiveLedger
			if err := json.Unmarshal(raw, &led); err != nil {
				continue
			}
			for _, e := range led.Takes {
				if e.TaskID == "" || e.LocalPath == "" {
					continue
				}
				localURL := "/v1/projects/" + p.Name() + e.LocalPath
				// Verify file actually exists on disk before pointing DB at it.
				absPath := filepath.Join(docsDir, p.Name(), strings.TrimPrefix(e.LocalPath, "/"))
				if _, err := os.Stat(absPath); err != nil {
					missing++
					results = append(results, result{
						Project: p.Name(), Episode: ep.Name(), Scene: e.Scene, TakeID: e.TakeID,
						TaskID: e.TaskID, LocalURL: localURL, Updated: false,
						Note: "file missing on disk",
					})
					continue
				}
				res := h.db.Model(&model.VideoRecord{}).
					Where("task_id = ? AND (local_url = '' OR local_url IS NULL)", e.TaskID).
					Update("local_url", localURL)
				if res.Error != nil {
					results = append(results, result{
						Project: p.Name(), Episode: ep.Name(), Scene: e.Scene, TakeID: e.TakeID,
						TaskID: e.TaskID, LocalURL: localURL, Updated: false,
						Note: res.Error.Error(),
					})
					continue
				}
				if res.RowsAffected > 0 {
					updated++
					results = append(results, result{
						Project: p.Name(), Episode: ep.Name(), Scene: e.Scene, TakeID: e.TakeID,
						TaskID: e.TaskID, LocalURL: localURL, Updated: true,
					})
				} else {
					skipped++
				}
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"updated":         updated,
		"skipped_existed": skipped,
		"missing_on_disk": missing,
		"details":         results,
	})
}

// CleanupExpiredOrphans soft-deletes succeeded video records whose mp4 is no
// longer reachable: local_url is empty and the original TOS URL has expired
// (record older than 24h). Idempotent. Returns counts + sample task_ids.
func (h *VideoHandler) CleanupExpiredOrphans(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	if h.db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db not configured"})
		return
	}
	cutoff := time.Now().Add(-24 * time.Hour)
	// Count what's about to be removed for the response.
	var victims []model.VideoRecord
	h.db.Where(
		"user_id = ? AND status = ? AND (local_url = '' OR local_url IS NULL) "+
			"AND video_url LIKE '%tos-cn-beijing.volces.com%' AND created_at < ?",
		userID, "succeeded", cutoff,
	).Limit(1000).Find(&victims)
	taskIDs := make([]string, 0, len(victims))
	for _, v := range victims {
		taskIDs = append(taskIDs, v.TaskID)
	}
	res := h.db.Where(
		"user_id = ? AND status = ? AND (local_url = '' OR local_url IS NULL) "+
			"AND video_url LIKE '%tos-cn-beijing.volces.com%' AND created_at < ?",
		userID, "succeeded", cutoff,
	).Delete(&model.VideoRecord{})
	c.JSON(http.StatusOK, gin.H{
		"deleted":         res.RowsAffected,
		"sample_task_ids": taskIDs[:minInt(len(taskIDs), 50)],
		"cutoff":          cutoff.Format(time.RFC3339),
		"note":            "Soft-deleted succeeded records older than 24h with no local_url and a TOS video_url (废片 — TOS expired, unrecoverable).",
	})
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
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

// ── Episode publish ────────────────────────────────────────────
//
// Why this exists:
//   The per-scene takes produced by the workflow live in
//     docs/<project>/production/<ep>/clips_v2/<Scene>_<takeId>.mp4
//   but the *canonical* episode directory that ep01–ep04 use (and that
//   manifest.json points at via `episodes[].scenes[].clip`) is
//     docs/<project>/episodes/<ep>/scenes/<Scene>.mp4
//   + an episode README.md.
//
//   Without this endpoint, EP05's picked takes stay invisible in the
//   script directory (user: "生成的EP05也没出现在剧本目录里"). This
//   publishes the currently-picked clips into the canonical tree so the
//   script repo tree matches the finished episodes.
//
// Contract:
//   POST /v1/videos/publish-episode
//   body: {
//     project: "swarm-universe",
//     episode: "ep05",
//     picked:  [ { scene: "S1", take_id: "t1" }, ... ],   (required, non-empty)
//     title?:   "夜袭",                                    (written into README)
//     description?: "...",
//   }
//   resp: {
//     published: [ { scene, src, dst, size } ],
//     missing:   [ { scene, expected } ],   // sources that didn't exist
//     episode_dir: "/episodes/ep05",
//     readme:     "/episodes/ep05/README.md",
//     note:       "..."
//   }
//
// Design notes:
//   - Idempotent: overwrites existing <Scene>.mp4 on each call. The editor
//     re-picks takes a lot during review so we always publish the CURRENT
//     pick (no complicated reconciliation).
//   - Only runs if the source file actually exists in clips_v2. If a scene
//     is missing we report it back in `missing` instead of failing the whole
//     publish — lets partial progress persist.
//   - README.md is regenerated every call. If the user edits it by hand they
//     should keep custom notes in a separate file (we only rewrite the header).

type publishPickedEntry struct {
	Scene  string `json:"scene" binding:"required"`
	TakeID string `json:"take_id" binding:"required"`
}

type publishEpisodeReq struct {
	Project     string               `json:"project" binding:"required"`
	Episode     string               `json:"episode" binding:"required"`
	Picked      []publishPickedEntry `json:"picked" binding:"required"`
	Title       string               `json:"title,omitempty"`
	Description string               `json:"description,omitempty"`
	// 用户反馈：点合成成片后看不到成片在哪里。原来 PublishEpisode 只回流
	// 各场景 picked take 到 episodes/<ep>/scenes/，合成后的整片留在 api 容器
	// 内的 merged_videos/，docs 树里看不到。这里加 FinalVideoURL：传入
	// "/v1/videos/merged/<uuid>.mp4"，后端解析出本地路径，拷到
	// episodes/<ep>/final.mp4，与 ep04 等老剧集布局对齐。
	FinalVideoURL string `json:"final_video_url,omitempty"`
}

type publishedEntry struct {
	Scene string `json:"scene"`
	Src   string `json:"src"`
	Dst   string `json:"dst"`
	Size  int64  `json:"size"`
}

type missingEntry struct {
	Scene    string `json:"scene"`
	Expected string `json:"expected"`
}

// PublishEpisode copies picked takes from production/<ep>/clips_v2/ to the
// canonical episodes/<ep>/scenes/ directory and regenerates the episode README.
func (h *VideoHandler) PublishEpisode(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req publishEpisodeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "detail": err.Error()})
		return
	}
	if !archiveValidEpRe.MatchString(req.Project) || !archiveValidEpRe.MatchString(req.Episode) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project or episode id"})
		return
	}
	if len(req.Picked) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "picked[] must not be empty"})
		return
	}
	for _, p := range req.Picked {
		if !archiveValidIDRe.MatchString(p.Scene) || !archiveValidIDRe.MatchString(p.TakeID) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid scene or take_id", "scene": p.Scene, "take_id": p.TakeID})
			return
		}
	}

	docsDir := os.Getenv("DOCS_DIR")
	if docsDir == "" {
		docsDir = "/app/docs"
	}
	srcDir := filepath.Join(docsDir, req.Project, "production", req.Episode, "clips_v2")
	dstDir := filepath.Join(docsDir, req.Project, "episodes", req.Episode, "scenes")
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "mkdir episodes/<ep>/scenes failed", "detail": err.Error()})
		return
	}

	// De-dup by scene (if user passed multiple picks for same scene,
	// later entry wins; usually shouldn't happen but belt-and-suspenders).
	pickByScene := map[string]string{}
	sceneOrder := []string{}
	for _, p := range req.Picked {
		if _, ok := pickByScene[p.Scene]; !ok {
			sceneOrder = append(sceneOrder, p.Scene)
		}
		pickByScene[p.Scene] = p.TakeID
	}

	published := make([]publishedEntry, 0, len(sceneOrder))
	missing := make([]missingEntry, 0)
	for _, scene := range sceneOrder {
		takeID := pickByScene[scene]
		srcName := fmt.Sprintf("%s_%s.mp4", scene, takeID)
		srcPath := filepath.Join(srcDir, srcName)
		if _, err := os.Stat(srcPath); err != nil {
			// Picked file missing — try fallback to the most recently modified
			// <scene>_*.mp4 for this scene. Workflow state often has a stale
			// picked_take id (e.g. user picked the in-memory take but archive
			// stored under a different t-number, or the picked take was rerun).
			fallbackName, fallbackPath := findLatestSceneTake(srcDir, scene)
			if fallbackPath == "" {
				missing = append(missing, missingEntry{
					Scene:    scene,
					Expected: toPublicPath(srcPath, docsDir, req.Project),
				})
				continue
			}
			_ = fallbackName
			srcPath = fallbackPath
		}
		dstName := scene + ".mp4"
		dstPath := filepath.Join(dstDir, dstName)
		n, err := copyFile(srcPath, dstPath)
		if err != nil {
			// Don't abort — surface the failure on this scene, keep going.
			missing = append(missing, missingEntry{
				Scene:    scene,
				Expected: fmt.Sprintf("%s (copy failed: %v)", toPublicPath(srcPath, docsDir, req.Project), err),
			})
			continue
		}
		published = append(published, publishedEntry{
			Scene: scene,
			Src:   toPublicPath(srcPath, docsDir, req.Project),
			Dst:   toPublicPath(dstPath, docsDir, req.Project),
			Size:  n,
		})
	}

	// 同步成片到 episodes/<ep>/final.mp4。来源优先级：
	//   1) req.FinalVideoURL（前端 runMerge 完成后传入的 /v1/videos/merged/<uuid>.mp4）
	//   2) clips_v2/final.mp4（老链路：用户手工把成片放进 clips_v2）
	// 这样剧本目录布局就和 ep04 等老剧集保持一致，docs 树里能直接看到成片。
	finalDst := filepath.Join(docsDir, req.Project, "episodes", req.Episode, "final.mp4")
	finalPublished := false
	var finalSize int64
	var finalSrcResolved string
	if req.FinalVideoURL != "" {
		// 仅接受 /v1/videos/merged/<uuid>.mp4 形式（合成产物）。其它形式
		// （TOS 公网、CDN 等）我们不在 publish 阶段下载，避免引入外网依赖。
		const prefix = "/v1/videos/merged/"
		if strings.HasPrefix(req.FinalVideoURL, prefix) {
			leaf := strings.TrimPrefix(req.FinalVideoURL, prefix)
			// 防穿目录
			if !strings.Contains(leaf, "/") && !strings.Contains(leaf, "..") && strings.HasSuffix(leaf, ".mp4") {
				candidate := filepath.Join(tool.MergedVideosDir(), leaf)
				if _, err := os.Stat(candidate); err == nil {
					finalSrcResolved = candidate
				}
			}
		}
	}
	if finalSrcResolved == "" {
		legacy := filepath.Join(srcDir, "final.mp4")
		if _, err := os.Stat(legacy); err == nil {
			finalSrcResolved = legacy
		}
	}
	if finalSrcResolved != "" {
		if n, err := copyFile(finalSrcResolved, finalDst); err == nil {
			finalPublished = true
			finalSize = n
		}
	}

	// Write/refresh episode README.md with the current picked roster.
	readmePath := filepath.Join(docsDir, req.Project, "episodes", req.Episode, "README.md")
	readmeRel := "/episodes/" + req.Episode + "/README.md"
	if err := writeEpisodeReadme(readmePath, req, published, missing, userID); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"published":    published,
			"missing":      missing,
			"episode_dir":  "/episodes/" + req.Episode,
			"readme":       readmeRel,
			"readme_error": err.Error(),
			"note": fmt.Sprintf("Published %d scene(s) to episodes/%s/scenes/ — README write FAILED.",
				len(published), req.Episode),
		})
		return
	}

	resp := gin.H{
		"published":   published,
		"missing":     missing,
		"episode_dir": "/episodes/" + req.Episode,
		"readme":      readmeRel,
		"note": fmt.Sprintf("Published %d scene(s) to episodes/%s/scenes/%s.",
			len(published), req.Episode,
			func() string {
				if len(missing) > 0 {
					return fmt.Sprintf(" %d scene(s) missing; listed in `missing`", len(missing))
				}
				return ""
			}()),
	}
	if finalPublished {
		resp["final_video"] = "/episodes/" + req.Episode + "/final.mp4"
		resp["final_size"] = finalSize
	}
	c.JSON(http.StatusOK, resp)
}

// writeEpisodeReadme regenerates episodes/<ep>/README.md with title, description,
// picked clips table, and any missing scenes. Intentionally idempotent — caller
// should keep custom notes in a separate file.
func writeEpisodeReadme(path string, req publishEpisodeReq, published []publishedEntry, missing []missingEntry, userID string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var b strings.Builder
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = strings.ToUpper(req.Episode)
	}
	fmt.Fprintf(&b, "# %s · %s\n\n", strings.ToUpper(req.Episode), title)
	if desc := strings.TrimSpace(req.Description); desc != "" {
		fmt.Fprintf(&b, "%s\n\n", desc)
	}
	fmt.Fprintf(&b, "最后更新：%s  ·  操作人：%s\n\n", time.Now().UTC().Format(time.RFC3339), userID)

	fmt.Fprintf(&b, "## 场景清单（picked takes）\n\n")
	if len(published) == 0 {
		fmt.Fprintf(&b, "_暂无已挑选的镜头。_\n\n")
	} else {
		fmt.Fprintf(&b, "| 场景 | 文件 | 大小 | 来源 |\n|---|---|---|---|\n")
		for _, p := range published {
			fmt.Fprintf(&b, "| `%s` | [`scenes/%s.mp4`](scenes/%s.mp4) | %.2f MB | `%s` |\n",
				p.Scene, p.Scene, p.Scene, float64(p.Size)/1024/1024, p.Src)
		}
		fmt.Fprintf(&b, "\n")
	}

	if len(missing) > 0 {
		fmt.Fprintf(&b, "## 缺失场景\n\n")
		for _, m := range missing {
			fmt.Fprintf(&b, "- `%s` → 源文件不存在：`%s`\n", m.Scene, m.Expected)
		}
		fmt.Fprintf(&b, "\n")
	}

	fmt.Fprintf(&b, "## 规范\n\n")
	fmt.Fprintf(&b, "- 原始 take 归档：`/production/%s/clips_v2/<Scene>_<takeId>.mp4`\n", req.Episode)
	fmt.Fprintf(&b, "- picked 镜头：`/episodes/%s/scenes/<Scene>.mp4`（本文件指向的那些）\n", req.Episode)
	fmt.Fprintf(&b, "- 合成成片：`/episodes/%s/final.mp4`\n", req.Episode)
	fmt.Fprintf(&b, "\n本 README 由 POST /v1/videos/publish-episode 自动重写，手工改动会在下次发布被覆盖。\n")

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(b.String()), 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// findLatestSceneTake scans srcDir for files matching <scene>_*.mp4 and
// returns the most recently modified one. Used as a fallback when the
// picked_take id stored in the workflow state doesn't match an archived
// filename (rerun renumbered, partial save, etc.). Returns ("","") if no
// take exists for the scene.
func findLatestSceneTake(srcDir, scene string) (string, string) {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return "", ""
	}
	prefix := scene + "_"
	var bestName, bestPath string
	var bestMod time.Time
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, ".mp4") {
			continue
		}
		// Reject scenes that share a prefix (e.g. S2 must not match S2b_*.mp4).
		// After stripping prefix, the next segment up to '_' or '.' is the take id.
		rest := name[len(prefix):]
		if rest == "" || rest[0] < 't' {
			// Take ids start with 't1', 't2', etc. — anything else means we
			// matched a different scene that just shares the prefix.
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(bestMod) {
			bestMod = info.ModTime()
			bestName = name
			bestPath = filepath.Join(srcDir, name)
		}
	}
	return bestName, bestPath
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
