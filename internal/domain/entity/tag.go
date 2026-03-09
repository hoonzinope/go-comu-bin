package entity

import "time"

type Tag struct {
	ID        int64
	Name      string
	CreatedAt time.Time
}

func NewTag(name string) *Tag {
	return &Tag{
		Name:      name,
		CreatedAt: time.Now(),
	}
}
