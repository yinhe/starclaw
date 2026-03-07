package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// KnowledgeBase represents a collection of documents for RAG
type KnowledgeBase struct {
	ID             string         `json:"id" gorm:"type:varchar(36);primaryKey"`
	UserID         string         `json:"user_id" gorm:"type:varchar(36);index;not null"`
	Name           string         `json:"name" gorm:"type:varchar(200);not null"`
	Description    string         `json:"description" gorm:"type:text"`
	EmbeddingModel string         `json:"embedding_model" gorm:"type:varchar(100);default:'text-embedding-3-small'"`
	ChunkSize      int            `json:"chunk_size" gorm:"default:500"`
	ChunkOverlap   int            `json:"chunk_overlap" gorm:"default:50"`
	DocumentCount  int            `json:"document_count" gorm:"default:0"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `json:"-" gorm:"index"`
}

func (kb *KnowledgeBase) BeforeCreate(tx *gorm.DB) error {
	if kb.ID == "" {
		kb.ID = uuid.New().String()
	}
	return nil
}

// Document represents an uploaded file in a knowledge base
type Document struct {
	ID              string         `json:"id" gorm:"type:varchar(36);primaryKey"`
	KnowledgeBaseID string         `json:"knowledge_base_id" gorm:"type:varchar(36);index;not null"`
	UserID          string         `json:"user_id" gorm:"type:varchar(36);index;not null"`
	Name            string         `json:"name" gorm:"type:varchar(500);not null"`
	ContentType     string         `json:"content_type" gorm:"type:varchar(100)"`
	FileURL         string         `json:"file_url,omitempty" gorm:"type:varchar(500)"` // URL to stored file (for binary files)
	Category        string         `json:"category,omitempty" gorm:"type:varchar(30)"`  // document, audio, video, image, code, text, archive
	Size            int64          `json:"size" gorm:"default:0"`
	ChunkCount      int            `json:"chunk_count" gorm:"default:0"`
	Status          string         `json:"status" gorm:"type:varchar(20);default:'pending'"` // pending, processing, ready, error
	ErrorMessage    string         `json:"error_message,omitempty" gorm:"type:text"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `json:"-" gorm:"index"`
}

func (d *Document) BeforeCreate(tx *gorm.DB) error {
	if d.ID == "" {
		d.ID = uuid.New().String()
	}
	return nil
}

// DocumentChunk stores a text chunk with its embedding vector
type DocumentChunk struct {
	ID              string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	DocumentID      string    `json:"document_id" gorm:"type:varchar(36);index;not null"`
	KnowledgeBaseID string    `json:"knowledge_base_id" gorm:"type:varchar(36);index;not null"`
	Content         string    `json:"content" gorm:"type:longtext;not null"`
	Embedding       []byte    `json:"-" gorm:"type:longblob"` // serialized float32 vector
	ChunkIndex      int       `json:"chunk_index" gorm:"default:0"`
	TokenCount      int       `json:"token_count" gorm:"default:0"`
	CreatedAt       time.Time `json:"created_at"`
}

func (dc *DocumentChunk) BeforeCreate(tx *gorm.DB) error {
	if dc.ID == "" {
		dc.ID = uuid.New().String()
	}
	return nil
}
