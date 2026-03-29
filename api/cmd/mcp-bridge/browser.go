package main

import (
	"encoding/base64"
	"fmt"
	"log"
	"sync"

	"github.com/playwright-community/playwright-go"
)

// --- Browser automation via Playwright ---

var (
	pwOnce     sync.Once
	pwInstance *playwright.Playwright
	pwBrowser  playwright.Browser
	pwPage     playwright.Page
	pwMu       sync.Mutex
	pwErr      error
)

// ensureBrowser lazily initializes Playwright + Chromium on first use.
func ensureBrowser() (playwright.Page, error) {
	pwMu.Lock()
	defer pwMu.Unlock()

	if pwPage != nil {
		return pwPage, nil
	}

	pwOnce.Do(func() {
		log.Println("[mcp-bridge] Initializing Playwright browser...")

		// Install browsers if needed (first run)
		installErr := playwright.Install(&playwright.RunOptions{
			Browsers: []string{"chromium"},
			Verbose:  false,
		})
		if installErr != nil {
			log.Printf("[mcp-bridge] Playwright install note: %v (may already be installed)", installErr)
		}

		pw, err := playwright.Run()
		if err != nil {
			pwErr = fmt.Errorf("failed to start Playwright: %w", err)
			return
		}
		pwInstance = pw

		browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
			Headless: playwright.Bool(false),
			Args:     []string{"--disable-blink-features=AutomationControlled"},
		})
		if err != nil {
			pwErr = fmt.Errorf("failed to launch browser: %w", err)
			return
		}
		pwBrowser = browser

		page, err := browser.NewPage()
		if err != nil {
			pwErr = fmt.Errorf("failed to create page: %w", err)
			return
		}
		pwPage = page
		log.Println("[mcp-bridge] Playwright browser ready")
	})

	if pwErr != nil {
		return nil, pwErr
	}
	return pwPage, nil
}

// --- Browser tool definitions ---

func getBrowserTools() []toolDef {
	return []toolDef{
		{
			Name:        "browser_navigate",
			Description: "在 Playwright 浏览器中打开指定 URL。可用于浏览网页、Web 应用（如微信网页版等）。",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"url": map[string]interface{}{
						"type":        "string",
						"description": "要打开的 URL",
					},
				},
				"required": []string{"url"},
			},
		},
		{
			Name:        "browser_snapshot",
			Description: "获取当前页面的无障碍快照（accessibility snapshot），返回结构化文本描述页面上所有可交互元素及其 ref 标识。用于「看懂」页面内容，不需要视觉模型。",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "browser_click",
			Description: "点击页面上的元素。需要先用 browser_snapshot 获取元素的 ref 标识。",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"ref": map[string]interface{}{
						"type":        "string",
						"description": "元素的 ref 标识（从 browser_snapshot 获取）",
					},
				},
				"required": []string{"ref"},
			},
		},
		{
			Name:        "browser_type",
			Description: "在页面元素中输入文本。需要先用 browser_snapshot 获取元素的 ref 标识。",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"ref": map[string]interface{}{
						"type":        "string",
						"description": "输入框元素的 ref 标识（从 browser_snapshot 获取）",
					},
					"text": map[string]interface{}{
						"type":        "string",
						"description": "要输入的文本",
					},
					"submit": map[string]interface{}{
						"type":        "boolean",
						"description": "输入后是否按 Enter 提交（默认 false）",
					},
				},
				"required": []string{"ref", "text"},
			},
		},
		{
			Name:        "browser_screenshot",
			Description: "截取当前页面的屏幕截图（返回 base64 PNG）。",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "browser_back",
			Description: "浏览器后退到上一页。",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "browser_press_key",
			Description: "在页面上按下键盘按键。如 Enter、Tab、Escape、ArrowDown 等。",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"key": map[string]interface{}{
						"type":        "string",
						"description": "按键名称，如 Enter、Tab、Escape、ArrowDown、Control+a",
					},
				},
				"required": []string{"key"},
			},
		},
		{
			Name:        "browser_close",
			Description: "关闭 Playwright 浏览器。",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
	}
}

// --- Browser tool implementations ---

func execBrowserNavigate(args map[string]interface{}) mcpToolResult {
	url, _ := args["url"].(string)
	if url == "" {
		return errResult("url is required")
	}

	page, err := ensureBrowser()
	if err != nil {
		return errResult("browser init failed: " + err.Error())
	}

	resp, err := page.Goto(url, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
	})
	if err != nil {
		return errResult("navigation failed: " + err.Error())
	}

	status := 0
	if resp != nil {
		status = resp.Status()
	}
	title, _ := page.Title()
	return textResult(fmt.Sprintf("Navigated to: %s\nTitle: %s\nStatus: %d\nNEXT: call browser_snapshot to see page content.", url, title, status))
}

func execBrowserSnapshot(args map[string]interface{}) mcpToolResult {
	page, err := ensureBrowser()
	if err != nil {
		return errResult("browser init failed: " + err.Error())
	}

	// Use JavaScript to extract a structured accessibility-like snapshot
	jsCode := `() => {
		function walk(el, depth) {
			if (!el || depth > 6) return '';
			const tag = el.tagName?.toLowerCase() || '';
			const skip = ['script','style','svg','path','noscript','meta','link','head'];
			if (skip.includes(tag)) return '';
			let lines = [];
			const role = el.getAttribute('role') || tag;
			const label = el.getAttribute('aria-label') || el.getAttribute('alt') || el.getAttribute('placeholder') || '';
			const text = el.childNodes.length === 1 && el.childNodes[0].nodeType === 3 ? el.textContent.trim() : '';
			const href = el.getAttribute('href') || '';
			const type = el.getAttribute('type') || '';
			const val = el.value || '';
			const indent = '  '.repeat(depth);
			let desc = role;
			if (label) desc += ' "' + label.substring(0,80) + '"';
			else if (text && text.length < 100) desc += ' "' + text.substring(0,80) + '"';
			if (href && href.length < 120) desc += ' [href=' + href + ']';
			if (type) desc += ' [type=' + type + ']';
			if (val && val.length < 80) desc += ' [value=' + val + ']';
			if (el.id) desc += ' #' + el.id;
			const isInteractive = ['a','button','input','select','textarea'].includes(tag) || el.getAttribute('role') === 'button' || el.onclick;
			if (isInteractive || text || label) lines.push(indent + (isInteractive ? '→ ' : '  ') + desc);
			for (const child of el.children || []) {
				const sub = walk(child, depth + 1);
				if (sub) lines.push(sub);
			}
			return lines.join('\n');
		}
		return walk(document.body, 0);
	}`

	result, err := page.Evaluate(jsCode)
	if err != nil {
		return errResult("snapshot failed: " + err.Error())
	}

	snapshot := fmt.Sprintf("%v", result)
	if len(snapshot) > 30000 {
		snapshot = snapshot[:30000] + "\n...[truncated]"
	}

	url := page.URL()
	title, _ := page.Title()
	header := fmt.Sprintf("Page: %s\nURL: %s\n---\nInteractive elements marked with →\n\n", title, url)
	return textResult(header + snapshot)
}

func execBrowserClick(args map[string]interface{}) mcpToolResult {
	ref, _ := args["ref"].(string)
	if ref == "" {
		return errResult("ref is required (get it from browser_snapshot)")
	}

	page, err := ensureBrowser()
	if err != nil {
		return errResult("browser init failed: " + err.Error())
	}

	// Use the accessibility ref to find and click the element
	// Playwright's locator system: use ARIA snapshot refs
	locator := page.GetByTestId(ref)
	count, _ := locator.Count()
	if count == 0 {
		// Fallback: try as text content
		locator = page.GetByText(ref)
		count, _ = locator.Count()
	}
	if count == 0 {
		// Fallback: try as role + name from the ref pattern
		locator = page.Locator(fmt.Sprintf("[data-ref='%s']", ref))
	}

	err = locator.First().Click()
	if err != nil {
		return errResult(fmt.Sprintf("click failed for ref '%s': %s. Try using browser_snapshot to get correct refs.", ref, err.Error()))
	}
	return textResult(fmt.Sprintf("Clicked element: %s\nNEXT: call browser_snapshot to see updated page.", ref))
}

func execBrowserType(args map[string]interface{}) mcpToolResult {
	ref, _ := args["ref"].(string)
	text, _ := args["text"].(string)
	submit, _ := args["submit"].(bool)

	if ref == "" || text == "" {
		return errResult("ref and text are required")
	}

	page, err := ensureBrowser()
	if err != nil {
		return errResult("browser init failed: " + err.Error())
	}

	locator := page.GetByTestId(ref)
	count, _ := locator.Count()
	if count == 0 {
		locator = page.Locator(fmt.Sprintf("[data-ref='%s']", ref))
	}

	err = locator.First().Fill(text)
	if err != nil {
		return errResult(fmt.Sprintf("type failed for ref '%s': %s", ref, err.Error()))
	}

	if submit {
		err = locator.First().Press("Enter")
		if err != nil {
			return errResult("type succeeded but Enter press failed: " + err.Error())
		}
	}

	action := "Typed"
	if submit {
		action = "Typed and submitted"
	}
	return textResult(fmt.Sprintf("%s '%s' in element: %s\nNEXT: call browser_snapshot to see result.", action, text, ref))
}

func execBrowserScreenshot(args map[string]interface{}) mcpToolResult {
	page, err := ensureBrowser()
	if err != nil {
		return errResult("browser init failed: " + err.Error())
	}

	data, err := page.Screenshot(playwright.PageScreenshotOptions{
		FullPage: playwright.Bool(false),
	})
	if err != nil {
		return errResult("screenshot failed: " + err.Error())
	}

	b64 := base64.StdEncoding.EncodeToString(data)
	return mcpToolResult{
		Content: []mcpContent{{Type: "image", Data: b64, MimeType: "image/png"}},
	}
}

func execBrowserBack() mcpToolResult {
	page, err := ensureBrowser()
	if err != nil {
		return errResult("browser init failed: " + err.Error())
	}

	_, err = page.GoBack()
	if err != nil {
		return errResult("back failed: " + err.Error())
	}
	title, _ := page.Title()
	return textResult(fmt.Sprintf("Went back. Current page: %s\nNEXT: call browser_snapshot to see page.", title))
}

func execBrowserPressKey(args map[string]interface{}) mcpToolResult {
	key, _ := args["key"].(string)
	if key == "" {
		return errResult("key is required")
	}

	page, err := ensureBrowser()
	if err != nil {
		return errResult("browser init failed: " + err.Error())
	}

	err = page.Keyboard().Press(key)
	if err != nil {
		return errResult("key press failed: " + err.Error())
	}
	return textResult(fmt.Sprintf("Pressed key: %s", key))
}

func execBrowserClose() mcpToolResult {
	pwMu.Lock()
	defer pwMu.Unlock()

	if pwBrowser != nil {
		pwBrowser.Close()
		pwBrowser = nil
	}
	if pwInstance != nil {
		pwInstance.Stop()
		pwInstance = nil
	}
	pwPage = nil
	pwOnce = sync.Once{} // allow re-init
	return textResult("Browser closed.")
}
