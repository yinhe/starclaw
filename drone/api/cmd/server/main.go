package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	pheromone "starclaw.net/pheromone/sdk"
)

// HarvestRecord tracks a single harvest run
type HarvestRecord struct {
	ID        string    `json:"id"`
	Source    string    `json:"source"`
	Status    string    `json:"status"` // running, completed, failed
	Collected int       `json:"collected"`
	Morphed   int       `json:"morphed"`
	Imported  int       `json:"imported"`
	Duration  string    `json:"duration"`
	Error     string    `json:"error,omitempty"`
	StartedAt time.Time `json:"started_at"`
}

var (
	mu       sync.Mutex
	records  []HarvestRecord
	running  map[string]bool
	ph       *pheromone.Client
)

func init() {
	running = make(map[string]bool)
}

func main() {
	port := envOr("DRONE_PORT", "8110")

	// Connect to Pheromone ESB
	natsURL := envOr("PHEROMONE_NATS_URL", "nats://pheromone-nats:4222")
	var phErr error
	ph, phErr = pheromone.New(natsURL, pheromone.ServiceInfo{
		Name:    "drone",
		Version: "1.0.0",
		Port:    8110,
		Tags:    []string{"harvester", "cocoon", "marketplace"},
	})
	if phErr != nil {
		log.Printf("[drone] pheromone connect failed (non-fatal): %v", phErr)
	} else {
		ph.StartHeartbeat(30 * time.Second)
		defer ph.Close()
		log.Printf("[drone] pheromone ESB connected (%s)", natsURL)
	}

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	r.Use(cors.New(cors.Config{
		AllowAllOrigins: true,
		AllowMethods:    []string{"GET", "POST", "OPTIONS"},
		AllowHeaders:    []string{"Origin", "Content-Type", "Authorization"},
	}))

	// Health
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"service": "drone-api",
			"status":  "ok",
			"sources": []string{"clawhub", "skillhub", "awesome_gpts", "coze", "dify", "gpts_store"},
		})
	})

	// Trigger harvest for a specific source
	r.POST("/harvest/:source", triggerHarvest)

	// Trigger all sources
	r.POST("/harvest", triggerAll)

	// List harvest records
	r.GET("/records", listRecords)

	// Stats
	r.GET("/stats", getStats)

	log.Printf("[drone] 🐝 Drone API starting on :%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("[drone] server failed: %v", err)
	}
}

func triggerHarvest(c *gin.Context) {
	source := c.Param("source")

	mu.Lock()
	if running[source] {
		mu.Unlock()
		c.JSON(http.StatusConflict, gin.H{"error": fmt.Sprintf("%s already running", source)})
		return
	}
	running[source] = true
	mu.Unlock()

	go runHarvest(source)

	c.JSON(http.StatusAccepted, gin.H{
		"message": fmt.Sprintf("harvest started: %s", source),
		"source":  source,
	})
}

func triggerAll(c *gin.Context) {
	sources := []string{"clawhub", "skillhub", "awesome_gpts", "coze", "dify"}
	started := []string{}

	for _, s := range sources {
		mu.Lock()
		if running[s] {
			mu.Unlock()
			continue
		}
		running[s] = true
		mu.Unlock()
		go runHarvest(s)
		started = append(started, s)
	}

	c.JSON(http.StatusAccepted, gin.H{
		"message": fmt.Sprintf("harvest started: %d sources", len(started)),
		"sources": started,
	})
}

func runHarvest(source string) {
	start := time.Now()
	rec := HarvestRecord{
		ID:        fmt.Sprintf("%s_%s", source, start.Format("20060102_150405")),
		Source:    source,
		Status:    "running",
		StartedAt: start,
	}

	// Publish start event
	publishEvent("drone.harvest.started", map[string]interface{}{
		"source": source, "started_at": start.Format(time.RFC3339),
	})

	defer func() {
		mu.Lock()
		running[source] = false
		records = append(records, rec)
		if len(records) > 100 {
			records = records[len(records)-100:]
		}
		mu.Unlock()
	}()

	// Run Python worker for this source
	cmd := exec.Command("python3", "-m", "scripts.worker", source)
	cmd.Dir = envOr("DRONE_WORKER_DIR", "/app")
	cmd.Env = append(os.Environ(),
		"DRONE_DATA_DIR="+envOr("DRONE_DATA_DIR", "/data/harvest"),
		"DRONE_CLAW_API="+envOr("DRONE_CLAW_API", "http://localhost:8080"),
		"DRONE_STARAI_API="+envOr("DRONE_STARAI_API", ""),
		"DRONE_STARAI_KEY="+envOr("DRONE_STARAI_KEY", ""),
	)

	out, err := cmd.CombinedOutput()
	rec.Duration = time.Since(start).Round(time.Second).String()

	if err != nil {
		rec.Status = "failed"
		rec.Error = fmt.Sprintf("%v: %s", err, lastLines(string(out), 5))
		log.Printf("[drone] ❌ harvest %s failed: %s", source, rec.Error)

		publishEvent("drone.harvest.failed", map[string]interface{}{
			"source": source, "error": rec.Error, "duration": rec.Duration,
		})
		return
	}

	// Parse output for stats
	rec.Status = "completed"
	outStr := string(out)
	rec.Collected = extractNumber(outStr, "collected")
	rec.Morphed = extractNumber(outStr, "morphed")
	rec.Imported = extractNumber(outStr, "imported")

	log.Printf("[drone] ✅ harvest %s: collected=%d morphed=%d imported=%d duration=%s",
		source, rec.Collected, rec.Morphed, rec.Imported, rec.Duration)

	publishEvent("drone.harvest.completed", map[string]interface{}{
		"source": source, "collected": rec.Collected, "morphed": rec.Morphed,
		"imported": rec.Imported, "duration": rec.Duration,
	})
}

func listRecords(c *gin.Context) {
	mu.Lock()
	defer mu.Unlock()

	// Return in reverse order (newest first)
	result := make([]HarvestRecord, len(records))
	for i, r := range records {
		result[len(records)-1-i] = r
	}
	c.JSON(200, gin.H{"records": result, "total": len(result)})
}

func getStats(c *gin.Context) {
	mu.Lock()
	defer mu.Unlock()

	totalCollected, totalMorphed, totalImported := 0, 0, 0
	completed, failed := 0, 0
	for _, r := range records {
		if r.Status == "completed" {
			completed++
			totalCollected += r.Collected
			totalMorphed += r.Morphed
			totalImported += r.Imported
		} else if r.Status == "failed" {
			failed++
		}
	}

	runningList := []string{}
	for s, r := range running {
		if r {
			runningList = append(runningList, s)
		}
	}

	c.JSON(200, gin.H{
		"total_runs":      len(records),
		"completed":       completed,
		"failed":          failed,
		"running":         runningList,
		"total_collected": totalCollected,
		"total_morphed":   totalMorphed,
		"total_imported":  totalImported,
	})
}

func publishEvent(subject string, data map[string]interface{}) {
	if ph == nil {
		return
	}
	payload, _ := json.Marshal(data)
	if err := ph.Publish("pheromone.events."+subject, payload); err != nil {
		log.Printf("[drone] pheromone publish failed: %v", err)
	}
}

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) <= n {
		return strings.TrimSpace(s)
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}

func extractNumber(output, keyword string) int {
	// Simple parser: find "N keyword" or "keyword=N" patterns
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, keyword) {
			// Try "keyword=N" format
			parts := strings.Split(line, keyword+"=")
			if len(parts) >= 2 {
				n := 0
				fmt.Sscanf(strings.TrimSpace(parts[1]), "%d", &n)
				if n > 0 {
					return n
				}
			}
			// Try "N keyword" format
			words := strings.Fields(line)
			for i, w := range words {
				if strings.Contains(w, keyword) && i > 0 {
					n := 0
					fmt.Sscanf(words[i-1], "%d", &n)
					if n > 0 {
						return n
					}
				}
			}
		}
	}
	return 0
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); strings.TrimSpace(v) != "" {
		return v
	}
	return fallback
}
