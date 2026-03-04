package entity

import "time"

type Board struct {
	ID          int64
	Name        string
	Description string
	CreatedAt   time.Time
}

func NewBoard(name, description string) *Board {
	return &Board{
		Name:        name,
		Description: description,
		CreatedAt:   time.Now(),
	}
}

func (b *Board) Update(name, description string) {
	b.Name = name
	b.Description = description
}
