package engine

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"
)

// PheromonePublisher publishes events to the Pheromone ESB via HTTP API.
// Lightweight — no NATS dependency needed.
type PheromonePublisher struct {
	apiURL string
	token  string
	client *http.Client
}

// NewPheromonePublisher creates a publisher from env vars.
// Returns nil if PHEROMONE_API_URL is not set (events silently skipped).
func NewPheromonePublisher() *PheromonePublisher {
	url := os.Getenv("PHEROMONE_API_URL")
	if url == "" {
		return nil
	}
	return &PheromonePublisher{
		apiURL: url,
		token:  os.Getenv("PHEROMONE_TOKEN"),
		client: &http.Client{Timeout: 3 * time.Second},
	}
}

// Publish sends an event to pheromone.events.{subject}.
func (p *PheromonePublisher) Publish(subject string, data interface{}) {
	if p == nil {
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
		req, err := http.NewRequest("POST", p.apiURL+"/api/events", bytes.NewReader(body))
		if err != nil {
			return
		}
		req.Header.Set("Content-Type", "application/json")
		if p.token != "" {
			req.Header.Set("Authorization", "Bearer "+p.token)
		}
		resp, err := p.client.Do(req)
		if err != nil {
			log.Printf("[pheromone] publish %s failed: %v", subject, err)
			return
		}
		resp.Body.Close()
	}()
}

// --- Chrysalis Event Types ---

// BattleCompleteEvent is published after a PK battle finishes.
type BattleCompleteEvent struct {
	ChallengerID string `json:"challenger_id"`
	OpponentID   string `json:"opponent_id"`
	WinnerID     string `json:"winner_id"`
	LoserID      string `json:"loser_id"`
	Rounds       int    `json:"rounds"`
	Timestamp    string `json:"timestamp"`
}

// MutationEvent is published when a fighter gains a mutation.
type MutationEvent struct {
	ClawID       string `json:"claw_id"`
	MutationName string `json:"mutation_name"`
	Rarity       string `json:"rarity"`
	Timestamp    string `json:"timestamp"`
}

// SeasonEndEvent is published when a season ends.
type SeasonEndEvent struct {
	SeasonID   string `json:"season_id"`
	SeasonName string `json:"season_name"`
	Timestamp  string `json:"timestamp"`
}

// FighterSyncEvent is published when a fighter's stats are synced.
type FighterSyncEvent struct {
	ClawID string `json:"claw_id"`
	Level  int    `json:"level"`
	Path   string `json:"evolution_path"`
}
