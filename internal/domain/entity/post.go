package entity

import "time"

type PostStatus string

const (
	PostStatusDraft     PostStatus = "draft"
	PostStatusPublished PostStatus = "published"
	PostStatusDeleted   PostStatus = "deleted"
)

type Post struct {
	ID        int64
	Title     string
	Content   string
	AuthorID  int64
	BoardID   int64
	Status    PostStatus
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

func NewPost(title, content string, authorID, boardID int64) *Post {
	now := time.Now()
	return &Post{
		Title:     title,
		Content:   content,
		AuthorID:  authorID,
		BoardID:   boardID,
		Status:    PostStatusPublished,
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
