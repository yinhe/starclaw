package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// ───────────────────────────────────────────────
// StarClaw MCP Bridge — Host Control Server
//
// Runs on the HOST machine (not in Docker).
// Exposes MCP JSON-RPC 2.0 tools so Claw agents
// inside Docker can control the host computer.
//
// Usage:
//   go run ./cmd/mcp-bridge              # default :9100
//   go run ./cmd/mcp-bridge -port 9100
//
// Then add in Claw Settings → MCP 工具:
//   Name:    host
//   BaseURL: http://host.docker.internal:9100
// ───────────────────────────────────────────────

var version = "0.1.0"

// --- JSON-RPC types ---

type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *rpcError   `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type mcpContent struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	Data     string `json:"data,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
}

type mcpToolResult struct {
	Content []mcpContent `json:"content"`
	IsError bool         `json:"isError,omitempty"`
}

// --- Tool definitions ---

type toolDef struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"inputSchema"`
}

func getTools() []toolDef {
	return []toolDef{
		{
			Name:        "shell_exec",
			Description: "在宿主机上执行 Shell 命令（Windows: PowerShell, Mac/Linux: bash）",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{
						"type":        "string",
						"description": "要执行的命令",
					},
					"cwd": map[string]interface{}{
						"type":        "string",
						"description": "工作目录（可选）",
					},
					"timeout_seconds": map[string]interface{}{
						"type":        "integer",
						"description": "超时秒数（默认30）",
					},
				},
				"required": []string{"command"},
			},
		},
		{
			Name:        "file_read",
			Description: "读取宿主机上的文件内容",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "文件绝对路径",
					},
					"max_lines": map[string]interface{}{
						"type":        "integer",
						"description": "最大读取行数（默认1000）",
					},
				},
				"required": []string{"path"},
			},
		},
		{
			Name:        "file_write",
			Description: "在宿主机上写入文件（会自动创建目录）",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "文件绝对路径",
					},
					"content": map[string]interface{}{
						"type":        "string",
						"description": "要写入的内容",
					},
					"append": map[string]interface{}{
						"type":        "boolean",
						"description": "是否追加（默认覆盖写入）",
					},
				},
				"required": []string{"path", "content"},
			},
		},
		{
			Name:        "file_list",
			Description: "列出宿主机上目录的文件和子目录",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "目录绝对路径",
					},
					"max_items": map[string]interface{}{
						"type":        "integer",
						"description": "最大返回条数（默认100）",
					},
				},
				"required": []string{"path"},
			},
		},
		{
			Name:        "system_info",
			Description: "获取宿主机系统信息（OS、CPU、内存、磁盘等）",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "open_url",
			Description: "在宿主机默认浏览器中打开 URL",
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
			Name:        "open_app",
			Description: "在宿主机上打开应用程序或文件",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"target": map[string]interface{}{
						"type":        "string",
						"description": "应用名称或文件路径",
					},
				},
				"required": []string{"target"},
			},
		},
		{
			Name:        "clipboard_read",
			Description: "读取宿主机剪贴板内容",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "clipboard_write",
			Description: "写入内容到宿主机剪贴板",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"text": map[string]interface{}{
						"type":        "string",
						"description": "要写入剪贴板的文本",
					},
				},
				"required": []string{"text"},
			},
		},
		// ── GUI Automation tools ──
		{
			Name:        "screen_capture",
			Description: "截取宿主机屏幕截图（返回 base64 PNG 图片）。Linux 需安装 scrot 或 ImageMagick，macOS/Windows 内置支持。",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"region": map[string]interface{}{
						"type":        "string",
						"description": "截取区域，格式 WxH+X+Y（如 800x600+100+200），留空则全屏",
					},
				},
			},
		},
		{
			Name:        "keyboard_type",
			Description: "在宿主机当前焦点窗口中模拟键盘输入文本。Linux 需安装 xdotool。",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"text": map[string]interface{}{
						"type":        "string",
						"description": "要输入的文本内容",
					},
					"delay_ms": map[string]interface{}{
						"type":        "integer",
						"description": "每次按键之间的延迟毫秒数（默认12）",
					},
				},
				"required": []string{"text"},
			},
		},
		{
			Name:        "key_combo",
			Description: "在宿主机当前焦点窗口模拟按下组合键。示例: 'ctrl+s'、'ctrl+shift+s'、'alt+F4'、'Return'、'ctrl+a'。Linux 需安装 xdotool。",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"keys": map[string]interface{}{
						"type":        "string",
						"description": "组合键，用 + 连接，如 ctrl+s、alt+F4、ctrl+shift+s、Return、Tab",
					},
				},
				"required": []string{"keys"},
			},
		},
		{
			Name:        "mouse_click",
			Description: "在宿主机屏幕指定坐标处点击鼠标。Linux 需安装 xdotool。",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"x": map[string]interface{}{
						"type":        "integer",
						"description": "屏幕 X 坐标",
					},
					"y": map[string]interface{}{
						"type":        "integer",
						"description": "屏幕 Y 坐标",
					},
					"button": map[string]interface{}{
						"type":        "string",
						"description": "鼠标按钮: left（默认）、right、middle",
					},
					"double": map[string]interface{}{
						"type":        "boolean",
						"description": "是否双击（默认单击）",
					},
				},
				"required": []string{"x", "y"},
			},
		},
		{
			Name:        "mouse_move",
			Description: "将宿主机鼠标移动到指定屏幕坐标。Linux 需安装 xdotool。",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"x": map[string]interface{}{
						"type":        "integer",
						"description": "屏幕 X 坐标",
					},
					"y": map[string]interface{}{
						"type":        "integer",
						"description": "屏幕 Y 坐标",
					},
				},
				"required": []string{"x", "y"},
			},
		},
		{
			Name:        "active_window",
			Description: "获取宿主机当前活动窗口的信息（标题、大小、位置等）。Linux 需安装 xdotool。",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
	}
}

// --- Tool implementations ---

func execShell(args map[string]interface{}) mcpToolResult {
	command, _ := args["command"].(string)
	if command == "" {
		return errResult("command is required")
	}

	cwd, _ := args["cwd"].(string)
	timeoutF, _ := args["timeout_seconds"].(float64)
	timeout := 30
	if timeoutF > 0 {
		timeout = int(timeoutF)
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("powershell", "-NoProfile", "-Command", command)
	} else {
		cmd = exec.Command("bash", "-c", command)
	}

	if cwd != "" {
		cmd.Dir = cwd
	}

	done := make(chan error, 1)
	var output []byte
	var execErr error

	go func() {
		output, execErr = cmd.CombinedOutput()
		done <- execErr
	}()

	select {
	case <-done:
		result := string(output)
		if execErr != nil {
			result += "\n[exit error: " + execErr.Error() + "]"
		}
		if len(result) > 50000 {
			result = result[:50000] + "\n...[truncated]"
		}
		return textResult(result)
	case <-time.After(time.Duration(timeout) * time.Second):
		cmd.Process.Kill()
		return errResult(fmt.Sprintf("command timed out after %ds", timeout))
	}
}

func execFileRead(args map[string]interface{}) mcpToolResult {
	path, _ := args["path"].(string)
	if path == "" {
		return errResult("path is required")
	}

	maxLinesF, _ := args["max_lines"].(float64)
	maxLines := 1000
	if maxLinesF > 0 {
		maxLines = int(maxLinesF)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return errResult("read error: " + err.Error())
	}

	content := string(data)
	lines := strings.Split(content, "\n")
	if len(lines) > maxLines {
		content = strings.Join(lines[:maxLines], "\n") + fmt.Sprintf("\n...[truncated, %d/%d lines shown]", maxLines, len(lines))
	}

	return textResult(content)
}

func execFileWrite(args map[string]interface{}) mcpToolResult {
	path, _ := args["path"].(string)
	content, _ := args["content"].(string)
	appendMode, _ := args["append"].(bool)

	if path == "" {
		return errResult("path is required")
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return errResult("mkdir error: " + err.Error())
	}

	flag := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	if appendMode {
		flag = os.O_WRONLY | os.O_CREATE | os.O_APPEND
	}

	f, err := os.OpenFile(path, flag, 0644)
	if err != nil {
		return errResult("open error: " + err.Error())
	}
	defer f.Close()

	n, err := f.WriteString(content)
	if err != nil {
		return errResult("write error: " + err.Error())
	}

	mode := "written"
	if appendMode {
		mode = "appended"
	}
	return textResult(fmt.Sprintf("%s %d bytes to %s", mode, n, path))
}

func execFileList(args map[string]interface{}) mcpToolResult {
	path, _ := args["path"].(string)
	if path == "" {
		return errResult("path is required")
	}

	maxItemsF, _ := args["max_items"].(float64)
	maxItems := 100
	if maxItemsF > 0 {
		maxItems = int(maxItemsF)
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return errResult("readdir error: " + err.Error())
	}

	var lines []string
	for i, e := range entries {
		if i >= maxItems {
			lines = append(lines, fmt.Sprintf("...[truncated, showing %d/%d]", maxItems, len(entries)))
			break
		}
		info, _ := e.Info()
		typeStr := "FILE"
		sizeStr := ""
		if e.IsDir() {
			typeStr = "DIR "
		} else if info != nil {
			sizeStr = fmt.Sprintf(" (%d bytes)", info.Size())
		}
		lines = append(lines, fmt.Sprintf("%s  %s%s", typeStr, e.Name(), sizeStr))
	}

	return textResult(strings.Join(lines, "\n"))
}

func execSystemInfo() mcpToolResult {
	hostname, _ := os.Hostname()
	cwd, _ := os.Getwd()
	home, _ := os.UserHomeDir()

	info := fmt.Sprintf(`OS:       %s
Arch:     %s
Hostname: %s
HomeDir:  %s
WorkDir:  %s
NumCPU:   %d
GoVer:    %s
Bridge:   v%s`,
		runtime.GOOS, runtime.GOARCH, hostname, home, cwd,
		runtime.NumCPU(), runtime.Version(), version)

	// Get disk / memory info via platform commands
	var extra string
	if runtime.GOOS == "windows" {
		out, err := exec.Command("powershell", "-NoProfile", "-Command",
			"Get-CimInstance Win32_OperatingSystem | Select-Object TotalVisibleMemorySize,FreePhysicalMemory | Format-List").CombinedOutput()
		if err == nil {
			extra = "\n" + strings.TrimSpace(string(out))
		}
	} else {
		out, err := exec.Command("bash", "-c", "free -h 2>/dev/null | head -3; echo; df -h / 2>/dev/null | head -2").CombinedOutput()
		if err == nil {
			extra = "\n" + strings.TrimSpace(string(out))
		}
	}

	return textResult(info + extra)
}

func execOpenURL(args map[string]interface{}) mcpToolResult {
	url, _ := args["url"].(string)
	if url == "" {
		return errResult("url is required")
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}

	if err := cmd.Start(); err != nil {
		return errResult("open url error: " + err.Error())
	}
	return textResult("opened: " + url)
}

func execOpenApp(args map[string]interface{}) mcpToolResult {
	target, _ := args["target"].(string)
	if target == "" {
		return errResult("target is required")
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", target)
	case "darwin":
		cmd = exec.Command("open", "-a", target)
	default:
		cmd = exec.Command("xdg-open", target)
	}

	if err := cmd.Start(); err != nil {
		return errResult("open app error: " + err.Error())
	}
	return textResult("opened: " + target)
}

func execClipboardRead() mcpToolResult {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("powershell", "-NoProfile", "-Command", "Get-Clipboard")
	case "darwin":
		cmd = exec.Command("pbpaste")
	default:
		cmd = exec.Command("xclip", "-selection", "clipboard", "-o")
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return errResult("clipboard read error: " + err.Error())
	}
	return textResult(string(out))
}

func execClipboardWrite(args map[string]interface{}) mcpToolResult {
	text, _ := args["text"].(string)
	if text == "" {
		return errResult("text is required")
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("powershell", "-NoProfile", "-Command", "Set-Clipboard", "-Value", text)
	case "darwin":
		cmd = exec.Command("pbcopy")
		cmd.Stdin = strings.NewReader(text)
	default:
		cmd = exec.Command("xclip", "-selection", "clipboard")
		cmd.Stdin = strings.NewReader(text)
	}

	if err := cmd.Run(); err != nil {
		return errResult("clipboard write error: " + err.Error())
	}
	return textResult(fmt.Sprintf("wrote %d chars to clipboard", len(text)))
}

// --- GUI Automation implementations ---

func execScreenCapture(args map[string]interface{}) mcpToolResult {
	region, _ := args["region"].(string)
	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("mcp_screenshot_%d.png", time.Now().UnixNano()))
	defer os.Remove(tmpFile)

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		// PowerShell screenshot
		ps := fmt.Sprintf(`
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing
$screen = [System.Windows.Forms.Screen]::PrimaryScreen.Bounds
$bmp = New-Object System.Drawing.Bitmap($screen.Width, $screen.Height)
$g = [System.Drawing.Graphics]::FromImage($bmp)
$g.CopyFromScreen($screen.Location, [System.Drawing.Point]::Empty, $screen.Size)
$bmp.Save('%s', [System.Drawing.Imaging.ImageFormat]::Png)
$g.Dispose()
$bmp.Dispose()`, strings.ReplaceAll(tmpFile, `\`, `\\`))
		cmd = exec.Command("powershell", "-NoProfile", "-Command", ps)
	case "darwin":
		if region != "" {
			// region format: WxH+X+Y -> screencapture -R x,y,w,h
			cmd = exec.Command("screencapture", "-x", tmpFile)
		} else {
			cmd = exec.Command("screencapture", "-x", tmpFile)
		}
	default: // linux
		if region != "" {
			cmd = exec.Command("import", "-window", "root", "-crop", region, tmpFile)
		} else {
			// Try scrot first, fallback to import (ImageMagick)
			if _, err := exec.LookPath("scrot"); err == nil {
				cmd = exec.Command("scrot", tmpFile)
			} else {
				cmd = exec.Command("import", "-window", "root", tmpFile)
			}
		}
	}

	if out, err := cmd.CombinedOutput(); err != nil {
		return errResult(fmt.Sprintf("screenshot failed: %s — %s", err.Error(), string(out)))
	}

	data, err := os.ReadFile(tmpFile)
	if err != nil {
		return errResult("read screenshot failed: " + err.Error())
	}

	b64 := base64.StdEncoding.EncodeToString(data)
	return mcpToolResult{
		Content: []mcpContent{{Type: "image", Data: b64, MimeType: "image/png"}},
	}
}

func execKeyboardType(args map[string]interface{}) mcpToolResult {
	text, _ := args["text"].(string)
	if text == "" {
		return errResult("text is required")
	}
	delayF, _ := args["delay_ms"].(float64)
	delay := 12
	if delayF > 0 {
		delay = int(delayF)
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		// Use PowerShell SendKeys; handle special chars
		escaped := strings.ReplaceAll(text, "'", "''")
		ps := fmt.Sprintf(`
Add-Type -AssemblyName System.Windows.Forms
Start-Sleep -Milliseconds 100
[System.Windows.Forms.SendKeys]::SendWait('%s')`, escaped)
		cmd = exec.Command("powershell", "-NoProfile", "-Command", ps)
	case "darwin":
		escaped := strings.ReplaceAll(text, `\`, `\\`)
		escaped = strings.ReplaceAll(escaped, `"`, `\"`)
		cmd = exec.Command("osascript", "-e", fmt.Sprintf(`tell application "System Events" to keystroke "%s"`, escaped))
	default: // linux
		cmd = exec.Command("xdotool", "type", "--delay", strconv.Itoa(delay), "--clearmodifiers", text)
	}

	if out, err := cmd.CombinedOutput(); err != nil {
		return errResult(fmt.Sprintf("keyboard_type failed: %s — %s", err.Error(), string(out)))
	}
	return textResult(fmt.Sprintf("typed %d chars", len(text)))
}

func execKeyCombo(args map[string]interface{}) mcpToolResult {
	keys, _ := args["keys"].(string)
	if keys == "" {
		return errResult("keys is required")
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		// Map combo to SendKeys format: ctrl+s -> ^s, alt+F4 -> %{F4}, shift -> +
		sendKey := mapKeysToSendKeys(keys)
		ps := fmt.Sprintf(`
Add-Type -AssemblyName System.Windows.Forms
Start-Sleep -Milliseconds 100
[System.Windows.Forms.SendKeys]::SendWait('%s')`, sendKey)
		cmd = exec.Command("powershell", "-NoProfile", "-Command", ps)
	case "darwin":
		cmd = exec.Command("osascript", "-e", mapKeysToAppleScript(keys))
	default: // linux — xdotool uses same format: ctrl+s, alt+F4
		cmd = exec.Command("xdotool", "key", "--clearmodifiers", keys)
	}

	if out, err := cmd.CombinedOutput(); err != nil {
		return errResult(fmt.Sprintf("key_combo failed: %s — %s", err.Error(), string(out)))
	}
	return textResult("pressed: " + keys)
}

func execMouseClick(args map[string]interface{}) mcpToolResult {
	xF, _ := args["x"].(float64)
	yF, _ := args["y"].(float64)
	button, _ := args["button"].(string)
	double, _ := args["double"].(bool)

	x, y := int(xF), int(yF)
	if button == "" {
		button = "left"
	}

	btnMap := map[string]string{"left": "1", "middle": "2", "right": "3"}
	btnNum := btnMap[button]
	if btnNum == "" {
		btnNum = "1"
	}

	var cmds []*exec.Cmd
	switch runtime.GOOS {
	case "windows":
		action := "click"
		if double {
			action = "doubleClick"
		}
		ps := fmt.Sprintf(`
Add-Type -AssemblyName System.Windows.Forms
[System.Windows.Forms.Cursor]::Position = New-Object System.Drawing.Point(%d, %d)
Start-Sleep -Milliseconds 50
$sig = '[DllImport("user32.dll")] public static extern void mouse_event(int f, int x, int y, int d, int e);'
$mouse = Add-Type -MemberDefinition $sig -Name M -Namespace W -PassThru
$mouse::mouse_event(0x0002, 0, 0, 0, 0)
$mouse::mouse_event(0x0004, 0, 0, 0, 0)`, x, y)
		if double {
			ps += `
Start-Sleep -Milliseconds 50
$mouse::mouse_event(0x0002, 0, 0, 0, 0)
$mouse::mouse_event(0x0004, 0, 0, 0, 0)`
		}
		_ = action
		cmds = append(cmds, exec.Command("powershell", "-NoProfile", "-Command", ps))
	case "darwin":
		action := "c"
		if double {
			action = "dc"
		}
		cmds = append(cmds, exec.Command("cliclick", fmt.Sprintf("%s:%d,%d", action, x, y)))
	default: // linux
		cmds = append(cmds, exec.Command("xdotool", "mousemove", "--sync", strconv.Itoa(x), strconv.Itoa(y)))
		clickArgs := []string{"click"}
		if double {
			clickArgs = append(clickArgs, "--repeat", "2", "--delay", "100")
		}
		clickArgs = append(clickArgs, btnNum)
		cmds = append(cmds, exec.Command("xdotool", clickArgs...))
	}

	for _, cmd := range cmds {
		if out, err := cmd.CombinedOutput(); err != nil {
			return errResult(fmt.Sprintf("mouse_click failed: %s — %s", err.Error(), string(out)))
		}
	}
	clickType := "click"
	if double {
		clickType = "double-click"
	}
	return textResult(fmt.Sprintf("%s %s at (%d, %d)", button, clickType, x, y))
}

func execMouseMove(args map[string]interface{}) mcpToolResult {
	xF, _ := args["x"].(float64)
	yF, _ := args["y"].(float64)
	x, y := int(xF), int(yF)

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		ps := fmt.Sprintf(`
Add-Type -AssemblyName System.Windows.Forms
[System.Windows.Forms.Cursor]::Position = New-Object System.Drawing.Point(%d, %d)`, x, y)
		cmd = exec.Command("powershell", "-NoProfile", "-Command", ps)
	case "darwin":
		cmd = exec.Command("cliclick", fmt.Sprintf("m:%d,%d", x, y))
	default:
		cmd = exec.Command("xdotool", "mousemove", "--sync", strconv.Itoa(x), strconv.Itoa(y))
	}

	if out, err := cmd.CombinedOutput(); err != nil {
		return errResult(fmt.Sprintf("mouse_move failed: %s — %s", err.Error(), string(out)))
	}
	return textResult(fmt.Sprintf("moved to (%d, %d)", x, y))
}

func execActiveWindow() mcpToolResult {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		ps := `
Add-Type @"
using System;
using System.Runtime.InteropServices;
using System.Text;
public class WinAPI {
    [DllImport("user32.dll")] public static extern IntPtr GetForegroundWindow();
    [DllImport("user32.dll")] public static extern int GetWindowText(IntPtr h, StringBuilder s, int n);
    [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr h, out RECT r);
    [StructLayout(LayoutKind.Sequential)] public struct RECT { public int L,T,R,B; }
}
"@
$h = [WinAPI]::GetForegroundWindow()
$sb = New-Object Text.StringBuilder 256
[WinAPI]::GetWindowText($h, $sb, 256)
$r = New-Object WinAPI+RECT
[WinAPI]::GetWindowRect($h, [ref]$r)
"Title: $($sb.ToString())"
"Position: $($r.L),$($r.T)"
"Size: $($r.R - $r.L)x$($r.B - $r.T)"`
		cmd = exec.Command("powershell", "-NoProfile", "-Command", ps)
	case "darwin":
		cmd = exec.Command("osascript", "-e", `
tell application "System Events"
	set fp to first process whose frontmost is true
	set wn to name of first window of fp
	set wp to position of first window of fp
	set ws to size of first window of fp
	return "Title: " & wn & "\nApp: " & name of fp & "\nPosition: " & item 1 of wp & "," & item 2 of wp & "\nSize: " & item 1 of ws & "x" & item 2 of ws
end tell`)
	default: // linux
		cmd = exec.Command("bash", "-c", `
WID=$(xdotool getactivewindow 2>/dev/null)
if [ -z "$WID" ]; then echo "no active window"; exit 0; fi
NAME=$(xdotool getwindowname "$WID" 2>/dev/null)
GEO=$(xdotool getwindowgeometry "$WID" 2>/dev/null)
SIZE=$(xdotool getwindowgeometry --shell "$WID" 2>/dev/null | grep -E 'WIDTH|HEIGHT')
echo "Title: $NAME"
echo "$GEO"
echo "$SIZE"`)
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return errResult(fmt.Sprintf("active_window failed: %s — %s", err.Error(), string(out)))
	}
	return textResult(strings.TrimSpace(string(out)))
}

// mapKeysToSendKeys converts "ctrl+s" style keys to Windows SendKeys format
func mapKeysToSendKeys(keys string) string {
	parts := strings.Split(strings.ToLower(keys), "+")
	prefix := ""
	key := parts[len(parts)-1]
	for _, p := range parts[:len(parts)-1] {
		switch p {
		case "ctrl", "control":
			prefix += "^"
		case "alt":
			prefix += "%"
		case "shift":
			prefix += "+"
		}
	}
	// Map special key names
	special := map[string]string{
		"return": "{ENTER}", "enter": "{ENTER}", "tab": "{TAB}",
		"escape": "{ESC}", "esc": "{ESC}", "backspace": "{BS}",
		"delete": "{DEL}", "home": "{HOME}", "end": "{END}",
		"up": "{UP}", "down": "{DOWN}", "left": "{LEFT}", "right": "{RIGHT}",
		"f1": "{F1}", "f2": "{F2}", "f3": "{F3}", "f4": "{F4}",
		"f5": "{F5}", "f6": "{F6}", "f7": "{F7}", "f8": "{F8}",
		"f9": "{F9}", "f10": "{F10}", "f11": "{F11}", "f12": "{F12}",
	}
	if sk, ok := special[key]; ok {
		key = sk
	}
	return prefix + key
}

// mapKeysToAppleScript converts "ctrl+s" to AppleScript keystroke
func mapKeysToAppleScript(keys string) string {
	parts := strings.Split(strings.ToLower(keys), "+")
	key := parts[len(parts)-1]
	var modifiers []string
	for _, p := range parts[:len(parts)-1] {
		switch p {
		case "ctrl", "control":
			modifiers = append(modifiers, "control down")
		case "alt", "option":
			modifiers = append(modifiers, "option down")
		case "shift":
			modifiers = append(modifiers, "shift down")
		case "cmd", "command", "super":
			modifiers = append(modifiers, "command down")
		}
	}
	// Map special keys
	special := map[string]string{
		"return": "return", "enter": "return", "tab": "tab",
		"escape": "escape", "esc": "escape", "delete": "delete",
		"up": "up arrow", "down": "down arrow", "left": "left arrow", "right": "right arrow",
		"f1": "f1", "f2": "f2", "f3": "f3", "f4": "f4",
	}
	action := "keystroke"
	if sk, ok := special[key]; ok {
		key = sk
		action = "key code" // Actually use "keystroke" for named keys too
		// AppleScript uses "key code" for special keys
		return fmt.Sprintf(`tell application "System Events" to %s "%s" using {%s}`, "keystroke", key, strings.Join(modifiers, ", "))
	}
	if len(modifiers) > 0 {
		return fmt.Sprintf(`tell application "System Events" to %s "%s" using {%s}`, action, key, strings.Join(modifiers, ", "))
	}
	return fmt.Sprintf(`tell application "System Events" to %s "%s"`, action, key)
}

// --- Helpers ---

func textResult(text string) mcpToolResult {
	return mcpToolResult{Content: []mcpContent{{Type: "text", Text: text}}}
}

func errResult(msg string) mcpToolResult {
	return mcpToolResult{Content: []mcpContent{{Type: "text", Text: msg}}, IsError: true}
}

// --- RPC dispatcher ---

func dispatch(method string, params json.RawMessage) (interface{}, *rpcError) {
	switch method {
	case "tools/list":
		return map[string]interface{}{"tools": getTools()}, nil

	case "tools/call":
		var p struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &rpcError{Code: -32602, Message: "invalid params: " + err.Error()}
		}
		result := callTool(p.Name, p.Arguments)
		return result, nil

	case "initialize":
		return map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"serverInfo":      map[string]string{"name": "starclaw-mcp-bridge", "version": version},
			"capabilities":    map[string]interface{}{"tools": map[string]bool{"listChanged": false}},
		}, nil

	default:
		return nil, &rpcError{Code: -32601, Message: "method not found: " + method}
	}
}

func callTool(name string, args map[string]interface{}) mcpToolResult {
	switch name {
	case "shell_exec":
		return execShell(args)
	case "file_read":
		return execFileRead(args)
	case "file_write":
		return execFileWrite(args)
	case "file_list":
		return execFileList(args)
	case "system_info":
		return execSystemInfo()
	case "open_url":
		return execOpenURL(args)
	case "open_app":
		return execOpenApp(args)
	case "clipboard_read":
		return execClipboardRead()
	case "clipboard_write":
		return execClipboardWrite(args)
	case "screen_capture":
		return execScreenCapture(args)
	case "keyboard_type":
		return execKeyboardType(args)
	case "key_combo":
		return execKeyCombo(args)
	case "mouse_click":
		return execMouseClick(args)
	case "mouse_move":
		return execMouseMove(args)
	case "active_window":
		return execActiveWindow()
	default:
		return errResult("unknown tool: " + name)
	}
}

// --- HTTP handler ---

func handler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet && r.URL.Path == "/health" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "mcp-bridge", "version": version})
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}

	var req jsonRPCRequest
	if err := json.Unmarshal(body, &req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jsonRPCResponse{
			JSONRPC: "2.0", ID: nil,
			Error: &rpcError{Code: -32700, Message: "parse error"},
		})
		return
	}

	result, rpcErr := dispatch(req.Method, req.Params)

	resp := jsonRPCResponse{JSONRPC: "2.0", ID: req.ID}
	if rpcErr != nil {
		resp.Error = rpcErr
	} else {
		resp.Result = result
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func main() {
	port := "9100"
	for i, arg := range os.Args {
		if (arg == "-port" || arg == "--port") && i+1 < len(os.Args) {
			port = os.Args[i+1]
		}
	}

	fmt.Printf(`
  ╔══════════════════════════════════════════╗
  ║   StarClaw MCP Bridge v%s            ║
  ║   Host Control Server                    ║
  ╠══════════════════════════════════════════╣
  ║   Listening on :%-5s                    ║
  ║   OS: %-8s  Arch: %-8s          ║
  ╠══════════════════════════════════════════╣
  ║   Tools: shell_exec, file_read,          ║
  ║          file_write, file_list,          ║
  ║          system_info, open_url,          ║
  ║          open_app, clipboard_read,       ║
  ║          clipboard_write, screen_capture,║
  ║          keyboard_type, key_combo,       ║
  ║          mouse_click, mouse_move,        ║
  ║          active_window                   ║
  ╠══════════════════════════════════════════╣
  ║   Add in Claw → Settings → MCP 工具:    ║
  ║   URL: http://host.docker.internal:%s ║
  ╚══════════════════════════════════════════╝
`, version, port, runtime.GOOS, runtime.GOARCH, port)

	http.HandleFunc("/", handler)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
