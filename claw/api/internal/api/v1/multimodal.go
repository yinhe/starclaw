package v1

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/yinhe/starclaw/internal/browser"
	"github.com/yinhe/starclaw/internal/config"
	"github.com/yinhe/starclaw/internal/model"
	"gorm.io/gorm"
)

type MultimodalHandler struct {
	cfg *config.Config
	db  *gorm.DB
}

func NewMultimodalHandler(cfg *config.Config, db *gorm.DB) *MultimodalHandler {
	return &MultimodalHandler{cfg: cfg, db: db}
}

// ServeScreenshot serves a cached browser screenshot by ID
func ServeScreenshot(c *gin.Context) {
	id := c.Param("id")
	data, mimeType, ok := browser.GetCache().Get(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "screenshot not found or expired"})
		return
	}
	c.Data(http.StatusOK, mimeType, data)
}

// UploadImage accepts an image file and returns a base64 data URL for use in vision messages
func (h *MultimodalHandler) UploadImage(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no file uploaded"})
		return
	}
	defer file.Close()

	// Validate file type
	ext := strings.ToLower(filepath.Ext(header.Filename))
	mimeTypes := map[string]string{
		".jpg": "image/jpeg", ".jpeg": "image/jpeg",
		".png": "image/png", ".gif": "image/gif",
		".webp": "image/webp", ".bmp": "image/bmp",
	}
	mime, ok := mimeTypes[ext]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported image format, use jpg/png/gif/webp"})
		return
	}

	// Limit to 20MB
	if header.Size > 20*1024*1024 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "image too large, max 20MB"})
		return
	}

	data, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read file"})
		return
	}

	b64 := base64.StdEncoding.EncodeToString(data)
	dataURL := fmt.Sprintf("data:%s;base64,%s", mime, b64)

	// Also save to disk so AI tools can access the file
	os.MkdirAll(uploadDir, 0755)
	fileID := uuid.New().String()
	storedName := fileID + ext
	destPath := filepath.Join(uploadDir, storedName)
	if err := os.WriteFile(destPath, data, 0644); err != nil {
		log.Printf("[multimodal] failed to save image to disk: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"id":       fileID,
		"url":      dataURL,
		"file_url": "/v1/uploads/" + storedName,
		"stored":   storedName,
		"filename": header.Filename,
		"size":     header.Size,
		"mime":     mime,
		"category": "image",
	})
}

// findSTTProvider looks for a provider with Whisper/STT support.
// Priority: qwen > openai > deepseek > any other OpenAI-compatible provider.
func (h *MultimodalHandler) findSTTProvider(userID string) (providerName, apiKey, baseURL string, found bool) {
	// Priority order for STT providers
	priority := []string{"qwen", "openai", "deepseek"}

	for _, prov := range priority {
		var cfg model.ModelConfig
		if err := h.db.Where("user_id = ? AND provider = ? AND is_enabled = ? AND api_key != '' AND api_key != 'claw-identity' AND deleted_at IS NULL", userID, prov, true).First(&cfg).Error; err == nil {
			base := cfg.BaseURL
			if base == "" {
				switch prov {
				case "qwen":
					base = "https://dashscope.aliyuncs.com/compatible-mode/v1"
				case "openai":
					base = "https://api.openai.com/v1"
				case "deepseek":
					base = "https://api.deepseek.com/v1"
				}
			}
			log.Printf("[STT] Using provider: %s, base: %s", prov, base)
			return prov, cfg.APIKey, base, true
		}
	}

	// Fallback: any enabled provider with a real API key (exclude star-ai which uses claw-identity)
	var cfg model.ModelConfig
	if err := h.db.Where("user_id = ? AND is_enabled = ? AND api_key != '' AND api_key != 'claw-identity' AND provider != 'star-ai' AND deleted_at IS NULL", userID, true).First(&cfg).Error; err == nil {
		base := cfg.BaseURL
		if base == "" {
			base = "https://api.openai.com/v1"
		}
		log.Printf("[STT] Fallback provider: %s", cfg.Provider)
		return cfg.Provider, cfg.APIKey, base, true
	}

	// Last resort: config file
	if h.cfg.OpenAI.APIKey != "" {
		base := h.cfg.OpenAI.BaseURL
		if base == "" {
			base = "https://api.openai.com/v1"
		}
		return "openai", h.cfg.OpenAI.APIKey, base, true
	}

	return "", "", "", false
}

// sttModelForProvider returns the correct whisper model name for each provider
func sttModelForProvider(prov string) string {
	switch prov {
	case "qwen":
		return "whisper-large-v3"
	default:
		return "whisper-1"
	}
}

// SpeechToText accepts an audio file and returns transcribed text using Whisper API
// Priority: Qwen (DashScope) > OpenAI > any configured provider
func (h *MultimodalHandler) SpeechToText(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no audio file uploaded"})
		return
	}
	defer file.Close()

	userID := c.GetString("user_id")
	provName, apiKey, baseURL, found := h.findSTTProvider(userID)
	if !found {
		c.JSON(http.StatusBadRequest, gin.H{"error": "\u672a\u914d\u7f6e\u652f\u6301\u8bed\u97f3\u8bc6\u522b\u7684\u6a21\u578b\u63d0\u4f9b\u5546\uff08\u9700\u8981 Qwen \u6216 OpenAI\uff09"})
		return
	}
	sttModel := sttModelForProvider(provName)

	// Read file
	data, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read audio"})
		return
	}

	// Build multipart request to OpenAI Whisper
	var buf bytes.Buffer
	boundary := "----StarClawBoundary" + uuid.New().String()[:8]
	w := &buf

	// file field
	fmt.Fprintf(w, "--%s\r\n", boundary)
	fmt.Fprintf(w, "Content-Disposition: form-data; name=\"file\"; filename=\"%s\"\r\n", header.Filename)
	fmt.Fprintf(w, "Content-Type: application/octet-stream\r\n\r\n")
	w.Write(data)
	fmt.Fprintf(w, "\r\n")

	// model field
	fmt.Fprintf(w, "--%s\r\n", boundary)
	fmt.Fprintf(w, "Content-Disposition: form-data; name=\"model\"\r\n\r\n")
	fmt.Fprintf(w, "%s\r\n", sttModel)

	// language field (optional, auto-detect)
	fmt.Fprintf(w, "--%s--\r\n", boundary)

	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, baseURL+"/audio/transcriptions", &buf)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create request"})
		return
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "STT request failed: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "STT API error: " + string(body)})
		return
	}

	var result struct {
		Text string `json:"text"`
	}
	json.Unmarshal(body, &result)

	c.JSON(http.StatusOK, gin.H{"text": result.Text})
}

// TextToSpeech converts text to audio using OpenAI TTS API
func (h *MultimodalHandler) TextToSpeech(c *gin.Context) {
	var req struct {
		Text  string `json:"text" binding:"required"`
		Voice string `json:"voice"` // alloy, echo, fable, onyx, nova, shimmer
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Voice == "" {
		req.Voice = "alloy"
	}

	apiKey := h.cfg.OpenAI.APIKey
	baseURL := h.cfg.OpenAI.BaseURL
	if apiKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "OpenAI API key not configured for TTS"})
		return
	}
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	payload, _ := json.Marshal(map[string]string{
		"model": "tts-1",
		"input": req.Text,
		"voice": req.Voice,
	})

	httpReq, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, baseURL+"/audio/speech", bytes.NewReader(payload))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create request"})
		return
	}
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "TTS request failed: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.JSON(resp.StatusCode, gin.H{"error": "TTS API error: " + string(body)})
		return
	}

	c.Header("Content-Type", "audio/mpeg")
	c.Header("Content-Disposition", "inline; filename=speech.mp3")
	io.Copy(c.Writer, resp.Body)
}
