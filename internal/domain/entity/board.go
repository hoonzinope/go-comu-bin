package entity

import (
	"time"

	"github.com/google/uuid"
)

type Board struct {
	ID          int64
	UUID        string
	Name        string
	Description string
	Hidden      bool
	CreatedAt   time.Time
}

func NewBoard(name, description string) *Board {
	return &Board{
		UUID:        uuid.NewString(),
		Name:        name,
		Description: description,
		CreatedAt:   time.Now(),
	}
}

func (b *Board) Update(name, description string) {
	b.Name = name
	b.Description = description
}

func (b *Board) SetHidden(hidden bool) {
	b.Hidden = hidden
}
