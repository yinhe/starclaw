package squad

import (
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// GitHTTPHandler implements the Git Smart HTTP protocol (v1).
// It enables remote Claw nodes to clone/fetch/push to mission repos
// via standard HTTP, supporting cross-network Git collaboration.
//
// Endpoints (relative to base path):
//
//	GET  /{repo}/info/refs?service=git-upload-pack    - fetch advertisement
//	GET  /{repo}/info/refs?service=git-receive-pack   - push advertisement
//	POST /{repo}/git-upload-pack                       - fetch pack data
//	POST /{repo}/git-receive-pack                      - push pack data
//	GET  /{repo}/HEAD                                  - HEAD reference
type GitHTTPHandler struct {
	reposDir string // base directory for bare repos
}

// NewGitHTTPHandler creates a new Git HTTP handler.
// reposDir should be the same directory used by GitManager for bare repos.
func NewGitHTTPHandler(reposDir string) *GitHTTPHandler {
	return &GitHTTPHandler{reposDir: reposDir}
}

// ServeHTTP routes Git Smart HTTP requests.
// The caller strips the prefix (e.g. /v1/git/) before calling this.
func (h *GitHTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")

	// Extract repo name from path: {repo-name}/rest-of-path
	parts := strings.SplitN(path, "/", 2)
	if len(parts) < 1 || parts[0] == "" {
		http.Error(w, "repository name required", http.StatusBadRequest)
		return
	}

	repoName := parts[0]
	subPath := ""
	if len(parts) > 1 {
		subPath = parts[1]
	}

	// Resolve repo directory (support both "mission-xxx" and "mission-xxx.git")
	repoDir := filepath.Join(h.reposDir, repoName)
	if !strings.HasSuffix(repoDir, ".git") {
		repoDir += ".git"
	}

	// Verify repo exists
	if _, err := os.Stat(repoDir); os.IsNotExist(err) {
		http.Error(w, "repository not found", http.StatusNotFound)
		return
	}

	log.Printf("[git-http] %s %s repo=%s sub=%s", r.Method, r.URL.String(), repoName, subPath)

	switch {
	case subPath == "info/refs":
		h.handleInfoRefs(w, r, repoDir)
	case subPath == "HEAD":
		h.handleHEAD(w, r, repoDir)
	case subPath == "git-upload-pack" && r.Method == "POST":
		h.handleServiceRPC(w, r, repoDir, "git-upload-pack")
	case subPath == "git-receive-pack" && r.Method == "POST":
		h.handleServiceRPC(w, r, repoDir, "git-receive-pack")
	default:
		http.Error(w, "not found", http.StatusNotFound)
	}
}

// handleInfoRefs implements GET /info/refs?service=git-upload-pack|git-receive-pack
// This is the "advertisement" step of the Smart HTTP protocol.
func (h *GitHTTPHandler) handleInfoRefs(w http.ResponseWriter, r *http.Request, repoDir string) {
	service := r.URL.Query().Get("service")
	if service != "git-upload-pack" && service != "git-receive-pack" {
		// Dumb HTTP protocol fallback
		h.serveStaticFile(w, filepath.Join(repoDir, "info", "refs"))
		return
	}

	// Run git service with --advertise-refs (stateless-rpc mode)
	cmd := hiddenCmd("git", service, "--stateless-rpc", "--advertise-refs", repoDir)
	cmd.Env = os.Environ()

	out, err := cmd.Output()
	if err != nil {
		log.Printf("[git-http] info/refs error: %v", err)
		http.Error(w, "git service error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", fmt.Sprintf("application/x-%s-advertisement", service))
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)

	// Smart HTTP protocol requires a pkt-line header
	pktLine := fmt.Sprintf("# service=%s\n", service)
	pktLineHeader := fmt.Sprintf("%04x%s", len(pktLine)+4, pktLine)
	w.Write([]byte(pktLineHeader))
	w.Write([]byte("0000")) // flush-pkt
	w.Write(out)
}

// handleServiceRPC implements POST /git-upload-pack and POST /git-receive-pack
// This is the actual data transfer step of the Smart HTTP protocol.
func (h *GitHTTPHandler) handleServiceRPC(w http.ResponseWriter, r *http.Request, repoDir, service string) {
	w.Header().Set("Content-Type", fmt.Sprintf("application/x-%s-result", service))
	w.Header().Set("Cache-Control", "no-cache")

	// Handle gzip-encoded request body
	var body io.Reader = r.Body
	if r.Header.Get("Content-Encoding") == "gzip" {
		gz, err := gzip.NewReader(r.Body)
		if err != nil {
			http.Error(w, "failed to decompress request", http.StatusBadRequest)
			return
		}
		defer gz.Close()
		body = gz
	}

	cmd := hiddenCmd("git", service, "--stateless-rpc", repoDir)
	cmd.Env = os.Environ()
	cmd.Stdin = body
	cmd.Stdout = w
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		log.Printf("[git-http] %s rpc error: %v", service, err)
	}
}

// handleHEAD serves the HEAD file from a bare repo.
func (h *GitHTTPHandler) handleHEAD(w http.ResponseWriter, r *http.Request, repoDir string) {
	h.serveStaticFile(w, filepath.Join(repoDir, "HEAD"))
}

// serveStaticFile serves a file from the repo directory.
func (h *GitHTTPHandler) serveStaticFile(w http.ResponseWriter, filePath string) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Cache-Control", "no-cache")
	w.Write(data)
}

// EnableReceivePack enables git receive-pack (push) on a bare repo.
// Required for the Smart HTTP push protocol.
func EnableReceivePack(repoDir string) error {
	cmd := hiddenCmd("git", "config", "--bool", "http.receivepack", "true")
	cmd.Dir = repoDir
	cmd.Env = os.Environ()
	return cmd.Run()
}

// UpdateServerInfo runs git update-server-info on a bare repo
// to ensure it can be served via dumb HTTP protocol as a fallback.
func UpdateServerInfo(repoDir string) error {
	cmd := hiddenCmd("git", "update-server-info")
	cmd.Dir = repoDir
	cmd.Env = os.Environ()
	return cmd.Run()
}
