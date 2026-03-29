package model

import (
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

type Board struct {
	UUID        string
	Name        string
	Description string
	Hidden      bool
	CreatedAt   time.Time
}

type Post struct {
	UUID       string
	Title      string
	Content    string
	AuthorID   int64
	AuthorUUID string
	BoardUUID  string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type Tag struct {
	ID        int64
	Name      string
	CreatedAt time.Time
}

type Comment struct {
	UUID       string
	Content    string
	AuthorID   int64
	AuthorUUID string
	PostUUID   string
	ParentID   *int64
	ParentUUID *string
	CreatedAt  time.Time
}

type Reaction struct {
	ID         int64
	TargetType entity.ReactionTargetType
	TargetUUID string
	Type       entity.ReactionType
	UserID     int64
	UserUUID   string
	CreatedAt  time.Time
}
