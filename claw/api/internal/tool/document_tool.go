package tool

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/yinhe/starclaw/internal/model"
	"gorm.io/gorm"
)

// DocumentTool generates conversation summaries and exports them as Word (.docx) documents.
type DocumentTool struct {
	db *gorm.DB
}

func NewDocumentTool(db *gorm.DB) *DocumentTool {
	return &DocumentTool{db: db}
}

func (t *DocumentTool) Name() string { return "document" }

func (t *DocumentTool) Description() string {
	return `文档技能：对话内容总结与 Word 文档导出。

操作：
- summarize：获取当前（或指定）对话的所有消息，返回结构化文本，可供 LLM 进一步总结。
- export_word：将提供的内容（标题 + 正文）生成 .docx Word 文档，返回下载链接。
- summary_to_word：一步完成——获取对话内容，格式化后直接导出为 Word 文档。

使用场景：会议纪要、对话归档、报告导出。`
}

func (t *DocumentTool) Parameters() interface{} {
	return &JSONSchema{
		Type: "object",
		Properties: map[string]Property{
			"action":          {Type: "string", Description: "Action: summarize, export_word, summary_to_word"},
			"conversation_id": {Type: "string", Description: "Target conversation ID. If empty, uses current conversation from context."},
			"title":           {Type: "string", Description: "Document title (for export_word / summary_to_word). Default: conversation title or '对话总结'."},
			"content":         {Type: "string", Description: "Document body content in plain text or Markdown (for export_word). Supports headings (## ), bold (**text**), lists (- item)."},
			"include_full":    {Type: "string", Description: "For summary_to_word: 'true' to append full conversation transcript after summary. Default: 'true'."},
			"max_messages":    {Type: "string", Description: "Max messages to include. Default: 200."},
		},
		Required: []string{"action"},
	}
}

type documentArgs struct {
	Action         string `json:"action"`
	ConversationID string `json:"conversation_id"`
	Title          string `json:"title"`
	Content        string `json:"content"`
	IncludeFull    string `json:"include_full"`
	MaxMessages    string `json:"max_messages"`
}

func (t *DocumentTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	var args documentArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %v", err)
	}

	// Auto-inject conversation_id from context if not provided
	if args.ConversationID == "" {
		if cid, ok := ctx.Value(CtxKeyConversationID).(string); ok && cid != "" {
			args.ConversationID = cid
		}
	}

	switch args.Action {
	case "summarize":
		return t.summarize(args)
	case "export_word":
		return t.exportWord(ctx, args)
	case "summary_to_word":
		return t.summaryToWord(ctx, args)
	default:
		return "", fmt.Errorf("unknown action: %s. Use: summarize, export_word, summary_to_word", args.Action)
	}
}

// summarize fetches conversation messages and returns structured text for LLM summarization.
func (t *DocumentTool) summarize(args documentArgs) (string, error) {
	if args.ConversationID == "" {
		return "", fmt.Errorf("conversation_id is required (or must be called within a conversation)")
	}

	// Fetch conversation
	var conv model.Conversation
	if err := t.db.Where("id = ?", args.ConversationID).First(&conv).Error; err != nil {
		return "", fmt.Errorf("conversation not found: %s", args.ConversationID)
	}

	// Fetch messages
	maxMsg := 200
	if args.MaxMessages != "" {
		fmt.Sscanf(args.MaxMessages, "%d", &maxMsg)
	}

	var messages []model.Message
	t.db.Where("conversation_id = ? AND role IN ('user','assistant')", args.ConversationID).
		Order("created_at ASC").Limit(maxMsg).Find(&messages)

	if len(messages) == 0 {
		return toJSON(map[string]interface{}{
			"status":  "success",
			"action":  "summarize",
			"message": "No messages found in this conversation.",
		}), nil
	}

	// Build structured text
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# 对话记录：%s\n\n", conv.Title))
	sb.WriteString(fmt.Sprintf("对话 ID：%s\n", conv.ID))
	sb.WriteString(fmt.Sprintf("消息数：%d\n", len(messages)))
	sb.WriteString(fmt.Sprintf("时间范围：%s ~ %s\n\n",
		messages[0].CreatedAt.Format("2006-01-02 15:04"),
		messages[len(messages)-1].CreatedAt.Format("2006-01-02 15:04")))
	sb.WriteString("---\n\n")

	for _, m := range messages {
		role := "用户"
		if m.Role == "assistant" {
			role = "助手"
		}
		sb.WriteString(fmt.Sprintf("**[%s] %s：**\n%s\n\n",
			m.CreatedAt.Format("15:04"), role, m.Content))
	}

	return toJSON(map[string]interface{}{
		"status":          "success",
		"action":          "summarize",
		"conversation_id": conv.ID,
		"title":           conv.Title,
		"message_count":   len(messages),
		"transcript":      sb.String(),
		"hint":            "You can now summarize this transcript and call export_word to generate a Word document, or use summary_to_word for a one-step export.",
	}), nil
}

// exportWord generates a .docx file from provided content.
func (t *DocumentTool) exportWord(ctx context.Context, args documentArgs) (string, error) {
	if args.Content == "" {
		return "", fmt.Errorf("content is required for export_word")
	}

	title := args.Title
	if title == "" {
		title = "文档导出"
	}

	// Generate docx
	docxBytes, err := generateDocx(title, args.Content)
	if err != nil {
		return "", fmt.Errorf("failed to generate docx: %v", err)
	}

	// Save to disk
	filename := fmt.Sprintf("doc_%s.docx", uuid.New().String()[:8])
	docDir := "/app/data/documents"
	os.MkdirAll(docDir, 0755)
	filePath := filepath.Join(docDir, filename)

	if err := os.WriteFile(filePath, docxBytes, 0644); err != nil {
		return "", fmt.Errorf("failed to save docx: %v", err)
	}

	log.Printf("[DocumentTool] Generated %s (%d bytes)", filename, len(docxBytes))

	return toJSON(map[string]interface{}{
		"status":       "success",
		"action":       "export_word",
		"filename":     filename,
		"size_bytes":   len(docxBytes),
		"download_url": fmt.Sprintf("/v1/docx/%s", filename),
		"message":      fmt.Sprintf("Word 文档已生成：%s（%d 字节）。点击链接下载。", filename, len(docxBytes)),
	}), nil
}

// summaryToWord fetches conversation, formats, and exports to Word in one step.
func (t *DocumentTool) summaryToWord(ctx context.Context, args documentArgs) (string, error) {
	if args.ConversationID == "" {
		return "", fmt.Errorf("conversation_id is required")
	}

	// Fetch conversation
	var conv model.Conversation
	if err := t.db.Where("id = ?", args.ConversationID).First(&conv).Error; err != nil {
		return "", fmt.Errorf("conversation not found: %s", args.ConversationID)
	}

	// Fetch messages
	maxMsg := 200
	if args.MaxMessages != "" {
		fmt.Sscanf(args.MaxMessages, "%d", &maxMsg)
	}
	var messages []model.Message
	t.db.Where("conversation_id = ? AND role IN ('user','assistant')", args.ConversationID).
		Order("created_at ASC").Limit(maxMsg).Find(&messages)

	title := args.Title
	if title == "" {
		title = conv.Title
	}
	if title == "" {
		title = "对话总结"
	}

	// Build document content
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("对话 ID：%s\n", conv.ID))
	sb.WriteString(fmt.Sprintf("消息数：%d\n", len(messages)))
	if len(messages) > 0 {
		sb.WriteString(fmt.Sprintf("时间范围：%s ~ %s\n",
			messages[0].CreatedAt.Format("2006-01-02 15:04"),
			messages[len(messages)-1].CreatedAt.Format("2006-01-02 15:04")))
	}
	sb.WriteString("\n")

	// If user provided summary content, prepend it
	if args.Content != "" {
		sb.WriteString("## 总结\n\n")
		sb.WriteString(args.Content)
		sb.WriteString("\n\n")
	}

	includeFull := args.IncludeFull != "false"
	if includeFull && len(messages) > 0 {
		sb.WriteString("## 完整对话记录\n\n")
		for _, m := range messages {
			role := "用户"
			if m.Role == "assistant" {
				role = "助手"
			}
			sb.WriteString(fmt.Sprintf("[%s] %s：\n%s\n\n",
				m.CreatedAt.Format("2006-01-02 15:04"), role, m.Content))
		}
	}

	// Generate docx
	docxBytes, err := generateDocx(title, sb.String())
	if err != nil {
		return "", fmt.Errorf("failed to generate docx: %v", err)
	}

	filename := fmt.Sprintf("summary_%s.docx", uuid.New().String()[:8])
	docDir := "/app/data/documents"
	os.MkdirAll(docDir, 0755)
	filePath := filepath.Join(docDir, filename)

	if err := os.WriteFile(filePath, docxBytes, 0644); err != nil {
		return "", fmt.Errorf("failed to save docx: %v", err)
	}

	log.Printf("[DocumentTool] Generated summary %s (%d bytes, %d messages)", filename, len(docxBytes), len(messages))

	return toJSON(map[string]interface{}{
		"status":        "success",
		"action":        "summary_to_word",
		"filename":      filename,
		"size_bytes":    len(docxBytes),
		"message_count": len(messages),
		"download_url":  fmt.Sprintf("/v1/docx/%s", filename),
		"message":       fmt.Sprintf("对话总结已导出为 Word 文档：%s。包含 %d 条消息。", filename, len(messages)),
	}), nil
}

// --- Minimal DOCX Generator (ZIP + XML, no external dependencies) ---

// generateDocx creates a valid .docx file from title and plain-text/markdown content.
func generateDocx(title, content string) ([]byte, error) {
	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)

	// [Content_Types].xml
	writeZipFile(w, "[Content_Types].xml", contentTypesXML)

	// _rels/.rels
	writeZipFile(w, "_rels/.rels", relsXML)

	// word/_rels/document.xml.rels
	writeZipFile(w, "word/_rels/document.xml.rels", documentRelsXML)

	// word/styles.xml
	writeZipFile(w, "word/styles.xml", stylesXML)

	// word/document.xml (the actual content)
	docXML := buildDocumentXML(title, content)
	writeZipFile(w, "word/document.xml", docXML)

	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeZipFile(w *zip.Writer, name, content string) {
	f, _ := w.Create(name)
	f.Write([]byte(content))
}

// buildDocumentXML converts title + markdown-like text to Word XML paragraphs.
func buildDocumentXML(title, content string) string {
	var body strings.Builder

	// Title paragraph
	body.WriteString(fmt.Sprintf(`<w:p><w:pPr><w:pStyle w:val="Title"/></w:pPr><w:r><w:t>%s</w:t></w:r></w:p>`, xmlEscape(title)))

	// Subtitle: generation time
	body.WriteString(fmt.Sprintf(`<w:p><w:pPr><w:pStyle w:val="Subtitle"/></w:pPr><w:r><w:rPr><w:color w:val="888888"/><w:sz w:val="20"/></w:rPr><w:t>Generated by StarClaw · %s</w:t></w:r></w:p>`,
		time.Now().Format("2006-01-02 15:04:05")))

	// Parse content line by line
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if trimmed == "" {
			// Empty paragraph (spacing)
			body.WriteString(`<w:p/>`)
			continue
		}

		if strings.HasPrefix(trimmed, "## ") {
			// Heading 2
			text := strings.TrimPrefix(trimmed, "## ")
			body.WriteString(fmt.Sprintf(`<w:p><w:pPr><w:pStyle w:val="Heading2"/></w:pPr><w:r><w:t>%s</w:t></w:r></w:p>`, xmlEscape(text)))
			continue
		}

		if strings.HasPrefix(trimmed, "# ") {
			// Heading 1
			text := strings.TrimPrefix(trimmed, "# ")
			body.WriteString(fmt.Sprintf(`<w:p><w:pPr><w:pStyle w:val="Heading1"/></w:pPr><w:r><w:t>%s</w:t></w:r></w:p>`, xmlEscape(text)))
			continue
		}

		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			// List item
			text := trimmed[2:]
			body.WriteString(fmt.Sprintf(`<w:p><w:pPr><w:pStyle w:val="ListParagraph"/><w:numPr><w:ilvl w:val="0"/><w:numId w:val="1"/></w:numPr></w:pPr><w:r><w:t>%s</w:t></w:r></w:p>`, xmlEscape(text)))
			continue
		}

		if trimmed == "---" {
			// Horizontal rule → empty paragraph with border
			body.WriteString(`<w:p><w:pPr><w:pBdr><w:bottom w:val="single" w:sz="6" w:space="1" w:color="CCCCCC"/></w:pBdr></w:pPr></w:p>`)
			continue
		}

		// Regular paragraph — handle **bold** inline
		body.WriteString(`<w:p>`)
		writeRichRuns(&body, trimmed)
		body.WriteString(`</w:p>`)
	}

	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:wpc="http://schemas.microsoft.com/office/word/2010/wordprocessingCanvas"
  xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006"
  xmlns:o="urn:schemas-microsoft-com:office:office"
  xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"
  xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math"
  xmlns:v="urn:schemas-microsoft-com:vml"
  xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"
  xmlns:w10="urn:schemas-microsoft-com:office:word"
  xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"
  xmlns:w14="http://schemas.microsoft.com/office/word/2010/wordml"
  xmlns:wpg="http://schemas.microsoft.com/office/word/2010/wordprocessingGroup"
  xmlns:wpi="http://schemas.microsoft.com/office/word/2010/wordprocessingInk"
  xmlns:wne="http://schemas.microsoft.com/office/word/2006/wordml"
  xmlns:wps="http://schemas.microsoft.com/office/word/2010/wordprocessingShape"
  mc:Ignorable="w14 wp14">
  <w:body>%s
    <w:sectPr><w:pgSz w:w="11906" w:h="16838"/><w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440" w:header="720" w:footer="720" w:gutter="0"/></w:sectPr>
  </w:body>
</w:document>`, body.String())
}

// writeRichRuns handles **bold** text within a line, outputting Word XML runs.
func writeRichRuns(sb *strings.Builder, text string) {
	parts := strings.Split(text, "**")
	for i, part := range parts {
		if part == "" {
			continue
		}
		escaped := xmlEscape(part)
		if i%2 == 1 {
			// Bold
			sb.WriteString(fmt.Sprintf(`<w:r><w:rPr><w:b/></w:rPr><w:t xml:space="preserve">%s</w:t></w:r>`, escaped))
		} else {
			sb.WriteString(fmt.Sprintf(`<w:r><w:t xml:space="preserve">%s</w:t></w:r>`, escaped))
		}
	}
}

func xmlEscape(s string) string {
	return html.EscapeString(s)
}

// --- Static OOXML boilerplate ---

const contentTypesXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>
</Types>`

const relsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`

const documentRelsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
</Relationships>`

const stylesXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:style w:type="paragraph" w:styleId="Title">
    <w:name w:val="Title"/>
    <w:pPr><w:spacing w:after="200"/><w:jc w:val="center"/></w:pPr>
    <w:rPr><w:b/><w:sz w:val="52"/><w:szCs w:val="52"/><w:color w:val="1F2937"/></w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="Subtitle">
    <w:name w:val="Subtitle"/>
    <w:pPr><w:spacing w:after="400"/><w:jc w:val="center"/></w:pPr>
    <w:rPr><w:sz w:val="22"/><w:szCs w:val="22"/><w:color w:val="888888"/></w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="Heading1">
    <w:name w:val="heading 1"/>
    <w:pPr><w:spacing w:before="360" w:after="120"/></w:pPr>
    <w:rPr><w:b/><w:sz w:val="36"/><w:szCs w:val="36"/><w:color w:val="1E40AF"/></w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="Heading2">
    <w:name w:val="heading 2"/>
    <w:pPr><w:spacing w:before="240" w:after="80"/></w:pPr>
    <w:rPr><w:b/><w:sz w:val="28"/><w:szCs w:val="28"/><w:color w:val="374151"/></w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="ListParagraph">
    <w:name w:val="List Paragraph"/>
    <w:pPr><w:ind w:left="720"/><w:spacing w:after="60"/></w:pPr>
  </w:style>
  <w:style w:type="paragraph" w:default="1" w:styleId="Normal">
    <w:name w:val="Normal"/>
    <w:pPr><w:spacing w:after="120" w:line="276" w:lineRule="auto"/></w:pPr>
    <w:rPr><w:sz w:val="22"/><w:szCs w:val="22"/><w:rFonts w:ascii="Microsoft YaHei" w:eastAsia="Microsoft YaHei" w:hAnsi="Microsoft YaHei"/></w:rPr>
  </w:style>
</w:styles>`
