package inference

import (
	"context"
	"log"
	"math/rand"
	"strings"
	"sync"
	"time"
)

// SpotChecker performs random quality checks on contributor inference responses.
// It re-sends a fraction of requests to a second contributor and compares output.
type SpotChecker struct {
	registry *ContributorRegistry
	router   *InferenceRouter
	rate     float64 // spot-check probability (0.0–1.0), default 0.01 (1%)
	mu       sync.Mutex
	rng      *rand.Rand
}

// NewSpotChecker creates a new spot-checker with the given check rate.
func NewSpotChecker(registry *ContributorRegistry, router *InferenceRouter, rate float64) *SpotChecker {
	if rate <= 0 {
		rate = 0.01 // default 1%
	}
	if rate > 1.0 {
		rate = 1.0
	}
	return &SpotChecker{
		registry: registry,
		router:   router,
		rate:     rate,
		rng:      rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// ShouldCheck returns true if this request should be spot-checked.
func (s *SpotChecker) ShouldCheck() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rng.Float64() < s.rate
}

// SpotCheckResult records the outcome of a spot-check.
type SpotCheckResult struct {
	OriginalNodeID  string  `json:"original_node_id"`
	VerifierNodeID  string  `json:"verifier_node_id"`
	Model           string  `json:"model"`
	Passed          bool    `json:"passed"`
	SimilarityScore float64 `json:"similarity_score"` // 0.0–1.0
	OriginalLen     int     `json:"original_len"`
	VerifierLen     int     `json:"verifier_len"`
	CheckedAt       int64   `json:"checked_at"`
}

// VerifyAsync performs an async spot-check by sending the same request to a different contributor.
// The original response text and the contributor's node ID are provided.
// Results are applied to the trust scores of both contributors.
func (s *SpotChecker) VerifyAsync(req *InferenceRequest, originalNodeID string, originalResponse string) {
	go func() {
		result := s.verify(req, originalNodeID, originalResponse)
		if result == nil {
			return // no second contributor available
		}

		// Update trust scores
		s.registry.mu.Lock()
		if orig, ok := s.registry.contributors[result.OriginalNodeID]; ok && orig.Trust != nil {
			orig.Trust.RecordSpotCheck(result.Passed)
		}
		if verifier, ok := s.registry.contributors[result.VerifierNodeID]; ok && verifier.Trust != nil {
			// Verifier gets credit for participating
			verifier.Trust.RecordHeartbeat()
		}
		s.registry.mu.Unlock()

		status := "PASS"
		if !result.Passed {
			status = "FAIL"
		}
		log.Printf("[inference/spotcheck] %s: original=%s verifier=%s model=%s similarity=%.2f",
			status, result.OriginalNodeID[:16], result.VerifierNodeID[:16],
			result.Model, result.SimilarityScore)
	}()
}

// verify performs the actual spot-check (synchronous, called from goroutine).
func (s *SpotChecker) verify(req *InferenceRequest, originalNodeID string, originalResponse string) *SpotCheckResult {
	// Select a different contributor for verification
	verifier := s.selectVerifier(req.Model, originalNodeID)
	if verifier == nil {
		return nil // no alternative available
	}

	// Send the same request (non-streaming for simplicity)
	verifyReq := &InferenceRequest{
		Model:       req.Model,
		Messages:    req.Messages,
		Temperature: 0.0, // deterministic for comparison
		MaxTokens:   req.MaxTokens,
		Stream:      false,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, _, err := s.router.Route(ctx, verifyReq)
	if err != nil {
		log.Printf("[inference/spotcheck] verification request failed: %v", err)
		return nil
	}
	defer resp.Body.Close()

	// Read response body (limited to 8KB for comparison)
	buf := make([]byte, 8192)
	n, _ := resp.Body.Read(buf)
	verifierResponse := string(buf[:n])

	// Compare responses
	similarity := textSimilarity(originalResponse, verifierResponse)

	result := &SpotCheckResult{
		OriginalNodeID:  originalNodeID,
		VerifierNodeID:  verifier.NodeID,
		Model:           req.Model,
		Passed:          similarity >= 0.5, // threshold: 50% similarity (LLMs are non-deterministic)
		SimilarityScore: similarity,
		OriginalLen:     len(originalResponse),
		VerifierLen:     len(verifierResponse),
		CheckedAt:       time.Now().Unix(),
	}

	return result
}

// selectVerifier picks a different contributor than the original for verification.
func (s *SpotChecker) selectVerifier(model string, excludeNodeID string) *ContributorInfo {
	s.registry.mu.RLock()
	defer s.registry.mu.RUnlock()

	for _, c := range s.registry.contributors {
		if c.NodeID == excludeNodeID || c.Status == "offline" || c.ActiveJobs >= c.MaxJobs {
			continue
		}
		for _, supported := range c.Models {
			if supported == model || supported == "*" {
				return c
			}
		}
	}
	return nil
}

// textSimilarity computes a simple word-overlap similarity between two texts.
// Returns 0.0–1.0 (Jaccard similarity on word sets).
func textSimilarity(a, b string) float64 {
	if a == "" && b == "" {
		return 1.0
	}
	if a == "" || b == "" {
		return 0.0
	}

	wordsA := wordSet(a)
	wordsB := wordSet(b)

	if len(wordsA) == 0 && len(wordsB) == 0 {
		return 1.0
	}

	// Jaccard: |intersection| / |union|
	intersection := 0
	for w := range wordsA {
		if wordsB[w] {
			intersection++
		}
	}

	union := len(wordsA)
	for w := range wordsB {
		if !wordsA[w] {
			union++
		}
	}

	if union == 0 {
		return 1.0
	}
	return float64(intersection) / float64(union)
}

// wordSet splits text into a set of lowercased words.
func wordSet(text string) map[string]bool {
	words := strings.Fields(strings.ToLower(text))
	set := make(map[string]bool, len(words))
	for _, w := range words {
		// Strip common punctuation
		w = strings.Trim(w, ".,;:!?\"'()[]{}\\/-")
		if len(w) > 1 {
			set[w] = true
		}
	}
	return set
}
