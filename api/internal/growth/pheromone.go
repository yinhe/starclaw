package growth

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"
)

// pheromonePublisher publishes growth events to Pheromone ESB via HTTP.
var pheromoneClient = &http.Client{Timeout: 3 * time.Second}

// PublishGrowthEvent sends a growth event to Pheromone (fire-and-forget).
func PublishGrowthEvent(subject string, data interface{}) {
	apiURL := os.Getenv("PHEROMONE_API_URL")
	if apiURL == "" {
		return
	}
	go func() {
		payload, err := json.Marshal(data)
		if err != nil {
			return
		}
		body, _ := json.Marshal(map[string]interface{}{
			"subject": "pheromone.events." + subject,
			"payload": json.RawMessage(payload),
		})
		req, err := http.NewRequest("POST", apiURL+"/api/events", bytes.NewReader(body))
		if err != nil {
			return
		}
		req.Header.Set("Content-Type", "application/json")
		if token := os.Getenv("PHEROMONE_TOKEN"); token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := pheromoneClient.Do(req)
		if err != nil {
			log.Printf("[growth] pheromone publish %s failed: %v", subject, err)
			return
		}
		resp.Body.Close()
	}()
}

// LevelUpEvent is published when a Claw node levels up.
type LevelUpEvent struct {
	ClawID   string `json:"claw_id"`
	UserID   string `json:"user_id"`
	OldLevel int    `json:"old_level"`
	NewLevel int    `json:"new_level"`
	Path     string `json:"evolution_path"`
	FormCode string `json:"form_code"`
	Title    string `json:"title"`
}

// EvolutionEvent is published when a Claw node changes evolution path.
type EvolutionEvent struct {
	ClawID  string `json:"claw_id"`
	UserID  string `json:"user_id"`
	OldPath string `json:"old_path"`
	NewPath string `json:"new_path"`
	Level   int    `json:"level"`
}

// MilestoneEvent is published when a new milestone is achieved.
type MilestoneEvent struct {
	ClawID string `json:"claw_id"`
	UserID string `json:"user_id"`
	Code   string `json:"code"`
	Title  string `json:"title"`
}
