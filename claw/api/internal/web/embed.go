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
	for _, dir := range []string{"dist", "web/dist", "../web/dist"} {
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
	webFS := resolveFS()
	if webFS == nil {
		log.Printf("[web] No frontend found (neither external dist/ nor embedded)")
		return
	}

	fileServer := http.FileServer(webFS)

	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path

		// Skip API routes
		if strings.HasPrefix(path, "/v1/") || strings.HasPrefix(path, "/metrics") {
			c.JSON(404, gin.H{"error": "not found"})
			return
		}

		// Static assets: serve directly (check both with and without leading slash)
		trimmed := strings.TrimPrefix(path, "/")
		if strings.Contains(trimmed, ".") {
			// Has file extension — serve the static file directly
			fileServer.ServeHTTP(c.Writer, c.Request)
			return
		}

		// SPA fallback: no extension = client-side route, serve index.html
		c.Request.URL.Path = "/"
		fileServer.ServeHTTP(c.Writer, c.Request)
	})
}
