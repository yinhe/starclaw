package v1

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// extractVisionFromFiles scans file attachments for images and videos,
// returning base64 data URLs suitable for OpenAI-style vision messages.
//
// - Images: read and encode as data:image/...;base64,...
// - Videos: extract up to maxFrames key frames via ffmpeg, encode each as JPEG data URL
func extractVisionFromFiles(files []FileAttachment, maxFrames int) []string {
	if maxFrames <= 0 {
		maxFrames = 4
	}

	var urls []string
	for _, f := range files {
		mime := strings.ToLower(f.Mime)
		switch {
		case isImageMime(mime):
			if url := imageFileToDataURL(f); url != "" {
				urls = append(urls, url)
			}
		case isVideoMime(mime):
			frames := extractVideoFrames(f, maxFrames)
			urls = append(urls, frames...)
		}
	}
	return urls
}

func isImageMime(mime string) bool {
	return strings.HasPrefix(mime, "image/")
}

func isVideoMime(mime string) bool {
	return strings.HasPrefix(mime, "video/") ||
		mime == "application/mp4" ||
		mime == "application/x-matroska"
}

// imageFileToDataURL reads an uploaded image file and returns a base64 data URL.
func imageFileToDataURL(f FileAttachment) string {
	path := resolveUploadPath(f.Stored)
	if path == "" {
		return ""
	}

	data, err := os.ReadFile(path)
	if err != nil {
		log.Printf("[vision] failed to read image %s: %v", f.Filename, err)
		return ""
	}

	// Limit to 20MB
	if len(data) > 20*1024*1024 {
		log.Printf("[vision] image %s too large (%d bytes), skipping", f.Filename, len(data))
		return ""
	}

	mime := f.Mime
	if mime == "" {
		mime = guessMimeFromExt(f.Filename)
	}

	b64 := base64.StdEncoding.EncodeToString(data)
	return fmt.Sprintf("data:%s;base64,%s", mime, b64)
}

// extractVideoFrames uses ffmpeg to extract key frames from a video file.
// Returns base64 JPEG data URLs for each extracted frame.
func extractVideoFrames(f FileAttachment, maxFrames int) []string {
	path := resolveUploadPath(f.Stored)
	if path == "" {
		return nil
	}

	// Get video duration to calculate frame positions
	duration := getVideoDuration(path)
	if duration <= 0 {
		duration = 10 // fallback
	}

	// Calculate timestamps to extract (evenly spaced)
	var timestamps []float64
	if duration <= 5 {
		timestamps = []float64{0.5}
	} else {
		step := duration / float64(maxFrames+1)
		for i := 1; i <= maxFrames; i++ {
			ts := step * float64(i)
			timestamps = append(timestamps, ts)
		}
	}

	tmpDir := os.TempDir()
	var urls []string

	for i, ts := range timestamps {
		framePath := filepath.Join(tmpDir, fmt.Sprintf("starclaw_frame_%d_%d.jpg", time.Now().UnixMilli(), i))

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		cmd := exec.CommandContext(ctx, "ffmpeg", "-y",
			"-ss", fmt.Sprintf("%.2f", ts),
			"-i", path,
			"-frames:v", "1",
			"-q:v", "3",
			"-vf", "scale='min(1280,iw)':'min(720,ih)':force_original_aspect_ratio=decrease",
			framePath,
		)
		out, err := cmd.CombinedOutput()
		cancel()

		if err != nil {
			log.Printf("[vision] ffmpeg frame extraction failed at %.1fs for %s: %v\n%s", ts, f.Filename, err, string(out))
			continue
		}

		data, err := os.ReadFile(framePath)
		os.Remove(framePath)
		if err != nil {
			continue
		}

		b64 := base64.StdEncoding.EncodeToString(data)
		urls = append(urls, fmt.Sprintf("data:image/jpeg;base64,%s", b64))
	}

	if len(urls) > 0 {
		log.Printf("[vision] extracted %d frames from video %s (%.1fs)", len(urls), f.Filename, duration)
	}
	return urls
}

// getVideoDuration uses ffprobe to get video duration in seconds.
func getVideoDuration(path string) float64 {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path,
	)
	out, err := cmd.Output()
	if err != nil {
		return 0
	}

	var dur float64
	fmt.Sscanf(strings.TrimSpace(string(out)), "%f", &dur)
	return dur
}

func resolveUploadPath(stored string) string {
	if stored == "" {
		return ""
	}
	// Try common upload directories
	candidates := []string{
		"/app/uploads/" + stored,
		"uploads/" + stored,
		stored,
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func guessMimeFromExt(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".bmp":
		return "image/bmp"
	default:
		return "image/jpeg"
	}
}
