package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Manager manages sandboxed workspaces for coding agents
type Manager struct {
	mu         sync.Mutex
	baseDir    string
	workspaces map[string]*Workspace
}

// Workspace represents an isolated coding environment
type Workspace struct {
	ID        string
	Path      string
	CreatedAt time.Time
}

// FileInfo describes a file or directory in a workspace
type FileInfo struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size"`
}

// ExecResult holds the result of code execution
type ExecResult struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
	Duration string `json:"duration"`
}

// NewManager creates a new sandbox manager
func NewManager() *Manager {
	baseDir := "/app/workspaces"
	os.MkdirAll(baseDir, 0755)
	return &Manager{
		baseDir:    baseDir,
		workspaces: make(map[string]*Workspace),
	}
}

// GetOrCreateWorkspace returns an existing workspace or creates a new one
func (m *Manager) GetOrCreateWorkspace(id string) *Workspace {
	m.mu.Lock()
	defer m.mu.Unlock()

	if ws, ok := m.workspaces[id]; ok {
		return ws
	}

	path := filepath.Join(m.baseDir, id)
	os.MkdirAll(path, 0755)

	ws := &Workspace{
		ID:        id,
		Path:      path,
		CreatedAt: time.Now(),
	}
	m.workspaces[id] = ws
	return ws
}

// ReadFile reads a file from the workspace
func (m *Manager) ReadFile(workspaceID, filePath string) (string, error) {
	ws := m.GetOrCreateWorkspace(workspaceID)
	absPath := m.safePath(ws, filePath)
	if absPath == "" {
		return "", fmt.Errorf("invalid path: %s", filePath)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return "", fmt.Errorf("cannot read file: %v", err)
	}

	// Limit file size to 100KB for LLM context
	content := string(data)
	if len(content) > 100*1024 {
		content = content[:100*1024] + "\n... [truncated, file too large]"
	}
	return content, nil
}

// WriteFile writes content to a file in the workspace
func (m *Manager) WriteFile(workspaceID, filePath, content string) error {
	ws := m.GetOrCreateWorkspace(workspaceID)
	absPath := m.safePath(ws, filePath)
	if absPath == "" {
		return fmt.Errorf("invalid path: %s", filePath)
	}

	// Ensure parent directory exists
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("cannot create directory: %v", err)
	}

	return os.WriteFile(absPath, []byte(content), 0644)
}

// ListFiles lists files and directories in a workspace path
func (m *Manager) ListFiles(workspaceID, dirPath string) ([]FileInfo, error) {
	ws := m.GetOrCreateWorkspace(workspaceID)

	targetDir := ws.Path
	if dirPath != "" && dirPath != "." && dirPath != "/" {
		targetDir = m.safePath(ws, dirPath)
		if targetDir == "" {
			return nil, fmt.Errorf("invalid path: %s", dirPath)
		}
	}

	entries, err := os.ReadDir(targetDir)
	if err != nil {
		return nil, fmt.Errorf("cannot list directory: %v", err)
	}

	var files []FileInfo
	for _, entry := range entries {
		info, _ := entry.Info()
		size := int64(0)
		if info != nil {
			size = info.Size()
		}
		relPath, _ := filepath.Rel(ws.Path, filepath.Join(targetDir, entry.Name()))
		files = append(files, FileInfo{
			Name:  entry.Name(),
			Path:  filepath.ToSlash(relPath),
			IsDir: entry.IsDir(),
			Size:  size,
		})
	}
	return files, nil
}

// SearchFiles searches for files matching a pattern in the workspace
func (m *Manager) SearchFiles(workspaceID, pattern string) ([]FileInfo, error) {
	ws := m.GetOrCreateWorkspace(workspaceID)

	var results []FileInfo
	err := filepath.WalkDir(ws.Path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip errors
		}
		if d.IsDir() {
			return nil
		}

		relPath, _ := filepath.Rel(ws.Path, path)
		name := d.Name()

		// Match filename against pattern (case-insensitive)
		if strings.Contains(strings.ToLower(name), strings.ToLower(pattern)) ||
			strings.Contains(strings.ToLower(relPath), strings.ToLower(pattern)) {
			info, _ := d.Info()
			size := int64(0)
			if info != nil {
				size = info.Size()
			}
			results = append(results, FileInfo{
				Name:  name,
				Path:  filepath.ToSlash(relPath),
				IsDir: false,
				Size:  size,
			})
		}

		if len(results) >= 50 {
			return filepath.SkipAll
		}
		return nil
	})

	return results, err
}

// GrepFiles searches file contents for a pattern
func (m *Manager) GrepFiles(workspaceID, query string) ([]GrepResult, error) {
	ws := m.GetOrCreateWorkspace(workspaceID)

	var results []GrepResult
	filepath.WalkDir(ws.Path, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}

		// Skip binary-ish files
		ext := strings.ToLower(filepath.Ext(path))
		if !isTextFile(ext) {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil || len(data) > 1024*1024 {
			return nil
		}

		lines := strings.Split(string(data), "\n")
		relPath, _ := filepath.Rel(ws.Path, path)

		for i, line := range lines {
			if strings.Contains(strings.ToLower(line), strings.ToLower(query)) {
				results = append(results, GrepResult{
					File:    filepath.ToSlash(relPath),
					Line:    i + 1,
					Content: truncate(line, 200),
				})
				if len(results) >= 50 {
					return filepath.SkipAll
				}
			}
		}
		return nil
	})

	return results, nil
}

// GrepResult holds a search match
type GrepResult struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Content string `json:"content"`
}

// Execute runs code in the workspace with a timeout
func (m *Manager) Execute(ctx context.Context, workspaceID, language, code string, timeoutSec int) (*ExecResult, error) {
	ws := m.GetOrCreateWorkspace(workspaceID)

	if timeoutSec <= 0 || timeoutSec > 60 {
		timeoutSec = 30
	}

	timeout := time.Duration(timeoutSec) * time.Second
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var cmd *exec.Cmd

	switch strings.ToLower(language) {
	case "python", "python3", "py":
		tmpFile := filepath.Join(ws.Path, "_exec.py")
		os.WriteFile(tmpFile, []byte(code), 0644)
		defer os.Remove(tmpFile)
		cmd = exec.CommandContext(execCtx, "python3", tmpFile)

	case "javascript", "node", "js":
		tmpFile := filepath.Join(ws.Path, "_exec.js")
		os.WriteFile(tmpFile, []byte(code), 0644)
		defer os.Remove(tmpFile)
		cmd = exec.CommandContext(execCtx, "node", tmpFile)

	case "typescript", "ts":
		tmpFile := filepath.Join(ws.Path, "_exec.ts")
		os.WriteFile(tmpFile, []byte(code), 0644)
		defer os.Remove(tmpFile)
		cmd = exec.CommandContext(execCtx, "ts-node", tmpFile)

	case "bun":
		tmpFile := filepath.Join(ws.Path, "_exec.ts")
		os.WriteFile(tmpFile, []byte(code), 0644)
		defer os.Remove(tmpFile)
		cmd = exec.CommandContext(execCtx, "bun", "run", tmpFile)

	case "bash", "sh", "shell":
		tmpFile := filepath.Join(ws.Path, "_exec.sh")
		os.WriteFile(tmpFile, []byte(code), 0755)
		defer os.Remove(tmpFile)
		cmd = exec.CommandContext(execCtx, "sh", tmpFile)

	case "go", "golang":
		tmpFile := filepath.Join(ws.Path, "_exec.go")
		os.WriteFile(tmpFile, []byte(code), 0644)
		defer os.Remove(tmpFile)
		cmd = exec.CommandContext(execCtx, "go", "run", tmpFile)

	case "ruby", "rb":
		tmpFile := filepath.Join(ws.Path, "_exec.rb")
		os.WriteFile(tmpFile, []byte(code), 0644)
		defer os.Remove(tmpFile)
		cmd = exec.CommandContext(execCtx, "ruby", tmpFile)

	case "php":
		tmpFile := filepath.Join(ws.Path, "_exec.php")
		os.WriteFile(tmpFile, []byte(code), 0644)
		defer os.Remove(tmpFile)
		cmd = exec.CommandContext(execCtx, "php", tmpFile)

	case "java":
		// Write a .java file, compile and run
		tmpFile := filepath.Join(ws.Path, "Main.java")
		os.WriteFile(tmpFile, []byte(code), 0644)
		defer os.Remove(tmpFile)
		defer os.Remove(filepath.Join(ws.Path, "Main.class"))
		cmd = exec.CommandContext(execCtx, "sh", "-c", "javac Main.java && java Main")

	case "rust", "rs":
		tmpFile := filepath.Join(ws.Path, "_exec.rs")
		outFile := filepath.Join(ws.Path, "_exec_rs")
		os.WriteFile(tmpFile, []byte(code), 0644)
		defer os.Remove(tmpFile)
		defer os.Remove(outFile)
		cmd = exec.CommandContext(execCtx, "sh", "-c", fmt.Sprintf("rustc %s -o %s && %s", tmpFile, outFile, outFile))

	case "c":
		tmpFile := filepath.Join(ws.Path, "_exec.c")
		outFile := filepath.Join(ws.Path, "_exec_c")
		os.WriteFile(tmpFile, []byte(code), 0644)
		defer os.Remove(tmpFile)
		defer os.Remove(outFile)
		cmd = exec.CommandContext(execCtx, "sh", "-c", fmt.Sprintf("gcc %s -o %s -lm && %s", tmpFile, outFile, outFile))

	case "cpp", "c++", "cxx":
		tmpFile := filepath.Join(ws.Path, "_exec.cpp")
		outFile := filepath.Join(ws.Path, "_exec_cpp")
		os.WriteFile(tmpFile, []byte(code), 0644)
		defer os.Remove(tmpFile)
		defer os.Remove(outFile)
		cmd = exec.CommandContext(execCtx, "sh", "-c", fmt.Sprintf("g++ %s -o %s -lm && %s", tmpFile, outFile, outFile))

	case "perl", "pl":
		tmpFile := filepath.Join(ws.Path, "_exec.pl")
		os.WriteFile(tmpFile, []byte(code), 0644)
		defer os.Remove(tmpFile)
		cmd = exec.CommandContext(execCtx, "perl", tmpFile)

	case "lua":
		tmpFile := filepath.Join(ws.Path, "_exec.lua")
		os.WriteFile(tmpFile, []byte(code), 0644)
		defer os.Remove(tmpFile)
		cmd = exec.CommandContext(execCtx, "lua", tmpFile)

	default:
		return nil, fmt.Errorf("unsupported language: %s (supported: python, javascript, typescript, bun, bash, go, ruby, php, java, rust, c, cpp, perl, lua)", language)
	}

	cmd.Dir = ws.Path

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

	result := &ExecResult{
		Stdout:   truncate(stdout.String(), 10000),
		Stderr:   truncate(stderr.String(), 5000),
		ExitCode: 0,
		Duration: fmt.Sprintf("%.2fs", duration.Seconds()),
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else if execCtx.Err() == context.DeadlineExceeded {
			result.ExitCode = -1
			result.Stderr += "\n[execution timed out after " + fmt.Sprintf("%d", timeoutSec) + "s]"
		} else {
			result.ExitCode = -1
			result.Stderr = err.Error()
		}
	}

	return result, nil
}

// RunCommand runs a shell command in the workspace
func (m *Manager) RunCommand(ctx context.Context, workspaceID, command string, timeoutSec int) (*ExecResult, error) {
	ws := m.GetOrCreateWorkspace(workspaceID)

	if timeoutSec <= 0 || timeoutSec > 60 {
		timeoutSec = 30
	}

	// Block dangerous commands
	lower := strings.ToLower(strings.TrimSpace(command))
	dangerous := []string{"rm -rf /", "mkfs", "dd if=", ":(){", "fork bomb"}
	for _, d := range dangerous {
		if strings.Contains(lower, d) {
			return nil, fmt.Errorf("command blocked for safety: %s", command)
		}
	}

	timeout := time.Duration(timeoutSec) * time.Second
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(execCtx, "sh", "-c", command)
	cmd.Dir = ws.Path

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

	result := &ExecResult{
		Stdout:   truncate(stdout.String(), 10000),
		Stderr:   truncate(stderr.String(), 5000),
		ExitCode: 0,
		Duration: fmt.Sprintf("%.2fs", duration.Seconds()),
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else if execCtx.Err() == context.DeadlineExceeded {
			result.ExitCode = -1
			result.Stderr += "\n[execution timed out after " + fmt.Sprintf("%d", timeoutSec) + "s]"
		} else {
			result.ExitCode = -1
			result.Stderr = err.Error()
		}
	}

	return result, nil
}

// safePath resolves a path within the workspace, preventing directory traversal
func (m *Manager) safePath(ws *Workspace, relPath string) string {
	// Clean and resolve
	cleaned := filepath.Clean(relPath)
	absPath := filepath.Join(ws.Path, cleaned)

	// Ensure it's within the workspace
	if !strings.HasPrefix(absPath, ws.Path) {
		return ""
	}
	return absPath
}

func isTextFile(ext string) bool {
	textExts := map[string]bool{
		".py": true, ".js": true, ".ts": true, ".go": true, ".rs": true,
		".java": true, ".c": true, ".cpp": true, ".h": true, ".hpp": true,
		".rb": true, ".php": true, ".swift": true, ".kt": true,
		".html": true, ".css": true, ".scss": true, ".less": true,
		".json": true, ".yaml": true, ".yml": true, ".toml": true, ".xml": true,
		".md": true, ".txt": true, ".csv": true, ".sql": true,
		".sh": true, ".bash": true, ".zsh": true, ".fish": true,
		".env": true, ".gitignore": true, ".dockerfile": true,
		".jsx": true, ".tsx": true, ".vue": true, ".svelte": true,
		".r": true, ".m": true, ".lua": true, ".pl": true,
		"": true, // files without extension (Makefile, Dockerfile, etc.)
	}
	return textExts[ext]
}

func truncate(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen] + "\n... [truncated]"
	}
	return s
}
