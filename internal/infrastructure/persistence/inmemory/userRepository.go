package inmemory

import (
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

type UserRepository struct {
	userDB struct {
		ID   int64
		Data map[int64]*entity.User
	}
}

func NewUserRepository() *UserRepository {
	return &UserRepository{
		userDB: struct {
			ID   int64
			Data map[int64]*entity.User
		}{
			ID:   0,
			Data: make(map[int64]*entity.User),
		},
	}
}

func (r *UserRepository) Save(user *entity.User) (int64, error) {
	r.userDB.ID++
	user.ID = r.userDB.ID
	r.userDB.Data[user.ID] = user
	return user.ID, nil
}

func (r *UserRepository) SelectUserByUsername(username string) (*entity.User, error) {
	for _, user := range r.userDB.Data {
		if user.Name == username {
			return user, nil
		}
	}
	return nil, nil
}

func (r *UserRepository) SelectUserByID(id int64) (*entity.User, error) {
	if user, exists := r.userDB.Data[id]; exists {
		return user, nil
	}
	return nil, nil
}

func (r *UserRepository) Delete(id int64) error {
	delete(r.userDB.Data, id)
	return nil
}
