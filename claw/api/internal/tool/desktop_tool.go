package tool

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// DesktopTool enables agents to control the user's desktop — take screenshots,
// move/click the mouse, type on the keyboard, manage windows, and launch apps.
// This is the "computer use" capability that lets agents operate 剪映, WPS, 微信, etc.
//
// Only available when Claw runs locally (via Spore). Returns an error in Docker.
type DesktopTool struct{}

func NewDesktopTool() *DesktopTool {
	return &DesktopTool{}
}

func (t *DesktopTool) Name() string { return "desktop" }

func (t *DesktopTool) Description() string {
	return `桌面操控工具 — 让AI直接操作用户的电脑桌面。

★ 推荐工作流（高效模式）：ui_tree 获取元素 → ui_click/ui_type 按名称操作 → ui_tree 确认结果
★ 备用工作流（视觉模式）：screenshot → 视觉模型分析 → mouse_click 坐标操作

UI自动化操作（精确、快速、首选）：
- ui_tree: 获取前台窗口的UI元素树（按钮、输入框、菜单等），返回结构化列表，每个元素有 id、name、type、坐标
- ui_click: 按元素名称点击（title="保存" 会精确找到并点击"保存"按钮）
- ui_type: 向指定输入框填入文本（title="搜索" text="关键词"）
- ui_select: 在下拉框中选择选项（title="分辨率" text="1080P"）
- ui_scroll: 滚动页面（button="down/up/left/right" seconds=滚动量1-10）
- ui_wait: 等待某个元素出现（title="导出完成" seconds=超时秒数）

浏览器操作（网页场景首选，通过 Chrome DevTools Protocol）：
- browser_navigate: 打开网页（text="https://example.com"）
- browser_click: 按CSS选择器或按钮文字点击（text="button.submit" 或 text="登录"）
- browser_type: 在输入框输入（title="input#search" text="关键词"）
- browser_read: 读取页面内容（text="text/links/inputs" 或 CSS选择器）
- browser_js: 执行JavaScript代码
- browser_tabs: 列出/切换标签页

Office 直连（Excel/Word，通过 COM Automation，无需截图）：
- excel_read: 读取 Excel 单元格（text="A1:D20" 或 "Sheet2!A1:C10"）
- excel_write: 写入单元格（title="A1" text="值" 或 title="A1" text='[["a","b"],["c","d"]]' 批量写入）
- excel_formula: 设置公式（title="C1" text="=SUM(A1:B1)"）
- word_read: 读取 Word 文档内容
- word_write: 写入 Word（button="append/replace/insert" text="内容"）
- word_format: 格式化 Word 选中内容（text="bold,fontsize:16,heading:1"）

文件操作：
- file_list: 列出目录文件（text="C:\\Users\\xxx\\Desktop"）
- file_read: 读取文件内容（text="路径"）
- file_write: 写入文件（title="路径" text="内容"）

像素级操作（兜底）：
- screenshot: 截取屏幕截图，返回图片URL
- mouse_click/mouse_move/mouse_drag: 坐标级鼠标操作
- keyboard_type/keyboard_hotkey/keyboard_key: 键盘操作

窗口管理：
- list_windows: 列出所有窗口
- focus_window: 切换窗口到前台
- launch_app: 启动应用（如 剪映、WPS、微信、Chrome）
- wait: 等待指定秒数`
}

func (t *DesktopTool) Parameters() interface{} {
	return JSONSchema{
		Type: "object",
		Properties: map[string]Property{
			"action": {
				Type:        "string",
				Description: "Desktop action to perform",
				Enum:        []string{"ui_tree", "ui_click", "ui_type", "ui_select", "ui_scroll", "ui_wait", "browser_navigate", "browser_click", "browser_type", "browser_read", "browser_js", "browser_tabs", "excel_read", "excel_write", "excel_formula", "word_read", "word_write", "word_format", "file_list", "file_read", "file_write", "screenshot", "mouse_click", "mouse_move", "mouse_drag", "keyboard_type", "keyboard_hotkey", "keyboard_key", "list_windows", "focus_window", "launch_app", "wait"},
			},
			"x":          {Type: "integer", Description: "X coordinate (pixels from left). For mouse_click, mouse_move, mouse_drag (start)."},
			"y":          {Type: "integer", Description: "Y coordinate (pixels from top). For mouse_click, mouse_move, mouse_drag (start)."},
			"x2":         {Type: "integer", Description: "End X coordinate for mouse_drag."},
			"y2":         {Type: "integer", Description: "End Y coordinate for mouse_drag."},
			"button":     {Type: "string", Description: "Mouse button: left (default), right, middle. For mouse_click."},
			"click_type": {Type: "string", Description: "Click type: single (default), double. For mouse_click."},
			"text":       {Type: "string", Description: "Text to type (for keyboard_type), hotkey combo (for keyboard_hotkey like 'ctrl+c'), key name (for keyboard_key like 'enter'), or app path/name (for launch_app)."},
			"title":      {Type: "string", Description: "Window title or partial match (for focus_window)."},
			"region":     {Type: "string", Description: "Screenshot region: 'full' (default) or 'x,y,width,height' for a specific area."},
			"seconds":    {Type: "integer", Description: "Seconds to wait (for wait action). Default 2."},
		},
		Required: []string{"action"},
	}
}

type desktopArgs struct {
	Action    string `json:"action"`
	X         int    `json:"x"`
	Y         int    `json:"y"`
	X2        int    `json:"x2"`
	Y2        int    `json:"y2"`
	Button    string `json:"button"`
	ClickType string `json:"click_type"`
	Text      string `json:"text"`
	Title     string `json:"title"`
	Region    string `json:"region"`
	Seconds   int    `json:"seconds"`
}

func (t *DesktopTool) Execute(ctx context.Context, args string) (string, error) {
	// Desktop control only works on local machines, not in Docker
	if isDockerEnv() {
		return "", fmt.Errorf("桌面操控工具仅在本地运行时可用（Spore 模式）。当前运行在 Docker 容器中，无法控制桌面。")
	}

	if runtime.GOOS != "windows" {
		return "", fmt.Errorf("桌面操控工具目前仅支持 Windows 系统（Spore Desktop）。当前系统: %s", runtime.GOOS)
	}

	var a desktopArgs
	if err := json.Unmarshal([]byte(args), &a); err != nil {
		return "", fmt.Errorf("invalid desktop args: %w", err)
	}

	switch a.Action {
	// UI Automation (precise, fast — preferred)
	case "ui_tree":
		return t.uiTree(ctx, a)
	case "ui_click":
		return t.uiClick(ctx, a)
	case "ui_type":
		return t.uiType(ctx, a)
	case "ui_select":
		return t.uiSelect(ctx, a)
	case "ui_scroll":
		return t.uiScroll(ctx, a)
	case "ui_wait":
		return t.uiWait(ctx, a)
	// Browser CDP (web pages)
	case "browser_navigate":
		return t.browserNavigate(ctx, a)
	case "browser_click":
		return t.browserClick(ctx, a)
	case "browser_type":
		return t.browserType(ctx, a)
	case "browser_read":
		return t.browserRead(ctx, a)
	case "browser_js":
		return t.browserJS(ctx, a)
	case "browser_tabs":
		return t.browserTabs(ctx, a)
	// Office COM Automation
	case "excel_read":
		return t.excelRead(ctx, a)
	case "excel_write":
		return t.excelWrite(ctx, a)
	case "excel_formula":
		return t.excelFormula(ctx, a)
	case "word_read":
		return t.wordRead(ctx, a)
	case "word_write":
		return t.wordWrite(ctx, a)
	case "word_format":
		return t.wordFormat(ctx, a)
	// File system
	case "file_list":
		return t.fileList(ctx, a)
	case "file_read":
		return t.fileRead(ctx, a)
	case "file_write":
		return t.fileWrite(ctx, a)
	// Pixel-level (fallback)
	case "screenshot":
		return t.screenshot(ctx, a)
	case "mouse_click":
		return t.mouseClick(ctx, a)
	case "mouse_move":
		return t.mouseMove(ctx, a)
	case "mouse_drag":
		return t.mouseDrag(ctx, a)
	case "keyboard_type":
		return t.keyboardType(ctx, a)
	case "keyboard_hotkey":
		return t.keyboardHotkey(ctx, a)
	case "keyboard_key":
		return t.keyboardKey(ctx, a)
	// Window management
	case "list_windows":
		return t.listWindows(ctx)
	case "focus_window":
		return t.focusWindow(ctx, a)
	case "launch_app":
		return t.launchApp(ctx, a)
	case "wait":
		return t.wait(a)
	default:
		return "", fmt.Errorf("unknown desktop action: %s", a.Action)
	}
}

// ── Screenshot ──

func (t *DesktopTool) screenshot(ctx context.Context, a desktopArgs) (string, error) {
	screenshotID := uuid.New().String()[:8]
	filename := fmt.Sprintf("screenshot_%s.png", screenshotID)
	savePath := filepath.Join(ImagesDir(), filename)

	// PowerShell script to capture screen
	var psScript string
	if a.Region != "" && a.Region != "full" {
		// Parse region "x,y,width,height"
		parts := strings.Split(a.Region, ",")
		if len(parts) != 4 {
			return "", fmt.Errorf("region format must be 'x,y,width,height', got: %s", a.Region)
		}
		psScript = fmt.Sprintf(`
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing
$x = %s; $y = %s; $w = %s; $h = %s
$bmp = New-Object System.Drawing.Bitmap($w, $h)
$g = [System.Drawing.Graphics]::FromImage($bmp)
$g.CopyFromScreen($x, $y, 0, 0, (New-Object System.Drawing.Size($w, $h)))
$g.Dispose()
$bmp.Save('%s', [System.Drawing.Imaging.ImageFormat]::Png)
$bmp.Dispose()
Write-Output "ok"
`, parts[0], parts[1], parts[2], parts[3], strings.ReplaceAll(savePath, `\`, `\\`))
	} else {
		psScript = fmt.Sprintf(`
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing
$screens = [System.Windows.Forms.Screen]::AllScreens
$bounds = [System.Drawing.Rectangle]::Empty
foreach ($s in $screens) { $bounds = [System.Drawing.Rectangle]::Union($bounds, $s.Bounds) }
$bmp = New-Object System.Drawing.Bitmap($bounds.Width, $bounds.Height)
$g = [System.Drawing.Graphics]::FromImage($bmp)
$g.CopyFromScreen($bounds.X, $bounds.Y, 0, 0, $bounds.Size)
$g.Dispose()
$bmp.Save('%s', [System.Drawing.Imaging.ImageFormat]::Png)
$bmp.Dispose()
$w = $bounds.Width; $h = $bounds.Height
Write-Output "$w x $h"
`, strings.ReplaceAll(savePath, `\`, `\\`))
	}

	out, err := runPowerShell(ctx, psScript)
	if err != nil {
		return "", fmt.Errorf("screenshot failed: %w\n%s", err, out)
	}

	// Check file exists
	fi, err := os.Stat(savePath)
	if err != nil {
		return "", fmt.Errorf("screenshot file not created: %v", err)
	}

	localURL := "/v1/images/" + filename
	sizeMB := float64(fi.Size()) / 1024 / 1024
	log.Printf("[DesktopTool] Screenshot saved: %s (%.1f MB)", localURL, sizeMB)

	return toJSON(map[string]interface{}{
		"action":     "screenshot",
		"status":     "success",
		"image_url":  localURL,
		"size_mb":    fmt.Sprintf("%.1f", sizeMB),
		"resolution": strings.TrimSpace(out),
		"message":    fmt.Sprintf("屏幕截图已保存。请用视觉模型分析图片内容来决定下一步操作。图片URL: %s", localURL),
	}), nil
}

// ── Mouse ──

func (t *DesktopTool) mouseClick(ctx context.Context, a desktopArgs) (string, error) {
	button := a.Button
	if button == "" {
		button = "left"
	}
	clickType := a.ClickType
	if clickType == "" {
		clickType = "single"
	}

	// Move + click via PowerShell with user32.dll
	clickCode := mouseClickPS(button, clickType)
	psScript := fmt.Sprintf(`
Add-Type @"
using System;
using System.Runtime.InteropServices;
public class WinAPI {
    [DllImport("user32.dll")] public static extern bool SetCursorPos(int X, int Y);
    [DllImport("user32.dll")] public static extern void mouse_event(uint dwFlags, int dx, int dy, int dwData, IntPtr dwExtraInfo);
}
"@
[WinAPI]::SetCursorPos(%d, %d)
Start-Sleep -Milliseconds 50
%s
Write-Output "clicked %s %s at %d,%d"
`, a.X, a.Y, clickCode, button, clickType, a.X, a.Y)

	out, err := runPowerShell(ctx, psScript)
	if err != nil {
		return "", fmt.Errorf("mouse_click failed: %w\n%s", err, out)
	}

	return toJSON(map[string]interface{}{
		"action":  "mouse_click",
		"status":  "success",
		"x":       a.X,
		"y":       a.Y,
		"button":  button,
		"type":    clickType,
		"message": fmt.Sprintf("已在坐标(%d, %d)执行%s%s键点击", a.X, a.Y, clickType, button),
	}), nil
}

func (t *DesktopTool) mouseMove(ctx context.Context, a desktopArgs) (string, error) {
	psScript := fmt.Sprintf(`
Add-Type @"
using System;
using System.Runtime.InteropServices;
public class WinAPI {
    [DllImport("user32.dll")] public static extern bool SetCursorPos(int X, int Y);
}
"@
[WinAPI]::SetCursorPos(%d, %d)
Write-Output "moved to %d,%d"
`, a.X, a.Y, a.X, a.Y)

	out, err := runPowerShell(ctx, psScript)
	if err != nil {
		return "", fmt.Errorf("mouse_move failed: %w\n%s", err, out)
	}

	return toJSON(map[string]interface{}{
		"action":  "mouse_move",
		"status":  "success",
		"x":       a.X,
		"y":       a.Y,
		"message": fmt.Sprintf("鼠标已移动到(%d, %d)", a.X, a.Y),
	}), nil
}

func (t *DesktopTool) mouseDrag(ctx context.Context, a desktopArgs) (string, error) {
	psScript := fmt.Sprintf(`
Add-Type @"
using System;
using System.Runtime.InteropServices;
public class WinAPI {
    [DllImport("user32.dll")] public static extern bool SetCursorPos(int X, int Y);
    [DllImport("user32.dll")] public static extern void mouse_event(uint dwFlags, int dx, int dy, int dwData, IntPtr dwExtraInfo);
}
"@
[WinAPI]::SetCursorPos(%d, %d)
Start-Sleep -Milliseconds 50
[WinAPI]::mouse_event(0x0002, 0, 0, 0, [IntPtr]::Zero)  # LEFTDOWN
Start-Sleep -Milliseconds 50
$steps = 20
for ($i = 1; $i -le $steps; $i++) {
    $cx = %d + ((%d - %d) * $i / $steps)
    $cy = %d + ((%d - %d) * $i / $steps)
    [WinAPI]::SetCursorPos([int]$cx, [int]$cy)
    Start-Sleep -Milliseconds 10
}
[WinAPI]::mouse_event(0x0004, 0, 0, 0, [IntPtr]::Zero)  # LEFTUP
Write-Output "dragged from %d,%d to %d,%d"
`, a.X, a.Y, a.X, a.X2, a.X, a.Y, a.Y2, a.Y, a.X, a.Y, a.X2, a.Y2)

	out, err := runPowerShell(ctx, psScript)
	if err != nil {
		return "", fmt.Errorf("mouse_drag failed: %w\n%s", err, out)
	}

	return toJSON(map[string]interface{}{
		"action":  "mouse_drag",
		"status":  "success",
		"from":    fmt.Sprintf("%d,%d", a.X, a.Y),
		"to":      fmt.Sprintf("%d,%d", a.X2, a.Y2),
		"message": fmt.Sprintf("已从(%d,%d)拖拽到(%d,%d)", a.X, a.Y, a.X2, a.Y2),
	}), nil
}

// ── Keyboard ──

func (t *DesktopTool) keyboardType(ctx context.Context, a desktopArgs) (string, error) {
	if a.Text == "" {
		return "", fmt.Errorf("text is required for keyboard_type")
	}

	// Use SendKeys via PowerShell for text input
	// For non-ASCII (Chinese etc.), use clipboard approach
	escaped := base64.StdEncoding.EncodeToString([]byte(a.Text))
	psScript := fmt.Sprintf(`
Add-Type -AssemblyName System.Windows.Forms
$bytes = [System.Convert]::FromBase64String('%s')
$text = [System.Text.Encoding]::UTF8.GetString($bytes)
[System.Windows.Forms.Clipboard]::SetText($text)
Start-Sleep -Milliseconds 100
[System.Windows.Forms.SendKeys]::SendWait('^v')
Start-Sleep -Milliseconds 100
Write-Output "typed $($text.Length) chars"
`, escaped)

	out, err := runPowerShell(ctx, psScript)
	if err != nil {
		return "", fmt.Errorf("keyboard_type failed: %w\n%s", err, out)
	}

	preview := a.Text
	if len(preview) > 50 {
		preview = preview[:50] + "..."
	}
	return toJSON(map[string]interface{}{
		"action":  "keyboard_type",
		"status":  "success",
		"length":  len(a.Text),
		"message": fmt.Sprintf("已输入文本: \"%s\"", preview),
	}), nil
}

func (t *DesktopTool) keyboardHotkey(ctx context.Context, a desktopArgs) (string, error) {
	if a.Text == "" {
		return "", fmt.Errorf("text is required for keyboard_hotkey (e.g. 'ctrl+c', 'alt+tab', 'ctrl+shift+s')")
	}

	// Parse hotkey combo: "ctrl+c" → "^c", "alt+tab" → "%{TAB}", "ctrl+shift+s" → "^+s"
	sendKeysStr := hotkeyToSendKeys(a.Text)

	psScript := fmt.Sprintf(`
Add-Type -AssemblyName System.Windows.Forms
[System.Windows.Forms.SendKeys]::SendWait('%s')
Write-Output "hotkey sent: %s"
`, sendKeysStr, a.Text)

	out, err := runPowerShell(ctx, psScript)
	if err != nil {
		return "", fmt.Errorf("keyboard_hotkey failed: %w\n%s", err, out)
	}

	return toJSON(map[string]interface{}{
		"action":  "keyboard_hotkey",
		"status":  "success",
		"hotkey":  a.Text,
		"message": fmt.Sprintf("已按下组合键: %s", a.Text),
	}), nil
}

func (t *DesktopTool) keyboardKey(ctx context.Context, a desktopArgs) (string, error) {
	if a.Text == "" {
		return "", fmt.Errorf("text is required for keyboard_key (e.g. 'enter', 'tab', 'escape')")
	}

	sendKeysStr := specialKeyToSendKeys(a.Text)

	psScript := fmt.Sprintf(`
Add-Type -AssemblyName System.Windows.Forms
[System.Windows.Forms.SendKeys]::SendWait('%s')
Write-Output "key sent: %s"
`, sendKeysStr, a.Text)

	out, err := runPowerShell(ctx, psScript)
	if err != nil {
		return "", fmt.Errorf("keyboard_key failed: %w\n%s", err, out)
	}

	return toJSON(map[string]interface{}{
		"action":  "keyboard_key",
		"status":  "success",
		"key":     a.Text,
		"message": fmt.Sprintf("已按下按键: %s", a.Text),
	}), nil
}

// ── Window Management ──

func (t *DesktopTool) listWindows(ctx context.Context) (string, error) {
	psScript := `
Get-Process | Where-Object { $_.MainWindowTitle -ne '' } | ForEach-Object {
    $id = $_.Id
    $name = $_.ProcessName
    $title = $_.MainWindowTitle
    Write-Output "$id|$name|$title"
}
`
	out, err := runPowerShell(ctx, psScript)
	if err != nil {
		return "", fmt.Errorf("list_windows failed: %w\n%s", err, out)
	}

	var windows []map[string]interface{}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 3)
		if len(parts) < 3 {
			continue
		}
		pid, _ := strconv.Atoi(parts[0])
		windows = append(windows, map[string]interface{}{
			"pid":     pid,
			"process": parts[1],
			"title":   parts[2],
		})
	}

	if windows == nil {
		windows = []map[string]interface{}{}
	}

	return toJSON(map[string]interface{}{
		"action":  "list_windows",
		"status":  "success",
		"count":   len(windows),
		"windows": windows,
		"message": fmt.Sprintf("当前有 %d 个可见窗口", len(windows)),
	}), nil
}

func (t *DesktopTool) focusWindow(ctx context.Context, a desktopArgs) (string, error) {
	if a.Title == "" {
		return "", fmt.Errorf("title is required for focus_window (window title or partial match)")
	}

	escaped := strings.ReplaceAll(a.Title, "'", "''")
	psScript := fmt.Sprintf(`
Add-Type @"
using System;
using System.Runtime.InteropServices;
public class WinAPI {
    [DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr hWnd);
    [DllImport("user32.dll")] public static extern bool ShowWindow(IntPtr hWnd, int nCmdShow);
}
"@
$procs = Get-Process | Where-Object { $_.MainWindowTitle -like '*%s*' -and $_.MainWindowHandle -ne 0 }
if ($procs.Count -eq 0) {
    Write-Output "NOT_FOUND"
} else {
    $p = $procs[0]
    [WinAPI]::ShowWindow($p.MainWindowHandle, 9)  # SW_RESTORE
    Start-Sleep -Milliseconds 100
    [WinAPI]::SetForegroundWindow($p.MainWindowHandle)
    Write-Output "FOCUSED|$($p.Id)|$($p.ProcessName)|$($p.MainWindowTitle)"
}
`, escaped)

	out, err := runPowerShell(ctx, psScript)
	if err != nil {
		return "", fmt.Errorf("focus_window failed: %w\n%s", err, out)
	}

	out = strings.TrimSpace(out)
	if out == "NOT_FOUND" {
		return toJSON(map[string]interface{}{
			"action":  "focus_window",
			"status":  "not_found",
			"message": fmt.Sprintf("未找到标题包含 \"%s\" 的窗口。请用 list_windows 查看所有窗口。", a.Title),
		}), nil
	}

	parts := strings.SplitN(out, "|", 4)
	result := map[string]interface{}{
		"action": "focus_window",
		"status": "success",
	}
	if len(parts) >= 4 {
		result["pid"], _ = strconv.Atoi(parts[1])
		result["process"] = parts[2]
		result["title"] = parts[3]
		result["message"] = fmt.Sprintf("已将窗口 \"%s\" 切到前台", parts[3])
	}

	return toJSON(result), nil
}

// ── Launch App ──

func (t *DesktopTool) launchApp(ctx context.Context, a desktopArgs) (string, error) {
	if a.Text == "" {
		return "", fmt.Errorf("text is required for launch_app (app name or full path)")
	}

	appPath := a.Text

	// Common app shortcuts (Chinese app names → executable paths)
	appAliases := map[string]string{
		"剪映":         `C:\Program Files\JianyingPro\JianyingPro.exe`,
		"剪映专业版":      `C:\Program Files\JianyingPro\JianyingPro.exe`,
		"capcut":     `C:\Program Files\JianyingPro\JianyingPro.exe`,
		"wps":        `C:\Users\` + os.Getenv("USERNAME") + `\AppData\Local\Kingsoft\WPS Office\ksolaunch.exe`,
		"微信":         `C:\Program Files\Tencent\WeChat\WeChat.exe`,
		"wechat":     `C:\Program Files\Tencent\WeChat\WeChat.exe`,
		"qq":         `C:\Program Files\Tencent\QQ\Bin\QQ.exe`,
		"钉钉":         `C:\Program Files\DingDing\DingtalkLauncher.exe`,
		"飞书":         `C:\Program Files\Lark\Lark.exe`,
		"chrome":     `C:\Program Files\Google\Chrome\Application\chrome.exe`,
		"edge":       `C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
		"firefox":    `C:\Program Files\Mozilla Firefox\firefox.exe`,
		"vscode":     `C:\Users\` + os.Getenv("USERNAME") + `\AppData\Local\Programs\Microsoft VS Code\Code.exe`,
		"记事本":        `notepad.exe`,
		"notepad":    `notepad.exe`,
		"计算器":        `calc.exe`,
		"画图":         `mspaint.exe`,
		"资源管理器":      `explorer.exe`,
		"explorer":   `explorer.exe`,
		"cmd":        `cmd.exe`,
		"powershell": `powershell.exe`,
		"terminal":   `wt.exe`,
	}

	lower := strings.ToLower(appPath)
	if alias, ok := appAliases[lower]; ok {
		appPath = alias
	}

	// Try to launch the app
	cmd := exec.CommandContext(ctx, "cmd", "/c", "start", "", appPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Fallback: try PowerShell Start-Process
		psScript := fmt.Sprintf(`Start-Process '%s' -ErrorAction Stop; Write-Output "launched"`,
			strings.ReplaceAll(appPath, "'", "''"))
		out2, err2 := runPowerShell(ctx, psScript)
		if err2 != nil {
			return "", fmt.Errorf("launch_app failed: %v\ncmd: %s\nps: %s", err, string(out), out2)
		}
	}

	// Wait a moment for the app to start
	time.Sleep(1 * time.Second)

	return toJSON(map[string]interface{}{
		"action":  "launch_app",
		"status":  "success",
		"app":     a.Text,
		"path":    appPath,
		"message": fmt.Sprintf("已启动应用: %s。请等待几秒后截图查看应用是否正常打开。", a.Text),
	}), nil
}

// ── Wait ──

func (t *DesktopTool) wait(a desktopArgs) (string, error) {
	seconds := a.Seconds
	if seconds <= 0 {
		seconds = 2
	}
	if seconds > 30 {
		seconds = 30
	}
	time.Sleep(time.Duration(seconds) * time.Second)
	return toJSON(map[string]interface{}{
		"action":  "wait",
		"status":  "success",
		"seconds": seconds,
		"message": fmt.Sprintf("已等待 %d 秒", seconds),
	}), nil
}

// ── Helpers ──

func isDockerEnv() bool {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	// Also check cgroup for Docker
	if data, err := os.ReadFile("/proc/1/cgroup"); err == nil {
		if strings.Contains(string(data), "docker") || strings.Contains(string(data), "containerd") {
			return true
		}
	}
	return false
}

func runPowerShell(ctx context.Context, script string) (string, error) {
	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// mouseClickPS returns the PowerShell mouse_event calls for a specific button and click type.
func mouseClickPS(button, clickType string) string {
	// mouse_event flags: LEFTDOWN=0x0002, LEFTUP=0x0004, RIGHTDOWN=0x0008, RIGHTUP=0x0010, MIDDLEDOWN=0x0020, MIDDLEUP=0x0040
	var down, up uint32
	switch button {
	case "right":
		down, up = 0x0008, 0x0010
	case "middle":
		down, up = 0x0020, 0x0040
	default: // left
		down, up = 0x0002, 0x0004
	}

	click := fmt.Sprintf("[WinAPI]::mouse_event(0x%04X, 0, 0, 0, [IntPtr]::Zero)\nStart-Sleep -Milliseconds 30\n[WinAPI]::mouse_event(0x%04X, 0, 0, 0, [IntPtr]::Zero)", down, up)
	if clickType == "double" {
		click = click + "\nStart-Sleep -Milliseconds 50\n" + click
	}
	return click
}

// hotkeyToSendKeys converts "ctrl+c" to SendKeys format "^c"
func hotkeyToSendKeys(hotkey string) string {
	parts := strings.Split(strings.ToLower(hotkey), "+")
	var prefix, key string

	for _, part := range parts {
		part = strings.TrimSpace(part)
		switch part {
		case "ctrl", "control":
			prefix += "^"
		case "alt":
			prefix += "%"
		case "shift":
			prefix += "+"
		case "win", "windows":
			prefix += "^{ESC}" // Approximate
		default:
			key = specialKeyToSendKeys(part)
		}
	}

	return prefix + key
}

// specialKeyToSendKeys converts a key name to SendKeys format
func specialKeyToSendKeys(key string) string {
	keyMap := map[string]string{
		"enter":       "{ENTER}",
		"return":      "{ENTER}",
		"tab":         "{TAB}",
		"escape":      "{ESC}",
		"esc":         "{ESC}",
		"backspace":   "{BACKSPACE}",
		"delete":      "{DELETE}",
		"del":         "{DELETE}",
		"home":        "{HOME}",
		"end":         "{END}",
		"pageup":      "{PGUP}",
		"pagedown":    "{PGDN}",
		"up":          "{UP}",
		"down":        "{DOWN}",
		"left":        "{LEFT}",
		"right":       "{RIGHT}",
		"space":       " ",
		"f1":          "{F1}",
		"f2":          "{F2}",
		"f3":          "{F3}",
		"f4":          "{F4}",
		"f5":          "{F5}",
		"f6":          "{F6}",
		"f7":          "{F7}",
		"f8":          "{F8}",
		"f9":          "{F9}",
		"f10":         "{F10}",
		"f11":         "{F11}",
		"f12":         "{F12}",
		"insert":      "{INSERT}",
		"printscreen": "{PRTSC}",
	}

	lower := strings.ToLower(key)
	if mapped, ok := keyMap[lower]; ok {
		return mapped
	}
	// Single character key
	if len(key) == 1 {
		return key
	}
	return "{" + strings.ToUpper(key) + "}"
}
