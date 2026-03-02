package entity

import "time"

type User struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Password  string    `json:"password"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

func (u *User) IsAdmin() bool {
	return u.Role == "admin"
}

func (u *User) NewUser(name, password string) {
	u.Name = name
	u.Password = password
	u.Role = "user"
	u.CreatedAt = time.Now()
}

func (u *User) NewAdmin(name, password string) {
	u.Name = name
	u.Password = password
	u.Role = "admin"
	u.CreatedAt = time.Now()
}
