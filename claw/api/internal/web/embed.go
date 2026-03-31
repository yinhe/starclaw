package web

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

//go:embed dist/*
var assets embed.FS

// HasEmbeddedAssets returns true if the embedded web assets contain files.
func HasEmbeddedAssets() bool {
	entries, err := fs.ReadDir(assets, "dist")
	if err != nil {
		return false
	}
	return len(entries) > 0
}

// resolveFS returns the filesystem to serve web assets from.
// Priority: external dist/ folder > embedded dist/ in binary.
// This allows frontend hot-updates without recompiling the Go binary.
func resolveFS() http.FileSystem {
	// Check for external dist/ folder next to the binary
	for _, dir := range []string{"dist", "web/dist", "../web/dist", "web"} {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			log.Printf("[web] Using external frontend: %s", dir)
			return http.Dir(dir)
		}
	}
	// Fallback to embedded
	if HasEmbeddedAssets() {
		log.Printf("[web] Using embedded frontend")
		subFS, _ := fs.Sub(assets, "dist")
		return http.FS(subFS)
	}
	return nil
}

// RegisterRoutes adds SPA static file serving to the gin engine.
// Priority: external dist/ folder > embedded dist/ in binary.
// SPA fallback: non-file paths return index.html for client-side routing.
func RegisterRoutes(r *gin.Engine) {
	// Check for external dist/ folder first
	for _, dir := range []string{"dist", "web/dist", "../web/dist", "web"} {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			log.Printf("[web] Using external frontend: %s", dir)
			registerExternal(r, dir)
			return
		}
	}
	// Fallback to embedded
	if HasEmbeddedAssets() {
		log.Printf("[web] Using embedded frontend")
		registerEmbedded(r)
		return
	}
	log.Printf("[web] No frontend found")
}

func registerExternal(r *gin.Engine, dir string) {
	// Serve /assets/* directly with proper MIME types
	r.Static("/assets", dir+"/assets")

	// Serve other static files (favicon, etc.)
	r.StaticFile("/vite.svg", dir+"/vite.svg")

	// SPA fallback: all non-API, non-file routes serve index.html
	indexPath := dir + "/index.html"
	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/v1/") || strings.HasPrefix(path, "/metrics") {
			c.JSON(404, gin.H{"error": "not found"})
			return
		}
		c.File(indexPath)
	})
}

func registerEmbedded(r *gin.Engine) {
	subFS, _ := fs.Sub(assets, "dist")
	fileServer := http.FileServer(http.FS(subFS))

	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/v1/") || strings.HasPrefix(path, "/metrics") {
			c.JSON(404, gin.H{"error": "not found"})
			return
		}
		if f, err := subFS.Open(strings.TrimPrefix(path, "/")); err == nil {
			f.Close()
			fileServer.ServeHTTP(c.Writer, c.Request)
			return
		}
		c.Request.URL.Path = "/"
		fileServer.ServeHTTP(c.Writer, c.Request)
	})
}
