package entity

import "time"

type Board struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

func (b *Board) NewBoard(name, description string) {
	b.Name = name
	b.Description = description
	b.CreatedAt = time.Now()
}

func (b *Board) UpdateBoard(name, description string) {
	b.Name = name
	b.Description = description
}
