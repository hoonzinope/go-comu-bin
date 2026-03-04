package entity

import "time"

type Post struct {
	ID        int64
	Title     string
	Content   string
	AuthorID  int64
	BoardID   int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewPost(title, content string, authorID, boardID int64) *Post {
	now := time.Now()
	return &Post{
		Title:     title,
		Content:   content,
		AuthorID:  authorID,
		BoardID:   boardID,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func (p *Post) Update(title, content string) {
	p.Title = title
	p.Content = content
	p.UpdatedAt = time.Now()
}
