package router

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
)

// renderFileToStyledHTML reads a file and returns a fully styled HTML document.
func renderFileToStyledHTML(absPath string) (string, error) {
	data, err := os.ReadFile(absPath)
	if err != nil {
		return "", err
	}
	content := string(data)
	ext := strings.ToLower(filepath.Ext(absPath))

	switch ext {
	case ".md", ".markdown":
		return markdownToStyledHTML(content, absPath), nil
	case ".html", ".htm":
		if looksLikeMarkdown(content) {
			return markdownToStyledHTML(content, absPath), nil
		}
		if !strings.Contains(content, "bp-style.css") && !strings.Contains(content, "<style>") {
			content = injectCSSIntoHTML(content)
		}
		return content, nil
	default:
		escaped := strings.ReplaceAll(content, "<", "&lt;")
		escaped = strings.ReplaceAll(escaped, ">", "&gt;")
		return fmt.Sprintf(`<!DOCTYPE html><html><head><meta charset="UTF-8"><style>%s pre{white-space:pre-wrap;font-size:0.9em;}</style></head><body><pre>%s</pre></body></html>`, bpStyleCSS, escaped), nil
	}
}

func looksLikeMarkdown(content string) bool {
	lines := strings.Split(content, "\n")
	mdSignals := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") || strings.HasPrefix(trimmed, "## ") || strings.HasPrefix(trimmed, "### ") {
			mdSignals++
		}
		if strings.HasPrefix(trimmed, "- **") || strings.HasPrefix(trimmed, "- ") {
			mdSignals++
		}
		if mdSignals >= 3 {
			return true
		}
	}
	return false
}

// sectionIcons maps section keywords to emoji icons for h2 headers
var sectionIcons = []struct {
	keywords []string
	icon     string
}{
	{[]string{"执行摘要", "摘要", "概述", "summary", "overview"}, "📋"},
	{[]string{"市场", "行业", "market", "industry"}, "📊"},
	{[]string{"产品", "服务", "技术", "product", "service", "tech"}, "🚀"},
	{[]string{"商业模式", "盈利", "收入", "business model", "revenue"}, "💰"},
	{[]string{"竞争", "竞品", "对比", "competition", "competitor"}, "⚔️"},
	{[]string{"团队", "人才", "组织", "team", "talent"}, "👥"},
	{[]string{"财务", "预测", "财报", "financial", "forecast"}, "📈"},
	{[]string{"融资", "投资", "资金", "funding", "investment"}, "🏦"},
	{[]string{"风险", "挑战", "risk", "challenge"}, "⚠️"},
	{[]string{"里程碑", "路线图", "规划", "milestone", "roadmap", "plan"}, "🗓️"},
	{[]string{"营销", "推广", "获客", "marketing", "growth"}, "📣"},
	{[]string{"用户", "客户", "目标", "user", "customer", "target"}, "🎯"},
	{[]string{"愿景", "使命", "价值", "vision", "mission"}, "🌟"},
	{[]string{"痛点", "需求", "问题", "pain", "problem", "need"}, "💡"},
	{[]string{"优势", "壁垒", "护城河", "advantage", "moat"}, "🛡️"},
	{[]string{"退出", "回报", "exit", "return"}, "🎯"},
	{[]string{"附录", "参考", "appendix", "reference"}, "📎"},
}

func getIconForHeading(text string) string {
	lower := strings.ToLower(text)
	for _, si := range sectionIcons {
		for _, kw := range si.keywords {
			if strings.Contains(lower, kw) {
				return si.icon
			}
		}
	}
	return "📌"
}

// markdownToStyledHTML converts markdown text to a professional styled HTML document
func markdownToStyledHTML(mdContent string, filePath string) string {
	// Strip HTML wrapper if present
	if idx := strings.Index(mdContent, "<body>"); idx >= 0 {
		end := strings.Index(mdContent, "</body>")
		if end > idx {
			mdContent = mdContent[idx+6 : end]
		}
	}

	// Convert markdown to HTML with table support
	md := goldmark.New(
		goldmark.WithExtensions(extension.Table),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
	)
	var buf bytes.Buffer
	_ = md.Convert([]byte(mdContent), &buf)
	htmlBody := buf.String()

	// Extract title from first h1
	title := "商业计划书"
	h1Re := regexp.MustCompile(`<h1[^>]*>(.*?)</h1>`)
	if m := h1Re.FindStringSubmatch(htmlBody); len(m) > 1 {
		title = strings.TrimSpace(m[1])
	} else if name := filepath.Base(filePath); name != "" {
		title = strings.TrimSuffix(name, filepath.Ext(name))
		title = strings.ReplaceAll(title, "_", " ")
	}

	// Post-process: add icons to h2 headings
	h2Re := regexp.MustCompile(`<h2([^>]*)>(.*?)</h2>`)
	htmlBody = h2Re.ReplaceAllStringFunc(htmlBody, func(match string) string {
		sub := h2Re.FindStringSubmatch(match)
		if len(sub) < 3 {
			return match
		}
		icon := getIconForHeading(sub[2])
		return fmt.Sprintf(`<h2%s><span class="section-icon">%s</span> %s</h2>`, sub[1], icon, sub[2])
	})

	// Post-process: add icons to h3 headings
	h3Re := regexp.MustCompile(`<h3([^>]*)>(.*?)</h3>`)
	htmlBody = h3Re.ReplaceAllStringFunc(htmlBody, func(match string) string {
		sub := h3Re.FindStringSubmatch(match)
		if len(sub) < 3 {
			return match
		}
		return fmt.Sprintf(`<h3%s>▀%s</h3>`, sub[1], sub[2])
	})

	// Post-process: convert blockquotes to styled info-boxes
	htmlBody = strings.ReplaceAll(htmlBody, "<blockquote>", `<div class="info-box">`)
	htmlBody = strings.ReplaceAll(htmlBody, "</blockquote>", `</div>`)

	// Post-process: replace first h1 with cover page
	if m := h1Re.FindStringIndex(htmlBody); m != nil {
		titleMatch := h1Re.FindStringSubmatch(htmlBody)
		coverHTML := fmt.Sprintf(`<div class="cover">
<div class="cover-icon">📊</div>
<h1>%s</h1>
<p class="subtitle">商业计划书 · Business Plan</p>
<div class="cover-line"></div>
<p class="meta">%s · 机密文件</p>
</div>
<div class="toc-divider"></div>`, titleMatch[1], time.Now().Format("2006年1月"))
		htmlBody = htmlBody[:m[0]] + coverHTML + htmlBody[m[1]:]
	}

	// Post-process: add section dividers before each h2
	htmlBody = strings.ReplaceAll(htmlBody, "<h2", `<div class="section-divider"></div><h2`)

	// Post-process: style strong numbers as accent
	numRe := regexp.MustCompile(`<strong>([^<]*\d+[^<]*)</strong>`)
	htmlBody = numRe.ReplaceAllString(htmlBody, `<strong class="num-accent">$1</strong>`)

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<title>%s</title>
<style>%s</style>
</head>
<body>
%s
<div class="footer">
<div class="footer-line"></div>
<p>📄 机密文件 · 仅供内部使用 · %s</p>
</div>
</body>
</html>`, title, bpStyleCSS, htmlBody, time.Now().Format("2006-01-02"))
}

func injectCSSIntoHTML(html string) string {
	cssTag := fmt.Sprintf("<style>%s</style>", bpStyleCSS)
	if idx := strings.Index(html, "</head>"); idx >= 0 {
		return html[:idx] + cssTag + html[idx:]
	}
	if idx := strings.Index(html, "<body>"); idx >= 0 {
		return html[:idx] + "<head>" + cssTag + "</head>" + html[idx:]
	}
	return cssTag + html
}

const bpStyleCSS = `
@page { size: A4; margin: 1.5cm 2cm; }
* { box-sizing: border-box; }
body { font-family: "PingFang SC", "Microsoft YaHei", "Noto Sans CJK SC", "Noto Sans SC", "Helvetica Neue", sans-serif; color: #1a1a2e; line-height: 1.9; max-width: 860px; margin: 0 auto; padding: 0 50px 40px; background: #fff; }

/* ========== 封面 ========== */
.cover { text-align: center; padding: 120px 40px 80px; margin-bottom: 20px; background: linear-gradient(160deg, #f8f9ff 0%, #eef1ff 40%, #f0e6ff 100%); border-radius: 16px; position: relative; overflow: hidden; }
.cover::before { content: ""; position: absolute; top: -60px; right: -60px; width: 200px; height: 200px; background: radial-gradient(circle, rgba(102,126,234,0.15) 0%, transparent 70%); border-radius: 50%; }
.cover::after { content: ""; position: absolute; bottom: -40px; left: -40px; width: 160px; height: 160px; background: radial-gradient(circle, rgba(118,75,162,0.12) 0%, transparent 70%); border-radius: 50%; }
.cover-icon { font-size: 4em; margin-bottom: 20px; filter: drop-shadow(0 4px 8px rgba(0,0,0,0.1)); }
.cover h1 { font-size: 2.6em; border: none; margin-bottom: 15px; background: linear-gradient(135deg, #667eea, #764ba2); -webkit-background-clip: text; -webkit-text-fill-color: transparent; background-clip: text; padding: 0; line-height: 1.3; position: relative; }
.cover .subtitle { font-size: 1.15em; color: #666; letter-spacing: 3px; font-weight: 300; }
.cover-line { width: 80px; height: 3px; background: linear-gradient(90deg, #667eea, #764ba2); margin: 25px auto; border-radius: 2px; }
.cover .meta { margin-top: 25px; color: #999; font-size: 0.95em; }
.toc-divider { height: 40px; }

/* ========== 章节标题 ========== */
h1 { font-size: 2em; color: #0f3460; border-bottom: 3px solid #0f3460; padding-bottom: 12px; text-align: center; margin-top: 50px; }
h2 { font-size: 1.45em; color: #16213e; margin-top: 45px; padding: 14px 20px; background: linear-gradient(135deg, rgba(102,126,234,0.08), rgba(118,75,162,0.05)); border-left: 5px solid #667eea; border-radius: 0 12px 12px 0; position: relative; }
.section-icon { font-size: 1.1em; margin-right: 4px; }
h3 { font-size: 1.12em; color: #2d3561; margin-top: 28px; padding: 6px 0; border-bottom: 1px dashed #ddd; }
.section-divider { height: 1px; background: linear-gradient(90deg, transparent, #ddd, transparent); margin: 35px 0 5px; }

/* ========== 正文 ========== */
p { margin: 12px 0; text-align: justify; color: #333; }
ul, ol { margin: 12px 0; padding-left: 8px; list-style: none; }
li { margin: 8px 0; padding: 6px 12px 6px 32px; position: relative; background: rgba(102,126,234,0.03); border-radius: 6px; border-left: 3px solid transparent; transition: border-color 0.2s; }
li:hover { border-left-color: #667eea; }
ul > li::before { content: "▀; color: #667eea; font-weight: bold; position: absolute; left: 12px; top: 6px; }
ol { counter-reset: li-counter; }
ol > li { counter-increment: li-counter; }
ol > li::before { content: counter(li-counter); background: linear-gradient(135deg, #667eea, #764ba2); color: white; font-size: 0.75em; font-weight: bold; width: 20px; height: 20px; border-radius: 50%; display: flex; align-items: center; justify-content: center; position: absolute; left: 8px; top: 9px; }
strong { color: #0f3460; }
.num-accent { color: #fff; background: linear-gradient(135deg, #667eea, #764ba2); padding: 1px 8px; border-radius: 4px; font-size: 0.95em; }

/* ========== 表格 ========== */
table { width: 100%; border-collapse: separate; border-spacing: 0; margin: 24px 0; font-size: 0.9em; border-radius: 10px; overflow: hidden; box-shadow: 0 2px 12px rgba(0,0,0,0.08); }
thead { background: linear-gradient(135deg, #0f3460, #16537e); }
th { color: white; padding: 14px 16px; text-align: left; font-weight: 600; letter-spacing: 0.5px; }
td { padding: 12px 16px; border-bottom: 1px solid #eef0f5; color: #444; }
tr:nth-child(even) { background: #f8f9fd; }
tr:last-child td { border-bottom: none; }
tbody tr:hover { background: #eef3ff; }

/* ========== 信息桀========== */
.info-box { background: linear-gradient(135deg, #e8f4fd, #f0f7ff); border-left: 4px solid #2196f3; padding: 16px 20px; margin: 18px 0; border-radius: 0 10px 10px 0; position: relative; }
.info-box::before { content: "💡"; position: absolute; left: -14px; top: -8px; font-size: 1.2em; background: white; border-radius: 50%; padding: 2px; }
.info-box p { margin: 4px 0; }

/* ========== 数据高亮卡片 ========== */
.metric-cards { display: flex; gap: 16px; flex-wrap: wrap; margin: 24px 0; }
.metric-card { flex: 1; min-width: 170px; padding: 22px 18px; border-radius: 14px; text-align: center; color: white; position: relative; overflow: hidden; box-shadow: 0 4px 20px rgba(0,0,0,0.12); }
.metric-card:nth-child(1) { background: linear-gradient(135deg, #667eea, #764ba2); }
.metric-card:nth-child(2) { background: linear-gradient(135deg, #f093fb, #f5576c); }
.metric-card:nth-child(3) { background: linear-gradient(135deg, #4facfe, #00f2fe); }
.metric-card:nth-child(4) { background: linear-gradient(135deg, #43e97b, #38f9d7); }
.metric-card::after { content: ""; position: absolute; top: -20px; right: -20px; width: 80px; height: 80px; background: rgba(255,255,255,0.1); border-radius: 50%; }
.metric-card .value { font-size: 1.9em; font-weight: bold; display: block; text-shadow: 0 2px 4px rgba(0,0,0,0.15); }
.metric-card .label { font-size: 0.85em; opacity: 0.9; margin-top: 6px; display: block; }

/* ========== CSS 条形囀========== */
.bar-chart { margin: 24px 0; padding: 16px; background: #fafbff; border-radius: 12px; }
.bar-item { display: flex; align-items: center; margin: 10px 0; }
.bar-label { width: 130px; font-size: 0.88em; font-weight: 600; flex-shrink: 0; color: #333; }
.bar-track { flex: 1; height: 26px; background: #e8eaf0; border-radius: 13px; overflow: hidden; box-shadow: inset 0 1px 3px rgba(0,0,0,0.08); }
.bar-fill { height: 100%; border-radius: 13px; display: flex; align-items: center; padding-left: 12px; color: white; font-size: 0.78em; font-weight: bold; background: linear-gradient(90deg, #667eea, #764ba2); }

/* ========== 时间纀========== */
.timeline { position: relative; padding-left: 35px; margin: 24px 0; }
.timeline::before { content: ""; position: absolute; left: 14px; top: 0; bottom: 0; width: 3px; background: linear-gradient(180deg, #667eea, #764ba2, #f093fb); border-radius: 2px; }
.timeline-item { position: relative; margin: 24px 0; padding: 14px 18px; background: #fafbff; border-radius: 10px; border: 1px solid #eef0f5; }
.timeline-item::before { content: ""; position: absolute; left: -27px; top: 18px; width: 14px; height: 14px; background: white; border: 3px solid #667eea; border-radius: 50%; box-shadow: 0 0 0 4px rgba(102,126,234,0.15); }
.timeline-item .date { font-weight: bold; color: #667eea; font-size: 0.9em; }

/* ========== 竞品对比 ========== */
.check { color: #4caf50; font-weight: bold; font-size: 1.3em; }
.cross { color: #f44336; font-weight: bold; font-size: 1.3em; }

/* ========== 高亮桀========== */
.highlight-box { background: linear-gradient(135deg, rgba(240,147,251,0.06), rgba(245,87,108,0.06)); border: 1px solid rgba(245,87,108,0.15); border-radius: 12px; padding: 22px; margin: 22px 0; position: relative; }
.highlight-box::before { content: "⭀; position: absolute; top: -10px; left: 16px; font-size: 1.2em; background: white; padding: 0 4px; }
.success-box { background: linear-gradient(135deg, #e8f5e9, #f1f8e9); border-left: 4px solid #4caf50; padding: 16px 20px; margin: 18px 0; border-radius: 0 10px 10px 0; }
.success-box::before { content: "✀; margin-right: 8px; }
.warning-box { background: linear-gradient(135deg, #fff8e1, #fff3e0); border-left: 4px solid #ff9800; padding: 16px 20px; margin: 18px 0; border-radius: 0 10px 10px 0; }
.warning-box::before { content: "⚠️"; margin-right: 8px; }

/* ========== 图表容器 ========== */
.chart-container { text-align: center; margin: 24px auto; padding: 20px; background: #fafbff; border-radius: 12px; border: 1px solid #eef0f5; }
.pie-legend { display: flex; justify-content: center; gap: 20px; flex-wrap: wrap; margin-top: 12px; }
.pie-legend-item { display: flex; align-items: center; gap: 6px; font-size: 0.85em; }
.pie-legend-color { width: 14px; height: 14px; border-radius: 4px; display: inline-block; }

/* ========== 两栏布局 ========== */
.two-col { display: flex; gap: 24px; margin: 24px 0; }
.two-col > div { flex: 1; padding: 16px; background: #fafbff; border-radius: 10px; border: 1px solid #eef0f5; }

/* ========== 页脚 ========== */
.footer { text-align: center; margin-top: 60px; padding-top: 0; color: #999; font-size: 0.85em; }
.footer-line { height: 2px; background: linear-gradient(90deg, transparent, #667eea, #764ba2, transparent); margin-bottom: 16px; border-radius: 1px; }
.footer p { margin: 4px 0; }

/* ========== 代码址========== */
code { background: #f0f2f8; padding: 2px 6px; border-radius: 4px; font-size: 0.88em; color: #e83e8c; }
pre { background: #1a1a2e; color: #e4e6f0; padding: 18px; border-radius: 10px; overflow-x: auto; font-size: 0.85em; line-height: 1.6; }
pre code { background: transparent; color: inherit; padding: 0; }

/* ========== 打印优化 ========== */
@media print {
  body { padding: 0; max-width: none; }
  .cover { page-break-after: always; border-radius: 0; }
  h2 { page-break-after: avoid; }
  table, .metric-cards, .bar-chart { page-break-inside: avoid; }
  .section-divider { margin: 20px 0 0; }
}
`
