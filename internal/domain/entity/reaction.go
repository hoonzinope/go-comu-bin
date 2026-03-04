package entity

import "time"

type Reaction struct {
	ID         int64
	TargetType string // "post" or "comment"
	TargetID   int64
	Type       string // "like" or "dislike"
	UserID     int64
	CreatedAt  time.Time
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
