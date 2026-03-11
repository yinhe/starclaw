package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DeployWebTool handles deployment lifecycle actions for web apps.
type DeployWebTool struct{}

func NewDeployWebTool() *DeployWebTool { return &DeployWebTool{} }

func (t *DeployWebTool) Name() string { return "deploy_web" }

func (t *DeployWebTool) Description() string {
	return "部署网站到云平台（MVP: Vercel Deploy Hook），支持 deploy/status/rollback 操作。"
}

func (t *DeployWebTool) Parameters() interface{} {
	return JSONSchema{
		Type: "object",
		Properties: map[string]Property{
			"action": {
				Type:        "string",
				Description: "Action: deploy, status, rollback",
				Enum:        []string{"deploy", "status", "rollback"},
			},
			"provider": {
				Type:        "string",
				Description: "Deployment provider (MVP: vercel)",
				Enum:        []string{"vercel"},
			},
			"deploy_hook_url": {
				Type:        "string",
				Description: "Vercel Deploy Hook URL for deploy/rollback",
			},
			"rollback_hook_url": {
				Type:        "string",
				Description: "Optional dedicated rollback hook URL",
			},
			"project_url": {
				Type:        "string",
				Description: "Production/preview URL to probe when action=status",
			},
			"target_env": {
				Type:        "string",
				Description: "Target env for deployment",
				Enum:        []string{"preview", "production"},
			},
			"note": {
				Type:        "string",
				Description: "Optional release note or reason",
			},
		},
		Required: []string{"action", "provider"},
	}
}

type deployWebArgs struct {
	Action          string `json:"action"`
	Provider        string `json:"provider"`
	DeployHookURL   string `json:"deploy_hook_url"`
	RollbackHookURL string `json:"rollback_hook_url"`
	ProjectURL      string `json:"project_url"`
	TargetEnv       string `json:"target_env"`
	Note            string `json:"note"`
}

func (t *DeployWebTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	args, err := ParseArgs[deployWebArgs](argsJSON)
	if err != nil {
		return "", err
	}

	action := strings.ToLower(strings.TrimSpace(args.Action))
	provider := strings.ToLower(strings.TrimSpace(args.Provider))
	if provider != "vercel" {
		return "", fmt.Errorf("unsupported provider: %s", args.Provider)
	}

	switch action {
	case "deploy":
		return t.triggerHook(ctx, args, "deploy")
	case "rollback":
		return t.triggerHook(ctx, args, "rollback")
	case "status":
		return t.checkStatus(ctx, args)
	default:
		return "", fmt.Errorf("unknown action: %s", args.Action)
	}
}

func (t *DeployWebTool) triggerHook(ctx context.Context, args deployWebArgs, phase string) (string, error) {
	hookURL := strings.TrimSpace(args.DeployHookURL)
	if phase == "rollback" && strings.TrimSpace(args.RollbackHookURL) != "" {
		hookURL = strings.TrimSpace(args.RollbackHookURL)
	}
	if hookURL == "" {
		return "", fmt.Errorf("%s requires deploy_hook_url (or rollback_hook_url)", phase)
	}

	reqBody := map[string]string{
		"target_env": args.TargetEnv,
		"note":       args.Note,
		"phase":      phase,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, hookURL, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 25 * time.Second}
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("hook request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	latencyMs := time.Since(start).Milliseconds()
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("hook HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	deploymentID := ""
	var raw map[string]interface{}
	if json.Unmarshal(respBody, &raw) == nil {
		if id, ok := raw["id"].(string); ok {
			deploymentID = id
		}
		if deploymentID == "" {
			if job, ok := raw["job"].(map[string]interface{}); ok {
				if id, ok := job["id"].(string); ok {
					deploymentID = id
				}
			}
		}
	}

	return toJSON(map[string]interface{}{
		"status":         "success",
		"action":         "deploy_web",
		"phase":          phase,
		"provider":       "vercel",
		"target_env":     defaultStr(args.TargetEnv, "production"),
		"deployment_id":  deploymentID,
		"http_status":    resp.StatusCode,
		"latency_ms":     latencyMs,
		"response_snippet": truncateText(string(respBody), 500),
		"message":        fmt.Sprintf("%s hook triggered successfully", strings.Title(phase)),
	}), nil
}

func (t *DeployWebTool) checkStatus(ctx context.Context, args deployWebArgs) (string, error) {
	url := strings.TrimSpace(args.ProjectURL)
	if url == "" {
		return "", fmt.Errorf("project_url is required for status action")
	}

	ok, code, latencyMs, err := probeURL(ctx, url, 15*time.Second)
	if err != nil {
		return "", err
	}

	return toJSON(map[string]interface{}{
		"status":      "success",
		"action":      "deploy_web",
		"phase":       "status",
		"provider":    "vercel",
		"project_url": url,
		"online":      ok,
		"http_status": code,
		"latency_ms":  latencyMs,
		"message":     "deployment status checked",
	}), nil
}

func defaultStr(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}

func truncateText(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
