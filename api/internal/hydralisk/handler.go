package hydralisk

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
)

// ════════════════════════════════════════════════════════════
// Hydralisk HTTP Handlers — exposed via Claw /v1/hydralisk/*
// ════════════════════════════════════════════════════════════

var (
	globalWorker *Worker
	workerOnce   sync.Once
)

// defaultExecutor is a placeholder executor that logs the job.
// In production, this should be replaced with real execution logic
// (e.g., calling agent runtime, running data pipelines, etc.)
func defaultExecutor(ctx context.Context, job *Job, progress func(int, string)) (string, error) {
	log.Printf("[hydralisk] executing job %s (type=%s)", job.ID[:8], job.Type)

	if job.Type == JobTypePipeline && len(job.Stages) > 0 {
		for i, stage := range job.Stages {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			default:
			}
			stage.Status = JobRunning
			pct := (i + 1) * 100 / len(job.Stages)
			progress(pct, stage.Name)
			stage.Status = JobCompleted
		}
		return `{"pipeline":"completed","stages":` + string(rune(len(job.Stages)+'0')) + `}`, nil
	}

	// For non-pipeline jobs, just mark as done
	progress(100, "done")
	return `{"status":"completed"}`, nil
}

// InitWorker initializes the global Hydralisk worker
func InitWorker(nodeID string, cfg *WorkerConfig) *Worker {
	workerOnce.Do(func() {
		globalWorker = NewWorker(cfg, nodeID, defaultExecutor)
	})
	return globalWorker
}

// GetWorker returns the global worker
func GetWorker() *Worker {
	return globalWorker
}

// SetExecutor replaces the default job executor with a custom one
func SetExecutor(exec JobExecutor) {
	if globalWorker != nil {
		globalWorker.executor = exec
	}
}

// HandleStats handles GET /v1/hydralisk/stats
func HandleStats(w http.ResponseWriter, r *http.Request) {
	worker := GetWorker()
	if worker == nil {
		http.Error(w, `{"error":"hydralisk worker not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(worker.Stats())
}

// HandleSubmit handles POST /v1/hydralisk/jobs
func HandleSubmit(w http.ResponseWriter, r *http.Request) {
	worker := GetWorker()
	if worker == nil {
		http.Error(w, `{"error":"hydralisk worker not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	var job Job
	if err := json.NewDecoder(r.Body).Decode(&job); err != nil {
		http.Error(w, `{"error":"invalid request: `+err.Error()+`"}`, http.StatusBadRequest)
		return
	}
	if job.Name == "" {
		http.Error(w, `{"error":"job name required"}`, http.StatusBadRequest)
		return
	}
	if job.Type == "" {
		job.Type = JobTypeBatch
	}

	result, err := worker.Submit(&job)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// HandleGetJob handles GET /v1/hydralisk/jobs?id=...
func HandleGetJob(w http.ResponseWriter, r *http.Request) {
	worker := GetWorker()
	if worker == nil {
		http.Error(w, `{"error":"hydralisk worker not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	jobID := r.URL.Query().Get("id")
	if jobID != "" {
		job, ok := worker.GetJob(jobID)
		if !ok {
			http.Error(w, `{"error":"job not found"}`, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(job)
		return
	}

	// List jobs
	status := JobStatus(r.URL.Query().Get("status"))
	jobType := JobType(r.URL.Query().Get("type"))
	jobs := worker.ListJobs(status, jobType, 50)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"jobs": jobs, "count": len(jobs)})
}

// HandleCancel handles POST /v1/hydralisk/jobs/cancel
func HandleCancel(w http.ResponseWriter, r *http.Request) {
	worker := GetWorker()
	if worker == nil {
		http.Error(w, `{"error":"hydralisk worker not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	var req struct {
		JobID string `json:"job_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}
	if err := worker.Cancel(req.JobID); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
}

// HandleGPU handles GET /v1/hydralisk/gpu
func HandleGPU(w http.ResponseWriter, r *http.Request) {
	worker := GetWorker()
	if worker == nil {
		http.Error(w, `{"error":"hydralisk worker not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	worker.mu.RLock()
	gpu := worker.gpu
	worker.mu.RUnlock()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(gpu)
}

// HandleSetGPU handles POST /v1/hydralisk/gpu
func HandleSetGPU(w http.ResponseWriter, r *http.Request) {
	worker := GetWorker()
	if worker == nil {
		http.Error(w, `{"error":"hydralisk worker not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var info GPUInfo
	if err := json.NewDecoder(r.Body).Decode(&info); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}
	worker.SetGPU(info)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
}
