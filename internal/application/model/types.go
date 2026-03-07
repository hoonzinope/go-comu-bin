package model

import (
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

type Board struct {
	ID          int64
	Name        string
	Description string
	CreatedAt   time.Time
}

type Post struct {
	ID        int64
	Title     string
	Content   string
	AuthorID  int64
	BoardID   int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Comment struct {
	ID        int64
	Content   string
	AuthorID  int64
	PostID    int64
	ParentID  *int64
	CreatedAt time.Time
}

type Reaction struct {
	ID         int64
	TargetType entity.ReactionTargetType
	TargetID   int64
	Type       entity.ReactionType
	UserID     int64
	CreatedAt  time.Time
}
