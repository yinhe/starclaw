package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw-queen/forum/internal/model"
	"gorm.io/gorm"
)

type ForumHandler struct {
	db *gorm.DB
}

func NewForumHandler(db *gorm.DB) *ForumHandler {
	return &ForumHandler{db: db}
}

// ---------- Categories ----------

func (h *ForumHandler) ListCategories(c *gin.Context) {
	var cats []model.ForumCategory
	h.db.Order("sort_order ASC").Find(&cats)
	c.JSON(http.StatusOK, gin.H{"categories": cats})
}

// ---------- Posts ----------

func (h *ForumHandler) CreatePost(c *gin.Context) {
	var req struct {
		AuthorID   string `json:"author_id" binding:"required"`
		AuthorName string `json:"author_name"`
		CategoryID string `json:"category_id"`
		Title      string `json:"title" binding:"required"`
		Content    string `json:"content" binding:"required"`
		Tags       string `json:"tags"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	post := model.Post{
		AuthorID:   req.AuthorID,
		AuthorName: req.AuthorName,
		CategoryID: req.CategoryID,
		Title:      req.Title,
		Content:    req.Content,
		Tags:       req.Tags,
	}
	if err := h.db.Create(&post).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create post"})
		return
	}

	// Increment category post count
	if req.CategoryID != "" {
		h.db.Model(&model.ForumCategory{}).Where("id = ?", req.CategoryID).
			UpdateColumn("post_count", gorm.Expr("post_count + 1"))
	}

	c.JSON(http.StatusCreated, gin.H{"post": post})
}

func (h *ForumHandler) ListPosts(c *gin.Context) {
	categoryID := c.Query("category_id")
	sort := c.DefaultQuery("sort", "latest") // latest, popular, featured

	q := h.db.Model(&model.Post{})
	if categoryID != "" {
		q = q.Where("category_id = ?", categoryID)
	}

	switch sort {
	case "popular":
		q = q.Order("like_count DESC, created_at DESC")
	case "featured":
		q = q.Where("featured = ?", true).Order("created_at DESC")
	default:
		q = q.Order("pinned DESC, created_at DESC")
	}

	var posts []model.Post
	q.Limit(50).Find(&posts)

	c.JSON(http.StatusOK, gin.H{"posts": posts, "total": len(posts)})
}

func (h *ForumHandler) GetPost(c *gin.Context) {
	id := c.Param("id")
	var post model.Post
	if err := h.db.First(&post, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "post not found"})
		return
	}

	// Increment view count
	h.db.Model(&post).UpdateColumn("view_count", gorm.Expr("view_count + 1"))
	post.ViewCount++

	// Get replies
	var replies []model.Reply
	h.db.Where("post_id = ?", id).Order("created_at ASC").Find(&replies)

	c.JSON(http.StatusOK, gin.H{"post": post, "replies": replies})
}

func (h *ForumHandler) DeletePost(c *gin.Context) {
	id := c.Param("id")
	if err := h.db.Where("id = ?", id).Delete(&model.Post{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete post"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "post deleted"})
}

// ---------- Replies ----------

func (h *ForumHandler) CreateReply(c *gin.Context) {
	postID := c.Param("id")
	var req struct {
		AuthorID   string `json:"author_id" binding:"required"`
		AuthorName string `json:"author_name"`
		Content    string `json:"content" binding:"required"`
		ParentID   string `json:"parent_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	reply := model.Reply{
		PostID:     postID,
		AuthorID:   req.AuthorID,
		AuthorName: req.AuthorName,
		Content:    req.Content,
		ParentID:   req.ParentID,
	}
	if err := h.db.Create(&reply).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create reply"})
		return
	}

	h.db.Model(&model.Post{}).Where("id = ?", postID).
		UpdateColumn("reply_count", gorm.Expr("reply_count + 1"))

	c.JSON(http.StatusCreated, gin.H{"reply": reply})
}

// ---------- Likes ----------

func (h *ForumHandler) LikePost(c *gin.Context) {
	postID := c.Param("id")
	var req struct {
		UserID string `json:"user_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check duplicate
	var existing model.PostLike
	if h.db.Where("user_id = ? AND post_id = ?", req.UserID, postID).First(&existing).Error == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "already liked"})
		return
	}

	h.db.Create(&model.PostLike{UserID: req.UserID, PostID: postID})
	h.db.Model(&model.Post{}).Where("id = ?", postID).
		UpdateColumn("like_count", gorm.Expr("like_count + 1"))

	c.JSON(http.StatusOK, gin.H{"message": "liked"})
}

// ---------- Search ----------

func (h *ForumHandler) Search(c *gin.Context) {
	q := c.Query("q")
	if q == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query required"})
		return
	}

	var posts []model.Post
	h.db.Where("title LIKE ? OR content LIKE ?", "%"+q+"%", "%"+q+"%").
		Order("created_at DESC").Limit(30).Find(&posts)

	c.JSON(http.StatusOK, gin.H{"posts": posts, "total": len(posts)})
}

// ---------- Stats ----------

func (h *ForumHandler) Stats(c *gin.Context) {
	var totalPosts, totalReplies int64
	h.db.Model(&model.Post{}).Count(&totalPosts)
	h.db.Model(&model.Reply{}).Count(&totalReplies)
	c.JSON(http.StatusOK, gin.H{"total_posts": totalPosts, "total_replies": totalReplies})
}
