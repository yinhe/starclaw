package rag

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ledongthuc/pdf"
)

// DocumentParser extracts text content from various file formats
type DocumentParser struct{}

func NewDocumentParser() *DocumentParser {
	return &DocumentParser{}
}

// Parse extracts text from file content based on filename extension
func (p *DocumentParser) Parse(filename string, content []byte) (string, error) {
	lower := strings.ToLower(filename)
	ext := strings.ToLower(filepath.Ext(filename))

	switch {
	case strings.HasSuffix(lower, ".pdf"):
		return p.parsePDF(content)
	case strings.HasSuffix(lower, ".docx"):
		return p.parseDOCX(content)
	case strings.HasSuffix(lower, ".xlsx"):
		return p.parseXLSX(content)
	case strings.HasSuffix(lower, ".pptx"):
		return p.parsePPTX(content)
	case strings.HasSuffix(lower, ".csv"):
		return p.parseCSV(content)
	case strings.HasSuffix(lower, ".rtf"):
		return p.parseRTF(content)
	case strings.HasSuffix(lower, ".doc"):
		return "", fmt.Errorf(".doc format not supported, please convert to .docx")
	case strings.HasSuffix(lower, ".xls"):
		return "", fmt.Errorf(".xls format not supported, please convert to .xlsx")
	case strings.HasSuffix(lower, ".ppt"):
		return "", fmt.Errorf(".ppt format not supported, please convert to .pptx")
	case isBinaryMediaFile(ext):
		// Binary files (audio/video/archive/image) — return metadata as searchable text
		return p.buildMediaMetadata(filename, content), nil
	default:
		// Assume text-based file
		return string(content), nil
	}
}

// CanParse returns true if the file extension is supported
func (p *DocumentParser) CanParse(filename string) bool {
	lower := strings.ToLower(filename)
	supportedExts := []string{
		// Documents
		".pdf", ".docx", ".xlsx", ".pptx", ".rtf",
		// Text & data
		".txt", ".md", ".csv", ".json", ".xml", ".yaml", ".yml",
		".log", ".html", ".htm", ".ini", ".toml",
		// Code
		".py", ".go", ".js", ".ts", ".java", ".c", ".cpp", ".rs",
		".rb", ".php", ".sql", ".sh", ".bat", ".ps1",
		".jsx", ".tsx", ".vue", ".svelte",
		".css", ".scss", ".less",
		".swift", ".kt", ".scala", ".r", ".m", ".lua",
		// Audio
		".mp3", ".wav", ".ogg", ".m4a", ".flac", ".aac", ".wma",
		// Video
		".mp4", ".webm", ".avi", ".mov", ".mkv", ".flv", ".wmv",
		// Images
		".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp", ".svg",
		// Archives
		".zip", ".rar", ".7z", ".tar", ".gz",
	}
	for _, ext := range supportedExts {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

// parsePDF extracts text from PDF bytes
func (p *DocumentParser) parsePDF(content []byte) (string, error) {
	reader := bytes.NewReader(content)
	pdfReader, err := pdf.NewReader(reader, int64(len(content)))
	if err != nil {
		return "", fmt.Errorf("failed to parse PDF: %w", err)
	}

	var buf strings.Builder
	numPages := pdfReader.NumPage()

	for i := 1; i <= numPages; i++ {
		page := pdfReader.Page(i)
		if page.V.IsNull() {
			continue
		}
		text, err := page.GetPlainText(nil)
		if err != nil {
			continue
		}
		buf.WriteString(text)
		buf.WriteString("\n")
	}

	result := strings.TrimSpace(buf.String())
	if result == "" {
		return "", fmt.Errorf("PDF contains no extractable text (may be scanned/image-based)")
	}
	return result, nil
}

// parseDOCX extracts text from DOCX (ZIP containing word/document.xml)
func (p *DocumentParser) parseDOCX(content []byte) (string, error) {
	reader := bytes.NewReader(content)
	zipReader, err := zip.NewReader(reader, int64(len(content)))
	if err != nil {
		return "", fmt.Errorf("failed to open DOCX: %w", err)
	}

	for _, file := range zipReader.File {
		if file.Name == "word/document.xml" {
			rc, err := file.Open()
			if err != nil {
				return "", fmt.Errorf("failed to read document.xml: %w", err)
			}
			defer rc.Close()

			xmlContent, err := io.ReadAll(rc)
			if err != nil {
				return "", fmt.Errorf("failed to read document.xml content: %w", err)
			}

			return extractTextFromXML(string(xmlContent)), nil
		}
	}

	return "", fmt.Errorf("DOCX file does not contain word/document.xml")
}

// isBinaryMediaFile returns true for non-text binary file types
func isBinaryMediaFile(ext string) bool {
	binaryExts := map[string]bool{
		".mp3": true, ".wav": true, ".ogg": true, ".m4a": true, ".flac": true, ".aac": true, ".wma": true,
		".mp4": true, ".webm": true, ".avi": true, ".mov": true, ".mkv": true, ".flv": true, ".wmv": true,
		".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true, ".bmp": true,
		".zip": true, ".rar": true, ".7z": true, ".tar": true, ".gz": true,
	}
	return binaryExts[ext]
}

// buildMediaMetadata creates a searchable text representation of a binary file
func (p *DocumentParser) buildMediaMetadata(filename string, content []byte) string {
	ext := strings.ToLower(filepath.Ext(filename))
	var category string
	switch {
	case strings.HasPrefix(ext, ".mp3") || ext == ".wav" || ext == ".ogg" || ext == ".m4a" || ext == ".flac" || ext == ".aac" || ext == ".wma":
		category = "音频文件"
	case ext == ".mp4" || ext == ".webm" || ext == ".avi" || ext == ".mov" || ext == ".mkv" || ext == ".flv" || ext == ".wmv":
		category = "视频文件"
	case ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".gif" || ext == ".webp" || ext == ".bmp":
		category = "图片文件"
	default:
		category = "二进制文件"
	}

	size := len(content)
	var sizeStr string
	if size < 1024 {
		sizeStr = fmt.Sprintf("%d B", size)
	} else if size < 1024*1024 {
		sizeStr = fmt.Sprintf("%.1f KB", float64(size)/1024)
	} else {
		sizeStr = fmt.Sprintf("%.1f MB", float64(size)/(1024*1024))
	}

	return fmt.Sprintf("[%s] 文件名: %s\n格式: %s\n大小: %s\n\n此文件为%s，已存储在知识库中，可通过文件名搜索引用。", category, filename, ext, sizeStr, category)
}

// parseXLSX extracts text from XLSX spreadsheet (ZIP containing xl/sharedStrings.xml + xl/worksheets/)
func (p *DocumentParser) parseXLSX(content []byte) (string, error) {
	reader := bytes.NewReader(content)
	zipReader, err := zip.NewReader(reader, int64(len(content)))
	if err != nil {
		return "", fmt.Errorf("failed to open XLSX: %w", err)
	}

	// Read shared strings
	var sharedStrings []string
	for _, file := range zipReader.File {
		if file.Name == "xl/sharedStrings.xml" {
			rc, err := file.Open()
			if err != nil {
				break
			}
			xmlContent, _ := io.ReadAll(rc)
			rc.Close()
			re := regexp.MustCompile(`<t[^>]*>([^<]*)</t>`)
			matches := re.FindAllStringSubmatch(string(xmlContent), -1)
			for _, m := range matches {
				if len(m) > 1 {
					sharedStrings = append(sharedStrings, m[1])
				}
			}
			break
		}
	}

	// Read worksheet data
	var buf strings.Builder
	for _, file := range zipReader.File {
		if !strings.HasPrefix(file.Name, "xl/worksheets/sheet") {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			continue
		}
		xmlContent, _ := io.ReadAll(rc)
		rc.Close()

		// Extract cell values — <v> tags
		vRe := regexp.MustCompile(`<c[^>]*t="s"[^>]*><v>(\d+)</v></c>`)
		vMatches := vRe.FindAllStringSubmatch(string(xmlContent), -1)
		for _, m := range vMatches {
			if len(m) > 1 {
				idx := 0
				fmt.Sscanf(m[1], "%d", &idx)
				if idx < len(sharedStrings) {
					buf.WriteString(sharedStrings[idx])
					buf.WriteString(" ")
				}
			}
		}

		// Also extract inline values (numbers etc)
		numRe := regexp.MustCompile(`<c[^>]*><v>([^<]+)</v></c>`)
		numMatches := numRe.FindAllStringSubmatch(string(xmlContent), -1)
		for _, m := range numMatches {
			if len(m) > 1 {
				buf.WriteString(m[1])
				buf.WriteString(" ")
			}
		}

		buf.WriteString("\n")
	}

	result := strings.TrimSpace(buf.String())
	if result == "" && len(sharedStrings) > 0 {
		result = strings.Join(sharedStrings, " ")
	}
	if result == "" {
		return "", fmt.Errorf("XLSX contains no extractable text")
	}
	return result, nil
}

// parsePPTX extracts text from PPTX presentation (ZIP containing ppt/slides/)
func (p *DocumentParser) parsePPTX(content []byte) (string, error) {
	reader := bytes.NewReader(content)
	zipReader, err := zip.NewReader(reader, int64(len(content)))
	if err != nil {
		return "", fmt.Errorf("failed to open PPTX: %w", err)
	}

	var buf strings.Builder
	slideCount := 0
	for _, file := range zipReader.File {
		if !strings.HasPrefix(file.Name, "ppt/slides/slide") || !strings.HasSuffix(file.Name, ".xml") {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			continue
		}
		xmlContent, _ := io.ReadAll(rc)
		rc.Close()

		slideCount++
		buf.WriteString(fmt.Sprintf("--- Slide %d ---\n", slideCount))

		// Extract text from <a:t> tags
		re := regexp.MustCompile(`<a:t>([^<]*)</a:t>`)
		matches := re.FindAllStringSubmatch(string(xmlContent), -1)
		for _, m := range matches {
			if len(m) > 1 && strings.TrimSpace(m[1]) != "" {
				buf.WriteString(m[1])
				buf.WriteString(" ")
			}
		}
		buf.WriteString("\n")
	}

	result := strings.TrimSpace(buf.String())
	if result == "" {
		return "", fmt.Errorf("PPTX contains no extractable text")
	}
	return result, nil
}

// parseCSV parses CSV into text representation
func (p *DocumentParser) parseCSV(content []byte) (string, error) {
	reader := csv.NewReader(bytes.NewReader(content))
	reader.LazyQuotes = true
	reader.FieldsPerRecord = -1 // allow variable fields

	records, err := reader.ReadAll()
	if err != nil {
		// Fallback to plain text
		return string(content), nil
	}

	var buf strings.Builder
	for i, row := range records {
		if i == 0 {
			buf.WriteString("[Header] ")
		}
		buf.WriteString(strings.Join(row, " | "))
		buf.WriteString("\n")
	}
	return strings.TrimSpace(buf.String()), nil
}

// parseRTF extracts plain text from RTF by stripping control words
func (p *DocumentParser) parseRTF(content []byte) (string, error) {
	text := string(content)
	// Remove RTF control words
	ctrlRe := regexp.MustCompile(`\\[a-z]+\d*\s?`)
	text = ctrlRe.ReplaceAllString(text, "")
	// Remove braces
	text = strings.ReplaceAll(text, "{", "")
	text = strings.ReplaceAll(text, "}", "")
	// Clean up whitespace
	spaceRe := regexp.MustCompile(`\s+`)
	text = spaceRe.ReplaceAllString(text, " ")
	return strings.TrimSpace(text), nil
}

// extractTextFromXML strips XML tags and extracts text content from DOCX XML
func extractTextFromXML(xmlStr string) string {
	// Extract content from <w:t> tags
	re := regexp.MustCompile(`<w:t[^>]*>([^<]*)</w:t>`)
	matches := re.FindAllStringSubmatch(xmlStr, -1)

	var parts []string
	for _, match := range matches {
		if len(match) > 1 && strings.TrimSpace(match[1]) != "" {
			parts = append(parts, match[1])
		}
	}

	// Also detect paragraph breaks <w:p>
	result := strings.Join(parts, " ")

	// Clean up multiple spaces
	spaceRe := regexp.MustCompile(`\s+`)
	result = spaceRe.ReplaceAllString(result, " ")

	return strings.TrimSpace(result)
}
