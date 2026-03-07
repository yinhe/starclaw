package rag

import (
	"context"
	"log"
	"sort"

	"github.com/yinhe/starclaw/internal/model"
	"gorm.io/gorm"
)

// SearchResult represents a chunk matched by similarity search
type SearchResult struct {
	ChunkID    string  `json:"chunk_id"`
	DocumentID string  `json:"document_id"`
	Content    string  `json:"content"`
	Score      float32 `json:"score"`
	ChunkIndex int     `json:"chunk_index"`
}

// Retriever performs similarity search over document chunks
type Retriever struct {
	db       *gorm.DB
	embedder EmbeddingProvider
}

// NewRetriever creates a new retriever
func NewRetriever(db *gorm.DB, embedder EmbeddingProvider) *Retriever {
	return &Retriever{db: db, embedder: embedder}
}

// Search finds the top-k most similar chunks to the query in the given knowledge base
func (r *Retriever) Search(ctx context.Context, kbID string, query string, topK int) ([]SearchResult, error) {
	if topK <= 0 {
		topK = 5
	}

	// Embed the query
	embeddings, err := r.embedder.Embed(ctx, []string{query})
	if err != nil {
		return nil, err
	}
	if len(embeddings) == 0 || len(embeddings[0]) == 0 {
		return nil, nil
	}
	queryVec := embeddings[0]

	// Load all chunks for this KB (for MVP; production would use vector DB)
	var chunks []model.DocumentChunk
	if err := r.db.Where("knowledge_base_id = ?", kbID).Find(&chunks).Error; err != nil {
		return nil, err
	}

	log.Printf("[RAG] Searching %d chunks in KB %s", len(chunks), kbID)

	// Compute similarity scores
	type scored struct {
		chunk model.DocumentChunk
		score float32
	}
	var results []scored

	for _, c := range chunks {
		if len(c.Embedding) == 0 {
			continue
		}
		vec := DeserializeVector(c.Embedding)
		sim := CosineSimilarity(queryVec, vec)
		results = append(results, scored{chunk: c, score: sim})
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	// Take top-k
	if len(results) > topK {
		results = results[:topK]
	}

	var output []SearchResult
	for _, r := range results {
		output = append(output, SearchResult{
			ChunkID:    r.chunk.ID,
			DocumentID: r.chunk.DocumentID,
			Content:    r.chunk.Content,
			Score:      r.score,
			ChunkIndex: r.chunk.ChunkIndex,
		})
	}

	return output, nil
}

// BuildContext constructs a context string from search results for injection into LLM prompt
func BuildContext(results []SearchResult, maxChars int) string {
	if len(results) == 0 {
		return ""
	}
	if maxChars <= 0 {
		maxChars = 4000
	}

	var context string
	for i, r := range results {
		entry := r.Content + "\n\n"
		if len([]rune(context))+len([]rune(entry)) > maxChars {
			break
		}
		if i > 0 {
			context += "---\n\n"
		}
		context += entry
	}

	return context
}
