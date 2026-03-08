package entity

import "time"

type CommentStatus string

const (
	CommentStatusActive  CommentStatus = "active"
	CommentStatusDeleted CommentStatus = "deleted"
)

type Comment struct {
	ID        int64
	Content   string
	AuthorID  int64
	PostID    int64
	ParentID  *int64 // 대댓글을 위한 부모 댓글 ID, nullable
	Status    CommentStatus
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

func NewComment(content string, authorID, postID int64, parentID *int64) *Comment {
	now := time.Now()
	return &Comment{
		Content:   content,
		AuthorID:  authorID,
		PostID:    postID,
		ParentID:  parentID,
		Status:    CommentStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func (c *Comment) Update(content string) {
	c.Content = content
	c.UpdatedAt = time.Now()
}

func (c *Comment) SoftDelete() {
	now := time.Now()
	c.Status = CommentStatusDeleted
	c.UpdatedAt = now
	c.DeletedAt = &now
}
