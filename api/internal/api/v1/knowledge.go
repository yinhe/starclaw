package v1

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/yinhe/starclaw/internal/model"
	"github.com/yinhe/starclaw/internal/rag"
	"gorm.io/gorm"
)

type KnowledgeHandler struct {
	db       *gorm.DB
	pipeline *rag.Pipeline
	embedder rag.EmbeddingProvider
}

func NewKnowledgeHandler(db *gorm.DB, pipeline *rag.Pipeline, embedder rag.EmbeddingProvider) *KnowledgeHandler {
	return &KnowledgeHandler{db: db, pipeline: pipeline, embedder: embedder}
}

// --- Knowledge Base CRUD ---

func (h *KnowledgeHandler) ListKBs(c *gin.Context) {
	userID := c.GetString("user_id")
	var kbs []model.KnowledgeBase
	h.db.Where("user_id = ?", userID).Order("updated_at DESC").Find(&kbs)
	c.JSON(http.StatusOK, gin.H{"knowledge_bases": kbs})
}

func (h *KnowledgeHandler) CreateKB(c *gin.Context) {
	userID := c.GetString("user_id")
	var req struct {
		Name           string `json:"name" binding:"required"`
		Description    string `json:"description"`
		EmbeddingModel string `json:"embedding_model"`
		ChunkSize      int    `json:"chunk_size"`
		ChunkOverlap   int    `json:"chunk_overlap"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	kb := model.KnowledgeBase{
		UserID:         userID,
		Name:           req.Name,
		Description:    req.Description,
		EmbeddingModel: req.EmbeddingModel,
		ChunkSize:      req.ChunkSize,
		ChunkOverlap:   req.ChunkOverlap,
	}
	if kb.EmbeddingModel == "" {
		kb.EmbeddingModel = "text-embedding-3-small"
	}
	if kb.ChunkSize == 0 {
		kb.ChunkSize = 500
	}
	if kb.ChunkOverlap == 0 {
		kb.ChunkOverlap = 50
	}

	if err := h.db.Create(&kb).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create knowledge base"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"knowledge_base": kb})
}

func (h *KnowledgeHandler) GetKB(c *gin.Context) {
	userID := c.GetString("user_id")
	id := c.Param("id")

	var kb model.KnowledgeBase
	if err := h.db.Where("id = ? AND user_id = ?", id, userID).First(&kb).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "knowledge base not found"})
		return
	}

	// Also return documents
	var docs []model.Document
	h.db.Where("knowledge_base_id = ?", id).Order("created_at DESC").Find(&docs)

	c.JSON(http.StatusOK, gin.H{"knowledge_base": kb, "documents": docs})
}

func (h *KnowledgeHandler) DeleteKB(c *gin.Context) {
	userID := c.GetString("user_id")
	id := c.Param("id")

	result := h.db.Where("id = ? AND user_id = ?", id, userID).Delete(&model.KnowledgeBase{})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "knowledge base not found"})
		return
	}

	// Clean up documents and chunks
	h.db.Where("knowledge_base_id = ?", id).Delete(&model.DocumentChunk{})
	h.db.Where("knowledge_base_id = ?", id).Delete(&model.Document{})

	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// --- Document Upload ---

func (h *KnowledgeHandler) UploadDocument(c *gin.Context) {
	userID := c.GetString("user_id")
	kbID := c.Param("id")

	// Verify KB ownership
	var kb model.KnowledgeBase
	if err := h.db.Where("id = ? AND user_id = ?", kbID, userID).First(&kb).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "knowledge base not found"})
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}
	defer file.Close()

	// Read file content — 100MB max
	content, err := io.ReadAll(io.LimitReader(file, 100*1024*1024))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read file"})
		return
	}

	// Determine content type
	contentType := header.Header.Get("Content-Type")
	name := header.Filename

	// Parse document content
	parser := rag.NewDocumentParser()
	if !parser.CanParse(name) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported file type: " + filepath.Ext(name)})
		return
	}
	textContent, parseErr := parser.Parse(name, content)
	if parseErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("failed to parse file: %v", parseErr)})
		return
	}

	// Determine file category
	category := kbFileCategory(name)

	// For binary files (audio/video/image/archive), store file to disk so it can be referenced
	var fileURL string
	if isBinaryKBFile(name) {
		ext := strings.ToLower(filepath.Ext(name))
		storedName := uuid.New().String() + ext
		os.MkdirAll("/app/uploads", 0755)
		destPath := filepath.Join("/app/uploads", storedName)
		if err := os.WriteFile(destPath, content, 0644); err == nil {
			fileURL = "/v1/uploads/" + storedName
		}
	}

	// Create document record
	doc := model.Document{
		KnowledgeBaseID: kbID,
		UserID:          userID,
		Name:            name,
		ContentType:     contentType,
		FileURL:         fileURL,
		Category:        category,
		Size:            int64(len(content)),
		Status:          "pending",
	}
	if err := h.db.Create(&doc).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create document"})
		return
	}

	// Process async — use Background context since the HTTP request context
	// is cancelled as soon as this handler returns
	go func() {
		_ = h.pipeline.IngestDocument(
			context.Background(),
			&doc,
			textContent,
			kb.ChunkSize,
			kb.ChunkOverlap,
		)
	}()

	c.JSON(http.StatusOK, gin.H{"document": doc})
}

// isBinaryKBFile returns true for non-text binary files that should be stored to disk
func isBinaryKBFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	binaryExts := map[string]bool{
		".mp3": true, ".wav": true, ".ogg": true, ".m4a": true, ".flac": true, ".aac": true, ".wma": true,
		".mp4": true, ".webm": true, ".avi": true, ".mov": true, ".mkv": true, ".flv": true, ".wmv": true,
		".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true, ".bmp": true,
		".zip": true, ".rar": true, ".7z": true, ".tar": true, ".gz": true,
		".pdf": true, ".docx": true, ".xlsx": true, ".pptx": true, ".rtf": true,
	}
	return binaryExts[ext]
}

// kbFileCategory returns a display category for the file
func kbFileCategory(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch {
	case ext == ".mp3" || ext == ".wav" || ext == ".ogg" || ext == ".m4a" || ext == ".flac" || ext == ".aac" || ext == ".wma":
		return "audio"
	case ext == ".mp4" || ext == ".webm" || ext == ".avi" || ext == ".mov" || ext == ".mkv" || ext == ".flv" || ext == ".wmv":
		return "video"
	case ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".gif" || ext == ".webp" || ext == ".bmp" || ext == ".svg":
		return "image"
	case ext == ".pdf" || ext == ".docx" || ext == ".xlsx" || ext == ".pptx" || ext == ".rtf":
		return "document"
	case ext == ".zip" || ext == ".rar" || ext == ".7z" || ext == ".tar" || ext == ".gz":
		return "archive"
	case ext == ".py" || ext == ".go" || ext == ".js" || ext == ".ts" || ext == ".java" || ext == ".c" || ext == ".cpp" ||
		ext == ".rs" || ext == ".rb" || ext == ".php" || ext == ".sql" || ext == ".sh" || ext == ".jsx" || ext == ".tsx":
		return "code"
	default:
		return "text"
	}
}

// UploadText allows direct text input instead of file upload
func (h *KnowledgeHandler) UploadText(c *gin.Context) {
	userID := c.GetString("user_id")
	kbID := c.Param("id")

	var kb model.KnowledgeBase
	if err := h.db.Where("id = ? AND user_id = ?", kbID, userID).First(&kb).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "knowledge base not found"})
		return
	}

	var req struct {
		Name    string `json:"name" binding:"required"`
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	doc := model.Document{
		KnowledgeBaseID: kbID,
		UserID:          userID,
		Name:            req.Name,
		ContentType:     "text/plain",
		Size:            int64(len(req.Content)),
		Status:          "pending",
	}
	if err := h.db.Create(&doc).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create document"})
		return
	}

	go func() {
		_ = h.pipeline.IngestDocument(
			context.Background(),
			&doc,
			req.Content,
			kb.ChunkSize,
			kb.ChunkOverlap,
		)
	}()

	c.JSON(http.StatusOK, gin.H{"document": doc})
}

func (h *KnowledgeHandler) DeleteDocument(c *gin.Context) {
	userID := c.GetString("user_id")
	docID := c.Param("doc_id")

	var doc model.Document
	if err := h.db.Where("id = ? AND user_id = ?", docID, userID).First(&doc).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "document not found"})
		return
	}

	h.pipeline.DeleteDocumentChunks(docID)
	h.db.Delete(&doc)

	// Update KB count
	var count int64
	h.db.Model(&model.Document{}).Where("knowledge_base_id = ? AND status = ?", doc.KnowledgeBaseID, "ready").Count(&count)
	h.db.Model(&model.KnowledgeBase{}).Where("id = ?", doc.KnowledgeBaseID).Update("document_count", count)

	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// --- Search ---

func (h *KnowledgeHandler) Search(c *gin.Context) {
	userID := c.GetString("user_id")
	kbID := c.Param("id")

	var kb model.KnowledgeBase
	if err := h.db.Where("id = ? AND user_id = ?", kbID, userID).First(&kb).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "knowledge base not found"})
		return
	}

	var req struct {
		Query string `json:"query" binding:"required"`
		TopK  int    `json:"top_k"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	retriever := rag.NewRetriever(h.db, h.embedder)
	results, err := retriever.Search(c.Request.Context(), kbID, req.Query, req.TopK)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("search failed: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"results": results})
}

// isTextContent is reserved for future use in knowledge base document type detection.
var _ = func(contentType, filename string) bool {
	if strings.HasPrefix(contentType, "text/") {
		return true
	}
	lower := strings.ToLower(filename)
	textExtensions := []string{".txt", ".md", ".csv", ".json", ".xml", ".yaml", ".yml", ".log", ".html", ".htm", ".py", ".go", ".js", ".ts", ".java", ".c", ".cpp", ".rs"}
	for _, ext := range textExtensions {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}
