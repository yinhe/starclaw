package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ForumCategory represents a forum board/section
type ForumCategory struct {
	ID          string `json:"id" gorm:"type:varchar(36);primaryKey"`
	Name        string `json:"name" gorm:"type:varchar(100);not null"`
	NameEn      string `json:"name_en" gorm:"type:varchar(100)"`
	Description string `json:"description" gorm:"type:varchar(500)"`
	Icon        string `json:"icon" gorm:"type:varchar(50)"`
	SortOrder   int    `json:"sort_order" gorm:"default:0"`
	PostCount   int    `json:"post_count" gorm:"default:0"`
	CreatedAt   time.Time `json:"created_at"`
}

// Post represents a forum post/topic
type Post struct {
	ID         string `json:"id" gorm:"type:varchar(36);primaryKey"`
	AuthorID   string `json:"author_id" gorm:"type:varchar(36);index;not null"`
	AuthorName string `json:"author_name" gorm:"type:varchar(100)"`
	CategoryID string `json:"category_id" gorm:"type:varchar(36);index"`
	Title      string `json:"title" gorm:"type:varchar(300);not null"`
	Content    string `json:"content" gorm:"type:longtext;not null"`
	Tags       string `json:"tags" gorm:"type:varchar(500)"`       // JSON array
	Pinned     bool   `json:"pinned" gorm:"default:false"`
	Featured   bool   `json:"featured" gorm:"default:false"`       // 精华帖
	ViewCount  int    `json:"view_count" gorm:"default:0"`
	LikeCount  int    `json:"like_count" gorm:"default:0"`
	ReplyCount int    `json:"reply_count" gorm:"default:0"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

func (p *Post) BeforeCreate(tx *gorm.DB) error {
	if p.ID == "" {
		p.ID = uuid.New().String()
	}
	return nil
}

// Reply represents a reply to a forum post
type Reply struct {
	ID         string `json:"id" gorm:"type:varchar(36);primaryKey"`
	PostID     string `json:"post_id" gorm:"type:varchar(36);index;not null"`
	AuthorID   string `json:"author_id" gorm:"type:varchar(36);index;not null"`
	AuthorName string `json:"author_name" gorm:"type:varchar(100)"`
	Content    string `json:"content" gorm:"type:longtext;not null"`
	LikeCount  int    `json:"like_count" gorm:"default:0"`
	ParentID   string `json:"parent_id" gorm:"type:varchar(36);index"` // for nested replies

	CreatedAt time.Time      `json:"created_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

func (r *Reply) BeforeCreate(tx *gorm.DB) error {
	if r.ID == "" {
		r.ID = uuid.New().String()
	}
	return nil
}

// PostLike tracks who liked what
type PostLike struct {
	ID     uint   `json:"id" gorm:"primaryKey"`
	UserID string `json:"user_id" gorm:"type:varchar(36);index;not null"`
	PostID string `json:"post_id" gorm:"type:varchar(36);index"`
	ReplyID string `json:"reply_id" gorm:"type:varchar(36);index"`
	CreatedAt time.Time `json:"created_at"`
}
