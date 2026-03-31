package sandbox

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ledongthuc/pdf"
)

// SafeReadDocument reads any supported document format with panic recovery.
// Returns extracted text or an error message — never panics.
func SafeReadDocument(path string) (content string, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("document parsing panic: %v", r)
		}
	}()

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".docx":
		return ReadDocxPlainText(path)
	case ".pdf":
		return ReadPdfPlainText(path)
	case ".pptx":
		return ReadZipXmlText(path, "ppt/slides/slide")
	case ".xlsx":
		return ReadXlsxText(path)
	default:
		return "", fmt.Errorf("unsupported document format: %s", ext)
	}
}

// ReadDocxPlainText extracts plain text from a .docx file (zip containing word/document.xml)
func ReadDocxPlainText(path string) (string, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return "", fmt.Errorf("open docx: %w", err)
	}
	defer r.Close()

	for _, f := range r.File {
		if f.Name == "word/document.xml" {
			rc, err := f.Open()
			if err != nil {
				return "", err
			}
			defer rc.Close()
			decoder := xml.NewDecoder(rc)
			var text strings.Builder
			for {
				t, err := decoder.Token()
				if err != nil {
					break
				}
				if charData, ok := t.(xml.CharData); ok {
					text.Write(charData)
				}
			}
			return text.String(), nil
		}
	}
	return "", fmt.Errorf("word/document.xml not found in docx")
}

// ReadPdfPlainText extracts plain text from a PDF file using ledongthuc/pdf library
func ReadPdfPlainText(path string) (string, error) {
	f, r, err := pdf.Open(path)
	if err != nil {
		return "", fmt.Errorf("open pdf: %w", err)
	}
	defer f.Close()

	var text strings.Builder
	totalPages := r.NumPage()
	for i := 1; i <= totalPages; i++ {
		p := r.Page(i)
		if p.V.IsNull() {
			continue
		}
		content, err := p.GetPlainText(nil)
		if err != nil {
			continue
		}
		s := strings.TrimSpace(content)
		if s != "" {
			if totalPages > 1 {
				text.WriteString(fmt.Sprintf("\n--- Page %d ---\n", i))
			}
			text.WriteString(s)
			text.WriteByte('\n')
		}
	}
	result := strings.TrimSpace(text.String())
	if len(result) < 10 {
		return fmt.Sprintf("[PDF: %s, %d pages, text extraction empty (possibly scanned/image PDF)]",
			filepath.Base(path), totalPages), nil
	}
	return result, nil
}

// ReadZipXmlText extracts plain text from XML files inside a zip archive (pptx/xlsx)
// prefix filters which XML files to read (e.g. "ppt/slides/slide" for pptx)
func ReadZipXmlText(path string, prefix string) (string, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return "", fmt.Errorf("open zip: %w", err)
	}
	defer r.Close()

	var text strings.Builder
	partNum := 0
	for _, f := range r.File {
		if !strings.HasPrefix(f.Name, prefix) || !strings.HasSuffix(f.Name, ".xml") {
			continue
		}
		partNum++
		rc, err := f.Open()
		if err != nil {
			continue
		}
		decoder := xml.NewDecoder(rc)
		var partText strings.Builder
		for {
			t, err := decoder.Token()
			if err != nil {
				break
			}
			if charData, ok := t.(xml.CharData); ok {
				s := strings.TrimSpace(string(charData))
				if s != "" {
					partText.WriteString(s)
					partText.WriteByte(' ')
				}
			}
		}
		rc.Close()
		if partText.Len() > 0 {
			text.WriteString(fmt.Sprintf("\n--- Part %d ---\n%s", partNum, partText.String()))
		}
	}
	if text.Len() == 0 {
		return fmt.Sprintf("[Document: %d parts, no text extracted]", partNum), nil
	}
	return text.String(), nil
}

// ReadXlsxText extracts text from a .xlsx file with proper shared-string support.
func ReadXlsxText(path string) (string, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return "", fmt.Errorf("open xlsx: %w", err)
	}
	defer r.Close()

	// First read shared strings
	var sharedStrings []string
	for _, f := range r.File {
		if f.Name == "xl/sharedStrings.xml" {
			rc, err := f.Open()
			if err != nil {
				break
			}
			decoder := xml.NewDecoder(rc)
			var current string
			inSI := false
			for {
				t, err := decoder.Token()
				if err != nil {
					break
				}
				switch se := t.(type) {
				case xml.StartElement:
					if se.Name.Local == "si" {
						inSI = true
						current = ""
					}
				case xml.EndElement:
					if se.Name.Local == "si" {
						sharedStrings = append(sharedStrings, current)
						inSI = false
					}
				case xml.CharData:
					if inSI {
						current += string(se)
					}
				}
			}
			rc.Close()
			break
		}
	}

	// Then read sheet data
	var text strings.Builder
	sheetNum := 0
	for _, f := range r.File {
		if !strings.HasPrefix(f.Name, "xl/worksheets/sheet") || !strings.HasSuffix(f.Name, ".xml") {
			continue
		}
		sheetNum++
		rc, err := f.Open()
		if err != nil {
			continue
		}
		decoder := xml.NewDecoder(rc)
		text.WriteString(fmt.Sprintf("\n--- Sheet %d ---\n", sheetNum))
		rowData := []string{}
		inV := false
		cellType := ""
		for {
			t, err := decoder.Token()
			if err != nil {
				break
			}
			switch se := t.(type) {
			case xml.StartElement:
				if se.Name.Local == "c" {
					cellType = ""
					for _, a := range se.Attr {
						if a.Name.Local == "t" {
							cellType = a.Value
						}
					}
				}
				if se.Name.Local == "v" {
					inV = true
				}
			case xml.EndElement:
				if se.Name.Local == "v" {
					inV = false
				}
				if se.Name.Local == "row" {
					text.WriteString(strings.Join(rowData, "\t") + "\n")
					rowData = nil
				}
			case xml.CharData:
				if inV {
					val := string(se)
					if cellType == "s" {
						idx := 0
						fmt.Sscanf(val, "%d", &idx)
						if idx < len(sharedStrings) {
							val = sharedStrings[idx]
						}
					}
					rowData = append(rowData, val)
				}
			}
		}
		rc.Close()
	}
	if text.Len() == 0 {
		return fmt.Sprintf("[XLSX: %d sheets, no text extracted]", sheetNum), nil
	}
	return text.String(), nil
}
