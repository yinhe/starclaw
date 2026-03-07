package v1

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
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

	// Read file content
	content, err := io.ReadAll(io.LimitReader(file, 10*1024*1024)) // 10MB max
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read file"})
		return
	}

	// Determine content type
	contentType := header.Header.Get("Content-Type")
	name := header.Filename

	// Parse document content (supports PDF, DOCX, and text files)
	parser := rag.NewDocumentParser()
	if !parser.CanParse(name) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported file type"})
		return
	}
	textContent, parseErr := parser.Parse(name, content)
	if parseErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("failed to parse file: %v", parseErr)})
		return
	}

	// Create document record
	doc := model.Document{
		KnowledgeBaseID: kbID,
		UserID:          userID,
		Name:            name,
		ContentType:     contentType,
		Size:            int64(len(content)),
		Status:          "pending",
	}
	if err := h.db.Create(&doc).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create document"})
		return
	}

	// Process async (for MVP, do it synchronously in goroutine)
	go func() {
		_ = h.pipeline.IngestDocument(
			c.Request.Context(),
			&doc,
			textContent,
			kb.ChunkSize,
			kb.ChunkOverlap,
		)
	}()

	c.JSON(http.StatusOK, gin.H{"document": doc})
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
			c.Request.Context(),
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

func isTextContent(contentType, filename string) bool {
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
