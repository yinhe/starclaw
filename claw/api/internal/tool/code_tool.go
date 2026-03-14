package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/yinhe/starclaw/internal/model"
	"github.com/yinhe/starclaw/internal/sandbox"
	"gorm.io/gorm"
)

// CodeTool provides file operations and code execution for the coding agent
type CodeTool struct {
	sandbox    *sandbox.Manager
	processMgr *sandbox.ProcessManager
	db         *gorm.DB
}

// NewCodeTool creates a new code tool
func NewCodeTool(sb *sandbox.Manager, pm *sandbox.ProcessManager, db *gorm.DB) *CodeTool {
	return &CodeTool{sandbox: sb, processMgr: pm, db: db}
}

func (t *CodeTool) Name() string { return "code" }

func (t *CodeTool) Description() string {
	return `自主编程工具，可以在沙箱环境中读写文件、搜索代码和执行程序。
支持操作：
- read_file: 读取文件内容
- write_file: 写入/创建文件
- list_files: 列出目录文件
- search_files: 按名称搜索文件
- grep: 搜索文件内容
- execute: 执行代码（Python、JavaScript、Bash、Go）
- run_command: 运行 Shell 命令（如 pip install、ls）
- start_app: 启动后台 Web 服务
- stop_app: 停止 Web 服务
- list_apps: 列出运行中的 Web 服务

修改文件前请先读取。启动网站：写文件 → 安装依赖 → start_app。`
}

func (t *CodeTool) Parameters() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"description": "The action to perform",
				"enum":        []string{"read_file", "write_file", "list_files", "search_files", "grep", "execute", "run_command", "start_app", "stop_app", "list_apps"},
			},
			"workspace_id": map[string]interface{}{
				"type":        "string",
				"description": "Workspace ID (use 'default' if not specified)",
			},
			"path": map[string]interface{}{
				"type":        "string",
				"description": "File or directory path (for read_file, write_file, list_files)",
			},
			"content": map[string]interface{}{
				"type":        "string",
				"description": "File content (for write_file)",
			},
			"pattern": map[string]interface{}{
				"type":        "string",
				"description": "Search pattern (for search_files, grep)",
			},
			"language": map[string]interface{}{
				"type":        "string",
				"description": "Programming language (for execute: python, javascript, typescript, bun, bash, go, ruby, php, java, rust, c, cpp, perl, lua)",
			},
			"code": map[string]interface{}{
				"type":        "string",
				"description": "Code to execute (for execute action)",
			},
			"command": map[string]interface{}{
				"type":        "string",
				"description": "Shell command to run (for run_command action)",
			},
		},
		"required": []string{"action"},
	}
}

type codeArgs struct {
	Action      string `json:"action"`
	WorkspaceID string `json:"workspace_id"`
	Path        string `json:"path"`
	Content     string `json:"content"`
	Pattern     string `json:"pattern"`
	Language    string `json:"language"`
	Code        string `json:"code"`
	Command     string `json:"command"`
}

func (t *CodeTool) Execute(ctx context.Context, args string) (string, error) {
	var a codeArgs
	if err := json.Unmarshal([]byte(args), &a); err != nil {
		return "", fmt.Errorf("invalid arguments: %v", err)
	}

	if a.WorkspaceID == "" {
		// Use user_id as workspace for per-user isolation
		if uid, ok := ctx.Value(CtxKeyUserID).(string); ok && uid != "" {
			a.WorkspaceID = uid
		} else {
			a.WorkspaceID = "default"
		}
	}

	// Resolve conversation-isolated workspace: default/{conversation_id}/
	origWorkspaceID := a.WorkspaceID
	convID := ""
	if cid, ok := ctx.Value(CtxKeyConversationID).(string); ok && cid != "" {
		convID = cid
	}
	effectiveWS := origWorkspaceID
	if convID != "" {
		effectiveWS = filepath.Join(origWorkspaceID, convID)
	}

	switch a.Action {
	case "read_file":
		content, err := t.sandbox.ReadFile(effectiveWS, a.Path)
		if err != nil {
			return toJSON(map[string]interface{}{"action": "read_file", "error": err.Error()}), nil
		}
		return toJSON(map[string]interface{}{
			"action":  "read_file",
			"path":    a.Path,
			"content": content,
		}), nil

	case "write_file":
		if a.Path == "" {
			return toJSON(map[string]interface{}{"action": "write_file", "error": "path is required for write_file"}), nil
		}
		if a.Content == "" {
			log.Printf("[CodeTool] write_file called with empty content for path=%q workspace=%s — rejecting", a.Path, effectiveWS)
			return toJSON(map[string]interface{}{"action": "write_file", "error": "content is empty — please provide the file content"}), nil
		}
		err := t.sandbox.WriteFile(effectiveWS, a.Path, a.Content)
		if err != nil {
			return toJSON(map[string]interface{}{"action": "write_file", "error": err.Error()}), nil
		}
		// Record file in database for conversation tracking
		// Store path relative to workspace root: {conv_id}/{filename}
		if t.db != nil {
			dbPath := a.Path
			if convID != "" {
				dbPath = filepath.Join(convID, a.Path)
			}
			t.recordFile(ctx, origWorkspaceID, dbPath, int64(len(a.Content)))
		}
		return toJSON(map[string]interface{}{
			"action": "write_file",
			"path":   a.Path,
			"status": "success",
			"bytes":  len(a.Content),
		}), nil

	case "list_files":
		files, err := t.sandbox.ListFiles(effectiveWS, a.Path)
		if err != nil {
			return toJSON(map[string]interface{}{"action": "list_files", "error": err.Error()}), nil
		}
		return toJSON(map[string]interface{}{
			"action": "list_files",
			"path":   a.Path,
			"files":  files,
		}), nil

	case "search_files":
		files, err := t.sandbox.SearchFiles(effectiveWS, a.Pattern)
		if err != nil {
			return toJSON(map[string]interface{}{"action": "search_files", "error": err.Error()}), nil
		}
		return toJSON(map[string]interface{}{
			"action":  "search_files",
			"pattern": a.Pattern,
			"results": files,
		}), nil

	case "grep":
		results, err := t.sandbox.GrepFiles(effectiveWS, a.Pattern)
		if err != nil {
			return toJSON(map[string]interface{}{"action": "grep", "error": err.Error()}), nil
		}
		return toJSON(map[string]interface{}{
			"action":  "grep",
			"pattern": a.Pattern,
			"matches": results,
		}), nil

	case "execute":
		result, err := t.sandbox.Execute(ctx, effectiveWS, a.Language, a.Code, 30)
		if err != nil {
			return toJSON(map[string]interface{}{"action": "execute", "error": err.Error()}), nil
		}
		return toJSON(map[string]interface{}{
			"action":   "execute",
			"language": a.Language,
			"result":   result,
		}), nil

	case "run_command":
		result, err := t.sandbox.RunCommand(ctx, effectiveWS, a.Command, 60)
		if err != nil {
			return toJSON(map[string]interface{}{"action": "run_command", "error": err.Error()}), nil
		}
		return toJSON(map[string]interface{}{
			"action":  "run_command",
			"command": a.Command,
			"result":  result,
		}), nil

	case "start_app":
		if t.processMgr == nil {
			return toJSON(map[string]interface{}{"action": "start_app", "error": "process manager not available"}), nil
		}
		app, err := t.processMgr.StartApp(t.sandbox, effectiveWS, a.Command)
		if err != nil {
			return toJSON(map[string]interface{}{"action": "start_app", "error": err.Error()}), nil
		}
		return toJSON(map[string]interface{}{
			"action":       "start_app",
			"status":       "started",
			"port":         app.Port,
			"workspace_id": effectiveWS,
			"url":          fmt.Sprintf("/v1/app/%s/", effectiveWS),
			"message":      "App is starting. User can access it at the url above. The app MUST listen on the PORT environment variable.",
		}), nil

	case "stop_app":
		if t.processMgr == nil {
			return toJSON(map[string]interface{}{"action": "stop_app", "error": "process manager not available"}), nil
		}
		stopped := t.processMgr.StopApp(effectiveWS)
		return toJSON(map[string]interface{}{
			"action":  "stop_app",
			"stopped": stopped,
		}), nil

	case "list_apps":
		if t.processMgr == nil {
			return toJSON(map[string]interface{}{"action": "list_apps", "error": "process manager not available"}), nil
		}
		apps := t.processMgr.ListApps()
		appList := make([]map[string]interface{}, len(apps))
		for i, app := range apps {
			appList[i] = map[string]interface{}{
				"workspace_id": app.WorkspaceID,
				"port":         app.Port,
				"command":      app.Command,
				"ready":        app.Ready,
				"started_at":   app.StartedAt.Format("15:04:05"),
				"url":          fmt.Sprintf("/v1/app/%s/", app.WorkspaceID),
			}
		}
		return toJSON(map[string]interface{}{
			"action": "list_apps",
			"apps":   appList,
		}), nil

	default:
		return toJSON(map[string]interface{}{
			"error": fmt.Sprintf("unknown action: %s", a.Action),
		}), nil
	}
}

// recordFile saves or updates a WorkspaceFile record in the database
func (t *CodeTool) recordFile(ctx context.Context, workspaceID, path string, size int64) {
	userID := ""
	if uid, ok := ctx.Value(CtxKeyUserID).(string); ok {
		userID = uid
	}
	convID := ""
	if cid, ok := ctx.Value(CtxKeyConversationID).(string); ok {
		convID = cid
	}

	name := filepath.Base(path)
	category := classifyFile(name)

	// Upsert: if same workspace+path exists, update it; otherwise create
	var existing model.WorkspaceFile
	result := t.db.Where("workspace_id = ? AND path = ?", workspaceID, path).First(&existing)
	if result.Error == nil {
		// Update existing record
		updates := map[string]interface{}{
			"size":     size,
			"name":     name,
			"category": category,
		}
		if convID != "" {
			updates["conversation_id"] = convID
		}
		if userID != "" {
			updates["user_id"] = userID
		}
		t.db.Model(&existing).Updates(updates)
		log.Printf("[CodeTool] Updated file record: %s/%s (conv=%s)", workspaceID, path, convID)
	} else {
		// Create new record
		f := model.WorkspaceFile{
			UserID:         userID,
			ConversationID: convID,
			WorkspaceID:    workspaceID,
			Path:           path,
			Name:           name,
			Category:       category,
			Size:           size,
		}
		t.db.Create(&f)
		log.Printf("[CodeTool] Created file record: %s/%s (conv=%s)", workspaceID, path, convID)
	}
}

// classifyFile determines if a file is code or document based on extension
func classifyFile(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	codeExts := map[string]bool{
		".py": true, ".js": true, ".ts": true, ".jsx": true, ".tsx": true,
		".go": true, ".java": true, ".c": true, ".cpp": true, ".h": true,
		".rs": true, ".rb": true, ".php": true, ".sh": true, ".bat": true,
		".ps1": true, ".sql": true, ".r": true, ".m": true, ".swift": true,
		".kt": true, ".scala": true, ".lua": true, ".pl": true, ".dart": true,
		".css": true, ".scss": true, ".less": true, ".vue": true, ".svelte": true,
		".json": true, ".yaml": true, ".yml": true, ".toml": true, ".xml": true,
		".ini": true, ".cfg": true, ".conf": true, ".env": true,
		".html": true, ".htm": true,
	}
	if codeExts[ext] {
		return "code"
	}
	return "document"
}

func toJSON(v interface{}) string {
	data, _ := json.Marshal(v)
	return string(data)
}
