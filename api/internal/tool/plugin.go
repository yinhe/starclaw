package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// PluginSpec defines the JSON schema for a tool plugin
type PluginSpec struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Version     string                 `json:"version"`
	Author      string                 `json:"author"`
	Parameters  *JSONSchema            `json:"parameters"`
	Endpoint    PluginEndpoint         `json:"endpoint"`
	Headers     map[string]string      `json:"headers,omitempty"`
	BodyTemplate string               `json:"body_template,omitempty"` // Go template with {{.arg_name}}
	ResponsePath string               `json:"response_path,omitempty"` // JSON path to extract result
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

type PluginEndpoint struct {
	URL    string `json:"url"`
	Method string `json:"method"` // GET or POST
}

// PluginTool is a Tool implementation backed by a JSON plugin spec
type PluginTool struct {
	spec PluginSpec
}

func NewPluginTool(spec PluginSpec) *PluginTool {
	return &PluginTool{spec: spec}
}

func (t *PluginTool) Name() string        { return t.spec.Name }
func (t *PluginTool) Description() string  { return t.spec.Description }
func (t *PluginTool) Parameters() interface{} { return t.spec.Parameters }

func (t *PluginTool) Execute(ctx context.Context, args string) (string, error) {
	// Parse arguments
	var argMap map[string]interface{}
	if err := json.Unmarshal([]byte(args), &argMap); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	// Build request URL (replace {{placeholders}} in URL)
	url := t.spec.Endpoint.URL
	for k, v := range argMap {
		url = strings.ReplaceAll(url, "{{"+k+"}}", fmt.Sprintf("%v", v))
	}

	// Build request body
	method := strings.ToUpper(t.spec.Endpoint.Method)
	if method == "" {
		method = "GET"
	}

	var body io.Reader
	if method == "POST" {
		if t.spec.BodyTemplate != "" {
			tmpl := t.spec.BodyTemplate
			for k, v := range argMap {
				tmpl = strings.ReplaceAll(tmpl, "{{"+k+"}}", fmt.Sprintf("%v", v))
			}
			body = strings.NewReader(tmpl)
		} else {
			jsonBody, _ := json.Marshal(argMap)
			body = strings.NewReader(string(jsonBody))
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	for k, v := range t.spec.Headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 100*1024)) // 100KB limit
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	return string(respBody), nil
}

// LoadPluginsFromDir scans a directory for .json plugin specs and registers them
func LoadPluginsFromDir(registry *Registry, dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil // No plugins directory, skip
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read plugins directory: %w", err)
	}

	count := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			log.Printf("[Plugin] Failed to read %s: %v", entry.Name(), err)
			continue
		}

		var spec PluginSpec
		if err := json.Unmarshal(data, &spec); err != nil {
			log.Printf("[Plugin] Invalid spec in %s: %v", entry.Name(), err)
			continue
		}

		if spec.Name == "" || spec.Endpoint.URL == "" {
			log.Printf("[Plugin] Skipping %s: missing name or endpoint", entry.Name())
			continue
		}

		registry.Register(NewPluginTool(spec))
		count++
		log.Printf("[Plugin] Loaded: %s (%s)", spec.Name, entry.Name())
	}

	if count > 0 {
		log.Printf("[Plugin] %d plugins loaded from %s", count, dir)
	}
	return nil
}
