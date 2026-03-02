package domain

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

// post getter, setter
func (p *Post) GetID() int64 {
	return p.ID
}

func (p *Post) GetTitle() string {
	return p.Title
}

func (p *Post) GetContent() string {
	return p.Content
}

func (p *Post) GetAuthorID() int64 {
	return p.AuthorID
}

func (p *Post) GetBoardID() int64 {
	return p.BoardID
}

func (p *Post) GetCreatedAt() time.Time {
	return p.CreatedAt
}

func (p *Post) GetUpdatedAt() time.Time {
	return p.UpdatedAt
}

func (p *Post) SetID(id int64) {
	p.ID = id
}

func (p *Post) SetTitle(title string) {
	p.Title = title
}

func (p *Post) SetContent(content string) {
	p.Content = content
}

func (p *Post) SetAuthorID(authorID int64) {
	p.AuthorID = authorID
}

func (p *Post) SetBoardID(boardID int64) {
	p.BoardID = boardID
}

func (p *Post) SetCreatedAt(createdAt time.Time) {
	p.CreatedAt = createdAt
}

func (p *Post) SetUpdatedAt(updatedAt time.Time) {
	p.UpdatedAt = updatedAt
}
