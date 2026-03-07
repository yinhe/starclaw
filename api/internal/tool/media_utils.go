package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
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

// GetFalAPIKey retrieves the fal.ai API key from user's model config.
func GetFalAPIKey(db *gorm.DB, userID string) string {
	if userID == "" {
		return ""
	}
	var cfg model.ModelConfig
	if err := db.Where("user_id = ? AND provider = ? AND api_key != ''", userID, "fal").First(&cfg).Error; err != nil {
		// Fallback to platform key
		if err := db.Where("is_platform = ? AND provider = ? AND api_key != ''", true, "fal").First(&cfg).Error; err != nil {
			return ""
		}
	}
	return cfg.APIKey
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

// SubmitToFal submits a request to fal.ai queue API and returns the request_id
func SubmitToFal(apiKey, endpoint string, body map[string]interface{}) (string, error) {
	bodyBytes, _ := json.Marshal(body)
	url := fmt.Sprintf("https://queue.fal.run/%s", endpoint)

	req, err := http.NewRequest("POST", url, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Key "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
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
	statusURL := fmt.Sprintf("https://queue.fal.run/%s/requests/%s/status", endpoint, requestID)

	for time.Now().Before(deadline) {
		req, _ := http.NewRequest("GET", statusURL, nil)
		req.Header.Set("Authorization", "Key "+apiKey)

		resp, err := http.DefaultClient.Do(req)
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
			// Fetch the actual result
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
	url := fmt.Sprintf("https://queue.fal.run/%s/requests/%s", endpoint, requestID)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Key "+apiKey)

	resp, err := http.DefaultClient.Do(req)
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
	cmd := exec.CommandContext(ctx, "ffprobe", "-v", "quiet", "-select_streams", "v:0",
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
	cmd := exec.CommandContext(ctx, "ffprobe", "-v", "quiet", "-show_entries", "format=duration",
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
	cmd := exec.CommandContext(ctx, "ffprobe", "-v", "quiet", "-select_streams", "a",
		"-show_entries", "stream=codec_type", "-of", "csv=p=0", videoPath)
	out, _ := cmd.Output()
	return strings.TrimSpace(string(out)) != ""
}

// ── TTS ──

// GenerateTTS calls DashScope CosyVoice TTS via Python SDK subprocess.
func GenerateTTS(apiKey, text, voice, outputPath string) error {
	log.Printf("[TTS] request: voice=%s, text_len=%d, output=%s", voice, len(text), outputPath)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "python3", "/app/scripts/tts.py", apiKey, text, voice, outputPath)
	cmdOutput, err := cmd.CombinedOutput()
	outputStr := strings.TrimSpace(string(cmdOutput))

	if err != nil {
		log.Printf("[TTS] Python script failed: %v\n%s", err, outputStr)
		return fmt.Errorf("TTS failed: %v  %s", err, outputStr)
	}

	if strings.HasPrefix(outputStr, "OK:") {
		log.Printf("[TTS] succeeded: %s", outputStr)
		return nil
	}

	return fmt.Errorf("TTS unexpected output: %s", outputStr)
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
	thumbDir := "/app/thumbnails"
	os.MkdirAll(thumbDir, 0755)

	thumbFile := recordID + ".jpg"
	thumbPath := filepath.Join(thumbDir, thumbFile)

	if _, err := os.Stat(thumbPath); err == nil {
		return "/v1/videos/thumbnails/" + thumbFile
	}

	inputPath := videoSource
	if strings.HasPrefix(videoSource, "/v1/videos/merged/") {
		filename := strings.TrimPrefix(videoSource, "/v1/videos/merged/")
		inputPath = filepath.Join("/app/merged_videos", filename)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ffmpeg", "-y",
		"-i", inputPath, "-ss", "1", "-frames:v", "1", "-q:v", "2", thumbPath)
	if _, err := cmd.CombinedOutput(); err != nil {
		cmd2 := exec.CommandContext(ctx, "ffmpeg", "-y",
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
	if strings.HasPrefix(videoURL, "/v1/videos/merged/") {
		fn := strings.TrimPrefix(videoURL, "/v1/videos/merged/")
		localPath := filepath.Join("/app/merged_videos", fn)
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
		localPath := "/app/music/" + filename
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
			localPath := filepath.Join("/app/images", filename)
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
