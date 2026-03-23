package handler

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"starclaw.net/nydus/api/internal/config"
)

const clawRepoName = "claw"

func clawBareRepoPath() string {
	return filepath.Join(config.C.Server.ReposDir, clawRepoName+".git")
}

// getLatestTagFromRepo reads the latest semver/version tag from a bare repo
func getLatestTagFromRepo(bareRepo string) (tag string, commitHash string, err error) {
	cmd := exec.Command("git", "--git-dir="+bareRepo, "tag", "-l", "v*", "--sort=-version:refname")
	out, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("git tag list: %w", err)
	}
	tags := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(tags) == 0 || tags[0] == "" {
		return "", "", fmt.Errorf("no version tags found")
	}
	tag = tags[0]

	cmd2 := exec.Command("git", "--git-dir="+bareRepo, "rev-parse", tag)
	hashOut, err := cmd2.Output()
	if err != nil {
		return tag, "", nil
	}
	commitHash = strings.TrimSpace(string(hashOut))
	return tag, commitHash, nil
}

// getTagMessage reads the annotated tag message (release notes)
func getTagMessage(bareRepo, tag string) string {
	cmd := exec.Command("git", "--git-dir="+bareRepo, "tag", "-l", tag, "--format=%(contents)")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// ListReleases returns all version tags from claw.git as releases.
func ListReleases(c *gin.Context) {
	bareRepo := clawBareRepoPath()
	if _, err := os.Stat(bareRepo); os.IsNotExist(err) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "claw.git not initialized yet"})
		return
	}

	cmd := exec.Command("git", "--git-dir="+bareRepo, "tag", "-l", "v*", "--sort=-version:refname")
	out, err := cmd.Output()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "failed to list tags"})
		return
	}

	tags := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(tags) == 0 || tags[0] == "" {
		c.JSON(200, gin.H{"releases": []interface{}{}})
		return
	}

	type releaseItem struct {
		TagName string `json:"tag_name"`
		Name    string `json:"name"`
		Body    string `json:"body"`
		Commit  string `json:"commit"`
		HTMLURL string `json:"html_url"`
		Latest  bool   `json:"latest"`
	}

	releases := make([]releaseItem, 0, len(tags))
	for i, tag := range tags {
		if tag == "" {
			continue
		}
		commit := ""
		cmd2 := exec.Command("git", "--git-dir="+bareRepo, "rev-parse", tag)
		if hashOut, err := cmd2.Output(); err == nil {
			commit = strings.TrimSpace(string(hashOut))
		}
		body := getTagMessage(bareRepo, tag)
		releases = append(releases, releaseItem{
			TagName: tag,
			Name:    "StarClaw " + tag,
			Body:    body,
			Commit:  commit,
			HTMLURL: fmt.Sprintf("https://github.com/yinhe/starclaw/releases/tag/%s", tag),
			Latest:  i == 0,
		})
	}

	c.JSON(200, gin.H{"releases": releases})
}

// GetLatestRelease returns release info from local claw.git (public, no auth).
func GetLatestRelease(c *gin.Context) {
	bareRepo := clawBareRepoPath()
	if _, err := os.Stat(bareRepo); os.IsNotExist(err) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "claw.git not initialized yet"})
		return
	}

	tag, commitHash, err := getLatestTagFromRepo(bareRepo)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": fmt.Sprintf("no releases: %v", err)})
		return
	}

	body := getTagMessage(bareRepo, tag)

	c.JSON(200, gin.H{
		"tag_name":   tag,
		"name":       "StarClaw " + tag,
		"body":       body,
		"html_url":   fmt.Sprintf("https://github.com/yinhe/starclaw/releases/tag/%s", tag),
		"commit":     commitHash,
		"source":     "nydus",
		"source_url": "/releases/source.tar.gz",
		"git_clone":  "git@nydus.starclaw.net:claw.git",
	})
}

// DownloadRelease serves a release asset file.
func DownloadRelease(c *gin.Context) {
	filename := c.Param("filename")
	filename = filepath.Base(filename)
	if strings.Contains(filename, "..") {
		c.JSON(400, gin.H{"error": "invalid filename"})
		return
	}

	localPath := filepath.Join(config.C.Server.ReposDir, "releases", filename)
	if _, err := os.Stat(localPath); os.IsNotExist(err) {
		c.JSON(404, gin.H{"error": "asset not found"})
		return
	}
	c.File(localPath)
}

// GetSporeLatest serves the latest Spore release info (spore-latest.json).
func GetSporeLatest(c *gin.Context) {
	localPath := filepath.Join(config.C.Server.ReposDir, "releases", "spore-latest.json")
	if _, err := os.Stat(localPath); os.IsNotExist(err) {
		c.JSON(404, gin.H{"error": "no spore releases yet"})
		return
	}
	data, err := os.ReadFile(localPath)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to read spore release info"})
		return
	}
	c.Data(200, "application/json; charset=utf-8", data)
}

// GetSourceTarball serves a tarball of the claw OSS repo.
func GetSourceTarball(c *gin.Context) {
	bareRepo := clawBareRepoPath()
	if _, err := os.Stat(bareRepo); os.IsNotExist(err) {
		c.JSON(404, gin.H{"error": "claw.git not found"})
		return
	}

	branch := "main"
	cmd := exec.Command("git", "--git-dir="+bareRepo, "rev-parse", "--verify", "refs/heads/main")
	if err := cmd.Run(); err != nil {
		branch = "master"
	}

	cmd = exec.Command("git", "--git-dir="+bareRepo, "archive", "--format=tar.gz", branch)
	out, err := cmd.Output()
	if err != nil {
		log.Printf("[releases] git archive failed: %v", err)
		c.JSON(500, gin.H{"error": "failed to create archive"})
		return
	}

	c.Header("Content-Type", "application/gzip")
	c.Header("Content-Disposition", "attachment; filename=claw-source.tar.gz")
	c.Data(200, "application/gzip", out)
}
