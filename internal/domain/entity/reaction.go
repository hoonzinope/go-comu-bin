package entity

import "time"

type ReactionTargetType string

const (
	ReactionTargetPost    ReactionTargetType = "post"
	ReactionTargetComment ReactionTargetType = "comment"
)

type ReactionType string

const (
	ReactionTypeLike    ReactionType = "like"
	ReactionTypeDislike ReactionType = "dislike"
)

type Reaction struct {
	ID         int64
	TargetType ReactionTargetType
	TargetID   int64
	Type       ReactionType
	UserID     int64
	CreatedAt  time.Time
}

func NewReaction(targetType ReactionTargetType, targetID int64, reactionType ReactionType, userID int64) *Reaction {
	return &Reaction{
		TargetType: targetType,
		TargetID:   targetID,
		Type:       reactionType,
		UserID:     userID,
		CreatedAt:  time.Now(),
	}
}

func (r *Reaction) Update(reactionType ReactionType) {
	r.Type = reactionType
}

func ParseReactionTargetType(raw string) (ReactionTargetType, bool) {
	switch ReactionTargetType(raw) {
	case ReactionTargetPost, ReactionTargetComment:
		return ReactionTargetType(raw), true
	default:
		return "", false
	}
}

func ParseReactionType(raw string) (ReactionType, bool) {
	switch ReactionType(raw) {
	case ReactionTypeLike, ReactionTypeDislike:
		return ReactionType(raw), true
	default:
		return "", false
	}
}
