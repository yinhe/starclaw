package rag

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
)

// EmbeddingProvider generates embeddings from text
type EmbeddingProvider interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	Dimension() int
}

// OpenAIEmbedding implements EmbeddingProvider using OpenAI API
type OpenAIEmbedding struct {
	apiKey  string
	baseURL string
	model   string
	dim     int
	client  *http.Client
}

type OpenAIEmbeddingConfig struct {
	APIKey  string
	BaseURL string
	Model   string // e.g. "text-embedding-3-small"
}

func NewOpenAIEmbedding(cfg OpenAIEmbeddingConfig) *OpenAIEmbedding {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	model := cfg.Model
	if model == "" {
		model = "text-embedding-3-small"
	}
	dim := 1536
	if model == "text-embedding-3-small" {
		dim = 1536
	} else if model == "text-embedding-3-large" {
		dim = 3072
	}
	return &OpenAIEmbedding{
		apiKey:  cfg.APIKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		model:   model,
		dim:     dim,
		client:  &http.Client{},
	}
}

func (e *OpenAIEmbedding) Dimension() int {
	return e.dim
}

func (e *OpenAIEmbedding) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	payload := map[string]interface{}{
		"model": e.model,
		"input": texts,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.apiKey)

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedding request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("embedding API error (status %d): %s", resp.StatusCode, string(errBody))
	}

	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
			Index     int       `json:"index"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode embedding response: %w", err)
	}

	embeddings := make([][]float32, len(texts))
	for _, d := range result.Data {
		if d.Index < len(embeddings) {
			embeddings[d.Index] = d.Embedding
		}
	}

	return embeddings, nil
}

// SerializeVector converts float32 slice to bytes for DB storage
func SerializeVector(v []float32) []byte {
	buf := new(bytes.Buffer)
	for _, f := range v {
		binary.Write(buf, binary.LittleEndian, f)
	}
	return buf.Bytes()
}

// DeserializeVector converts bytes back to float32 slice
func DeserializeVector(data []byte) []float32 {
	count := len(data) / 4
	result := make([]float32, count)
	reader := bytes.NewReader(data)
	for i := 0; i < count; i++ {
		binary.Read(reader, binary.LittleEndian, &result[i])
	}
	return result
}

// CosineSimilarity computes similarity between two vectors
func CosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return float32(dot / (math.Sqrt(normA) * math.Sqrt(normB)))
}
