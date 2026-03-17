package api

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/yinhe/starclaw-spore/pkg/manifest"
	"github.com/yinhe/starclaw-spore/pkg/platform"
	"github.com/yinhe/starclaw-spore/pkg/runtime"
)

// Server is the Desktop API backend.
type Server struct {
	mgr     *runtime.Manager
	mux     *http.ServeMux
	webFS   fs.FS // embedded frontend
	logSubs map[string][]chan string
	logMu   sync.Mutex
}

// NewServer creates a new Desktop API server.
func NewServer(mgr *runtime.Manager, webFS fs.FS) *Server {
	s := &Server{
		mgr:     mgr,
		mux:     http.NewServeMux(),
		webFS:   webFS,
		logSubs: make(map[string][]chan string),
	}
	s.registerRoutes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) registerRoutes() {
	// API routes
	s.mux.HandleFunc("/api/instances", s.corsWrap(s.handleInstances))
	s.mux.HandleFunc("/api/instances/", s.corsWrap(s.handleInstanceAction))
	s.mux.HandleFunc("/api/platform", s.corsWrap(s.handlePlatform))
	s.mux.HandleFunc("/api/install", s.corsWrap(s.handleInstall))
	s.mux.HandleFunc("/api/logs/", s.handleLogStream) // SSE, no CORS wrap needed

	// Serve embedded frontend (SPA)
	if s.webFS != nil {
		fileServer := http.FileServer(http.FS(s.webFS))
		s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			// Try exact file first
			path := strings.TrimPrefix(r.URL.Path, "/")
			if path == "" {
				path = "index.html"
			}
			if f, err := s.webFS.Open(path); err == nil {
				f.Close()
				fileServer.ServeHTTP(w, r)
				return
			}
			// SPA fallback
			r.URL.Path = "/"
			fileServer.ServeHTTP(w, r)
		})
	}
}

func (s *Server) corsWrap(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}
		handler(w, r)
	}
}

func jsonResp(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func jsonErr(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// ── Instance List ──

type InstanceInfo struct {
	Name      string            `json:"name"`
	Version   string            `json:"version"`
	Status    string            `json:"status"`
	Location  string            `json:"location"`
	DataDir   string            `json:"data_dir"`
	LogDir    string            `json:"log_dir"`
	Ports     []int             `json:"ports"`
	Health    string            `json:"health"`
	HealthURL string            `json:"health_url"`
	Env       map[string]string `json:"env"`
	Binary    string            `json:"binary"`
}

func (s *Server) handleInstances(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		jsonErr(w, 405, "method not allowed")
		return
	}

	instances, err := s.mgr.List()
	if err != nil {
		jsonErr(w, 500, err.Error())
		return
	}

	var result []InstanceInfo
	for _, inst := range instances {
		info := InstanceInfo{
			Name:     inst.Name,
			Version:  inst.Version,
			Status:   inst.Status,
			Location: inst.InstallDir,
			DataDir:  inst.DataDir,
			LogDir:   inst.LogDir,
			Binary:   inst.Manifest.Binary,
			Env:      inst.Manifest.Env,
		}

		for _, p := range inst.Manifest.Network.Ports {
			info.Ports = append(info.Ports, p.Port)
		}

		info.HealthURL = inst.Manifest.Health.Endpoint
		if inst.Status == "running" && info.HealthURL != "" {
			ok, _ := s.mgr.HealthCheck(inst.Name)
			if ok {
				info.Health = "healthy"
			} else {
				info.Health = "unhealthy"
			}
		} else if inst.Status == "running" {
			info.Health = "unknown"
		} else {
			info.Health = ""
		}

		result = append(result, info)
	}

	if result == nil {
		result = []InstanceInfo{}
	}
	jsonResp(w, result)
}

// ── Instance Actions ──

func (s *Server) handleInstanceAction(w http.ResponseWriter, r *http.Request) {
	// /api/instances/{name}/{action}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/instances/"), "/")
	if len(parts) < 1 || parts[0] == "" {
		jsonErr(w, 400, "missing instance name")
		return
	}

	name := parts[0]
	action := ""
	if len(parts) >= 2 {
		action = parts[1]
	}

	switch {
	case r.Method == "GET" && action == "":
		s.handleInstanceInfo(w, name)
	case r.Method == "POST" && action == "start":
		s.handleStart(w, r, name)
	case r.Method == "POST" && action == "stop":
		s.handleStop(w, name)
	case r.Method == "POST" && action == "restart":
		s.handleRestart(w, r, name)
	case r.Method == "DELETE" && action == "":
		s.handleUninstall(w, name)
	case r.Method == "PUT" && action == "env":
		s.handleUpdateEnv(w, r, name)
	default:
		jsonErr(w, 404, "unknown action: "+action)
	}
}

func (s *Server) handleInstanceInfo(w http.ResponseWriter, name string) {
	inst, err := s.mgr.Get(name)
	if err != nil {
		jsonErr(w, 404, err.Error())
		return
	}

	info := InstanceInfo{
		Name:     inst.Name,
		Version:  inst.Version,
		Status:   inst.Status,
		Location: inst.InstallDir,
		DataDir:  inst.DataDir,
		LogDir:   inst.LogDir,
		Binary:   inst.Manifest.Binary,
		Env:      inst.Manifest.Env,
	}
	for _, p := range inst.Manifest.Network.Ports {
		info.Ports = append(info.Ports, p.Port)
	}
	info.HealthURL = inst.Manifest.Health.Endpoint
	if inst.Status == "running" && info.HealthURL != "" {
		ok, _ := s.mgr.HealthCheck(inst.Name)
		if ok {
			info.Health = "healthy"
		} else {
			info.Health = "unhealthy"
		}
	}
	jsonResp(w, info)
}

func (s *Server) handleStart(w http.ResponseWriter, r *http.Request, name string) {
	var body struct {
		Env []string `json:"env"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	if err := s.mgr.StartWithEnv(name, body.Env); err != nil {
		jsonErr(w, 500, err.Error())
		return
	}
	jsonResp(w, map[string]string{"status": "started"})
}

func (s *Server) handleStop(w http.ResponseWriter, name string) {
	if err := s.mgr.Stop(name); err != nil {
		jsonErr(w, 500, err.Error())
		return
	}
	jsonResp(w, map[string]string{"status": "stopped"})
}

func (s *Server) handleRestart(w http.ResponseWriter, r *http.Request, name string) {
	_ = s.mgr.Stop(name)
	time.Sleep(2 * time.Second)

	var body struct {
		Env []string `json:"env"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	if err := s.mgr.StartWithEnv(name, body.Env); err != nil {
		jsonErr(w, 500, err.Error())
		return
	}
	jsonResp(w, map[string]string{"status": "restarted"})
}

func (s *Server) handleUninstall(w http.ResponseWriter, name string) {
	if err := s.mgr.Uninstall(name); err != nil {
		jsonErr(w, 500, err.Error())
		return
	}
	jsonResp(w, map[string]string{"status": "uninstalled"})
}

func (s *Server) handleUpdateEnv(w http.ResponseWriter, r *http.Request, name string) {
	inst, err := s.mgr.Get(name)
	if err != nil {
		jsonErr(w, 404, err.Error())
		return
	}

	var body struct {
		Env map[string]string `json:"env"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonErr(w, 400, "invalid JSON")
		return
	}

	// Update manifest env
	manifestPath := filepath.Join(inst.InstallDir, "manifest.json")
	mf, err := manifest.Load(manifestPath)
	if err != nil {
		jsonErr(w, 500, "load manifest: "+err.Error())
		return
	}

	if mf.Env == nil {
		mf.Env = make(map[string]string)
	}
	for k, v := range body.Env {
		mf.Env[k] = v
	}

	if err := mf.Save(manifestPath); err != nil {
		jsonErr(w, 500, "save manifest: "+err.Error())
		return
	}

	jsonResp(w, map[string]string{"status": "updated"})
}

// ── Install ──

func (s *Server) handleInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		jsonErr(w, 405, "method not allowed")
		return
	}

	var body struct {
		Path string `json:"path"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonErr(w, 400, "invalid JSON")
		return
	}

	if body.Path == "" {
		jsonErr(w, 400, "path is required")
		return
	}

	inst, err := s.mgr.InstallFromDir(body.Path, body.Name)
	if err != nil {
		jsonErr(w, 500, err.Error())
		return
	}

	jsonResp(w, map[string]string{
		"name":     inst.Name,
		"version":  inst.Version,
		"location": inst.InstallDir,
	})
}

// ── Platform ──

func (s *Server) handlePlatform(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		jsonErr(w, 405, "method not allowed")
		return
	}
	p := platform.Detect()
	jsonResp(w, map[string]interface{}{
		"os":         p.OS,
		"arch":       p.Arch,
		"kernel":     p.Kernel,
		"hostname":   p.Hostname,
		"init":       p.InitSystem,
		"spore_home": p.SporeHome,
		"version":    "0.1.0",
	})
}

// ── Log Streaming (SSE) ──

func (s *Server) handleLogStream(w http.ResponseWriter, r *http.Request) {
	// /api/logs/{name}
	name := strings.TrimPrefix(r.URL.Path, "/api/logs/")
	if name == "" {
		jsonErr(w, 400, "missing instance name")
		return
	}

	inst, err := s.mgr.Get(name)
	if err != nil {
		jsonErr(w, 404, err.Error())
		return
	}

	logPath := filepath.Join(inst.LogDir, name+".log")

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		jsonErr(w, 500, "streaming not supported")
		return
	}

	// Send last 100 lines of existing log
	if lines, err := tailFile(logPath, 100); err == nil {
		for _, line := range lines {
			fmt.Fprintf(w, "data: %s\n\n", line)
		}
		flusher.Flush()
	}

	// Watch for new lines
	ctx := r.Context()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	var lastSize int64
	if fi, err := os.Stat(logPath); err == nil {
		lastSize = fi.Size()
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			fi, err := os.Stat(logPath)
			if err != nil {
				continue
			}
			if fi.Size() <= lastSize {
				continue
			}

			f, err := os.Open(logPath)
			if err != nil {
				continue
			}
			f.Seek(lastSize, 0)
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				fmt.Fprintf(w, "data: %s\n\n", scanner.Text())
			}
			flusher.Flush()
			lastSize = fi.Size()
			f.Close()
		}
	}
}

func tailFile(path string, n int) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	buf := make([]byte, 0, 1024*1024)
	scanner.Buffer(buf, 1024*1024)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines, nil
}

// ListenAndServe starts the Desktop API server.
func (s *Server) ListenAndServe(addr string) error {
	log.Printf("[spore-desktop] Starting on %s", addr)
	return http.ListenAndServe(addr, s)
}
