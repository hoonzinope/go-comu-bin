package port

import "github.com/hoonzinope/go-comu-bin/internal/domain/entity"

type UserRepository interface {
	Save(*entity.User) (int64, error)
	SelectUserByUsername(username string) (*entity.User, error)
	SelectUserByID(id int64) (*entity.User, error)
	SelectUserByIDIncludingDeleted(id int64) (*entity.User, error)
	Update(*entity.User) error
	Delete(id int64) error
}
