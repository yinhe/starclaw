package router

import (
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw/nydus/internal/handler"
	"github.com/yinhe/starclaw/nydus/internal/middleware"
)

// Setup creates the Gin engine with all routes.
func Setup() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	// CORS — allow all origins (Nydus is internal infra)
	r.Use(cors.New(cors.Config{
		AllowAllOrigins: true,
		AllowMethods:    []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:    []string{"Origin", "Content-Type", "Authorization", "X-Nydus-Secret"},
		ExposeHeaders:   []string{"Content-Length"},
		MaxAge:          12 * time.Hour,
	}))

	// Root — JSON for API clients, redirect browsers to web UI
	r.GET("/", func(c *gin.Context) {
		accept := c.GetHeader("Accept")
		if strings.Contains(accept, "text/html") {
			c.Redirect(302, "/ui/")
			return
		}
		c.JSON(200, gin.H{
			"service": "nydus-server",
			"status":  "ok",
			"endpoints": gin.H{
				"health":   "GET /health",
				"releases": "GET /releases/latest",
				"source":   "GET /releases/source.tar.gz",
				"repos":    "GET /v1/repos",
				"commits":  "GET /v1/commits?repo=claw",
				"deploys":  "GET /v1/deploys",
			},
		})
	})

	// Health (public)
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "nydus-server"})
	})

	// Public release mirror endpoints (no auth — for Claw auto-update fallback)
	releases := r.Group("/releases")
	{
		releases.GET("/latest", handler.GetLatestRelease)
		releases.GET("/spore/latest", handler.GetSporeLatest)
		releases.GET("/download/:filename", handler.DownloadRelease)
		releases.GET("/source.tar.gz", handler.GetSourceTarball)
	}

	// Public API for web UI (read-only, no auth needed)
	// PublicOnly flag — handlers will filter out non-public repos
	v1 := r.Group("/v1")
	v1.Use(func(c *gin.Context) {
		c.Set("public_only", true)
		c.Next()
	})
	{
		v1.GET("/repos", handler.ListRepos)
		v1.GET("/repos/:name", handler.GetRepo)
		v1.GET("/repos/:name/tree", handler.GetRepoTree)
		v1.GET("/repos/:name/readme", handler.GetRepoReadme)
		v1.GET("/repos/:name/branches", handler.GetRepoBranches)
		v1.GET("/repos/:name/tags", handler.GetRepoTags)
		v1.GET("/commits", handler.GetRecentCommits)
		v1.GET("/deploys", handler.ListDeploys)
		v1.GET("/releases/latest", handler.GetLatestRelease)
		v1.GET("/releases", handler.ListReleases)
		v1.GET("/stats", handler.GetServerStats)
	}

	// Protected API (requires X-Nydus-Secret) — sees ALL repos including private
	api := r.Group("/api")
	api.Use(middleware.SecretAuth())
	{
		api.GET("/repos", handler.ListRepos)
		api.POST("/repos", handler.CreateRepo)
		api.GET("/repos/:name", handler.GetRepo)
		api.GET("/repos/:name/tree", handler.GetRepoTree)
		api.GET("/repos/:name/readme", handler.GetRepoReadme)
		api.GET("/repos/:name/branches", handler.GetRepoBranches)
		api.GET("/repos/:name/tags", handler.GetRepoTags)
		api.DELETE("/repos/:name", handler.DeleteRepo)
		api.POST("/repos/:name/deploy", handler.TriggerDeploy)
		api.GET("/commits", handler.GetRecentCommits)
		api.GET("/deploys", handler.ListDeploys)
		api.GET("/stats", handler.GetServerStats)
		api.GET("/releases/latest", handler.GetLatestRelease)
		api.GET("/releases", handler.ListReleases)
	}

	// ── Node registration (requires admin secret — nodes self-register via Claw) ──
	api.POST("/nodes/register", handler.RegisterNode)
	api.GET("/nodes", handler.ListNodes)
	api.GET("/nodes/:node_id", handler.GetNode)
	api.PUT("/nodes/:node_id/role", handler.UpdateNodeRole)
	api.DELETE("/nodes/:node_id", handler.DeleteNode)

	// ── Repo access management (admin) ──
	api.POST("/repos/:name/access", handler.GrantAccess)
	api.GET("/repos/:name/access", handler.ListAccess)
	api.DELETE("/repos/:name/access/:node_id", handler.RevokeAccess)

	// ── Authenticated node endpoints (Ed25519 or Bearer token) ──
	node := r.Group("/node")
	node.Use(middleware.ClawAuth())
	{
		node.GET("/me", handler.MyNode)
		node.GET("/repos/:name/access", handler.CheckAccess)
	}

	// Hook endpoint (called by post-receive, uses same secret)
	r.POST("/hooks/push", middleware.SecretAuth(), handler.HookPush)

	return r
}
