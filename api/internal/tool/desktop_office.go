package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// ── Office COM Automation ──
//
// Controls Excel, Word, PowerPoint via Windows COM Automation (PowerShell).
// Much faster and more reliable than UI Automation for Office tasks:
//   - Read/write Excel cells, formulas, charts
//   - Create/edit Word documents, apply formatting
//   - Modify PowerPoint slides
//
// No external dependencies — uses PowerShell's built-in COM interop.

// ── excel_read: Read cells from Excel ──

func (t *DesktopTool) excelRead(ctx context.Context, a desktopArgs) (string, error) {
	// text = range like "A1:C10" or "Sheet1!A1:D20", title = file path (optional, uses active workbook)
	cellRange := a.Text
	if cellRange == "" {
		cellRange = "A1:J20" // default: read first 20 rows, 10 columns
	}

	filePath := ""
	if a.Title != "" {
		filePath = fmt.Sprintf(`$wb = $excel.Workbooks.Open('%s')`, strings.ReplaceAll(a.Title, "'", "''"))
	} else {
		filePath = `$wb = $excel.ActiveWorkbook; if ($wb -eq $null) { Write-Output '{"error":"no active workbook"}'; exit }`
	}

	// Parse sheet name if included (e.g., "Sheet2!A1:C10")
	sheetName := ""
	if idx := strings.Index(cellRange, "!"); idx > 0 {
		sheetName = cellRange[:idx]
		cellRange = cellRange[idx+1:]
	}

	sheetSelect := ""
	if sheetName != "" {
		sheetSelect = fmt.Sprintf(`$ws = $wb.Sheets.Item('%s')`, sheetName)
	} else {
		sheetSelect = `$ws = $wb.ActiveSheet`
	}

	psScript := fmt.Sprintf(`
$excel = [Runtime.Interopservices.Marshal]::GetActiveObject('Excel.Application')
%s
%s

$range = $ws.Range('%s')
$rows = @()
for ($r = 1; $r -le $range.Rows.Count; $r++) {
    $row = @()
    for ($c = 1; $c -le $range.Columns.Count; $c++) {
        $cell = $range.Cells.Item($r, $c)
        $row += $cell.Text
    }
    $rows += ,($row -join '|')
}

$result = @{
    sheet = $ws.Name
    range_addr = '%s'
    rows = $rows
    row_count = $range.Rows.Count
    col_count = $range.Columns.Count
}
Write-Output ($result | ConvertTo-Json -Compress)
`, filePath, sheetSelect, cellRange, cellRange)

	out, err := runPowerShell(ctx, psScript)
	if err != nil {
		return "", fmt.Errorf("excel_read failed: %w\n%.500s", err, out)
	}

	out = strings.TrimSpace(out)
	var parsed interface{}
	if json.Unmarshal([]byte(out), &parsed) == nil {
		return toJSON(map[string]interface{}{
			"action": "excel_read", "status": "success",
			"data": parsed, "message": fmt.Sprintf("已读取 Excel %s", cellRange),
		}), nil
	}

	return toJSON(map[string]interface{}{
		"action": "excel_read", "status": "success",
		"raw": out, "message": "已读取 Excel 数据",
	}), nil
}

// ── excel_write: Write to Excel cells ──

func (t *DesktopTool) excelWrite(ctx context.Context, a desktopArgs) (string, error) {
	// title = cell address like "A1" or "Sheet1!B5"
	// text = value to write (or JSON array for multiple cells: [["a","b"],["c","d"]])
	if a.Title == "" || a.Text == "" {
		return "", fmt.Errorf("excel_write requires 'title' (cell address like 'A1') and 'text' (value)")
	}

	cellAddr := a.Title
	sheetName := ""
	if idx := strings.Index(cellAddr, "!"); idx > 0 {
		sheetName = cellAddr[:idx]
		cellAddr = cellAddr[idx+1:]
	}

	sheetSelect := ""
	if sheetName != "" {
		sheetSelect = fmt.Sprintf(`$ws = $wb.Sheets.Item('%s')`, sheetName)
	} else {
		sheetSelect = `$ws = $wb.ActiveSheet`
	}

	// Check if text is a JSON 2D array (batch write)
	var matrix [][]string
	isMatrix := json.Unmarshal([]byte(a.Text), &matrix) == nil && len(matrix) > 0

	var writeCode string
	if isMatrix {
		// Batch write from JSON array
		writeCode = fmt.Sprintf(`
$data = '%s' | ConvertFrom-Json
$startCell = $ws.Range('%s')
for ($r = 0; $r -lt $data.Count; $r++) {
    for ($c = 0; $c -lt $data[$r].Count; $c++) {
        $startCell.Offset($r, $c).Value2 = $data[$r][$c]
    }
}
$written = $data.Count * $data[0].Count
Write-Output "wrote $written cells"
`, strings.ReplaceAll(a.Text, "'", "''"), cellAddr)
	} else {
		// Single cell write
		writeCode = fmt.Sprintf(`
$ws.Range('%s').Value2 = '%s'
Write-Output "wrote 1 cell"
`, cellAddr, strings.ReplaceAll(a.Text, "'", "''"))
	}

	psScript := fmt.Sprintf(`
$excel = [Runtime.Interopservices.Marshal]::GetActiveObject('Excel.Application')
$wb = $excel.ActiveWorkbook
if ($wb -eq $null) { Write-Output '{"error":"no active workbook"}'; exit }
%s
%s
`, sheetSelect, writeCode)

	out, err := runPowerShell(ctx, psScript)
	if err != nil {
		return "", fmt.Errorf("excel_write failed: %w\n%.500s", err, out)
	}

	return toJSON(map[string]interface{}{
		"action":  "excel_write",
		"status":  "success",
		"cell":    a.Title,
		"value":   a.Text,
		"message": fmt.Sprintf("已写入 Excel %s", a.Title),
	}), nil
}

// ── excel_formula: Set a formula in Excel ──

func (t *DesktopTool) excelFormula(ctx context.Context, a desktopArgs) (string, error) {
	if a.Title == "" || a.Text == "" {
		return "", fmt.Errorf("excel_formula requires 'title' (cell address) and 'text' (formula like '=SUM(A1:A10)')")
	}

	psScript := fmt.Sprintf(`
$excel = [Runtime.Interopservices.Marshal]::GetActiveObject('Excel.Application')
$wb = $excel.ActiveWorkbook
$ws = $wb.ActiveSheet
$ws.Range('%s').Formula = '%s'
$result = $ws.Range('%s').Text
Write-Output "formula_result=$result"
`, a.Title, strings.ReplaceAll(a.Text, "'", "''"), a.Title)

	out, err := runPowerShell(ctx, psScript)
	if err != nil {
		return "", fmt.Errorf("excel_formula failed: %w\n%.500s", err, out)
	}

	result := strings.TrimSpace(out)
	result = strings.TrimPrefix(result, "formula_result=")

	return toJSON(map[string]interface{}{
		"action":  "excel_formula",
		"status":  "success",
		"cell":    a.Title,
		"formula": a.Text,
		"result":  result,
		"message": fmt.Sprintf("已在 %s 设置公式 %s = %s", a.Title, a.Text, result),
	}), nil
}

// ── word_read: Read Word document content ──

func (t *DesktopTool) wordRead(ctx context.Context, _ desktopArgs) (string, error) {
	psScript := `
$word = [Runtime.Interopservices.Marshal]::GetActiveObject('Word.Application')
$doc = $word.ActiveDocument
if ($doc -eq $null) { Write-Output '{"error":"no active document"}'; exit }
$text = $doc.Content.Text
if ($text.Length -gt 5000) { $text = $text.Substring(0, 5000) + "...(truncated)" }
$result = @{
    name = $doc.Name
    path = $doc.FullName
    pages = $doc.ComputeStatistics(2)
    words = $doc.ComputeStatistics(0)
    text = $text
}
Write-Output ($result | ConvertTo-Json -Compress)
`
	out, err := runPowerShell(ctx, psScript)
	if err != nil {
		return "", fmt.Errorf("word_read failed: %w\n%.500s", err, out)
	}

	out = strings.TrimSpace(out)
	var parsed interface{}
	if json.Unmarshal([]byte(out), &parsed) == nil {
		return toJSON(map[string]interface{}{
			"action": "word_read", "status": "success",
			"data": parsed, "message": "已读取 Word 文档内容",
		}), nil
	}

	return toJSON(map[string]interface{}{
		"action": "word_read", "status": "success",
		"raw": out, "message": "已读取 Word 文档",
	}), nil
}

// ── word_write: Append or replace text in Word ──

func (t *DesktopTool) wordWrite(ctx context.Context, a desktopArgs) (string, error) {
	if a.Text == "" {
		return "", fmt.Errorf("word_write requires 'text' (content to write)")
	}

	mode := a.Button // "append" (default), "replace", "insert"
	if mode == "" {
		mode = "append"
	}

	textB64 := encodeBase64(a.Text)

	var writeCode string
	switch mode {
	case "replace":
		writeCode = `$doc.Content.Text = $text`
	case "insert":
		writeCode = `
$sel = $word.Selection
$sel.TypeText($text)
`
	default: // append
		writeCode = "$range = $doc.Content\n$range.InsertAfter(\"`n\" + $text)"
	}

	psScript := fmt.Sprintf(`
$word = [Runtime.Interopservices.Marshal]::GetActiveObject('Word.Application')
$doc = $word.ActiveDocument
if ($doc -eq $null) { Write-Output '{"error":"no active document"}'; exit }

$bytes = [System.Convert]::FromBase64String('%s')
$text = [System.Text.Encoding]::UTF8.GetString($bytes)

%s

Write-Output "ok|$($text.Length)|%s"
`, textB64, writeCode, mode)

	out, err := runPowerShell(ctx, psScript)
	if err != nil {
		return "", fmt.Errorf("word_write failed: %w\n%.500s", err, out)
	}

	preview := a.Text
	if len(preview) > 80 {
		preview = preview[:80] + "..."
	}
	return toJSON(map[string]interface{}{
		"action":  "word_write",
		"status":  "success",
		"mode":    mode,
		"message": fmt.Sprintf("已%s Word 内容: \"%s\"", map[string]string{"append": "追加", "replace": "替换", "insert": "插入"}[mode], preview),
	}), nil
}

// ── word_format: Apply formatting to Word selection ──

func (t *DesktopTool) wordFormat(ctx context.Context, a desktopArgs) (string, error) {
	// text = format commands: "bold", "italic", "underline", "fontsize:16", "fontname:微软雅黑", "heading:1"
	if a.Text == "" {
		return "", fmt.Errorf("word_format requires 'text' (format like 'bold', 'fontsize:16', 'heading:1')")
	}

	cmds := strings.Split(a.Text, ",")
	var psLines []string
	for _, cmd := range cmds {
		cmd = strings.TrimSpace(cmd)
		parts := strings.SplitN(cmd, ":", 2)
		key := strings.ToLower(parts[0])
		val := ""
		if len(parts) > 1 {
			val = parts[1]
		}

		switch key {
		case "bold":
			psLines = append(psLines, `$sel.Font.Bold = -1`)
		case "italic":
			psLines = append(psLines, `$sel.Font.Italic = -1`)
		case "underline":
			psLines = append(psLines, `$sel.Font.Underline = 1`)
		case "fontsize":
			psLines = append(psLines, fmt.Sprintf(`$sel.Font.Size = %s`, val))
		case "fontname":
			psLines = append(psLines, fmt.Sprintf(`$sel.Font.Name = '%s'`, val))
		case "color":
			psLines = append(psLines, fmt.Sprintf(`$sel.Font.Color = %s`, val))
		case "heading":
			psLines = append(psLines, fmt.Sprintf(`$sel.Style = "Heading %s"`, val))
		case "align":
			alignMap := map[string]string{"left": "0", "center": "1", "right": "2", "justify": "3"}
			if v, ok := alignMap[strings.ToLower(val)]; ok {
				psLines = append(psLines, fmt.Sprintf(`$sel.ParagraphFormat.Alignment = %s`, v))
			}
		}
	}

	if len(psLines) == 0 {
		return "", fmt.Errorf("unrecognized format: %s (try: bold, italic, fontsize:16, heading:1)", a.Text)
	}

	psScript := fmt.Sprintf(`
$word = [Runtime.Interopservices.Marshal]::GetActiveObject('Word.Application')
$sel = $word.Selection
%s
Write-Output "formatted: %s"
`, strings.Join(psLines, "\n"), a.Text)

	out, err := runPowerShell(ctx, psScript)
	if err != nil {
		return "", fmt.Errorf("word_format failed: %w\n%.500s", err, out)
	}

	return toJSON(map[string]interface{}{
		"action":  "word_format",
		"status":  "success",
		"format":  a.Text,
		"message": fmt.Sprintf("已应用格式: %s", a.Text),
	}), nil
}

// ── file_list: List files in a directory ──

func (t *DesktopTool) fileList(ctx context.Context, a desktopArgs) (string, error) {
	dir := a.Text
	if dir == "" {
		dir = "."
	}

	psScript := fmt.Sprintf(`
$items = Get-ChildItem -Path '%s' -ErrorAction Stop | Select-Object Name, Length, LastWriteTime, @{N='Type';E={if($_.PSIsContainer){'dir'}else{'file'}}} | ForEach-Object {
    @{name=$_.Name; size=$_.Length; modified=$_.LastWriteTime.ToString('yyyy-MM-dd HH:mm'); type=$_.Type}
}
$result = @{ path='%s'; count=$items.Count; items=$items }
Write-Output ($result | ConvertTo-Json -Depth 3 -Compress)
`, strings.ReplaceAll(dir, "'", "''"), dir)

	out, err := runPowerShell(ctx, psScript)
	if err != nil {
		return "", fmt.Errorf("file_list failed: %w\n%.300s", err, out)
	}

	out = strings.TrimSpace(out)
	var parsed interface{}
	if json.Unmarshal([]byte(out), &parsed) == nil {
		return toJSON(map[string]interface{}{
			"action": "file_list", "status": "success",
			"data": parsed, "message": fmt.Sprintf("已列出 %s 的文件", dir),
		}), nil
	}
	return toJSON(map[string]interface{}{
		"action": "file_list", "status": "success", "raw": out,
	}), nil
}

// ── file_read: Read file content ──

func (t *DesktopTool) fileRead(ctx context.Context, a desktopArgs) (string, error) {
	if a.Text == "" {
		return "", fmt.Errorf("file_read requires 'text' (file path)")
	}

	psScript := fmt.Sprintf(`
$content = Get-Content -Path '%s' -Raw -Encoding UTF8 -ErrorAction Stop
if ($content.Length -gt 5000) { $content = $content.Substring(0, 5000) + "...(truncated at 5000 chars)" }
$info = Get-Item '%s'
$result = @{
    path = $info.FullName
    size = $info.Length
    modified = $info.LastWriteTime.ToString('yyyy-MM-dd HH:mm')
    content = $content
}
Write-Output ($result | ConvertTo-Json -Compress)
`, strings.ReplaceAll(a.Text, "'", "''"), strings.ReplaceAll(a.Text, "'", "''"))

	out, err := runPowerShell(ctx, psScript)
	if err != nil {
		return "", fmt.Errorf("file_read failed: %w\n%.300s", err, out)
	}

	out = strings.TrimSpace(out)
	var parsed interface{}
	if json.Unmarshal([]byte(out), &parsed) == nil {
		return toJSON(map[string]interface{}{
			"action": "file_read", "status": "success",
			"data": parsed, "message": fmt.Sprintf("已读取 %s", a.Text),
		}), nil
	}
	return toJSON(map[string]interface{}{
		"action": "file_read", "status": "success", "raw": out,
	}), nil
}

// ── file_write: Write content to a file ──

func (t *DesktopTool) fileWrite(ctx context.Context, a desktopArgs) (string, error) {
	if a.Title == "" || a.Text == "" {
		return "", fmt.Errorf("file_write requires 'title' (file path) and 'text' (content)")
	}

	textB64 := encodeBase64(a.Text)
	psScript := fmt.Sprintf(`
$bytes = [System.Convert]::FromBase64String('%s')
$content = [System.Text.Encoding]::UTF8.GetString($bytes)
[System.IO.File]::WriteAllText('%s', $content, [System.Text.Encoding]::UTF8)
$info = Get-Item '%s'
Write-Output "wrote $($info.Length) bytes"
`, textB64, strings.ReplaceAll(a.Title, "'", "''"), strings.ReplaceAll(a.Title, "'", "''"))

	out, err := runPowerShell(ctx, psScript)
	if err != nil {
		return "", fmt.Errorf("file_write failed: %w\n%.300s", err, out)
	}

	return toJSON(map[string]interface{}{
		"action":  "file_write",
		"status":  "success",
		"path":    a.Title,
		"message": fmt.Sprintf("已写入 %s (%s)", a.Title, strings.TrimSpace(out)),
	}), nil
}
