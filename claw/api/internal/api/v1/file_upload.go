package v1

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/yinhe/starclaw/internal/sandbox"
)

// uploadDir returns the uploads directory from the centralized path manager.
func getUploadDir() string {
	return sandbox.UploadsDir()
}

const maxUploadSize = 100 * 1024 * 1024 // 100MB

// Supported file types for chat attachments
var allowedExtensions = map[string]string{
	// Documents
	".pdf":  "application/pdf",
	".doc":  "application/msword",
	".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	".xls":  "application/vnd.ms-excel",
	".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	".ppt":  "application/vnd.ms-powerpoint",
	".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
	".txt":  "text/plain",
	".csv":  "text/csv",
	".md":   "text/markdown",
	".json": "application/json",
	".xml":  "application/xml",
	".html": "text/html",
	".rtf":  "application/rtf",
	// Images
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".gif":  "image/gif",
	".webp": "image/webp",
	".bmp":  "image/bmp",
	".svg":  "image/svg+xml",
	// Audio
	".mp3":  "audio/mpeg",
	".wav":  "audio/wav",
	".ogg":  "audio/ogg",
	".m4a":  "audio/mp4",
	".flac": "audio/flac",
	".aac":  "audio/aac",
	".wma":  "audio/x-ms-wma",
	// Video
	".mp4":  "video/mp4",
	".webm": "video/webm",
	".avi":  "video/x-msvideo",
	".mov":  "video/quicktime",
	".mkv":  "video/x-matroska",
	".flv":  "video/x-flv",
	".wmv":  "video/x-ms-wmv",
	// Archives
	".zip": "application/zip",
	".rar": "application/vnd.rar",
	".7z":  "application/x-7z-compressed",
	".tar": "application/x-tar",
	".gz":  "application/gzip",
	// Code
	".py":   "text/x-python",
	".js":   "text/javascript",
	".ts":   "text/typescript",
	".go":   "text/x-go",
	".java": "text/x-java",
	".c":    "text/x-c",
	".cpp":  "text/x-c++",
	".rs":   "text/x-rust",
	".rb":   "text/x-ruby",
	".php":  "text/x-php",
	".sql":  "text/x-sql",
	".sh":   "text/x-shellscript",
	".yaml": "text/yaml",
	".yml":  "text/yaml",
	".toml": "text/x-toml",
	".ini":  "text/plain",
	".log":  "text/plain",
}

// fileCategory returns a category string for the file extension
func fileCategory(ext string) string {
	ext = strings.ToLower(ext)
	switch {
	case strings.HasPrefix(allowedExtensions[ext], "image/"):
		return "image"
	case strings.HasPrefix(allowedExtensions[ext], "audio/"):
		return "audio"
	case strings.HasPrefix(allowedExtensions[ext], "video/"):
		return "video"
	case ext == ".pdf" || ext == ".doc" || ext == ".docx" || ext == ".xls" || ext == ".xlsx" ||
		ext == ".ppt" || ext == ".pptx" || ext == ".rtf":
		return "document"
	case ext == ".zip" || ext == ".rar" || ext == ".7z" || ext == ".tar" || ext == ".gz":
		return "archive"
	case ext == ".py" || ext == ".js" || ext == ".ts" || ext == ".go" || ext == ".java" ||
		ext == ".c" || ext == ".cpp" || ext == ".rs" || ext == ".rb" || ext == ".php" ||
		ext == ".sql" || ext == ".sh":
		return "code"
	default:
		return "text"
	}
}

// UploadFile handles general file upload for chat attachments
func UploadFile(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no file uploaded"})
		return
	}
	defer file.Close()

	if header.Size > maxUploadSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("file too large, max %dMB", maxUploadSize/(1024*1024))})
		return
	}

	ext := strings.ToLower(filepath.Ext(header.Filename))
	mime, ok := allowedExtensions[ext]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported file type: " + ext})
		return
	}

	// Generate unique filename
	fileID := uuid.New().String()
	storedName := fileID + ext
	destPath := filepath.Join(getUploadDir(), storedName)

	out, err := os.Create(destPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save file"})
		return
	}
	defer out.Close()

	written, err := io.Copy(out, file)
	if err != nil {
		os.Remove(destPath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to write file"})
		return
	}

	category := fileCategory(ext)

	c.JSON(http.StatusOK, gin.H{
		"id":       fileID,
		"filename": header.Filename,
		"stored":   storedName,
		"url":      "/v1/uploads/" + storedName,
		"size":     written,
		"mime":     mime,
		"category": category,
	})
}

// ServeUploadedFile serves a file from the uploads directory
func ServeUploadedFile(c *gin.Context) {
	filename := c.Param("filename")

	// Security: prevent directory traversal
	if strings.Contains(filename, "..") || strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid filename"})
		return
	}

	filePath := filepath.Join(getUploadDir(), filename)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}

	ext := strings.ToLower(filepath.Ext(filename))
	if mime, ok := allowedExtensions[ext]; ok {
		c.Header("Content-Type", mime)
	}
	c.Header("Cache-Control", "public, max-age=86400")
	c.File(filePath)
}
