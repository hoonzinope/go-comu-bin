package domain

import "time"

type Board struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

// board getter, setter
func (b *Board) GetID() int64 {
	return b.ID
}

func (b *Board) GetName() string {
	return b.Name
}

func (b *Board) GetDescription() string {
	return b.Description
}

func (b *Board) GetCreatedAt() time.Time {
	return b.CreatedAt
}

func (b *Board) SetID(id int64) {
	b.ID = id
}

func (b *Board) SetName(name string) {
	b.Name = name
}

func (b *Board) SetDescription(description string) {
	b.Description = description
}

func (b *Board) SetCreatedAt(createdAt time.Time) {
	b.CreatedAt = createdAt
}
