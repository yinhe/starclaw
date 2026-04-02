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
		// Fallback to platform key
		if err := db.Where("is_platform = ? AND provider = ? AND api_key != ''", true, "qwen").First(&cfg).Error; err != nil {
			return "", ""
		}
	}
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

// SubmitToFal submits a request to fal.ai queue API and returns the request_id.
// If apiKey starts with "starai://", routes through StarAI Router proxy.
func SubmitToFal(apiKey, endpoint string, body map[string]interface{}) (string, error) {
	bodyBytes, _ := json.Marshal(body)

	var url string
	var client *http.Client
	var authHeader string

	if isStarAIKey(apiKey) {
		// Route through StarAI Router proxy
		url = StarAIProxyURL("fal", "/"+endpoint)
		c, _ := GetStarAIClient()
		if c == nil {
			return "", fmt.Errorf("StarAI proxy not initialized")
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
		return "", err
	}
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fal.ai request failed: %v", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("fal.ai error (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("failed to parse fal.ai response: %s", string(respBody))
	}

	requestID, _ := result["request_id"].(string)
	if requestID == "" {
		return "", fmt.Errorf("no request_id in fal.ai response: %s", string(respBody))
	}

	return requestID, nil
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

	for time.Now().Before(deadline) {
		req, _ := http.NewRequest("GET", statusURL, nil)
		if authHeader != "" {
			req.Header.Set("Authorization", authHeader)
		}

		resp, err := client.Do(req)
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var status map[string]interface{}
		json.Unmarshal(body, &status)

		s, _ := status["status"].(string)
		if s == "COMPLETED" {
			return FetchFalResult(apiKey, endpoint, requestID)
		}
		if s == "FAILED" {
			errMsg, _ := status["error"].(string)
			return nil, fmt.Errorf("fal.ai generation failed: %s", errMsg)
		}

		time.Sleep(5 * time.Second)
	}
	return nil, fmt.Errorf("fal.ai polling timeout after %v", timeout)
}

// FetchFalResult fetches the completed result from fal.ai
func FetchFalResult(apiKey, endpoint, requestID string) (map[string]interface{}, error) {
	var url string
	var client *http.Client
	var authHeader string

	if isStarAIKey(apiKey) {
		url = StarAIProxyURL("fal", "/"+endpoint+"/requests/"+requestID)
		c, _ := GetStarAIClient()
		if c == nil {
			return nil, fmt.Errorf("StarAI proxy not initialized")
		}
		client = c
	} else {
		url = fmt.Sprintf("https://queue.fal.run/%s/requests/%s", endpoint, requestID)
		client = http.DefaultClient
		authHeader = "Key " + apiKey
	}

	req, _ := http.NewRequest("GET", url, nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse result: %s", string(body))
	}
	return result, nil
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
		return fmt.Errorf("TTS submit failed: %v", err)
	}
	respBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode != 200 {
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

	cmd := hiddenCmdCtx(ctx, "python3", "/app/scripts/tts.py", apiKey, text, voice, outputPath)
	cmdOutput, err := cmd.CombinedOutput()
	outputStr := strings.TrimSpace(string(cmdOutput))

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
	Text  string  `json:"text"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
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
