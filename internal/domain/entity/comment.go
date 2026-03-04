package entity

import "time"

type Comment struct {
	ID        int64
	Content   string
	AuthorID  int64
	PostID    int64
	ParentID  *int64 // 대댓글을 위한 부모 댓글 ID, nullable
	CreatedAt time.Time
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
