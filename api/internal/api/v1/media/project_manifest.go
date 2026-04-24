package media

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ── Project Manifest Helpers ───────────────────────────────
//
// Surfaces two endpoints for the workflow preflight self-check:
//
//   POST /v1/projects/:project/ref/suggest
//     body: { character_key?, character_label?, hint?, broken_ref?, limit? }
//     resp: { candidates: [{ path, size, score, reason, mtime }] }
//
//     Fuzzy-scans docs/<project>/ for plausible reference images tied to the
//     given character. Used by the "一键修复" flow when a manifest.ref HEADs 404.
//
//   PUT  /v1/projects/:project/manifest/characters/:key
//     body: { ref }
//     resp: { character, previous_ref }
//
//     Validates the new ref actually exists in docs/<project>/ then writes the
//     change back to manifest.json atomically.
//
// Why this exists: the frontend preflight discovers missing refs and needs a
// way to (a) propose plausible replacements and (b) commit the chosen one
// without asking the user to hand-edit JSON.

// ProjectManifestHandler groups the helper endpoints for a single project.
type ProjectManifestHandler struct {
	db *gorm.DB
}

func NewProjectManifestHandler(db *gorm.DB) *ProjectManifestHandler {
	return &ProjectManifestHandler{db: db}
}

var (
	// Single mutex protects any manifest.json read-modify-write across the
	// whole server. Projects are few and manifests are tiny — serializing is
	// simpler and safer than per-project locks.
	manifestWriteMu sync.Mutex
	// Restrict file scanning to image types we can hand to Seedance.
	manifestScanExts = map[string]bool{
		".png": true, ".jpg": true, ".jpeg": true, ".webp": true,
	}
)

// ── Suggest ──

type suggestReq struct {
	CharacterKey   string `json:"character_key"`
	CharacterLabel string `json:"character_label"`
	Hint           string `json:"hint"`
	BrokenRef      string `json:"broken_ref"`
	Limit          int    `json:"limit"`
	// Optional kind filter: "sheets" | "raw" | "nano" | "variants" | "" (=all).
	// Used by the NodePropertyPanel tab switcher to narrow candidates.
	Kind string `json:"kind"`
}

type suggestCandidate struct {
	Path   string `json:"path"`   // "/entities/characters/sumi/sheets/unified_sheet_v7.png"
	URL    string `json:"url"`    // "/v1/projects/<project>/entities/characters/..."
	Size   int64  `json:"size"`   // bytes
	MTime  string `json:"mtime"`  // RFC3339
	Score  int    `json:"score"`  // higher = better
	Reason string `json:"reason"` // human explanation of score breakdown
	// Kind classifies the candidate by subdir (sheets/raw/nano/variants/legacy),
	// so the UI can render tabs and the promote flow can pick a canonical source.
	Kind string `json:"kind"`
}

// resolveKeyFromManifest opens manifest.json and, if exactly one character
// label matches (case-insensitive exact match against label), returns its key.
// This is our escape hatch when the frontend only has a Chinese label like
// "林见月" and the workflow snapshot predates key propagation — without it,
// a Chinese label never substring-matches against pinyin paths and the
// candidate bar renders (0).
func resolveKeyFromManifest(projectDir, label string) string {
	if label == "" {
		return ""
	}
	manifestPath := filepath.Join(projectDir, "assets", "manifest.json")
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		return ""
	}
	var m struct {
		Characters []struct {
			Key   string `json:"key"`
			Label string `json:"label"`
		} `json:"characters"`
		Props []struct {
			Key   string `json:"key"`
			Label string `json:"label"`
		} `json:"props"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return ""
	}
	want := strings.ToLower(strings.TrimSpace(label))
	for _, ch := range m.Characters {
		if strings.ToLower(ch.Label) == want {
			return ch.Key
		}
	}
	for _, p := range m.Props {
		if strings.ToLower(p.Label) == want {
			return p.Key
		}
	}
	return ""
}

// classifyCandidate inspects a relSlash path like
// "/entities/characters/lin_jianyue/sheets/unified_sheet_v6.png" and returns
// ("sheets", true). Legacy paths (/assets/ or /production/) return
// ("legacy", true).
func classifyCandidate(relSlash string) (kind string, ok bool) {
	lower := strings.ToLower(relSlash)
	switch {
	case strings.Contains(lower, "/entities/") && strings.Contains(lower, "/sheets/"):
		return "sheets", true
	case strings.Contains(lower, "/entities/") && strings.Contains(lower, "/raw/"):
		return "raw", true
	case strings.Contains(lower, "/entities/") && strings.Contains(lower, "/nano/"):
		return "nano", true
	case strings.Contains(lower, "/entities/") && strings.Contains(lower, "/variants/"):
		return "variants", true
	case strings.Contains(lower, "/production/") || strings.Contains(lower, "/assets/"):
		return "legacy", true
	}
	return "other", false
}

// SuggestRef walks the project's docs dir for plausible character image files.
func (h *ProjectManifestHandler) SuggestRef(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	project := c.Param("project")
	if !archiveValidEpRe.MatchString(project) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project id"})
		return
	}

	var req suggestReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body", "detail": err.Error()})
		return
	}
	if req.Limit <= 0 || req.Limit > 50 {
		req.Limit = 8
	}

	docsDir := os.Getenv("DOCS_DIR")
	if docsDir == "" {
		docsDir = "/app/docs"
	}
	projectDir := filepath.Join(docsDir, project)
	if _, err := os.Stat(projectDir); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "project dir not found", "detail": err.Error()})
		return
	}

	// Hint resolution — this is the anti-empty-result logic.
	// When the frontend only has a Chinese label ("林见月") and no key, we
	// look the label up in manifest.json to recover the pinyin key, so
	// substring scoring can actually hit the path "/entities/characters/lin_jianyue/...".
	resolvedKey := strings.TrimSpace(req.CharacterKey)
	if resolvedKey == "" && strings.TrimSpace(req.CharacterLabel) != "" {
		resolvedKey = resolveKeyFromManifest(projectDir, req.CharacterLabel)
	}

	// Collect hint tokens. Lowercased, deduped later by scoreCandidate.
	hints := []string{}
	for _, s := range []string{resolvedKey, req.CharacterKey, req.Hint, req.CharacterLabel} {
		s = strings.ToLower(strings.TrimSpace(s))
		if s != "" {
			hints = append(hints, s)
		}
	}
	if req.BrokenRef != "" {
		// Tokenize the broken ref — the last path segment often holds a
		// close match (e.g. ".../sumi/ref.png" → hint "sumi").
		parts := strings.Split(strings.ToLower(req.BrokenRef), "/")
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" && p != "ref.png" && !strings.HasPrefix(p, ".") {
				hints = append(hints, p)
			}
		}
	}
	if len(hints) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "provide at least one of character_key, hint, character_label, broken_ref"})
		return
	}

	candidates := make([]suggestCandidate, 0, 64)
	err := filepath.Walk(projectDir, func(path string, info os.FileInfo, werr error) error {
		if werr != nil || info == nil {
			return nil // keep walking
		}
		if info.IsDir() {
			// Skip hidden dirs and very deep archives.
			name := info.Name()
			if strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if !manifestScanExts[ext] {
			return nil
		}

		rel, err := filepath.Rel(projectDir, path)
		if err != nil {
			return nil
		}
		relSlash := "/" + filepath.ToSlash(rel)
		relLower := strings.ToLower(relSlash)

		score, reason := scoreCandidate(relLower, info, hints)
		if score <= 0 {
			return nil
		}

		kind, _ := classifyCandidate(relSlash)
		// Optional kind filter (UI tab switcher). Empty/"all" = no filter.
		if req.Kind != "" && req.Kind != "all" && kind != req.Kind {
			return nil
		}

		candidates = append(candidates, suggestCandidate{
			Path:   relSlash,
			URL:    "/v1/projects/" + project + relSlash,
			Size:   info.Size(),
			MTime:  info.ModTime().UTC().Format(time.RFC3339),
			Score:  score,
			Reason: reason,
			Kind:   kind,
		})
		return nil
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "walk failed", "detail": err.Error()})
		return
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Score != candidates[j].Score {
			return candidates[i].Score > candidates[j].Score
		}
		return candidates[i].Size > candidates[j].Size
	})
	if len(candidates) > req.Limit {
		candidates = candidates[:req.Limit]
	}

	c.JSON(http.StatusOK, gin.H{
		"candidates":    candidates,
		"hints_used":    hints,
		"scanned_under": "/" + project,
	})
}

// scoreCandidate assigns a heuristic score to a candidate image relative to
// the hint tokens. Returns (score, humanReason).
//
// The logic is intentionally simple and easy to explain in the UI:
//
//	+50  for each hint that appears in the path
//	+15  if the path lives under /production/ (curated, high-quality)
//	+12  if the filename contains a version suffix (v6, v7 …) — prefer newest
//	+8   if the filename contains "sheet" / "unified" / "ref"
//	+5   per MB up to 40 (bigger usually = higher resolution)
//	+3   bonus for newest mtime in the last 30 days
//	-20  if the file is in /_archive_drafts/ (historical rejects)
//	-10  if the file is a frame in /props/*_frame_\d+.png (motion sheet tile)
func scoreCandidate(relLower string, info os.FileInfo, hints []string) (int, string) {
	score := 0
	reasons := []string{}

	for _, h := range hints {
		if h == "" {
			continue
		}
		if strings.Contains(relLower, h) {
			score += 50
			reasons = append(reasons, "hit:"+h)
		}
	}
	if score == 0 {
		return 0, ""
	}

	// v2 entities tree is authoritative; boost it hardest so the candidate
	// bar always surfaces entities/*/sheets/*.png above legacy copies.
	if strings.Contains(relLower, "/entities/") && strings.Contains(relLower, "/sheets/") {
		score += 40
		reasons = append(reasons, "entities-sheet")
	} else if strings.Contains(relLower, "/entities/") && strings.Contains(relLower, "/variants/") {
		score += 20
		reasons = append(reasons, "entities-variant")
	} else if strings.Contains(relLower, "/entities/") && strings.Contains(relLower, "/nano/") {
		score += 25
		reasons = append(reasons, "entities-nano")
	} else if strings.Contains(relLower, "/entities/") && strings.Contains(relLower, "/raw/") {
		score += 10
		reasons = append(reasons, "entities-raw")
	} else if strings.Contains(relLower, "/production/") {
		// Legacy pre-v2 location — still usable but demoted.
		score += 5
		reasons = append(reasons, "production")
	} else if strings.Contains(relLower, "/assets/") {
		score += 3
		reasons = append(reasons, "assets")
	}
	if strings.Contains(relLower, "/_archive_drafts/") || strings.Contains(relLower, "/_archive/") {
		score -= 20
		reasons = append(reasons, "archive-draft")
	}

	base := filepath.Base(relLower)
	// Version suffix bump: v2, v6, v10 … prefer higher.
	if vBump := extractVersionBump(base); vBump > 0 {
		score += 12 + vBump*2
		reasons = append(reasons, fmt.Sprintf("v%d", vBump))
	}

	for _, kw := range []string{"unified", "sheet", "ref", "portrait", "character_sheet"} {
		if strings.Contains(base, kw) {
			score += 4
			reasons = append(reasons, kw)
			break
		}
	}

	// Tile frames (e.g. bread_frame_2.png) are animation source — not standalone refs.
	if strings.Contains(base, "_frame_") {
		score -= 10
		reasons = append(reasons, "frame-tile")
	}

	if sz := info.Size(); sz > 0 {
		mb := int(sz / (1024 * 1024))
		if mb > 40 {
			mb = 40
		}
		score += mb * 5
		if mb > 0 {
			reasons = append(reasons, fmt.Sprintf("%dMB", mb))
		}
	}

	// Freshness: within 30d gets a small bump — lets brand-new files surface.
	if time.Since(info.ModTime()) < 30*24*time.Hour {
		score += 3
		reasons = append(reasons, "new")
	}

	if len(reasons) > 6 {
		reasons = reasons[:6]
	}
	return score, strings.Join(reasons, " ")
}

// extractVersionBump finds the highest vN suffix in a filename and returns N.
// E.g. "unified_sheet_v6.png" → 6, "sumi_ref_v7.png" → 7.
func extractVersionBump(base string) int {
	best := 0
	i := 0
	for i < len(base) {
		if base[i] != 'v' {
			i++
			continue
		}
		// Require preceding non-alpha (avoid matching "conversion" etc.)
		if i > 0 && isAlphaNum(base[i-1]) && base[i-1] != '_' {
			i++
			continue
		}
		j := i + 1
		for j < len(base) && base[j] >= '0' && base[j] <= '9' {
			j++
		}
		if j > i+1 {
			if n, err := strconv.Atoi(base[i+1 : j]); err == nil && n > best {
				best = n
			}
			i = j
			continue
		}
		i++
	}
	return best
}

func isAlphaNum(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

// ── Promote to Sheet ─────────────────────────────────
//
// POST /v1/projects/:project/entities/:kind/:key/promote
//
//	body: { source_path?: "/entities/.../nano/xyz.png", source_url?: "https://..." }
//	resp: { new_ref, new_url, previous_ref, version, size_bytes }
//
// Copies a candidate image (typically from nano/ or raw/) into
// entities/<kind>/<key>/sheets/unified_sheet_v<N+1>.png and flips the
// manifest.ref (characters only for v1 — props have a different shape).
//
// Exactly one of source_path / source_url must be provided:
//   - source_path: a project-relative path (e.g. returned by /ref/suggest)
//     that will be copied on-disk.
//   - source_url : any http(s) URL that will be downloaded to the same place.
//
// The old manifest ref is preserved under `previous_ref` in the response; the
// old sheet file is left in place (sheets/ is append-only).

type promoteReq struct {
	SourcePath string `json:"source_path"`
	SourceURL  string `json:"source_url"`
}

// promoteSheetVRe matches unified_sheet_v<N>.png so nextSheetVersion can pick N+1.
var promoteSheetVRe = regexp.MustCompile(`(?i)^unified_sheet_v(\d+)\.png$`)

func nextSheetVersion(sheetsDir string) int {
	entries, err := os.ReadDir(sheetsDir)
	if err != nil {
		return 1
	}
	best := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		m := promoteSheetVRe.FindStringSubmatch(e.Name())
		if len(m) == 2 {
			if n, err := strconv.Atoi(m[1]); err == nil && n > best {
				best = n
			}
		}
	}
	return best + 1
}

// PromoteToSheet handles POST /v1/projects/:project/entities/:kind/:key/promote.
func (h *ProjectManifestHandler) PromoteToSheet(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	project := c.Param("project")
	kind := c.Param("kind")
	key := c.Param("key")
	for _, p := range []string{project, kind, key} {
		if !archiveValidEpRe.MatchString(p) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid path component"})
			return
		}
	}
	if kind != "characters" && kind != "props" && kind != "scenes" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "kind must be characters|props|scenes"})
		return
	}

	var req promoteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body", "detail": err.Error()})
		return
	}
	req.SourcePath = strings.TrimSpace(req.SourcePath)
	req.SourceURL = strings.TrimSpace(req.SourceURL)
	if req.SourcePath == "" && req.SourceURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "provide source_path OR source_url"})
		return
	}
	if req.SourcePath != "" && req.SourceURL != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "pick exactly one: source_path OR source_url"})
		return
	}
	if req.SourcePath != "" {
		if !strings.HasPrefix(req.SourcePath, "/") || strings.Contains(req.SourcePath, "..") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "source_path must be /entities/... or similar project-relative path"})
			return
		}
	}

	docsDir := os.Getenv("DOCS_DIR")
	if docsDir == "" {
		docsDir = "/app/docs"
	}
	projectDir := filepath.Join(docsDir, project)
	sheetsDir := filepath.Join(projectDir, "entities", kind, key, "sheets")
	if err := os.MkdirAll(sheetsDir, 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot mkdir sheets", "detail": err.Error()})
		return
	}

	version := nextSheetVersion(sheetsDir)
	destName := fmt.Sprintf("unified_sheet_v%d.png", version)
	destFS := filepath.Join(sheetsDir, destName)
	relSlash := fmt.Sprintf("/entities/%s/%s/sheets/%s", kind, key, destName)

	if req.SourcePath != "" {
		srcFS := filepath.Join(projectDir, filepath.FromSlash(req.SourcePath))
		fi, err := os.Stat(srcFS)
		if err != nil || fi.IsDir() {
			c.JSON(http.StatusBadRequest, gin.H{"error": "source_path does not resolve to a file", "detail": fmt.Sprintf("%v", err), "resolved": srcFS})
			return
		}
		if err := promoteCopyFile(srcFS, destFS); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "copy failed", "detail": err.Error()})
			return
		}
	} else {
		if err := promoteDownloadToFile(req.SourceURL, destFS); err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "download failed", "detail": err.Error()})
			return
		}
	}
	fi, _ := os.Stat(destFS)
	sizeBytes := int64(0)
	if fi != nil {
		sizeBytes = fi.Size()
	}

	// Patch manifest.characters[<key>].ref atomically (characters only).
	var previousRef string
	if kind == "characters" {
		manifestWriteMu.Lock()
		defer manifestWriteMu.Unlock()

		manifestPath := filepath.Join(projectDir, "assets", "manifest.json")
		raw, err := os.ReadFile(manifestPath)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "manifest.json not found", "detail": err.Error()})
			return
		}
		var m map[string]interface{}
		if err := json.Unmarshal(raw, &m); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "manifest corrupt", "detail": err.Error()})
			return
		}
		chars, ok := m["characters"].([]interface{})
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "manifest missing characters[]"})
			return
		}
		found := false
		for i, ch := range chars {
			cm, ok := ch.(map[string]interface{})
			if !ok {
				continue
			}
			if k, _ := cm["key"].(string); k == key {
				if pr, _ := cm["ref"].(string); pr != "" {
					previousRef = pr
				}
				cm["ref"] = relSlash
				chars[i] = cm
				found = true
				break
			}
		}
		if !found {
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("character key %q not found", key)})
			return
		}
		m["characters"] = chars
		m["updated_at"] = time.Now().UTC().Format("2006-01-02")

		out, err := json.MarshalIndent(m, "", "  ")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "marshal failed", "detail": err.Error()})
			return
		}
		tmp := manifestPath + ".tmp"
		if err := os.WriteFile(tmp, out, 0o644); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "write tmp failed", "detail": err.Error()})
			return
		}
		if err := os.Rename(tmp, manifestPath); err != nil {
			_ = os.Remove(tmp)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "rename failed", "detail": err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"new_ref":      relSlash,
		"new_url":      "/v1/projects/" + project + relSlash,
		"previous_ref": previousRef,
		"version":      version,
		"size_bytes":   sizeBytes,
		"note":         fmt.Sprintf("%s/%s promoted to sheet v%d (%.2f MB)", kind, key, version, float64(sizeBytes)/1024/1024),
	})
}

// promoteCopyFile is a simple filesystem copy with truncation on existing dest.
func promoteCopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return nil
}

// promoteDownloadToFile streams a remote URL into a file; 40 MiB cap.
func promoteDownloadToFile(url, dst string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("remote returned %d", resp.StatusCode)
	}
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.CopyN(f, resp.Body, 40*1024*1024); err != nil && err != io.EOF {
		if err == io.ErrUnexpectedEOF {
			_, _ = io.Copy(f, resp.Body)
			return nil
		}
		return err
	}
	return nil
}

type setCharRefReq struct {
	Ref string `json:"ref" binding:"required"`
}

// SetCharacterRef updates one character's ref field in manifest.json atomically.
func (h *ProjectManifestHandler) SetCharacterRef(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	project := c.Param("project")
	key := c.Param("key")
	if !archiveValidEpRe.MatchString(project) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project id"})
		return
	}
	if !archiveValidEpRe.MatchString(key) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid character key"})
		return
	}

	var req setCharRefReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body", "detail": err.Error()})
		return
	}
	ref := strings.TrimSpace(req.Ref)
	if !strings.HasPrefix(ref, "/") || strings.Contains(ref, "..") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ref must be an absolute path under the project (e.g. /production/...)"})
		return
	}

	docsDir := os.Getenv("DOCS_DIR")
	if docsDir == "" {
		docsDir = "/app/docs"
	}
	projectDir := filepath.Join(docsDir, project)
	manifestPath := filepath.Join(projectDir, "assets", "manifest.json")

	// Verify the new ref actually resolves to a real file under the project.
	targetAbs := filepath.Join(projectDir, filepath.FromSlash(ref))
	fi, err := os.Stat(targetAbs)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ref does not resolve to an existing file", "detail": err.Error(), "resolved_to": targetAbs})
		return
	}
	if fi.IsDir() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ref points to a directory"})
		return
	}

	manifestWriteMu.Lock()
	defer manifestWriteMu.Unlock()

	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "manifest.json not found", "detail": err.Error()})
		return
	}

	// Parse as a generic map to preserve unknown fields and key order as much
	// as we can (MarshalIndent is stable for map[string]interface{} on Go 1.12+
	// as alpha-sorted; for a manifest we control, that's acceptable).
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "manifest corrupt", "detail": err.Error()})
		return
	}

	chars, ok := m["characters"].([]interface{})
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "manifest missing characters[]"})
		return
	}

	var previousRef string
	found := false
	for i, ch := range chars {
		cm, ok := ch.(map[string]interface{})
		if !ok {
			continue
		}
		if k, _ := cm["key"].(string); k == key {
			if pr, _ := cm["ref"].(string); pr != "" {
				previousRef = pr
			}
			cm["ref"] = ref
			chars[i] = cm
			found = true
			break
		}
	}
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("character key %q not found in manifest", key)})
		return
	}
	m["characters"] = chars
	m["updated_at"] = time.Now().UTC().Format("2006-01-02")

	// Atomic write: tmp → rename.
	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "marshal failed", "detail": err.Error()})
		return
	}
	tmp := manifestPath + ".tmp"
	if err := os.WriteFile(tmp, out, 0o644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "write tmp failed", "detail": err.Error()})
		return
	}
	if err := os.Rename(tmp, manifestPath); err != nil {
		_ = os.Remove(tmp)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "rename failed", "detail": err.Error()})
		return
	}

	// Return the patched character back.
	var patched map[string]interface{}
	for _, ch := range chars {
		if cm, ok := ch.(map[string]interface{}); ok {
			if k, _ := cm["key"].(string); k == key {
				patched = cm
				break
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"character":    patched,
		"previous_ref": previousRef,
		"size_bytes":   fi.Size(),
		"note":         fmt.Sprintf("%s.ref updated to %s (%.2f MB)", key, ref, float64(fi.Size())/1024/1024),
	})
}
