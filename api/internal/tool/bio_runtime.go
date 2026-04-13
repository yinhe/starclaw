package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func resolvePythonInvocation() (string, []string, error) {
	if configured := strings.TrimSpace(os.Getenv("STARCLAW_PYTHON")); configured != "" {
		return configured, nil, nil
	}

	candidates := []struct {
		name string
		args []string
	}{
		{name: "python3"},
		{name: "python"},
		{name: "py", args: []string{"-3"}},
	}

	for _, candidate := range candidates {
		if _, err := exec.LookPath(candidate.name); err == nil {
			return candidate.name, candidate.args, nil
		}
	}

	return "", nil, fmt.Errorf("python interpreter not found")
}

func resolveScriptPath(scriptName string) (string, error) {
	if strings.TrimSpace(scriptName) == "" {
		return "", fmt.Errorf("script name is required")
	}
	if filepath.IsAbs(scriptName) {
		if info, err := os.Stat(scriptName); err == nil && !info.IsDir() {
			return scriptName, nil
		}
	}

	candidates := []string{}
	if scriptDir := strings.TrimSpace(os.Getenv("STARCLAW_SCRIPT_DIR")); scriptDir != "" {
		candidates = append(candidates, filepath.Join(scriptDir, scriptName))
	}
	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates,
			filepath.Join(wd, "scripts", scriptName),
			filepath.Join(wd, "api", "scripts", scriptName),
		)
	}
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		candidates = append(candidates,
			filepath.Join(exeDir, "scripts", scriptName),
			filepath.Join(exeDir, "..", "scripts", scriptName),
			filepath.Join(exeDir, "..", "..", "scripts", scriptName),
		)
	}
	candidates = append(candidates, filepath.Join(string(os.PathSeparator), "app", "scripts", scriptName))

	seen := map[string]bool{}
	for _, candidate := range candidates {
		if candidate == "" || seen[candidate] {
			continue
		}
		seen[candidate] = true
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("script not found: %s", scriptName)
}

func runPythonScriptArgs(ctx context.Context, scriptName string, args ...string) (string, error) {
	pythonCmd, pythonArgs, err := resolvePythonInvocation()
	if err != nil {
		return "", err
	}
	scriptPath, err := resolveScriptPath(scriptName)
	if err != nil {
		return "", err
	}

	cmdArgs := append(append([]string{}, pythonArgs...), scriptPath)
	cmdArgs = append(cmdArgs, args...)
	cmd := hiddenCmdCtx(ctx, pythonCmd, cmdArgs...)
	cmd.Env = append(os.Environ(), "PYTHONUTF8=1")
	output, err := cmd.CombinedOutput()
	outputStr := strings.TrimSpace(string(output))
	if err != nil {
		return outputStr, fmt.Errorf("%s failed: %v", scriptName, err)
	}
	return outputStr, nil
}

func runPythonJSONScript(ctx context.Context, scriptName string, payload interface{}) (string, error) {
	pythonCmd, pythonArgs, err := resolvePythonInvocation()
	if err != nil {
		return "", err
	}
	scriptPath, err := resolveScriptPath(scriptName)
	if err != nil {
		return "", err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	cmdArgs := append(append([]string{}, pythonArgs...), scriptPath)
	cmd := hiddenCmdCtx(ctx, pythonCmd, cmdArgs...)
	cmd.Env = append(os.Environ(), "PYTHONUTF8=1")
	cmd.Stdin = bytes.NewReader(body)
	output, err := cmd.CombinedOutput()
	outputStr := strings.TrimSpace(string(output))
	if err != nil {
		return "", fmt.Errorf("%s failed: %v %s", scriptName, err, outputStr)
	}
	if !json.Valid([]byte(outputStr)) {
		return "", fmt.Errorf("%s returned non-json output: %s", scriptName, outputStr)
	}
	return outputStr, nil
}
