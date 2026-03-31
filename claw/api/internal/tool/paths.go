package tool

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

// Central path management for all media storage directories.
// Inside Docker container: /app/videos, /app/merged_videos, /app/images, etc.
// On host: these map to data/videos, data/merged_videos, data/images via volume mounts.

var (
	dataDir     string
	dataDirOnce sync.Once
)

// SetDataDir sets the root data directory (called once at startup from config).
func SetDataDir(dir string) {
	dataDir = dir
}

// GetDataDir returns the root data directory.
// Default: /app (Docker container), auto-detects host environment.
func GetDataDir() string {
	dataDirOnce.Do(func() {
		if dataDir == "" {
			dataDir = os.Getenv("STARCLAW_STORAGE_DATA_DIR")
		}
		if dataDir == "" {
			if runtime.GOOS == "windows" {
				// On Windows, never use /app (C:\app is unrelated).
				// Default to ./data relative to working directory.
				dataDir = "./data"
			} else if _, err := os.Stat("/app"); err == nil {
				// Inside Docker container, /app is the standard root
				dataDir = "/app"
			} else {
				// Try common Linux install paths
				for _, d := range []string{"/opt/starclaw/data", "/opt/claw/data"} {
					if _, err := os.Stat(d); err == nil {
						dataDir = d
						return
					}
				}
				dataDir = "/app"
			}
		}
	})
	return dataDir
}

// ── Path accessors for each media type ──

// VideosDir returns the directory for individual generated video clips.
func VideosDir() string {
	dir := filepath.Join(GetDataDir(), "videos")
	os.MkdirAll(dir, 0755)
	return dir
}

// MergedVideosDir returns the directory for merged/composed videos.
func MergedVideosDir() string {
	dir := filepath.Join(GetDataDir(), "merged_videos")
	os.MkdirAll(dir, 0755)
	return dir
}

// ThumbnailsDir returns the directory for video thumbnails.
func ThumbnailsDir() string {
	dir := filepath.Join(GetDataDir(), "thumbnails")
	os.MkdirAll(dir, 0755)
	return dir
}

// ImagesDir returns the directory for generated images and extracted frames.
func ImagesDir() string {
	dir := filepath.Join(GetDataDir(), "images")
	os.MkdirAll(dir, 0755)
	return dir
}

// MusicDir returns the directory for generated music.
func MusicDir() string {
	dir := filepath.Join(GetDataDir(), "music")
	os.MkdirAll(dir, 0755)
	return dir
}

// UploadsDir returns the directory for user uploads.
func UploadsDir() string {
	dir := filepath.Join(GetDataDir(), "uploads")
	os.MkdirAll(dir, 0755)
	return dir
}

// ── URL ↔ Path helpers ──

// VideoClipURL returns the API URL for a video clip file.
func VideoClipURL(filename string) string {
	return "/v1/videos/clips/" + filename
}

// MergedVideoURL returns the API URL for a merged video file.
func MergedVideoURL(filename string) string {
	return "/v1/videos/merged/" + filename
}

// HostDataDir returns a human-readable description of where data is stored
// on the host machine (for AI system prompt injection).
func HostDataDir() string {
	d := GetDataDir()
	if d == "/app" {
		// Inside Docker — the host path depends on docker-compose volume mapping
		// Standard mapping: ./data/videos → /app/videos, etc.
		return "data/ (宿主机项目目录下)"
	}
	return d
}

// DataDirSummary returns a structured summary of all resource directories
// suitable for injection into AI system prompts.
func DataDirSummary() string {
	return `## StarClaw 资源目录
所有生成的媒体资源存储在以下目录中（宿主机 data/ 目录）：
- data/videos/     — AI生成的视频片段（clips）
- data/merged_videos/ — 合成后的视频（merged/MV/短剧等）
- data/images/     — AI生成的图片和提取的帧
- data/thumbnails/ — 视频缩略图
- data/music/      — AI生成的音乐
- data/uploads/    — 用户上传的文件

视频画廊（/videos 页面）显示所有 video_records 表中的记录。
通过 video_generation.list_videos 可以查看当前会话或全局已生成的视频。`
}
