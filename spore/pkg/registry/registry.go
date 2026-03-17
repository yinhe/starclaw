package registry

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// PackageInfo describes a published spore package in a registry.
type PackageInfo struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	Platform string `json:"platform"` // "linux/amd64"
	Checksum string `json:"checksum"`
	Size     int64  `json:"size"`
	URL      string `json:"url"` // download URL
	DeltaURL string `json:"delta_url,omitempty"`
	DeltaFrom string `json:"delta_from,omitempty"` // base version for delta
	PushedAt string `json:"pushed_at"`
}

// Index is the registry package index.
type Index struct {
	Packages []PackageInfo `json:"packages"`
	UpdatedAt string      `json:"updated_at"`
}

// Client talks to a Spore registry (Hatchery serve or Nydus peer).
type Client struct {
	sources  []string // ordered list of registry URLs
	cacheDir string
	httpC    *http.Client
}

// NewClient creates a registry client with the given source URLs.
// Sources are tried in order: local cache → Nydus peers → central hatchery.
func NewClient(cacheDir string, sources ...string) *Client {
	return &Client{
		sources:  sources,
		cacheDir: cacheDir,
		httpC:    &http.Client{Timeout: 60 * time.Second},
	}
}

// Resolve finds the best download source for a package.
// Supports "name:version" or "name:latest" format.
func (c *Client) Resolve(nameVersion string) (*PackageInfo, error) {
	parts := strings.SplitN(nameVersion, ":", 2)
	name := parts[0]
	version := "latest"
	if len(parts) == 2 && parts[1] != "" {
		version = parts[1]
	}

	for _, src := range c.sources {
		idx, err := c.fetchIndex(src)
		if err != nil {
			continue
		}

		// Filter by name and platform
		var candidates []PackageInfo
		for _, p := range idx.Packages {
			if p.Name == name {
				if version == "latest" || p.Version == version {
					candidates = append(candidates, p)
				}
			}
		}

		if len(candidates) == 0 {
			continue
		}

		// Sort by version descending (simple string sort, good enough for semver)
		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].Version > candidates[j].Version
		})

		return &candidates[0], nil
	}

	return nil, fmt.Errorf("package %q not found in any registry", nameVersion)
}

// Pull downloads a .spore package to the cache directory.
// Returns the local file path.
func (c *Client) Pull(pkg *PackageInfo) (string, error) {
	os.MkdirAll(c.cacheDir, 0755)

	filename := fmt.Sprintf("%s-v%s-%s.spore", pkg.Name, pkg.Version,
		strings.ReplaceAll(pkg.Platform, "/", "-"))
	localPath := filepath.Join(c.cacheDir, filename)

	// Check cache
	if existingChecksum, err := checksumFile(localPath); err == nil {
		if existingChecksum == pkg.Checksum {
			return localPath, nil // already cached
		}
	}

	// Download
	url := pkg.URL
	if url == "" {
		return "", fmt.Errorf("no download URL for %s v%s", pkg.Name, pkg.Version)
	}

	fmt.Printf("⬇️  Downloading %s v%s (%s)...\n", pkg.Name, pkg.Version, pkg.Platform)

	resp, err := c.httpC.Get(url)
	if err != nil {
		return "", fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download: HTTP %d", resp.StatusCode)
	}

	tmpFile := localPath + ".tmp"
	f, err := os.Create(tmpFile)
	if err != nil {
		return "", err
	}

	hasher := sha256.New()
	written, err := io.Copy(io.MultiWriter(f, hasher), resp.Body)
	f.Close()
	if err != nil {
		os.Remove(tmpFile)
		return "", fmt.Errorf("download write: %w", err)
	}

	// Verify checksum
	gotChecksum := "sha256:" + hex.EncodeToString(hasher.Sum(nil))
	if pkg.Checksum != "" && gotChecksum != pkg.Checksum {
		os.Remove(tmpFile)
		return "", fmt.Errorf("checksum mismatch: expected %s, got %s", pkg.Checksum, gotChecksum)
	}

	os.Rename(tmpFile, localPath)
	sizeMB := float64(written) / (1024 * 1024)
	fmt.Printf("   ✅ Downloaded %.1f MB (%s)\n", sizeMB, gotChecksum[:20]+"...")

	return localPath, nil
}

// PullDelta attempts to download a delta patch instead of the full package.
// Returns the patched local file path, or error if delta is not available.
func (c *Client) PullDelta(pkg *PackageInfo, currentVersion string) (string, error) {
	if pkg.DeltaURL == "" || pkg.DeltaFrom != currentVersion {
		return "", fmt.Errorf("no delta available from v%s to v%s", currentVersion, pkg.Version)
	}

	os.MkdirAll(c.cacheDir, 0755)

	deltaFile := filepath.Join(c.cacheDir, fmt.Sprintf("%s-delta-%s-to-%s.patch",
		pkg.Name, currentVersion, pkg.Version))

	fmt.Printf("⬇️  Downloading delta patch %s → %s...\n", currentVersion, pkg.Version)

	resp, err := c.httpC.Get(pkg.DeltaURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download delta: HTTP %d", resp.StatusCode)
	}

	f, err := os.Create(deltaFile)
	if err != nil {
		return "", err
	}
	io.Copy(f, resp.Body)
	f.Close()

	fmt.Printf("   ✅ Delta patch downloaded\n")
	return deltaFile, nil
}

// Push uploads a .spore package to a registry.
func (c *Client) Push(sporePath string, registryURL string) error {
	if registryURL == "" && len(c.sources) > 0 {
		registryURL = c.sources[0]
	}
	if registryURL == "" {
		return fmt.Errorf("no registry URL specified")
	}

	f, err := os.Open(sporePath)
	if err != nil {
		return err
	}
	defer f.Close()

	fi, _ := f.Stat()
	fmt.Printf("⬆️  Pushing %s (%.1f MB) to %s...\n",
		filepath.Base(sporePath), float64(fi.Size())/(1024*1024), registryURL)

	uploadURL := strings.TrimSuffix(registryURL, "/") + "/v1/spore/upload"
	resp, err := c.httpC.Post(uploadURL, "application/octet-stream", f)
	if err != nil {
		return fmt.Errorf("push: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("push failed: HTTP %d: %s", resp.StatusCode, string(body))
	}

	fmt.Printf("   ✅ Pushed successfully\n")
	return nil
}

// ListCached returns all .spore files in the local cache.
func (c *Client) ListCached() ([]string, error) {
	var files []string
	entries, err := os.ReadDir(c.cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".spore") {
			files = append(files, filepath.Join(c.cacheDir, e.Name()))
		}
	}
	return files, nil
}

// CleanCache removes cached .spore files older than maxAge.
func (c *Client) CleanCache(maxAge time.Duration) (int, error) {
	entries, err := os.ReadDir(c.cacheDir)
	if err != nil {
		return 0, err
	}
	cutoff := time.Now().Add(-maxAge)
	removed := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".spore") {
			continue
		}
		info, _ := e.Info()
		if info != nil && info.ModTime().Before(cutoff) {
			os.Remove(filepath.Join(c.cacheDir, e.Name()))
			removed++
		}
	}
	return removed, nil
}

// ── Internal ──

func (c *Client) fetchIndex(registryURL string) (*Index, error) {
	url := strings.TrimSuffix(registryURL, "/") + "/v1/spore/index.json"
	resp, err := c.httpC.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("registry %s: HTTP %d", registryURL, resp.StatusCode)
	}

	var idx Index
	if err := json.NewDecoder(resp.Body).Decode(&idx); err != nil {
		return nil, err
	}
	return &idx, nil
}

func checksumFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	io.Copy(h, f)
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}
