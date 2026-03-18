package web

import (
	"embed"
	"io/fs"
	"net/http"
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

// RegisterRoutes adds SPA static file serving to the gin engine.
// All requests not matched by API routes will be served from the embedded dist/ directory.
// SPA fallback: non-file paths return index.html for client-side routing.
func RegisterRoutes(r *gin.Engine) {
	if !HasEmbeddedAssets() {
		return
	}

	subFS, _ := fs.Sub(assets, "dist")
	fileServer := http.FileServer(http.FS(subFS))

	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path

		// Skip API routes
		if strings.HasPrefix(path, "/v1/") || strings.HasPrefix(path, "/metrics") {
			c.JSON(404, gin.H{"error": "not found"})
			return
		}

		// Try to serve the exact file
		if f, err := subFS.Open(strings.TrimPrefix(path, "/")); err == nil {
			f.Close()
			fileServer.ServeHTTP(c.Writer, c.Request)
			return
		}

		// SPA fallback: serve index.html for all other paths
		c.Request.URL.Path = "/"
		fileServer.ServeHTTP(c.Writer, c.Request)
	})
}
