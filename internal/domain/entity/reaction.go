package entity

import "time"

type Reaction struct {
	ID         int64     `json:"id"`
	TargetType string    `json:"target_type"` // "post" or "comment"
	TargetID   int64     `json:"target_id"`
	Type       string    `json:"type"` // "like" or "dislike"
	UserID     int64     `json:"user_id"`
	CreatedAt  time.Time `json:"created_at"`
}

func NewReaction(targetType string, targetID int64, reactionType string, userID int64) *Reaction {
	return &Reaction{
		TargetType: targetType,
		TargetID:   targetID,
		Type:       reactionType,
		UserID:     userID,
		CreatedAt:  time.Now(),
	}
}

func (r *Reaction) Update(reactionType string) {
	r.Type = reactionType
}
