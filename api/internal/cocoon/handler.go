package cocoon

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// ════════════════════════════════════════════════════════════
// Cocoon v1 HTTP Handlers — /v1/cocoon/*
// ════════════════════════════════════════════════════════════

// HandleStats handles GET /cocoon/stats
func HandleStats(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"cocoon engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"stats":  e.Stats(),
		"config": e.Config(),
	})
}

// HandleParseSpec handles POST /cocoon/specs
func HandleParseSpec(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"cocoon engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		Name         string        `json:"name"`
		Version      string        `json:"version"`
		Type         PackageType   `json:"type"`
		Description  string        `json:"description"`
		Author       string        `json:"author"`
		EntryPoint   string        `json:"entry_point"`
		Dependencies []string      `json:"dependencies"`
		Tools        []string      `json:"tools"`
		Platforms    []BuildTarget `json:"platforms"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	spec := e.ParseSpec(req.Name, req.Version, req.Type, req.Description, req.Author, req.EntryPoint, req.Dependencies, req.Tools, req.Platforms)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(spec)
}

// HandleListSpecs handles GET /cocoon/specs
func HandleListSpecs(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"cocoon engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	specs := e.ListSpecs()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"specs": specs,
		"count": len(specs),
	})
}

// HandleGetSpec handles GET /cocoon/specs/detail?id=xxx
func HandleGetSpec(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"cocoon engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	specID := r.URL.Query().Get("id")
	if specID == "" {
		http.Error(w, `{"error":"id required"}`, http.StatusBadRequest)
		return
	}
	spec, err := e.GetSpec(specID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(spec)
}

// HandleStartBuild handles POST /cocoon/builds
func HandleStartBuild(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"cocoon engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		SpecID string      `json:"spec_id"`
		Target BuildTarget `json:"target"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.Target == "" {
		req.Target = TargetLinuxAMD64
	}
	build, err := e.StartBuild(req.SpecID, req.Target)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(build)
}

// HandleFinishBuild handles POST /cocoon/builds/finish
func HandleFinishBuild(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"cocoon engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		BuildID    string `json:"build_id"`
		Success    bool   `json:"success"`
		OutputPath string `json:"output_path"`
		Checksum   string `json:"checksum"`
		OutputSize int64  `json:"output_size"`
		Error      string `json:"error"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if err := e.FinishBuild(req.BuildID, req.Success, req.OutputPath, req.Checksum, req.OutputSize, req.Error); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "build_id": req.BuildID})
}

// HandleListBuilds handles GET /cocoon/builds?limit=20
func HandleListBuilds(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"cocoon engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	builds := e.ListBuilds(limit)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"builds": builds,
		"count":  len(builds),
	})
}

// HandlePublish handles POST /cocoon/publish
func HandlePublish(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"cocoon engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		BuildID string        `json:"build_id"`
		Target  PublishTarget `json:"target"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.Target == "" {
		req.Target = PublishNydus
	}
	pub, err := e.Publish(req.BuildID, req.Target)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(pub)
}

// HandleListPublishes handles GET /cocoon/publishes?limit=20
func HandleListPublishes(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"cocoon engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	pubs := e.ListPublishes(limit)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"publishes": pubs,
		"count":     len(pubs),
	})
}
