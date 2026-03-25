package entity

import (
	"time"

	"github.com/google/uuid"
)

type PostStatus string

const (
	PostStatusDraft     PostStatus = "draft"
	PostStatusPublished PostStatus = "published"
	PostStatusDeleted   PostStatus = "deleted"
)

type Post struct {
	ID        int64
	UUID      string
	Title     string
	Content   string
	AuthorID  int64
	BoardID   int64
	Status    PostStatus
	CreatedAt time.Time
	PublishedAt *time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

func NewPost(title, content string, authorID, boardID int64) *Post {
	now := time.Now()
	return &Post{
		UUID:      uuid.NewString(),
		Title:     title,
		Content:   content,
		AuthorID:  authorID,
		BoardID:   boardID,
		Status:    PostStatusPublished,
		CreatedAt: now,
		PublishedAt: &now,
		UpdatedAt: now,
	}
}

func NewDraftPost(title, content string, authorID, boardID int64) *Post {
	now := time.Now()
	return &Post{
		UUID:      uuid.NewString(),
		Title:     title,
		Content:   content,
		AuthorID:  authorID,
		BoardID:   boardID,
		Status:    PostStatusDraft,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func (p *Post) Update(title, content string) {
	p.Title = title
	p.Content = content
	p.UpdatedAt = time.Now()
}

func (p *Post) SoftDelete() {
	now := time.Now()
	p.Status = PostStatusDeleted
	p.UpdatedAt = now
	p.DeletedAt = &now
}

func (p *Post) Publish() {
	p.Status = PostStatusPublished
	p.DeletedAt = nil
	now := time.Now()
	p.PublishedAt = &now
	p.UpdatedAt = now
}
