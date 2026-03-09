package entity

import "time"

type PostTagStatus string

const (
	PostTagStatusActive  PostTagStatus = "active"
	PostTagStatusDeleted PostTagStatus = "deleted"
)

type PostTag struct {
	PostID    int64
	TagID     int64
	CreatedAt time.Time
	Status    PostTagStatus
}

func NewPostTag(postID, tagID int64) *PostTag {
	return &PostTag{
		PostID:    postID,
		TagID:     tagID,
		CreatedAt: time.Now(),
		Status:    PostTagStatusActive,
	}
}

func (p *PostTag) Activate() {
	p.Status = PostTagStatusActive
}

func (p *PostTag) SoftDelete() {
	p.Status = PostTagStatusDeleted
}
