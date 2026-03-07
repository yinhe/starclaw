package rag

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
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

	switch {
	case strings.HasSuffix(lower, ".pdf"):
		return p.parsePDF(content)
	case strings.HasSuffix(lower, ".docx"):
		return p.parseDOCX(content)
	case strings.HasSuffix(lower, ".doc"):
		return "", fmt.Errorf(".doc format not supported, please convert to .docx")
	default:
		// Assume text-based file
		return string(content), nil
	}
}

// CanParse returns true if the file extension is supported
func (p *DocumentParser) CanParse(filename string) bool {
	lower := strings.ToLower(filename)
	supportedExts := []string{
		".pdf", ".docx",
		".txt", ".md", ".csv", ".json", ".xml", ".yaml", ".yml",
		".log", ".html", ".htm",
		".py", ".go", ".js", ".ts", ".java", ".c", ".cpp", ".rs",
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
