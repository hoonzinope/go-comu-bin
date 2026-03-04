package entity

import "time"

type Board struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
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
