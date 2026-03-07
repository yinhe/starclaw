package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
	Type string `json:"type"`
	Text string `json:"text"`
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
  ║          clipboard_write                 ║
  ╠══════════════════════════════════════════╣
  ║   Add in Claw → Settings → MCP 工具:    ║
  ║   URL: http://host.docker.internal:%s ║
  ╚══════════════════════════════════════════╝
`, version, port, runtime.GOOS, runtime.GOARCH, port)

	http.HandleFunc("/", handler)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
