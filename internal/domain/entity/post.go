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

func (p *Post) NewPost(title, content string, authorID, boardID int64) {
	p.Title = title
	p.Content = content
	p.AuthorID = authorID
	p.BoardID = boardID
	p.CreatedAt = time.Now()
	p.UpdatedAt = time.Now()
}

func (p *Post) UpdatePost(title, content string) {
	p.Title = title
	p.Content = content
	p.UpdatedAt = time.Now()
}
