package entity

import "time"

type Post struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	AuthorID  int64     `json:"author_id"`
	BoardID   int64     `json:"board_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
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
