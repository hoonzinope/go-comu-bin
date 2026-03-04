package entity

import "time"

type Comment struct {
	ID        int64     `json:"id"`
	Content   string    `json:"content"`
	AuthorID  int64     `json:"author_id"`
	PostID    int64     `json:"post_id"`
	ParentID  *int64    `json:"parent_id"` // 대댓글을 위한 부모 댓글 ID, nullable
	CreatedAt time.Time `json:"created_at"`
}

func NewComment(content string, authorID, postID int64, parentID *int64) *Comment {
	return &Comment{
		Content:   content,
		AuthorID:  authorID,
		PostID:    postID,
		ParentID:  parentID,
		CreatedAt: time.Now(),
	}
}

func (c *Comment) Update(content string) {
	c.Content = content
}
