package molt

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Version is set at build time via:
//
//	go build -ldflags "-X github.com/yinhe/starclaw/internal/molt.Version=2026.0310.1214"
//
// Format: YYYY.MMDD.HHmm (UTC)
var Version = "dev"

const (
	owner      = "yinhe"
	repo       = "starclaw"
	checkEvery = 1 * time.Hour
)

// Update sources: GitHub (primary) → Nydus mirror (fallback for China / offline)
var UpdateSources = []UpdateSource{
	{Name: "github", ReleaseURL: "https://api.github.com/repos/yinhe/starclaw/releases/latest", Timeout: 8 * time.Second},
	{Name: "nydus", ReleaseURL: "https://nydus.starclaw.net/releases/latest", Timeout: 5 * time.Second},
}

// SourceURLs are tarball URLs for source-based updates, tried in order.
// GitHub archive is always available; Nydus is a China-network mirror.
var SourceURLs = []string{
	"https://github.com/yinhe/starclaw/archive/refs/heads/main.tar.gz",
	"https://nydus.starclaw.net/releases/source.tar.gz",
}

type UpdateSource struct {
	Name       string
	ReleaseURL string
	Timeout    time.Duration
}

type ReleaseInfo struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	Body        string `json:"body"`
	HTMLURL     string `json:"html_url"`
	PublishedAt string `json:"published_at"`
	Source      string `json:"source,omitempty"`
}

type VersionInfo struct {
	Current       string    `json:"current"`
	Latest        string    `json:"latest"`
	LatestURL     string    `json:"latest_url"`
	UpdateAvail   bool      `json:"update_available"`
	ReleaseNotes  string    `json:"release_notes,omitempty"`
	LastCheckedAt time.Time `json:"last_checked_at"`
	Source        string    `json:"source,omitempty"`
}

var (
	mu          sync.RWMutex
	latestInfo  *ReleaseInfo
	lastChecked time.Time

	// Hive integration: when set, Molt notifies Hive Controller on new version
	hiveURL      string
	hiveNotified string // version we already notified about (debounce)
)

// StartChecker starts a background goroutine that periodically checks for new releases.
// Adds random jitter (0-5 min) before first check to avoid thundering herd
// when many Hive containers start simultaneously.
func StartChecker() {
	go func() {
		// Random jitter before first check: 0-5 minutes
		jitter := time.Duration(rand.Intn(300)) * time.Second
		if jitter > 0 {
			time.Sleep(jitter)
		}
		check()
		ticker := time.NewTicker(checkEvery)
		defer ticker.Stop()
		for range ticker.C {
			check()
		}
	}()
}

// SetHiveURL configures Molt to notify a Hive Controller when a new version is detected.
// Called from main.go if hive.url is configured (i.e., this Claw runs inside Hive).
func SetHiveURL(url string) {
	mu.Lock()
	hiveURL = url
	mu.Unlock()
	if url != "" {
		log.Printf("[molt] hive notification enabled → %s", url)
	}
}

// ForceCheck triggers an immediate version check
func ForceCheck() {
	check()
}

func check() {
	info, err := fetchLatestRelease()
	if err != nil {
		log.Printf("[molt] failed to check for updates: %v", err)
		return
	}

	mu.Lock()
	latestInfo = info
	lastChecked = time.Now()
	mu.Unlock()

	latest := trimV(info.TagName)
	if latest > Version {
		log.Printf("[molt] new version available: %s → %s (via %s)", Version, latest, info.Source)
		notifyHive(latest, info.Source)
	}
}

// notifyHive sends an upgrade notification to the Hive Controller (if configured).
// Debounces: only notifies once per version.
func notifyHive(latestVersion, source string) {
	mu.RLock()
	url := hiveURL
	alreadyNotified := hiveNotified
	mu.RUnlock()

	if url == "" || latestVersion == alreadyNotified {
		return
	}

	body := fmt.Sprintf(`{"current_version":"%s","latest_version":"%s","source":"%s"}`,
		Version, latestVersion, source)

	resp, err := http.Post(url+"/hive/upgrade-notify", "application/json",
		strings.NewReader(body))
	if err != nil {
		log.Printf("[molt] hive notify failed: %v", err)
		return
	}
	resp.Body.Close()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusAccepted {
		mu.Lock()
		hiveNotified = latestVersion
		mu.Unlock()
		log.Printf("[molt] notified hive: upgrade %s → %s", Version, latestVersion)
	} else {
		log.Printf("[molt] hive notify returned %d", resp.StatusCode)
	}
}

// GetVersionInfo returns current and latest version information
func GetVersionInfo() VersionInfo {
	mu.RLock()
	defer mu.RUnlock()

	vi := VersionInfo{
		Current:       Version,
		Latest:        Version,
		UpdateAvail:   false,
		LastCheckedAt: lastChecked,
	}

	if latestInfo != nil {
		latest := trimV(latestInfo.TagName)
		vi.Latest = latest
		vi.LatestURL = latestInfo.HTMLURL
		vi.ReleaseNotes = latestInfo.Body
		vi.UpdateAvail = latest > Version
		vi.Source = latestInfo.Source
	}

	return vi
}

// fetchLatestRelease checks ALL update sources and returns the one with the highest version.
// This ensures a newer release on Nydus is not missed just because GitHub returned an older one.
func fetchLatestRelease() (*ReleaseInfo, error) {
	var best *ReleaseInfo
	var lastErr error
	for _, src := range UpdateSources {
		info, err := fetchFromSource(src)
		if err != nil {
			log.Printf("[molt] %s check failed: %v", src.Name, err)
			lastErr = err
			continue
		}
		info.Source = src.Name
		if best == nil || trimV(info.TagName) > trimV(best.TagName) {
			best = info
		}
	}
	if best != nil {
		return best, nil
	}
	return nil, fmt.Errorf("all update sources failed, last: %v", lastErr)
}

func fetchFromSource(src UpdateSource) (*ReleaseInfo, error) {
	req, _ := http.NewRequest("GET", src.ReleaseURL, nil)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "StarClaw/"+Version)

	client := &http.Client{Timeout: src.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned status %d", src.Name, resp.StatusCode)
	}

	var info ReleaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}
	return &info, nil
}

func trimV(tag string) string {
	if len(tag) > 0 && tag[0] == 'v' {
		return tag[1:]
	}
	return tag
}
