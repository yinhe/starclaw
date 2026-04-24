package tool

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/yinhe/starclaw/internal/model"
	"gorm.io/gorm"
)

// ── Shared API Key Helpers ──

// GetDashScopeAPIKey retrieves the DashScope (qwen) API key from user's model config.
// Also checks for platform-level configs. Always returns the China mainland host for video/TTS.
func GetDashScopeAPIKey(db *gorm.DB, userID string) (string, string) {
	if userID == "" {
		return "", ""
	}
	var cfg model.ModelConfig
	// Try user's own key first
	if err := db.Where("user_id = ? AND provider = ? AND api_key != ''", userID, "qwen").First(&cfg).Error; err != nil {
		log.Printf("[DashScope] no key for user %s: %v", userID, err)
		// Fallback to platform key
		if err := db.Where("is_platform = ? AND provider = ? AND api_key != ''", true, "qwen").First(&cfg).Error; err != nil {
			log.Printf("[DashScope] no platform key: %v", err)
			// Fallback to system user's key (owner-configured keys)
			if err := db.Where("user_id = ? AND provider = ? AND api_key != ''", "system", "qwen").First(&cfg).Error; err != nil {
				log.Printf("[DashScope] no system key: %v", err)
				return "", ""
			}
		}
	}
	log.Printf("[DashScope] found key for user=%s, cfg_id=%s, provider=%s", userID, cfg.ID, cfg.Provider)
	return cfg.APIKey, "dashscope.aliyuncs.com"
}

// GetDashScopeAPIKeyCtx checks user's own key first, falls back to StarAI proxy.
// If user configured their own DashScope key, use it directly (free, no 星能 deduction).
// Otherwise route through StarAI proxy (deducts 星能).
func GetDashScopeAPIKeyCtx(ctx context.Context, db *gorm.DB, userID string) (string, string) {
	// User's own key takes priority (no 星能 cost)
	if key, host := GetDashScopeAPIKey(db, userID); key != "" {
		return key, host
	}
	// Fallback to StarAI proxy (deducts 星能)
	if client, _ := GetStarAIClient(); client != nil {
		return "starai://qwen", "starai-proxy"
	}
	return "", ""
}

// GetFalAPIKeyCtx always routes fal.ai calls through StarAI proxy.
// fal.ai is not available as a direct provider on Claw — must go through StarAI (deducts 星能).
func GetFalAPIKeyCtx(ctx context.Context, db *gorm.DB, userID string) string {
	if client, _ := GetStarAIClient(); client != nil {
		return "starai://fal"
	}
	return "" // fal.ai not available without StarAI connection
}

func GetVolcengineAPIKey(db *gorm.DB, userID string) (string, string) {
	defaultBaseURL := "https://ark.cn-beijing.volces.com/api/v3"
	if userID == "" {
		return "", defaultBaseURL
	}
	var cfg model.ModelConfig
	if err := db.Where("user_id = ? AND provider = ? AND api_key != ''", userID, "volcengine").First(&cfg).Error; err != nil {
		if err := db.Where("is_platform = ? AND provider = ? AND api_key != ''", true, "volcengine").First(&cfg).Error; err != nil {
			if err := db.Where("user_id = ? AND provider = ? AND api_key != ''", "system", "volcengine").First(&cfg).Error; err != nil {
				return "", defaultBaseURL
			}
		}
	}
	if cfg.BaseURL != "" {
		return cfg.APIKey, cfg.BaseURL
	}
	return cfg.APIKey, defaultBaseURL
}

func GetVolcengineAPIKeyCtx(_ context.Context, db *gorm.DB, userID string) (string, string) {
	if key, baseURL := GetVolcengineAPIKey(db, userID); key != "" {
		return key, baseURL
	}
	if client, _ := GetStarAIClient(); client != nil {
		return "starai://volcengine", "starai-proxy"
	}
	return "", ""
}

// ── Generation Audit Log ──

// LogGeneration creates a unified GenerationLog entry for reconciliation with Router.
// superProvider is auto-detected from apiKey: "starai://*" → "starai", otherwise "direct".
func LogGeneration(db *gorm.DB, apiKey string, opts GenLogOpts) string {
	sp := "direct"
	if isStarAIKey(apiKey) {
		sp = "starai"
	}
	entry := model.GenerationLog{
		UserID:         opts.UserID,
		ConversationID: opts.ConversationID,
		SuperProvider:  sp,
		Provider:       opts.Provider,
		Model:          opts.Model,
		Type:           opts.Type,
		TaskID:         opts.TaskID,
		RecordID:       opts.RecordID,
		Prompt:         opts.Prompt,
		Status:         opts.Status,
	}
	if err := db.Create(&entry).Error; err != nil {
		log.Printf("[GenLog] failed to create: %v", err)
		return ""
	}
	log.Printf("[GenLog] %s/%s %s via %s (task=%s record=%s)", opts.Provider, opts.Model, opts.Type, sp, opts.TaskID, opts.RecordID)
	return entry.ID
}

// UpdateGenLog updates a GenerationLog entry's status and optional fields.
func UpdateGenLog(db *gorm.DB, logID string, status string, resultURL string, errMsg string) {
	if logID == "" {
		return
	}
	updates := map[string]interface{}{"status": status}
	if resultURL != "" {
		updates["result_url"] = resultURL
	}
	if errMsg != "" {
		updates["error_msg"] = errMsg
	}
	if status == "succeeded" || status == "failed" {
		now := time.Now()
		updates["completed_at"] = &now
	}
	db.Model(&model.GenerationLog{}).Where("id = ?", logID).Updates(updates)
}

// GenLogOpts holds parameters for LogGeneration
type GenLogOpts struct {
	UserID         string
	ConversationID string
	Provider       string // fal, dashscope, minimax
	Model          string // veo3, flux-dev, etc.
	Type           string // image, video, audio
	TaskID         string
	RecordID       string
	Prompt         string
	Status         string // pending, running
}

// ── File Path Resolution ──

// ResolveClipToLocal resolves a video URL to a local file path.
// Handles: local /v1/videos/clips/, /v1/videos/merged/ paths, absolute paths, and remote HTTP URLs.
// For remote URLs it downloads to dest. Returns the usable local path.
func ResolveClipToLocal(videoURL, dest string) (string, error) {
	// New path: /v1/videos/clips/filename → VideosDir()/filename
	if strings.HasPrefix(videoURL, "/v1/videos/clips/") {
		fn := strings.TrimPrefix(videoURL, "/v1/videos/clips/")
		localPath := filepath.Join(VideosDir(), fn)
		if _, err := os.Stat(localPath); err == nil {
			return localPath, nil
		}
		// local file missing, fall through to download
	}
	if strings.HasPrefix(videoURL, "/v1/videos/merged/") {
		fn := strings.TrimPrefix(videoURL, "/v1/videos/merged/")
		localPath := filepath.Join(MergedVideosDir(), fn)
		if _, err := os.Stat(localPath); err == nil {
			return localPath, nil
		}
		// local file missing, fall through to download
	}
	if strings.HasPrefix(videoURL, "/app/") || strings.HasPrefix(videoURL, "/tmp/") {
		if _, err := os.Stat(videoURL); err == nil {
			return videoURL, nil
		}
		return "", fmt.Errorf("local file not found: %s", videoURL)
	}
	if !strings.HasPrefix(videoURL, "http://") && !strings.HasPrefix(videoURL, "https://") {
		return "", fmt.Errorf("unsupported URL scheme: %s", videoURL)
	}
	if err := DownloadFile(videoURL, dest); err != nil {
		return "", fmt.Errorf("download failed: %v", err)
	}
	return dest, nil
}

// SaveClipLocally downloads a remote URL to the videos directory and returns the local serving path.
// If already local, returns the existing path. Used to persist clips for reliable merging.
func SaveClipLocally(remoteURL string) (string, error) {
	if strings.HasPrefix(remoteURL, "/v1/videos/clips/") || strings.HasPrefix(remoteURL, "/v1/videos/merged/") {
		return remoteURL, nil // already local
	}
	if !strings.HasPrefix(remoteURL, "http://") && !strings.HasPrefix(remoteURL, "https://") {
		return remoteURL, nil // not a remote URL
	}
	outputDir := VideosDir()
	localFile := fmt.Sprintf("clip_%s.mp4", generateShortID())
	localPath := filepath.Join(outputDir, localFile)
	if err := DownloadFile(remoteURL, localPath); err != nil {
		return remoteURL, err // return original URL on failure
	}
	return VideoClipURL(localFile), nil
}

func generateShortID() string {
	b := make([]byte, 4)
	if _, err := io.ReadFull(strings.NewReader(fmt.Sprintf("%d", time.Now().UnixNano())), b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano()%100000000)
	}
	return fmt.Sprintf("%d", time.Now().UnixNano()%100000000)
}

// ── File Download ──

// DownloadFile downloads a URL to a local file path
func DownloadFile(url, dest string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, resp.Body)
	return err
}

// ── fal.ai Queue Helpers ──

// isStarAIKey returns true if the API key is a StarAI proxy marker
func isStarAIKey(apiKey string) bool {
	return strings.HasPrefix(apiKey, "starai://")
}

// SubmitToFal submits a request to fal.ai queue API and returns the request_id
// plus the status endpoint (extracted from fal.ai's status_url response).
// The status endpoint may differ from the submit endpoint (e.g. "fal-ai/flux" vs "fal-ai/flux/schnell").
// If apiKey starts with "starai://", routes through StarAI Router proxy.
func SubmitToFal(apiKey, endpoint string, body map[string]interface{}) (string, string, error) {
	bodyBytes, _ := json.Marshal(body)

	var url string
	var client *http.Client
	var authHeader string

	if isStarAIKey(apiKey) {
		// Route through StarAI Router proxy
		url = StarAIProxyURL("fal", "/"+endpoint)
		c, _ := GetStarAIClient()
		if c == nil {
			return "", "", fmt.Errorf("StarAI proxy not initialized")
		}
		client = c
		// No auth header needed — SignedTransport adds X-Claw-* headers
		log.Printf("[StarAI] fal submit via proxy: %s", url)
	} else {
		// Direct fal.ai call
		url = fmt.Sprintf("https://queue.fal.run/%s", endpoint)
		client = http.DefaultClient
		authHeader = "Key " + apiKey
	}

	req, err := http.NewRequest("POST", url, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return "", "", err
	}
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("fal.ai request failed: %v", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return "", "", fmt.Errorf("fal.ai error (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", "", fmt.Errorf("failed to parse fal.ai response: %s", string(respBody))
	}

	requestID, _ := result["request_id"].(string)
	if requestID == "" {
		return "", "", fmt.Errorf("no request_id in fal.ai response: %s", string(respBody))
	}

	// Extract status endpoint from fal.ai's status_url (may differ from submit endpoint).
	// e.g. submit to "fal-ai/flux/schnell" but status_url uses "fal-ai/flux"
	statusEndpoint := endpoint
	if statusURL, ok := result["status_url"].(string); ok && statusURL != "" {
		// Parse: https://queue.fal.run/{endpoint}/requests/{id}/status
		const prefix = "https://queue.fal.run/"
		if strings.HasPrefix(statusURL, prefix) {
			path := strings.TrimPrefix(statusURL, prefix)
			if idx := strings.Index(path, "/requests/"); idx > 0 {
				derivedEndpoint := path[:idx]
				if strings.Contains(endpoint, "nano-banana") && strings.Contains(endpoint, "/edit") && !strings.Contains(derivedEndpoint, "/edit") {
					statusEndpoint = derivedEndpoint
					log.Printf("[fal.ai] using nano-banana status endpoint: submit=%s status=%s", endpoint, derivedEndpoint)
				} else if strings.Contains(endpoint, "/edit") && !strings.Contains(derivedEndpoint, "/edit") {
					log.Printf("[fal.ai] keeping edit endpoint: submit=%s status=%s", endpoint, derivedEndpoint)
					statusEndpoint = endpoint
				} else {
					statusEndpoint = derivedEndpoint
				}
				if statusEndpoint != endpoint {
					log.Printf("[fal.ai] status endpoint differs: submit=%s status=%s", endpoint, statusEndpoint)
				}
			}
		}
	}

	return requestID, statusEndpoint, nil
}

func hasFalResultPayload(result map[string]interface{}) bool {
	if images, ok := result["images"].([]interface{}); ok && len(images) > 0 {
		if img, ok := images[0].(map[string]interface{}); ok {
			if u, ok := img["url"].(string); ok && u != "" {
				return true
			}
		}
	}
	if img, ok := result["image"].(map[string]interface{}); ok {
		if u, ok := img["url"].(string); ok && u != "" {
			return true
		}
	}
	if u, ok := result["image_url"].(string); ok && u != "" {
		return true
	}
	if u, ok := result["video_url"].(string); ok && u != "" {
		return true
	}
	if u, ok := result["audio_url"].(string); ok && u != "" {
		return true
	}
	return false
}

// PollFalStatus polls a fal.ai queue request until completion or timeout.
// Returns the full result JSON map on success.
func PollFalStatus(apiKey, endpoint, requestID string, timeout time.Duration) (map[string]interface{}, error) {
	deadline := time.Now().Add(timeout)

	var statusURL string
	var client *http.Client
	var authHeader string

	if isStarAIKey(apiKey) {
		statusURL = StarAIProxyURL("fal", "/"+endpoint+"/requests/"+requestID+"/status")
		c, _ := GetStarAIClient()
		if c == nil {
			return nil, fmt.Errorf("StarAI proxy not initialized")
		}
		client = c
	} else {
		statusURL = fmt.Sprintf("https://queue.fal.run/%s/requests/%s/status", endpoint, requestID)
		client = http.DefaultClient
		authHeader = "Key " + apiKey
	}

	errCount := 0
	for time.Now().Before(deadline) {
		req, _ := http.NewRequest("GET", statusURL, nil)
		if authHeader != "" {
			req.Header.Set("Authorization", authHeader)
		}

		resp, err := client.Do(req)
		if err != nil {
			errCount++
			log.Printf("[fal.ai] poll error (%d): %v", errCount, err)
			if errCount >= 5 {
				return nil, fmt.Errorf("fal.ai polling failed after %d network errors: %v", errCount, err)
			}
			time.Sleep(5 * time.Second)
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		// Non-retryable HTTP errors
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			return nil, fmt.Errorf("fal.ai auth error (HTTP %d): %s", resp.StatusCode, string(body))
		}
		if resp.StatusCode >= 500 {
			errCount++
			log.Printf("[fal.ai] poll server error (%d): HTTP %d: %s", errCount, resp.StatusCode, string(body))
			if errCount >= 5 {
				return nil, fmt.Errorf("fal.ai polling failed: HTTP %d after %d retries", resp.StatusCode, errCount)
			}
			time.Sleep(5 * time.Second)
			continue
		}

		var status map[string]interface{}
		json.Unmarshal(body, &status)

		s, _ := status["status"].(string)
		if hasFalResultPayload(status) && s != "FAILED" {
			return status, nil
		}
		if s == "COMPLETED" {
			return FetchFalResult(apiKey, endpoint, requestID)
		}
		if s == "FAILED" {
			errMsg, _ := status["error"].(string)
			return nil, fmt.Errorf("fal.ai generation failed: %s", errMsg)
		}

		errCount = 0 // reset on successful poll
		time.Sleep(5 * time.Second)
	}
	return nil, fmt.Errorf("fal.ai polling timeout after %v", timeout)
}

func falEndpointVariants(endpoint string) []string {
	variants := []string{endpoint}
	trimmed := strings.TrimSuffix(endpoint, "/edit")
	if trimmed != endpoint && trimmed != "" {
		variants = append(variants, trimmed)
	}
	return variants
}

// FetchFalResult fetches the completed result from fal.ai
func FetchFalResult(apiKey, endpoint, requestID string) (map[string]interface{}, error) {
	var lastErr error
	for _, candidate := range falEndpointVariants(endpoint) {
		var url string
		var client *http.Client
		var authHeader string

		if isStarAIKey(apiKey) {
			url = StarAIProxyURL("fal", "/"+candidate+"/requests/"+requestID)
			c, _ := GetStarAIClient()
			if c == nil {
				return nil, fmt.Errorf("StarAI proxy not initialized")
			}
			client = c
		} else {
			url = fmt.Sprintf("https://queue.fal.run/%s/requests/%s", candidate, requestID)
			client = http.DefaultClient
			authHeader = "Key " + apiKey
		}

		req, _ := http.NewRequest("GET", url, nil)
		if authHeader != "" {
			req.Header.Set("Authorization", authHeader)
		}

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode >= 400 {
			lastErr = fmt.Errorf("fal.ai result error (HTTP %d): %s", resp.StatusCode, string(body))
			continue
		}

		var result map[string]interface{}
		if err := json.Unmarshal(body, &result); err != nil {
			lastErr = fmt.Errorf("failed to parse result: %s", string(body))
			continue
		}
		if hasFalResultPayload(result) || len(result) > 0 {
			return result, nil
		}
		lastErr = fmt.Errorf("empty fal result for endpoint %s", candidate)
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("fal.ai result fetch failed for request %s", requestID)
}

// ── Video/Audio Probing ──

// ProbeVideoDimensions returns the width and height of a video file using ffprobe
func ProbeVideoDimensions(videoPath string) (int, int) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := hiddenCmdCtx(ctx, "ffprobe", "-v", "quiet", "-select_streams", "v:0",
		"-show_entries", "stream=width,height", "-of", "csv=p=0:s=x", videoPath)
	out, err := cmd.Output()
	if err != nil {
		return 1280, 720
	}
	parts := strings.Split(strings.TrimSpace(string(out)), "x")
	if len(parts) >= 2 {
		w, _ := strconv.Atoi(parts[0])
		h, _ := strconv.Atoi(parts[1])
		if w > 0 && h > 0 {
			return w, h
		}
	}
	return 1280, 720
}

// ProbeDuration returns the duration in seconds of a media file
func ProbeDuration(filePath string) float64 {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := hiddenCmdCtx(ctx, "ffprobe", "-v", "quiet", "-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1", filePath)
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	if d, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64); err == nil && d > 0 {
		return d
	}
	return 0
}

// ProbeHasAudio checks if a video file has an audio stream
func ProbeHasAudio(videoPath string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := hiddenCmdCtx(ctx, "ffprobe", "-v", "quiet", "-select_streams", "a",
		"-show_entries", "stream=codec_type", "-of", "csv=p=0", videoPath)
	out, _ := cmd.Output()
	return strings.TrimSpace(string(out)) != ""
}

// ── TTS ──

// GenerateTTS calls DashScope CosyVoice TTS.
// If apiKey is a StarAI proxy key (starai://qwen), routes through StarAI HTTP proxy.
// Otherwise, calls DashScope via Python SDK subprocess.
func GenerateTTS(apiKey, text, voice, outputPath string) error {
	log.Printf("[TTS] request: voice=%s, text_len=%d, output=%s, via_starai=%v", voice, len(text), outputPath, isStarAIKey(apiKey))

	if isStarAIKey(apiKey) {
		return generateTTSViaStarAI(text, voice, outputPath)
	}
	return generateTTSViaPython(apiKey, text, voice, outputPath)
}

// generateTTSViaStarAI routes TTS through StarAI proxy using DashScope HTTP API.
func generateTTSViaStarAI(text, voice, outputPath string) error {
	client, _ := GetStarAIClient()
	if client == nil {
		return fmt.Errorf("StarAI proxy not initialized")
	}

	// DashScope CosyVoice TTS async endpoint
	reqURL := StarAIProxyURL("dashscope", "/api/v1/services/aigc/text2audio/speech-synthesizer")
	reqBody := map[string]interface{}{
		"model": "cosyvoice-v1",
		"input": map[string]interface{}{
			"text": text,
		},
		"parameters": map[string]interface{}{
			"voice":       voice,
			"format":      "mp3",
			"sample_rate": 22050,
		},
	}
	bodyBytes, _ := json.Marshal(reqBody)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", reqURL, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return fmt.Errorf("build TTS request failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-DashScope-Async", "enable")

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[TTS/StarAI] submit HTTP failed: %v", err)
		return fmt.Errorf("TTS submit failed: %v", err)
	}
	respBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Printf("[TTS/StarAI] submit error HTTP %d: %s", resp.StatusCode, string(respBody))
		return fmt.Errorf("TTS submit error (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	var submitResult struct {
		Output struct {
			TaskID string `json:"task_id"`
		} `json:"output"`
		RequestID string `json:"request_id"`
	}
	if err := json.Unmarshal(respBody, &submitResult); err != nil {
		return fmt.Errorf("TTS submit parse failed: %s", string(respBody))
	}
	taskID := submitResult.Output.TaskID
	if taskID == "" {
		return fmt.Errorf("TTS submit: no task_id in response: %s", string(respBody))
	}
	log.Printf("[TTS/StarAI] submitted task_id=%s", taskID)

	// Poll task status
	pollURL := StarAIProxyURL("dashscope", "/api/v1/tasks/"+taskID)
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(2 * time.Second)

		pollReq, _ := http.NewRequestWithContext(ctx, "GET", pollURL, nil)
		pollResp, err := client.Do(pollReq)
		if err != nil {
			continue
		}
		pollBody, _ := io.ReadAll(pollResp.Body)
		pollResp.Body.Close()

		var taskResult struct {
			Output struct {
				TaskStatus string `json:"task_status"`
				AudioURL   string `json:"audio_url"`
				Results    []struct {
					URL string `json:"url"`
				} `json:"results"`
			} `json:"output"`
		}
		json.Unmarshal(pollBody, &taskResult)

		switch taskResult.Output.TaskStatus {
		case "SUCCEEDED":
			audioURL := taskResult.Output.AudioURL
			if audioURL == "" && len(taskResult.Output.Results) > 0 {
				audioURL = taskResult.Output.Results[0].URL
			}
			if audioURL == "" {
				return fmt.Errorf("TTS succeeded but no audio URL: %s", string(pollBody))
			}
			// Download audio to output path
			dlResp, err := http.Get(audioURL)
			if err != nil {
				return fmt.Errorf("TTS download failed: %v", err)
			}
			defer dlResp.Body.Close()
			f, err := os.Create(outputPath)
			if err != nil {
				return fmt.Errorf("TTS create file failed: %v", err)
			}
			written, err := io.Copy(f, dlResp.Body)
			f.Close()
			if err != nil {
				return fmt.Errorf("TTS write failed: %v", err)
			}
			log.Printf("[TTS/StarAI] succeeded: %d bytes → %s", written, outputPath)
			return nil

		case "FAILED":
			return fmt.Errorf("TTS task failed: %s", string(pollBody))
		}
	}
	return fmt.Errorf("TTS task timeout (90s)")
}

// generateTTSViaPython calls DashScope CosyVoice TTS via Python SDK subprocess (direct API key).
func generateTTSViaPython(apiKey, text, voice, outputPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	outputStr, err := runPythonScriptArgs(ctx, "tts.py", apiKey, text, voice, outputPath)

	if err != nil {
		log.Printf("[TTS] Python script failed: %v\n%s", err, outputStr)
		return fmt.Errorf("TTS failed: %v  %s", err, outputStr)
	}

	if strings.HasPrefix(outputStr, "OK:") {
		log.Printf("[TTS] succeeded: %s", outputStr)
		return nil
	}

	return fmt.Errorf("TTS unexpected output: %s", outputStr)
}

// ── Volcengine TTS 2.0 ──

// isVolcengineVoice returns true if the voice_type is a Volcengine TTS voice.
// Volcengine voices contain suffixes like _bigtts, _streaming, or _tob.
// DashScope CosyVoice voices are short names like "longyuan", "longhua".
func isVolcengineVoice(voice string) bool {
	return strings.Contains(voice, "_bigtts") || strings.Contains(voice, "_streaming") ||
		strings.Contains(voice, "_tob") || strings.HasPrefix(voice, "BV")
}

// isVolcengine2Voice returns true for Volcengine 2.0 voices that support context_texts.
func isVolcengine2Voice(voice string) bool {
	return strings.Contains(voice, "_uranus_bigtts") || strings.Contains(voice, "_mars_bigtts") ||
		strings.Contains(voice, "_jupiter_bigtts") || strings.Contains(voice, "_moon_bigtts")
}

// GetVolcengineTTSAPIKey retrieves the Volcengine TTS API key (X-Api-Key for openspeech.bytedance.com).
// Checks model_configs table (provider="volcengine-tts"), then env var VOLCENGINE_TTS_API_KEY.
func GetVolcengineTTSAPIKey(db *gorm.DB, userID string) string {
	if db != nil {
		var cfg model.ModelConfig
		// Try user's own key
		if userID != "" {
			if err := db.Where("user_id = ? AND provider = ? AND api_key != ''", userID, "volcengine-tts").First(&cfg).Error; err == nil {
				return cfg.APIKey
			}
		}
		// Try platform-level key
		if err := db.Where("is_platform = ? AND provider = ? AND api_key != ''", true, "volcengine-tts").First(&cfg).Error; err == nil {
			return cfg.APIKey
		}
		// Try system user's key
		if err := db.Where("user_id = ? AND provider = ? AND api_key != ''", "system", "volcengine-tts").First(&cfg).Error; err == nil {
			return cfg.APIKey
		}
	}
	return os.Getenv("VOLCENGINE_TTS_API_KEY")
}

// GenerateVolcengineTTS calls Volcengine TTS V3 HTTP Chunked API (openspeech.bytedance.com).
// instruction is the emotion/style context for 2.0 voices (mapped to context_texts, not billed).
// Example instructions: "用害怕紧张的语气说", "你可以用撒娇的语气说话吗？"
func GenerateVolcengineTTS(apiKey, text, voice, instruction, outputPath string) error {
	is2 := isVolcengine2Voice(voice)
	resourceID := "seed-tts-1.0"
	if is2 {
		resourceID = "seed-tts-2.0"
	}

	log.Printf("[TTS/Volcengine] request: voice=%s, resource=%s, text_len=%d, instruction=%q, output=%s",
		voice, resourceID, len(text), instruction, outputPath)

	reqParams := map[string]interface{}{
		"text":    text,
		"speaker": voice,
		"audio_params": map[string]interface{}{
			"format":      "mp3",
			"sample_rate": 24000,
		},
	}

	// Add context_texts for emotion instruction (TTS 2.0 only, not billed)
	if instruction != "" && is2 {
		additionsMap := map[string]interface{}{
			"context_texts": []string{instruction},
		}
		additionsJSON, _ := json.Marshal(additionsMap)
		reqParams["additions"] = string(additionsJSON)
	}

	reqBody := map[string]interface{}{
		"user":       map[string]interface{}{"uid": "claw-tts"},
		"req_params": reqParams,
	}

	bodyBytes, _ := json.Marshal(reqBody)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://openspeech.bytedance.com/api/v3/tts/unidirectional",
		strings.NewReader(string(bodyBytes)))
	if err != nil {
		return fmt.Errorf("build Volcengine TTS request failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", apiKey)
	req.Header.Set("X-Api-Resource-Id", resourceID)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("Volcengine TTS HTTP failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Volcengine TTS HTTP %d: %s", resp.StatusCode, string(body))
	}

	// Read chunked response: each line is JSON with base64 audio in "data" field
	// Response format: {"code":0,"data":"BASE64"}...{"code":20000000,"message":"ok","data":null}
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("Volcengine TTS read failed: %v", err)
	}

	var audioData []byte
	for _, line := range strings.Split(string(respBody), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var chunk struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Data    string `json:"data"`
		}
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			continue
		}
		if chunk.Code == 20000000 {
			break // success end marker
		}
		if chunk.Code != 0 {
			return fmt.Errorf("Volcengine TTS error: code=%d msg=%s", chunk.Code, chunk.Message)
		}
		if chunk.Data != "" {
			decoded, err := base64.StdEncoding.DecodeString(chunk.Data)
			if err != nil {
				log.Printf("[TTS/Volcengine] base64 decode error on chunk: %v", err)
				continue
			}
			audioData = append(audioData, decoded...)
		}
	}

	if len(audioData) == 0 {
		return fmt.Errorf("Volcengine TTS: no audio data received (resp=%d bytes)", len(respBody))
	}

	if err := os.WriteFile(outputPath, audioData, 0644); err != nil {
		return fmt.Errorf("Volcengine TTS write failed: %v", err)
	}

	log.Printf("[TTS/Volcengine] succeeded: %d bytes → %s", len(audioData), outputPath)
	return nil
}

// ── TTS Duration Fitting (professional dubbing sync) ──

const (
	maxTempoSpeedup  = 1.35 // max TTS speedup before intelligibility degrades
	minSegmentGap    = 0.3  // minimum 300ms gap between consecutive segments
	maxTempoSlowdown = 0.75 // max TTS slowdown for short segments
)

// FitTTSToWindow adjusts a TTS audio file to fit within a time window.
// If TTS is longer than the window: speed up with atempo (max 1.35x), then hard-trim if needed.
// If TTS is shorter: optionally slow down for very short segments.
// Returns the adjusted audio path and the actual fitted duration.
func FitTTSToWindow(inputAudio string, windowDuration float64, tmpDir string, index int) (string, float64, error) {
	actualDur := ProbeDuration(inputAudio)
	if actualDur <= 0 {
		return inputAudio, windowDuration, fmt.Errorf("failed to probe TTS duration: %s", inputAudio)
	}

	// If TTS fits within window (with small tolerance), no adjustment needed
	if actualDur <= windowDuration+0.1 {
		// If window is much longer than TTS and TTS is very short, optionally slow down
		if windowDuration > 0 && actualDur < windowDuration*0.5 && actualDur > 0.5 {
			ratio := actualDur / windowDuration
			if ratio < maxTempoSlowdown {
				ratio = maxTempoSlowdown
			}
			outputPath := filepath.Join(tmpDir, fmt.Sprintf("fitted_%03d.mp3", index))
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			cmd := hiddenCmdCtx(ctx, "ffmpeg", "-y", "-i", inputAudio,
				"-af", fmt.Sprintf("atempo=%.4f", ratio),
				"-c:a", "libmp3lame", "-q:a", "2", outputPath)
			if _, err := cmd.CombinedOutput(); err != nil {
				log.Printf("[DubbingFit] slowdown failed (ratio=%.2f), using original: %v", ratio, err)
				return inputAudio, actualDur, nil
			}
			fittedDur := ProbeDuration(outputPath)
			if fittedDur > 0 {
				log.Printf("[DubbingFit] seg %d: slowed %.2fs→%.2fs (ratio=%.2f) to fill %.2fs window", index, actualDur, fittedDur, ratio, windowDuration)
				return outputPath, fittedDur, nil
			}
		}
		return inputAudio, actualDur, nil
	}

	// TTS is too long — need to speed up
	ratio := actualDur / windowDuration // e.g., 7s / 5s = 1.4x speedup needed

	outputPath := filepath.Join(tmpDir, fmt.Sprintf("fitted_%03d.mp3", index))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if ratio <= maxTempoSpeedup {
		// Speed up within intelligibility limit
		cmd := hiddenCmdCtx(ctx, "ffmpeg", "-y", "-i", inputAudio,
			"-af", fmt.Sprintf("atempo=%.4f", ratio),
			"-c:a", "libmp3lame", "-q:a", "2", outputPath)
		if out, err := cmd.CombinedOutput(); err != nil {
			log.Printf("[DubbingFit] atempo failed: %v\n%s", err, string(out))
			return inputAudio, actualDur, nil // fallback to original
		}
		fittedDur := ProbeDuration(outputPath)
		if fittedDur <= 0 {
			fittedDur = windowDuration
		}
		log.Printf("[DubbingFit] seg %d: sped up %.2fs→%.2fs (atempo=%.2fx) for %.2fs window", index, actualDur, fittedDur, ratio, windowDuration)
		return outputPath, fittedDur, nil
	}

	// Ratio too high for pure atempo — speed up at max rate then hard-trim
	cmd := hiddenCmdCtx(ctx, "ffmpeg", "-y", "-i", inputAudio,
		"-af", fmt.Sprintf("atempo=%.4f", maxTempoSpeedup),
		"-t", fmt.Sprintf("%.3f", windowDuration),
		"-c:a", "libmp3lame", "-q:a", "2", outputPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("[DubbingFit] atempo+trim failed: %v\n%s", err, string(out))
		return inputAudio, actualDur, nil
	}
	fittedDur := ProbeDuration(outputPath)
	if fittedDur <= 0 {
		fittedDur = windowDuration
	}
	log.Printf("[DubbingFit] seg %d: sped up+trimmed %.2fs→%.2fs (max atempo=%.2fx, trimmed to %.2fs)", index, actualDur, fittedDur, maxTempoSpeedup, windowDuration)
	return outputPath, fittedDur, nil
}

// FitTTSUniform applies a pre-calculated global tempo ratio to a TTS audio file.
// Unlike FitTTSToWindow which calculates per-segment ratios, this ensures all segments
// in a video share the same speech rate for consistent dubbing.
// If the adjusted audio still overflows the window, it is hard-trimmed.
func FitTTSUniform(inputAudio string, rawDuration, globalRatio, windowDuration float64, tmpDir string, index int) (string, float64, error) {
	if rawDuration <= 0 {
		return inputAudio, windowDuration, fmt.Errorf("invalid raw duration")
	}

	// No adjustment needed if ratio is ~1.0 and audio fits window
	needsAtempo := globalRatio < 0.98 || globalRatio > 1.02
	adjustedDur := rawDuration / globalRatio

	if !needsAtempo && rawDuration <= windowDuration+0.1 {
		return inputAudio, rawDuration, nil
	}

	outputPath := filepath.Join(tmpDir, fmt.Sprintf("uniform_%03d.mp3", index))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if needsAtempo {
		args := []string{"-y", "-i", inputAudio, "-af", fmt.Sprintf("atempo=%.4f", globalRatio)}
		// Hard-trim if adjusted duration still overflows window
		if adjustedDur > windowDuration+0.1 {
			args = append(args, "-t", fmt.Sprintf("%.3f", windowDuration))
		}
		args = append(args, "-c:a", "libmp3lame", "-q:a", "2", outputPath)
		cmd := hiddenCmdCtx(ctx, "ffmpeg", args...)
		if out, err := cmd.CombinedOutput(); err != nil {
			log.Printf("[DubbingFit] uniform atempo failed seg %d: %v\n%s", index, err, string(out))
			return inputAudio, rawDuration, nil
		}
	} else if rawDuration > windowDuration+0.1 {
		// No atempo needed but audio overflows — just trim
		cmd := hiddenCmdCtx(ctx, "ffmpeg", "-y", "-i", inputAudio,
			"-t", fmt.Sprintf("%.3f", windowDuration),
			"-c:a", "libmp3lame", "-q:a", "2", outputPath)
		if out, err := cmd.CombinedOutput(); err != nil {
			log.Printf("[DubbingFit] trim failed seg %d: %v\n%s", index, err, string(out))
			return inputAudio, rawDuration, nil
		}
	} else {
		return inputAudio, rawDuration, nil
	}

	fittedDur := ProbeDuration(outputPath)
	if fittedDur <= 0 {
		fittedDur = windowDuration
	}
	log.Printf("[DubbingFit] seg %d: uniform %.2fs→%.2fs (globalRatio=%.3f, window=%.2fs)", index, rawDuration, fittedDur, globalRatio, windowDuration)
	return outputPath, fittedDur, nil
}

// FitNarrationSegments adjusts narration segment timings based on actual TTS durations.
// It enforces minimum gaps between segments and prevents overlaps.
// Returns updated segments with corrected start/end times matching actual audio.
func FitNarrationSegments(segments []NarrationSegment, audioDurations []float64, totalVideoDuration float64) []NarrationSegment {
	if len(segments) == 0 || len(audioDurations) != len(segments) {
		return segments
	}

	fitted := make([]NarrationSegment, len(segments))
	copy(fitted, segments)

	for i := range fitted {
		actualDur := audioDurations[i]
		windowDur := fitted[i].End - fitted[i].Start

		// Use actual duration if it's shorter than window (natural pacing)
		// Use window if audio was trimmed to fit
		useDur := actualDur
		if useDur > windowDur {
			useDur = windowDur
		}

		fitted[i].End = fitted[i].Start + useDur

		// Enforce minimum gap with next segment
		if i < len(fitted)-1 {
			nextStart := segments[i+1].Start
			if fitted[i].End+minSegmentGap > nextStart {
				// Shrink current segment end to maintain gap
				maxEnd := nextStart - minSegmentGap
				if maxEnd > fitted[i].Start+0.5 { // keep at least 0.5s
					fitted[i].End = maxEnd
				}
			}
		}

		// Clamp to video duration
		if fitted[i].End > totalVideoDuration {
			fitted[i].End = totalVideoDuration
		}
	}

	return fitted
}

// ── Subtitle Utilities ──

// NarrationSegment represents one voiceover segment with timing
type NarrationSegment struct {
	Text        string  `json:"text"`
	Start       float64 `json:"start"`
	End         float64 `json:"end"`
	Voice       string  `json:"voice,omitempty"`
	Instruction string  `json:"instruction,omitempty"` // Volcengine 2.0: emotion/style instruction (context_texts, not billed)
}

// SubtitleStyle holds adaptive subtitle rendering parameters based on video orientation
type SubtitleStyle struct {
	FontSize int
	FontName string
	MarginV  int
	MarginL  int
	MarginR  int
	Outline  int
	Shadow   int
	MaxChars int  // max characters per subtitle segment
	Enabled  bool // false to skip subtitle burning
}

// ForceStyleString builds the ASS force_style parameter for ffmpeg subtitles filter
func (s SubtitleStyle) ForceStyleString() string {
	return fmt.Sprintf("FontSize=%d,FontName=%s,PrimaryColour=&H00FFFFFF,OutlineColour=&H00000000,Outline=%d,Shadow=%d,MarginV=%d,MarginL=%d,MarginR=%d,WrapStyle=0",
		s.FontSize, s.FontName, s.Outline, s.Shadow, s.MarginV, s.MarginL, s.MarginR)
}

// GetSubtitleStyle returns optimal subtitle parameters based on video dimensions and user preference.
func GetSubtitleStyle(videoPath string, userStyle string) SubtitleStyle {
	if userStyle == "none" {
		return SubtitleStyle{Enabled: false, MaxChars: 12}
	}

	w, h := ProbeVideoDimensions(videoPath)
	isPortrait := h > w
	isSquare := w == h

	style := SubtitleStyle{
		FontName: "Noto Sans CJK SC",
		Outline:  2,
		Shadow:   1,
		Enabled:  true,
	}

	if isPortrait {
		style.FontSize = 14
		style.MarginV = 80
		style.MarginL = 40
		style.MarginR = 40
		style.MaxChars = 10
	} else if isSquare {
		style.FontSize = 18
		style.MarginV = 50
		style.MarginL = 30
		style.MarginR = 30
		style.MaxChars = 14
	} else {
		style.FontSize = 22
		style.MarginV = 30
		style.MarginL = 20
		style.MarginR = 20
		style.MaxChars = 18
	}

	switch userStyle {
	case "small":
		style.FontSize -= 4
		if style.FontSize < 10 {
			style.FontSize = 10
		}
	case "large":
		style.FontSize += 4
		if style.FontSize > 28 {
			style.FontSize = 28
		}
	}

	return style
}

// GenerateSRT creates an SRT subtitle file from narration segments
func GenerateSRT(segments []NarrationSegment, outputPath string) error {
	var sb strings.Builder
	for i, seg := range segments {
		startH := int(seg.Start) / 3600
		startM := (int(seg.Start) % 3600) / 60
		startS := int(seg.Start) % 60
		startMs := int((seg.Start - float64(int(seg.Start))) * 1000)

		endH := int(seg.End) / 3600
		endM := (int(seg.End) % 3600) / 60
		endS := int(seg.End) % 60
		endMs := int((seg.End - float64(int(seg.End))) * 1000)

		sb.WriteString(fmt.Sprintf("%d\n", i+1))
		sb.WriteString(fmt.Sprintf("%02d:%02d:%02d,%03d --> %02d:%02d:%02d,%03d\n", startH, startM, startS, startMs, endH, endM, endS, endMs))
		sb.WriteString(seg.Text + "\n\n")
	}
	return os.WriteFile(outputPath, []byte(sb.String()), 0644)
}

// SplitNarrationText splits long text into shorter segments at punctuation boundaries.
func SplitNarrationText(text string, maxChars int) []string {
	if maxChars <= 0 {
		maxChars = 12
	}
	runes := []rune(text)
	if len(runes) <= maxChars {
		return []string{text}
	}

	breakPuncts := "，。！？、；：…—·!?;:."
	var result []string
	start := 0

	for start < len(runes) {
		if start+maxChars >= len(runes) {
			seg := strings.TrimSpace(string(runes[start:]))
			if seg != "" {
				result = append(result, seg)
			}
			break
		}

		end := start + maxChars
		bestBreak := -1
		for i := end; i > start; i-- {
			if strings.ContainsRune(breakPuncts, runes[i]) {
				bestBreak = i + 1
				break
			}
		}

		if bestBreak > start {
			seg := strings.TrimSpace(string(runes[start:bestBreak]))
			if seg != "" {
				result = append(result, seg)
			}
			start = bestBreak
		} else {
			seg := strings.TrimSpace(string(runes[start:end]))
			if seg != "" {
				result = append(result, seg)
			}
			start = end
		}
	}

	return result
}

// SplitNarrationToSegments splits narration text into timed subtitle segments.
func SplitNarrationToSegments(text string, startTime, endTime float64, maxChars int) []NarrationSegment {
	parts := SplitNarrationText(text, maxChars)
	if len(parts) == 0 {
		return []NarrationSegment{{Text: text, Start: startTime, End: endTime}}
	}
	if len(parts) == 1 {
		return []NarrationSegment{{Text: parts[0], Start: startTime, End: endTime}}
	}

	totalDuration := endTime - startTime
	totalChars := 0
	for _, p := range parts {
		totalChars += utf8.RuneCountInString(p)
	}
	if totalChars == 0 {
		totalChars = 1
	}

	var segments []NarrationSegment
	currentTime := startTime
	for _, p := range parts {
		chars := utf8.RuneCountInString(p)
		segDuration := totalDuration * float64(chars) / float64(totalChars)
		if segDuration < 0.5 {
			segDuration = 0.5
		}
		segments = append(segments, NarrationSegment{
			Text:  p,
			Start: currentTime,
			End:   currentTime + segDuration,
		})
		currentTime += segDuration
	}
	if len(segments) > 0 {
		segments[len(segments)-1].End = endTime
	}

	return segments
}

// ── Thumbnail Extraction ──

// ExtractThumbnail uses ffmpeg to grab a frame from a video and save as JPEG.
func ExtractThumbnail(db *gorm.DB, recordID, videoSource string) string {
	thumbDir := ThumbnailsDir()

	thumbFile := recordID + ".jpg"
	thumbPath := filepath.Join(thumbDir, thumbFile)

	if _, err := os.Stat(thumbPath); err == nil {
		return "/v1/videos/thumbnails/" + thumbFile
	}

	inputPath := videoSource
	if strings.HasPrefix(videoSource, "/v1/videos/clips/") {
		filename := strings.TrimPrefix(videoSource, "/v1/videos/clips/")
		inputPath = filepath.Join(VideosDir(), filename)
	} else if strings.HasPrefix(videoSource, "/v1/videos/merged/") {
		filename := strings.TrimPrefix(videoSource, "/v1/videos/merged/")
		inputPath = filepath.Join(MergedVideosDir(), filename)
	}

	// Skip if input is a remote URL (ffmpeg can't reliably handle these on Windows)
	if strings.HasPrefix(inputPath, "http://") || strings.HasPrefix(inputPath, "https://") {
		return ""
	}
	// Skip if local file doesn't exist — avoids ffmpeg popup windows on Windows
	if _, err := os.Stat(inputPath); err != nil {
		return ""
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := hiddenCmdCtx(ctx, "ffmpeg", "-y",
		"-i", inputPath, "-ss", "1", "-frames:v", "1", "-q:v", "2", thumbPath)
	if _, err := cmd.CombinedOutput(); err != nil {
		cmd2 := hiddenCmdCtx(ctx, "ffmpeg", "-y",
			"-i", inputPath, "-frames:v", "1", "-q:v", "2", thumbPath)
		if _, err2 := cmd2.CombinedOutput(); err2 != nil {
			log.Printf("[Thumbnail] extraction failed for %s: %v", recordID, err2)
			return ""
		}
	}

	thumbURL := "/v1/videos/thumbnails/" + thumbFile
	db.Model(&model.VideoRecord{}).Where("id = ?", recordID).Update("img_url", thumbURL)

	return thumbURL
}

// ── Resolve Helpers ──

// ResolveLocalVideoPath resolves a video URL to a local file path if stored locally,
// or downloads it to tmpDir. Returns the local path.
func ResolveLocalVideoPath(videoURL, tmpDir, filename string) (string, error) {
	if strings.HasPrefix(videoURL, "/v1/videos/clips/") {
		fn := strings.TrimPrefix(videoURL, "/v1/videos/clips/")
		localPath := filepath.Join(VideosDir(), fn)
		if _, err := os.Stat(localPath); err == nil {
			return localPath, nil
		}
	}
	if strings.HasPrefix(videoURL, "/v1/videos/merged/") {
		fn := strings.TrimPrefix(videoURL, "/v1/videos/merged/")
		localPath := filepath.Join(MergedVideosDir(), fn)
		if _, err := os.Stat(localPath); err == nil {
			return localPath, nil
		}
	}
	dest := filepath.Join(tmpDir, filename)
	if err := DownloadFile(videoURL, dest); err != nil {
		return "", fmt.Errorf("failed to download video: %v", err)
	}
	return dest, nil
}

// ResolveMusicPath resolves a music_id to a local file path
func ResolveMusicPath(db *gorm.DB, musicID, tmpDir string) (string, error) {
	var music model.MusicRecord
	if err := db.Where("id = ?", musicID).First(&music).Error; err != nil {
		return "", fmt.Errorf("music not found: %s", musicID)
	}
	if music.Status != "succeeded" {
		return "", fmt.Errorf("music not ready (status: %s)", music.Status)
	}

	if strings.HasPrefix(music.LocalURL, "/v1/music/") {
		filename := strings.TrimPrefix(music.LocalURL, "/v1/music/")
		localPath := filepath.Join(MusicDir(), filename)
		if _, err := os.Stat(localPath); err == nil {
			return localPath, nil
		}
	}

	if music.AudioURL != "" {
		dlPath := filepath.Join(tmpDir, "music.mp3")
		if err := DownloadFile(music.AudioURL, dlPath); err != nil {
			return "", fmt.Errorf("failed to download music: %v", err)
		}
		return dlPath, nil
	}

	return "", fmt.Errorf("music has no audio file")
}

// ResolveImagePath finds the local image file for an image record, downloading if needed
func ResolveImagePath(db *gorm.DB, imageID, tmpDir string, idx int) (string, error) {
	var rec model.ImageRecord
	if err := db.Where("id = ?", imageID).First(&rec).Error; err != nil {
		return "", fmt.Errorf("image record not found: %s", imageID)
	}
	if rec.Status != "succeeded" {
		return "", fmt.Errorf("image not ready (status: %s)", rec.Status)
	}

	if rec.LocalURL != "" {
		if strings.HasPrefix(rec.LocalURL, "/v1/images/") {
			filename := strings.TrimPrefix(rec.LocalURL, "/v1/images/")
			localPath := filepath.Join(ImagesDir(), filename)
			if _, err := os.Stat(localPath); err == nil {
				return localPath, nil
			}
		}
	}

	if rec.ImageURL != "" {
		ext := ".png"
		if strings.Contains(strings.ToLower(rec.ImageURL), ".jpg") {
			ext = ".jpg"
		}
		dlPath := filepath.Join(tmpDir, fmt.Sprintf("img_%03d%s", idx, ext))
		if err := DownloadFile(rec.ImageURL, dlPath); err != nil {
			return "", fmt.Errorf("failed to download image: %v", err)
		}
		return dlPath, nil
	}

	return "", fmt.Errorf("image has no URL")
}

// ResolveImageURL gets the remote image URL from an ImageRecord (needed for DashScope i2v)
func ResolveImageURL(db *gorm.DB, imageID string) (string, error) {
	var rec model.ImageRecord
	if err := db.Where("id = ?", imageID).First(&rec).Error; err != nil {
		return "", fmt.Errorf("image record not found: %s", imageID)
	}
	if rec.Status != "succeeded" {
		return "", fmt.Errorf("image not ready (status: %s)", rec.Status)
	}
	if rec.ImageURL != "" {
		return rec.ImageURL, nil
	}
	return "", fmt.Errorf("image has no remote URL: %s", imageID)
}

func ResolveImageInputForProvider(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("empty image input")
	}
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") || strings.HasPrefix(raw, "asset://") || strings.HasPrefix(raw, "data:image/") {
		return raw, nil
	}
	localPath := ""
	if strings.HasPrefix(raw, "/v1/images/") {
		filename := strings.TrimPrefix(raw, "/v1/images/")
		localPath = filepath.Join(ImagesDir(), filename)
	} else if strings.HasPrefix(raw, "/v1/projects/") {
		// /v1/projects/<project>/<fp> → $DOCS_DIR/<project>/<fp>（默认 /app/docs）
		// 用于直接把 manifest/剧本目录下的本地图片（如 lin_jianyue/unified_sheet_v6.png）
		// 读盘编码为 base64 data URL，供 Seedance 等远端 API 直接消费，无需再上传 TOS。
		stripped := strings.TrimPrefix(raw, "/v1/projects/")
		if q := strings.IndexByte(stripped, '?'); q >= 0 {
			stripped = stripped[:q]
		}
		if strings.Contains(stripped, "..") {
			return "", fmt.Errorf("path traversal blocked: %s", raw)
		}
		parts := strings.SplitN(stripped, "/", 2)
		if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
			docsDir := os.Getenv("DOCS_DIR")
			if docsDir == "" {
				docsDir = "/app/docs"
			}
			localPath = filepath.Join(docsDir, parts[0], parts[1])
		}
	} else if strings.HasPrefix(raw, "/app/") || filepath.IsAbs(raw) {
		localPath = raw
	}
	if localPath == "" {
		return raw, nil
	}
	data, err := os.ReadFile(localPath)
	if err != nil {
		return "", fmt.Errorf("read image failed: %v", err)
	}
	ext := strings.ToLower(filepath.Ext(localPath))
	mime := "image/png"
	switch ext {
	case ".jpg", ".jpeg":
		mime = "image/jpeg"
	case ".webp":
		mime = "image/webp"
	case ".bmp":
		mime = "image/bmp"
	case ".gif":
		mime = "image/gif"
	case ".tif", ".tiff":
		mime = "image/tiff"
	}
	return fmt.Sprintf("data:%s;base64,%s", mime, base64.StdEncoding.EncodeToString(data)), nil
}
