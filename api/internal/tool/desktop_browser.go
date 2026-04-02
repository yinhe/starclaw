package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// ── Browser Automation via Chrome DevTools Protocol (CDP) ──
//
// Connects to Chrome/Edge's --remote-debugging-port to provide precise web control.
// Much more reliable than screenshot+click for web tasks:
//   - Navigate to URLs
//   - Click elements by CSS selector
//   - Type into form fields
//   - Read page text/HTML
//   - Execute JavaScript
//   - Take page screenshots
//
// Requires Chrome/Edge launched with: --remote-debugging-port=9222

const cdpDefaultPort = 9222

// cdpSend sends a CDP command via the /json/protocol HTTP endpoint.
func cdpSend(ctx context.Context, port int, method string, params map[string]interface{}) (json.RawMessage, error) {
	if port == 0 {
		port = cdpDefaultPort
	}

	// Get the first page's WebSocket URL
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/json", port))
	if err != nil {
		return nil, fmt.Errorf("CDP not available (is Chrome running with --remote-debugging-port=%d?): %w", port, err)
	}
	defer resp.Body.Close()

	var pages []struct {
		ID    string `json:"id"`
		URL   string `json:"url"`
		Title string `json:"title"`
		Type  string `json:"type"`
		WSURL string `json:"webSocketDebuggerUrl"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&pages); err != nil {
		return nil, fmt.Errorf("CDP parse error: %w", err)
	}

	// Find the first page target
	var targetID string
	for _, p := range pages {
		if p.Type == "page" {
			targetID = p.ID
			break
		}
	}
	if targetID == "" && len(pages) > 0 {
		targetID = pages[0].ID
	}
	if targetID == "" {
		return nil, fmt.Errorf("no browser tabs found")
	}

	// Use HTTP endpoint for simple commands (avoids WebSocket complexity)
	// For CDP commands, we use PowerShell's WebSocket support
	paramsJSON, _ := json.Marshal(params)
	psScript := fmt.Sprintf(`
$ws = New-Object System.Net.WebSockets.ClientWebSocket
$uri = New-Object System.Uri("ws://127.0.0.1:%d/devtools/page/%s")
$ct = New-Object System.Threading.CancellationTokenSource(10000)
$ws.ConnectAsync($uri, $ct.Token).Wait()

$id = 1
$msg = '{"id":' + $id.ToString() + ',"method":"%s","params":%s}'
$bytes = [System.Text.Encoding]::UTF8.GetBytes($msg)
$seg = New-Object System.ArraySegment[byte]($bytes, 0, $bytes.Length)
$ws.SendAsync($seg, [System.Net.WebSockets.WebSocketMessageType]::Text, $true, $ct.Token).Wait()

$buf = New-Object byte[] 1048576
$result = ""
do {
    $recv = $ws.ReceiveAsync((New-Object System.ArraySegment[byte]($buf, 0, $buf.Length)), $ct.Token)
    $recv.Wait()
    $result += [System.Text.Encoding]::UTF8.GetString($buf, 0, $recv.Result.Count)
} while (-not $recv.Result.EndOfMessage)

$ws.CloseAsync([System.Net.WebSockets.WebSocketCloseStatus]::NormalClosure, "", $ct.Token).Wait()
Write-Output $result
`, port, targetID, method, string(paramsJSON))

	out, err := runPowerShell(ctx, psScript)
	if err != nil {
		return nil, fmt.Errorf("CDP command failed: %w\n%.500s", err, out)
	}

	return json.RawMessage(strings.TrimSpace(out)), nil
}

// ensureCDPBrowser checks if a CDP-enabled browser is running, and launches one if not.
func ensureCDPBrowser(ctx context.Context, port int) error {
	if port == 0 {
		port = cdpDefaultPort
	}

	// Check if already running
	client := &http.Client{Timeout: 2 * time.Second}
	if resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/json/version", port)); err == nil {
		resp.Body.Close()
		return nil // Already running
	}

	// Try to find and launch Chrome/Edge with debugging port
	var browserPath string
	if runtime.GOOS == "windows" {
		candidates := []string{
			`C:\Program Files\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
			`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
		}
		for _, c := range candidates {
			if _, err := exec.LookPath(c); err == nil {
				browserPath = c
				break
			}
			// exec.LookPath doesn't work for full paths, check directly
			cmd := hiddenCmd("cmd", "/c", fmt.Sprintf(`if exist "%s" echo FOUND`, c))
			if out, _ := cmd.Output(); strings.Contains(string(out), "FOUND") {
				browserPath = c
				break
			}
		}
	}

	if browserPath == "" {
		return fmt.Errorf("未找到 Chrome 或 Edge 浏览器")
	}

	cmd := hiddenCmdCtx(ctx, browserPath,
		fmt.Sprintf("--remote-debugging-port=%d", port),
		"--no-first-run",
		"--no-default-browser-check",
	)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动浏览器失败: %w", err)
	}

	// Wait for CDP to be ready
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/json/version", port)); err == nil {
			resp.Body.Close()
			return nil
		}
		time.Sleep(300 * time.Millisecond)
	}
	return fmt.Errorf("浏览器已启动但 CDP 端口 %d 未就绪", port)
}

// ── browser_navigate: Navigate to a URL ──

func (t *DesktopTool) browserNavigate(ctx context.Context, a desktopArgs) (string, error) {
	url := a.Text
	if url == "" {
		return "", fmt.Errorf("browser_navigate requires 'text' (URL to navigate to)")
	}
	if !strings.HasPrefix(url, "http") {
		url = "https://" + url
	}

	if err := ensureCDPBrowser(ctx, cdpDefaultPort); err != nil {
		return "", err
	}

	result, err := cdpSend(ctx, cdpDefaultPort, "Page.navigate", map[string]interface{}{"url": url})
	if err != nil {
		return "", fmt.Errorf("navigate failed: %w", err)
	}

	// Wait for page load
	time.Sleep(2 * time.Second)

	// Get current page info
	titleResult, _ := cdpSend(ctx, cdpDefaultPort, "Runtime.evaluate", map[string]interface{}{
		"expression": "document.title",
	})

	title := extractCDPValue(titleResult)

	return toJSON(map[string]interface{}{
		"action":  "browser_navigate",
		"status":  "success",
		"url":     url,
		"title":   title,
		"raw":     string(result),
		"message": fmt.Sprintf("已导航到 %s (标题: %s)", url, title),
	}), nil
}

// ── browser_click: Click an element by CSS selector ──

func (t *DesktopTool) browserClick(ctx context.Context, a desktopArgs) (string, error) {
	selector := a.Text
	if selector == "" {
		return "", fmt.Errorf("browser_click requires 'text' (CSS selector like 'button.submit' or '#login-btn')")
	}

	if err := ensureCDPBrowser(ctx, cdpDefaultPort); err != nil {
		return "", err
	}

	// Use JavaScript to find and click the element
	js := fmt.Sprintf(`
(function() {
    var el = document.querySelector('%s');
    if (!el) {
        // Try by text content
        var all = document.querySelectorAll('button, a, [role="button"], input[type="submit"]');
        for (var i = 0; i < all.length; i++) {
            if (all[i].textContent.trim().includes('%s')) { el = all[i]; break; }
        }
    }
    if (!el) return JSON.stringify({status:'not_found'});
    el.click();
    return JSON.stringify({status:'clicked', tag:el.tagName, text:el.textContent.trim().substring(0,50)});
})()
`, strings.ReplaceAll(selector, "'", "\\'"), strings.ReplaceAll(selector, "'", "\\'"))

	result, err := cdpSend(ctx, cdpDefaultPort, "Runtime.evaluate", map[string]interface{}{
		"expression":    js,
		"returnByValue": true,
	})
	if err != nil {
		return "", fmt.Errorf("browser_click failed: %w", err)
	}

	value := extractCDPValue(result)

	var clickResult map[string]string
	if json.Unmarshal([]byte(value), &clickResult) == nil {
		if clickResult["status"] == "not_found" {
			return toJSON(map[string]interface{}{
				"action": "browser_click", "status": "not_found", "selector": selector,
				"message": fmt.Sprintf("未找到元素 \"%s\"。请用 browser_read 查看页面结构。", selector),
			}), nil
		}
		return toJSON(map[string]interface{}{
			"action": "browser_click", "status": "success",
			"selector": selector, "tag": clickResult["tag"], "text": clickResult["text"],
			"message": fmt.Sprintf("已点击 <%s> \"%s\"", clickResult["tag"], clickResult["text"]),
		}), nil
	}

	return toJSON(map[string]interface{}{
		"action": "browser_click", "status": "success", "selector": selector,
		"message": fmt.Sprintf("已点击 \"%s\"", selector),
	}), nil
}

// ── browser_type: Type into an input field by CSS selector ──

func (t *DesktopTool) browserType(ctx context.Context, a desktopArgs) (string, error) {
	selector := a.Title // title = CSS selector
	text := a.Text
	if selector == "" || text == "" {
		return "", fmt.Errorf("browser_type requires 'title' (CSS selector) and 'text' (content to type)")
	}

	if err := ensureCDPBrowser(ctx, cdpDefaultPort); err != nil {
		return "", err
	}

	escapedText := strings.ReplaceAll(text, "\\", "\\\\")
	escapedText = strings.ReplaceAll(escapedText, "'", "\\'")
	escapedText = strings.ReplaceAll(escapedText, "\n", "\\n")

	js := fmt.Sprintf(`
(function() {
    var el = document.querySelector('%s');
    if (!el) {
        // Try by placeholder or label
        var inputs = document.querySelectorAll('input, textarea, [contenteditable]');
        for (var i = 0; i < inputs.length; i++) {
            var p = inputs[i].placeholder || inputs[i].getAttribute('aria-label') || '';
            if (p.includes('%s')) { el = inputs[i]; break; }
        }
    }
    if (!el) return JSON.stringify({status:'not_found'});
    el.focus();
    el.value = '%s';
    el.dispatchEvent(new Event('input', {bubbles:true}));
    el.dispatchEvent(new Event('change', {bubbles:true}));
    return JSON.stringify({status:'typed', tag:el.tagName, name:el.name||el.id||''});
})()
`, strings.ReplaceAll(selector, "'", "\\'"),
		strings.ReplaceAll(selector, "'", "\\'"),
		escapedText)

	result, err := cdpSend(ctx, cdpDefaultPort, "Runtime.evaluate", map[string]interface{}{
		"expression":    js,
		"returnByValue": true,
	})
	if err != nil {
		return "", fmt.Errorf("browser_type failed: %w", err)
	}

	value := extractCDPValue(result)
	var typeResult map[string]string
	if json.Unmarshal([]byte(value), &typeResult) == nil {
		if typeResult["status"] == "not_found" {
			return toJSON(map[string]interface{}{
				"action": "browser_type", "status": "not_found", "selector": selector,
				"message": fmt.Sprintf("未找到输入框 \"%s\"", selector),
			}), nil
		}
	}

	preview := text
	if len(preview) > 50 {
		preview = preview[:50] + "..."
	}
	return toJSON(map[string]interface{}{
		"action": "browser_type", "status": "success",
		"selector": selector, "text": preview,
		"message": fmt.Sprintf("已在 \"%s\" 中输入 \"%s\"", selector, preview),
	}), nil
}

// ── browser_read: Read page content (text, title, URL, form fields) ──

func (t *DesktopTool) browserRead(ctx context.Context, a desktopArgs) (string, error) {
	if err := ensureCDPBrowser(ctx, cdpDefaultPort); err != nil {
		return "", err
	}

	what := a.Text // "text", "html", "links", "inputs", or CSS selector
	if what == "" {
		what = "text"
	}

	var js string
	switch what {
	case "text":
		js = `JSON.stringify({title:document.title, url:location.href, text:document.body.innerText.substring(0,3000)})`
	case "links":
		js = `JSON.stringify({title:document.title, url:location.href, links:Array.from(document.querySelectorAll('a[href]')).slice(0,50).map(a=>({text:a.textContent.trim().substring(0,60),href:a.href}))})`
	case "inputs":
		js = `JSON.stringify({title:document.title, url:location.href, inputs:Array.from(document.querySelectorAll('input,textarea,select,button')).slice(0,50).map(el=>({tag:el.tagName,type:el.type||'',name:el.name||el.id||'',placeholder:el.placeholder||'',value:(el.value||'').substring(0,100),text:el.textContent.trim().substring(0,50)}))})`
	default:
		// Treat as CSS selector — read that element's text
		js = fmt.Sprintf(`(function(){var el=document.querySelector('%s');if(!el)return JSON.stringify({error:'not found'});return JSON.stringify({tag:el.tagName,text:el.textContent.trim().substring(0,2000),html:el.innerHTML.substring(0,2000)})})()`,
			strings.ReplaceAll(what, "'", "\\'"))
	}

	result, err := cdpSend(ctx, cdpDefaultPort, "Runtime.evaluate", map[string]interface{}{
		"expression":    js,
		"returnByValue": true,
	})
	if err != nil {
		return "", fmt.Errorf("browser_read failed: %w", err)
	}

	value := extractCDPValue(result)

	var parsed interface{}
	if json.Unmarshal([]byte(value), &parsed) == nil {
		return toJSON(map[string]interface{}{
			"action":  "browser_read",
			"status":  "success",
			"what":    what,
			"data":    parsed,
			"message": fmt.Sprintf("已读取页面内容 (%s)", what),
		}), nil
	}

	return toJSON(map[string]interface{}{
		"action": "browser_read", "status": "success", "what": what,
		"data": value, "message": "已读取页面内容",
	}), nil
}

// ── browser_js: Execute arbitrary JavaScript ──

func (t *DesktopTool) browserJS(ctx context.Context, a desktopArgs) (string, error) {
	js := a.Text
	if js == "" {
		return "", fmt.Errorf("browser_js requires 'text' (JavaScript code to execute)")
	}

	if err := ensureCDPBrowser(ctx, cdpDefaultPort); err != nil {
		return "", err
	}

	result, err := cdpSend(ctx, cdpDefaultPort, "Runtime.evaluate", map[string]interface{}{
		"expression":    js,
		"returnByValue": true,
		"awaitPromise":  true,
	})
	if err != nil {
		return "", fmt.Errorf("browser_js failed: %w", err)
	}

	value := extractCDPValue(result)

	return toJSON(map[string]interface{}{
		"action":  "browser_js",
		"status":  "success",
		"result":  value,
		"message": "JavaScript 执行完成",
	}), nil
}

// ── browser_tabs: List or switch browser tabs ──

func (t *DesktopTool) browserTabs(ctx context.Context, a desktopArgs) (string, error) {
	if err := ensureCDPBrowser(ctx, cdpDefaultPort); err != nil {
		return "", err
	}

	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/json", cdpDefaultPort))
	if err != nil {
		return "", fmt.Errorf("CDP not available: %w", err)
	}
	defer resp.Body.Close()

	var pages []struct {
		ID    string `json:"id"`
		URL   string `json:"url"`
		Title string `json:"title"`
		Type  string `json:"type"`
	}
	json.NewDecoder(resp.Body).Decode(&pages)

	tabs := []map[string]string{}
	for _, p := range pages {
		if p.Type == "page" {
			tabs = append(tabs, map[string]string{
				"id": p.ID, "title": p.Title, "url": p.URL,
			})
		}
	}

	// If title specified, activate that tab
	if a.Title != "" {
		for _, tab := range tabs {
			if strings.Contains(strings.ToLower(tab["title"]), strings.ToLower(a.Title)) ||
				strings.Contains(tab["url"], a.Title) {
				http.Get(fmt.Sprintf("http://127.0.0.1:%d/json/activate/%s", cdpDefaultPort, tab["id"]))
				return toJSON(map[string]interface{}{
					"action": "browser_tabs", "status": "switched",
					"tab": tab, "message": fmt.Sprintf("已切换到标签: %s", tab["title"]),
				}), nil
			}
		}
		return toJSON(map[string]interface{}{
			"action": "browser_tabs", "status": "not_found",
			"tabs": tabs, "message": fmt.Sprintf("未找到标签 \"%s\"", a.Title),
		}), nil
	}

	return toJSON(map[string]interface{}{
		"action": "browser_tabs", "status": "success",
		"count": len(tabs), "tabs": tabs,
		"message": fmt.Sprintf("当前有 %d 个标签页", len(tabs)),
	}), nil
}

// extractCDPValue extracts the string value from a CDP Runtime.evaluate response.
func extractCDPValue(raw json.RawMessage) string {
	var resp struct {
		Result struct {
			Result struct {
				Value interface{} `json:"value"`
				Type  string      `json:"type"`
			} `json:"result"`
		} `json:"result"`
	}
	if json.Unmarshal(raw, &resp) == nil {
		switch v := resp.Result.Result.Value.(type) {
		case string:
			return v
		default:
			b, _ := json.Marshal(v)
			return string(b)
		}
	}
	return string(raw)
}
