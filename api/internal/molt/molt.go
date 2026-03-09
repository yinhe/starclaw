package molt

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

const (
	Version    = "0.5.7"
	owner      = "yinhe"
	repo       = "starclaw"
	checkEvery = 1 * time.Hour
)

type ReleaseInfo struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	Body        string `json:"body"`
	HTMLURL     string `json:"html_url"`
	PublishedAt string `json:"published_at"`
}

type VersionInfo struct {
	Current       string    `json:"current"`
	Latest        string    `json:"latest"`
	LatestURL     string    `json:"latest_url"`
	UpdateAvail   bool      `json:"update_available"`
	ReleaseNotes  string    `json:"release_notes,omitempty"`
	LastCheckedAt time.Time `json:"last_checked_at"`
}

var (
	mu          sync.RWMutex
	latestInfo  *ReleaseInfo
	lastChecked time.Time
)

// StartChecker starts a background goroutine that periodically checks for new releases
func StartChecker() {
	go func() {
		// Check immediately on startup
		check()
		ticker := time.NewTicker(checkEvery)
		defer ticker.Stop()
		for range ticker.C {
			check()
		}
	}()
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
	if latest != Version {
		log.Printf("[molt] new version available: %s → %s (%s)", Version, latest, info.HTMLURL)
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
		vi.UpdateAvail = latest != Version
	}

	return vi
}

func fetchLatestRelease() (*ReleaseInfo, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", owner, repo)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "StarClaw/"+Version)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
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
