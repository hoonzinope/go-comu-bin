package domain

import "time"

type Comment struct {
	ID        int64     `json:"id"`
	Content   string    `json:"content"`
	AuthorID  int64     `json:"author_id"`
	PostID    int64     `json:"post_id"`
	ParentID  *int64    `json:"parent_id"` // 대댓글을 위한 부모 댓글 ID, nullable
	CreatedAt time.Time `json:"created_at"`
}

// comment getter, setter
func (c *Comment) GetID() int64 {
	return c.ID
}

func (c *Comment) GetContent() string {
	return c.Content
}

func (c *Comment) GetAuthorID() int64 {
	return c.AuthorID
}

func (c *Comment) GetPostID() int64 {
	return c.PostID
}

func (c *Comment) GetParentID() *int64 {
	return c.ParentID
}

func (c *Comment) GetCreatedAt() time.Time {
	return c.CreatedAt
}

func (c *Comment) SetID(id int64) {
	c.ID = id
}

func (c *Comment) SetContent(content string) {
	c.Content = content
}

func (c *Comment) SetAuthorID(authorID int64) {
	c.AuthorID = authorID
}

func (c *Comment) SetPostID(postID int64) {
	c.PostID = postID
}

func (c *Comment) SetParentID(parentID *int64) {
	c.ParentID = parentID
}

func (c *Comment) SetCreatedAt(createdAt time.Time) {
	c.CreatedAt = createdAt
}
