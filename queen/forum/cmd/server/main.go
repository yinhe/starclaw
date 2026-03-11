package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/yinhe/starclaw-queen/forum/internal/handler"
	"github.com/yinhe/starclaw-queen/forum/internal/model"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	dsn := getEnv("FORUM_DSN", "root:starclaw@tcp(127.0.0.1:3306)/starclaw_queen?charset=utf8mb4&parseTime=True&loc=Local")
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect database: %v", err)
	}
	db.AutoMigrate(&model.ForumCategory{}, &model.Post{}, &model.Reply{}, &model.PostLike{})

	seedCategories(db)

	mode := getEnv("GIN_MODE", "debug")
	if mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	h := handler.NewForumHandler(db)

	forum := r.Group("/forum")
	{
		forum.GET("/categories", h.ListCategories)
		forum.GET("/posts", h.ListPosts)
		forum.POST("/posts", h.CreatePost)
		forum.GET("/posts/:id", h.GetPost)
		forum.DELETE("/posts/:id", h.DeletePost)
		forum.POST("/posts/:id/replies", h.CreateReply)
		forum.POST("/posts/:id/like", h.LikePost)
		forum.GET("/search", h.Search)
		forum.GET("/stats", h.Stats)
	}

	r.GET("/metrics", gin.WrapH(promhttp.Handler()))
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "queen-forum"})
	})

	port := getEnv("FORUM_PORT", "8093")
	log.Printf("[forum] Queen Forum service starting on :%s", port)
	if err := r.Run(fmt.Sprintf(":%s", port)); err != nil {
		log.Fatalf("Failed to start forum service: %v", err)
	}
}

func seedCategories(db *gorm.DB) {
	cats := []model.ForumCategory{
		{ID: "agent-tips", Name: "Agent 玩法", NameEn: "Agent Tips", Description: "分享 Agent 配置技巧和最佳实践", Icon: "Bot", SortOrder: 1},
		{ID: "workflow-share", Name: "工作流分享", NameEn: "Workflow Sharing", Description: "分享你的工作流模板和自动化方案", Icon: "GitBranch", SortOrder: 2},
		{ID: "tech-discuss", Name: "技术讨论", NameEn: "Tech Discussion", Description: "LLM、RAG、MCP 等技术话题", Icon: "Code2", SortOrder: 3},
		{ID: "showcase", Name: "作品展示", NameEn: "Showcase", Description: "展示你用 StarClaw 创造的作品", Icon: "Sparkles", SortOrder: 4},
		{ID: "feedback", Name: "Bug 反馈", NameEn: "Bug Reports", Description: "报告问题和建议改进", Icon: "Bug", SortOrder: 5},
		{ID: "general", Name: "综合讨论", NameEn: "General", Description: "自由话题", Icon: "MessageSquare", SortOrder: 6},
	}
	for _, cat := range cats {
		db.Where("id = ?", cat.ID).FirstOrCreate(&cat)
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
