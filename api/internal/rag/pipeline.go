package rag

import (
	"context"
	"fmt"
	"log"

	"github.com/yinhe/starclaw/internal/model"
	"gorm.io/gorm"
)

// Pipeline handles document ingestion: text ↀchunks ↀembeddings ↀstore
type Pipeline struct {
	db       *gorm.DB
	embedder EmbeddingProvider
}

// NewPipeline creates a new ingestion pipeline
func NewPipeline(db *gorm.DB, embedder EmbeddingProvider) *Pipeline {
	return &Pipeline{db: db, embedder: embedder}
}

// IngestDocument processes a document's text content into searchable chunks
func (p *Pipeline) IngestDocument(ctx context.Context, doc *model.Document, content string, chunkSize, chunkOverlap int) error {
	// Update status
	p.db.Model(doc).Update("status", "processing")

	// Split into chunks
	chunks := ChunkText(content, chunkSize, chunkOverlap)
	if len(chunks) == 0 {
		p.db.Model(doc).Updates(map[string]interface{}{
			"status":        "error",
			"error_message": "no content to process",
		})
		return fmt.Errorf("no content to process")
	}

	log.Printf("[RAG] Processing document %s: %d chunks", doc.ID, len(chunks))

	// Embed in batches of 20
	batchSize := 20
	var allChunkModels []model.DocumentChunk

	for i := 0; i < len(chunks); i += batchSize {
		end := i + batchSize
		if end > len(chunks) {
			end = len(chunks)
		}
		batch := chunks[i:end]

		embeddings, err := p.embedder.Embed(ctx, batch)
		if err != nil {
			p.db.Model(doc).Updates(map[string]interface{}{
				"status":        "error",
				"error_message": fmt.Sprintf("embedding failed at batch %d: %v", i/batchSize, err),
			})
			return fmt.Errorf("embedding failed: %w", err)
		}

		for j, text := range batch {
			chunkIdx := i + j
			var embBytes []byte
			if j < len(embeddings) && len(embeddings[j]) > 0 {
				embBytes = SerializeVector(embeddings[j])
			}

			allChunkModels = append(allChunkModels, model.DocumentChunk{
				DocumentID:      doc.ID,
				KnowledgeBaseID: doc.KnowledgeBaseID,
				Content:         text,
				Embedding:       embBytes,
				ChunkIndex:      chunkIdx,
				TokenCount:      EstimateTokens(text),
			})
		}

		log.Printf("[RAG] Embedded batch %d/%d for document %s", i/batchSize+1, (len(chunks)+batchSize-1)/batchSize, doc.ID)
	}

	// Store chunks in DB
	if len(allChunkModels) > 0 {
		// Insert in batches to avoid huge queries
		for i := 0; i < len(allChunkModels); i += 100 {
			end := i + 100
			if end > len(allChunkModels) {
				end = len(allChunkModels)
			}
			if err := p.db.Create(allChunkModels[i:end]).Error; err != nil {
				p.db.Model(doc).Updates(map[string]interface{}{
					"status":        "error",
					"error_message": fmt.Sprintf("failed to store chunks: %v", err),
				})
				return err
			}
		}
	}

	// Update document and KB stats
	p.db.Model(doc).Updates(map[string]interface{}{
		"status":      "ready",
		"chunk_count": len(allChunkModels),
	})

	// Update KB document count
	var count int64
	p.db.Model(&model.Document{}).Where("knowledge_base_id = ? AND status = ?", doc.KnowledgeBaseID, "ready").Count(&count)
	p.db.Model(&model.KnowledgeBase{}).Where("id = ?", doc.KnowledgeBaseID).Update("document_count", count)

	log.Printf("[RAG] Document %s ready: %d chunks stored", doc.ID, len(allChunkModels))
	return nil
}

// DeleteDocumentChunks removes all chunks for a document
func (p *Pipeline) DeleteDocumentChunks(docID string) error {
	return p.db.Where("document_id = ?", docID).Delete(&model.DocumentChunk{}).Error
}
