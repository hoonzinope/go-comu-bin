package inmemory

import (
	"sync"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var _ port.UserRepository = (*UserRepository)(nil)

type UserRepository struct {
	mu          sync.RWMutex
	coordinator *txCoordinator
	userDB      struct {
		ID   int64
		Data map[int64]*entity.User
	}
}

type userRepositoryState struct {
	ID   int64
	Data map[int64]*entity.User
}

func NewUserRepository() *UserRepository {
	return &UserRepository{
		coordinator: newTxCoordinator(),
		userDB: struct {
			ID   int64
			Data map[int64]*entity.User
		}{
			ID:   0,
			Data: make(map[int64]*entity.User),
		},
	}
}

func (r *UserRepository) attachCoordinator(coordinator *txCoordinator) {
	r.coordinator = coordinator
}

func (r *UserRepository) Save(user *entity.User) (int64, error) {
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.save(user)
}

func (r *UserRepository) save(user *entity.User) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, existingUser := range r.userDB.Data {
		if existingUser.UUID == user.UUID || existingUser.Name == user.Name {
			return 0, customError.ErrUserAlreadyExists
		}
	}
	r.userDB.ID++
	saved := cloneUser(user)
	saved.ID = r.userDB.ID
	r.userDB.Data[saved.ID] = saved
	user.ID = saved.ID
	return saved.ID, nil
}

func (r *UserRepository) SelectUserByUsername(username string) (*entity.User, error) {
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.selectUserByUsername(username)
}

func (r *UserRepository) selectUserByUsername(username string) (*entity.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, user := range r.userDB.Data {
		if user.Name == username && !user.IsDeleted() {
			return cloneUser(user), nil
		}
	}
	return nil, nil
}

func (r *UserRepository) SelectUserByUUID(userUUID string) (*entity.User, error) {
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.selectUserByUUID(userUUID)
}

func (r *UserRepository) selectUserByUUID(userUUID string) (*entity.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, user := range r.userDB.Data {
		if user.UUID == userUUID && !user.IsDeleted() {
			return cloneUser(user), nil
		}
	}
	return nil, nil
}

func (r *UserRepository) SelectUserByID(id int64) (*entity.User, error) {
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.selectUserByID(id)
}

func (r *UserRepository) selectUserByID(id int64) (*entity.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if user, exists := r.userDB.Data[id]; exists && !user.IsDeleted() {
		return cloneUser(user), nil
	}
	return nil, nil
}

func (r *UserRepository) SelectUserByIDIncludingDeleted(id int64) (*entity.User, error) {
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.selectUserByIDIncludingDeleted(id)
}

func (r *UserRepository) selectUserByIDIncludingDeleted(id int64) (*entity.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if user, exists := r.userDB.Data[id]; exists {
		return cloneUser(user), nil
	}
	return nil, nil
}

func (r *UserRepository) Update(user *entity.User) error {
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.update(user)
}

func (r *UserRepository) update(user *entity.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.userDB.Data[user.ID]; exists {
		for id, existingUser := range r.userDB.Data {
			if id == user.ID {
				continue
			}
			if existingUser.UUID == user.UUID || existingUser.Name == user.Name {
				return customError.ErrUserAlreadyExists
			}
		}
		r.userDB.Data[user.ID] = cloneUser(user)
	}
	return nil
}

func (r *UserRepository) Delete(id int64) error {
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.delete(id)
}

func (r *UserRepository) delete(id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.userDB.Data, id)
	return nil
}

func (r *UserRepository) snapshot() userRepositoryState {
	r.mu.RLock()
	defer r.mu.RUnlock()

	state := userRepositoryState{
		ID:   r.userDB.ID,
		Data: make(map[int64]*entity.User, len(r.userDB.Data)),
	}
	for id, user := range r.userDB.Data {
		state.Data[id] = cloneUser(user)
	}
	return state
}

func (r *UserRepository) restore(state userRepositoryState) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.userDB.ID = state.ID
	r.userDB.Data = make(map[int64]*entity.User, len(state.Data))
	for id, user := range state.Data {
		r.userDB.Data[id] = cloneUser(user)
	}
}

func cloneUser(user *entity.User) *entity.User {
	if user == nil {
		return nil
	}
	out := *user
	if user.SuspendedUntil != nil {
		suspendedUntil := *user.SuspendedUntil
		out.SuspendedUntil = &suspendedUntil
	}
	if user.DeletedAt != nil {
		deletedAt := *user.DeletedAt
		out.DeletedAt = &deletedAt
	}
	return &out
}
