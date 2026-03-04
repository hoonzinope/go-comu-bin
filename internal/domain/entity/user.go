package entity

import "time"

type User struct {
	ID        int64
	Name      string
	Password  string
	Role      string
	CreatedAt time.Time
}

func (u *User) IsAdmin() bool {
	return u.Role == "admin"
}

func NewUser(name, password string) *User {
	return &User{
		Name:      name,
		Password:  password,
		Role:      "user",
		CreatedAt: time.Now(),
	}
}

func NewAdmin(name, password string) *User {
	return &User{
		Name:      name,
		Password:  password,
		Role:      "admin",
		CreatedAt: time.Now(),
	}
}
