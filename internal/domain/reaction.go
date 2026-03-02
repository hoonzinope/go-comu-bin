package domain

import "time"

type Reaction struct {
	ID         int64     `json:"id"`
	TargetType string    `json:"target_type"` // "post" or "comment"
	TargetID   int64     `json:"target_id"`
	Type       string    `json:"type"` // "like" or "dislike"
	UserID     int64     `json:"user_id"`
	CreatedAt  time.Time `json:"created_at"`
}

// reaction getter, setter
func (r *Reaction) GetID() int64 {
	return r.ID
}

func (r *Reaction) GetTargetType() string {
	return r.TargetType
}

func (r *Reaction) GetTargetID() int64 {
	return r.TargetID
}

func (r *Reaction) GetType() string {
	return r.Type
}

func (r *Reaction) GetUserID() int64 {
	return r.UserID
}

func (r *Reaction) GetCreatedAt() time.Time {
	return r.CreatedAt
}

func (r *Reaction) SetID(id int64) {
	r.ID = id
}

func (r *Reaction) SetTargetType(targetType string) {
	r.TargetType = targetType
}

func (r *Reaction) SetTargetID(targetID int64) {
	r.TargetID = targetID
}

func (r *Reaction) SetType(reactionType string) {
	r.Type = reactionType
}

func (r *Reaction) SetUserID(userID int64) {
	r.UserID = userID
}

func (r *Reaction) SetCreatedAt(createdAt time.Time) {
	r.CreatedAt = createdAt
}
