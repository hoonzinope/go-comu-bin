package inmemory

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
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

func (r *UserRepository) Save(ctx context.Context, user *entity.User) (int64, error) {
	_ = ctx
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.save(user)
}

func (r *UserRepository) save(user *entity.User) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, existingUser := range r.userDB.Data {
		if existingUser.UUID == user.UUID || existingUser.Name == user.Name || emailsConflict(existingUser.Email, user.Email) {
			return 0, customerror.ErrUserAlreadyExists
		}
	}
	r.userDB.ID++
	saved := cloneUser(user)
	saved.ID = r.userDB.ID
	r.userDB.Data[saved.ID] = saved
	user.ID = saved.ID
	return saved.ID, nil
}

func (r *UserRepository) SelectUserByUsername(ctx context.Context, username string) (*entity.User, error) {
	_ = ctx
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

func (r *UserRepository) SelectUserByEmail(ctx context.Context, email string) (*entity.User, error) {
	_ = ctx
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.selectUserByEmail(email)
}

func (r *UserRepository) selectUserByEmail(email string) (*entity.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	email = normalizeEmail(email)
	for _, user := range r.userDB.Data {
		if normalizeEmail(user.Email) == email && email != "" && !user.IsDeleted() {
			return cloneUser(user), nil
		}
	}
	return nil, nil
}

func (r *UserRepository) SelectUserByUUID(ctx context.Context, userUUID string) (*entity.User, error) {
	_ = ctx
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

func (r *UserRepository) SelectUserByID(ctx context.Context, id int64) (*entity.User, error) {
	_ = ctx
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

func (r *UserRepository) SelectUserByIDIncludingDeleted(ctx context.Context, id int64) (*entity.User, error) {
	_ = ctx
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

func (r *UserRepository) SelectUsersByIDsIncludingDeleted(ctx context.Context, ids []int64) (map[int64]*entity.User, error) {
	_ = ctx
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.selectUsersByIDsIncludingDeleted(ids)
}

func (r *UserRepository) selectUsersByIDsIncludingDeleted(ids []int64) (map[int64]*entity.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make(map[int64]*entity.User, len(ids))
	for _, id := range ids {
		if _, exists := out[id]; exists {
			continue
		}
		if user, exists := r.userDB.Data[id]; exists {
			out[id] = cloneUser(user)
		}
	}
	return out, nil
}

func (r *UserRepository) SelectGuestCleanupCandidates(ctx context.Context, now time.Time, pendingGrace, activeUnusedGrace time.Duration, limit int) ([]*entity.User, error) {
	_ = ctx
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.selectGuestCleanupCandidates(now, pendingGrace, activeUnusedGrace, limit)
}

func (r *UserRepository) selectGuestCleanupCandidates(now time.Time, pendingGrace, activeUnusedGrace time.Duration, limit int) ([]*entity.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if limit <= 0 {
		return []*entity.User{}, nil
	}
	items := make([]*entity.User, 0, limit)
	for _, user := range r.userDB.Data {
		if user == nil || !user.IsGuest() || user.IsDeleted() {
			continue
		}
		if !guestEligibleForCleanup(user, now, pendingGrace, activeUnusedGrace) {
			continue
		}
		items = append(items, cloneUser(user))
	}
	sort.Slice(items, func(i, j int) bool {
		return guestCleanupEligibleAt(items[i]).Before(guestCleanupEligibleAt(items[j]))
	})
	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (r *UserRepository) Update(ctx context.Context, user *entity.User) error {
	_ = ctx
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
			if existingUser.UUID == user.UUID || existingUser.Name == user.Name || emailsConflict(existingUser.Email, user.Email) {
				return customerror.ErrUserAlreadyExists
			}
		}
		r.userDB.Data[user.ID] = cloneUser(user)
	}
	return nil
}

func (r *UserRepository) Delete(ctx context.Context, id int64) error {
	_ = ctx
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
	if user.GuestIssuedAt != nil {
		guestIssuedAt := *user.GuestIssuedAt
		out.GuestIssuedAt = &guestIssuedAt
	}
	if user.GuestActivatedAt != nil {
		guestActivatedAt := *user.GuestActivatedAt
		out.GuestActivatedAt = &guestActivatedAt
	}
	if user.GuestExpiredAt != nil {
		guestExpiredAt := *user.GuestExpiredAt
		out.GuestExpiredAt = &guestExpiredAt
	}
	if user.EmailVerifiedAt != nil {
		emailVerifiedAt := *user.EmailVerifiedAt
		out.EmailVerifiedAt = &emailVerifiedAt
	}
	if user.DeletedAt != nil {
		deletedAt := *user.DeletedAt
		out.DeletedAt = &deletedAt
	}
	return &out
}

func emailsConflict(left, right string) bool {
	left = normalizeEmail(left)
	right = normalizeEmail(right)
	return left != "" && right != "" && left == right
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func guestEligibleForCleanup(user *entity.User, now time.Time, pendingGrace, activeUnusedGrace time.Duration) bool {
	switch user.GuestStatus {
	case entity.GuestStatusPending:
		if user.GuestIssuedAt == nil {
			return false
		}
		return !user.GuestIssuedAt.Add(pendingGrace).After(now)
	case entity.GuestStatusExpired:
		if user.GuestExpiredAt == nil {
			return false
		}
		return !user.GuestExpiredAt.Add(pendingGrace).After(now)
	case entity.GuestStatusActive:
		basis := user.GuestActivatedAt
		if basis == nil {
			basis = user.GuestIssuedAt
		}
		if basis == nil {
			return false
		}
		return !basis.Add(activeUnusedGrace).After(now)
	default:
		return false
	}
}

func guestCleanupEligibleAt(user *entity.User) time.Time {
	switch user.GuestStatus {
	case entity.GuestStatusPending:
		if user.GuestIssuedAt != nil {
			return *user.GuestIssuedAt
		}
	case entity.GuestStatusExpired:
		if user.GuestExpiredAt != nil {
			return *user.GuestExpiredAt
		}
	case entity.GuestStatusActive:
		if user.GuestActivatedAt != nil {
			return *user.GuestActivatedAt
		}
		if user.GuestIssuedAt != nil {
			return *user.GuestIssuedAt
		}
	}
	return user.CreatedAt
}
