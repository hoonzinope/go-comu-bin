package inmemory

import (
	"context"
	"sync"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var _ port.EmailVerificationTokenRepository = (*EmailVerificationTokenRepository)(nil)

type EmailVerificationTokenRepository struct {
	mu          sync.RWMutex
	coordinator *txCoordinator
	tokens      map[string]*entity.EmailVerificationToken
}

type emailVerificationTokenRepositoryState struct {
	Tokens map[string]*entity.EmailVerificationToken
}

func NewEmailVerificationTokenRepository() *EmailVerificationTokenRepository {
	return &EmailVerificationTokenRepository{
		coordinator: newTxCoordinator(),
		tokens:      make(map[string]*entity.EmailVerificationToken),
	}
}

func (r *EmailVerificationTokenRepository) attachCoordinator(coordinator *txCoordinator) {
	r.coordinator = coordinator
}

func (r *EmailVerificationTokenRepository) Save(ctx context.Context, token *entity.EmailVerificationToken) error {
	_ = ctx
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.save(token)
}

func (r *EmailVerificationTokenRepository) save(token *entity.EmailVerificationToken) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tokens[token.TokenHash] = cloneEmailVerificationToken(token)
	return nil
}

func (r *EmailVerificationTokenRepository) SelectByTokenHash(ctx context.Context, tokenHash string) (*entity.EmailVerificationToken, error) {
	_ = ctx
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.selectByTokenHash(tokenHash)
}

func (r *EmailVerificationTokenRepository) selectByTokenHash(tokenHash string) (*entity.EmailVerificationToken, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	token, ok := r.tokens[tokenHash]
	if !ok {
		return nil, nil
	}
	return cloneEmailVerificationToken(token), nil
}

func (r *EmailVerificationTokenRepository) SelectLatestByUser(ctx context.Context, userID int64) (*entity.EmailVerificationToken, error) {
	_ = ctx
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.selectLatestByUser(userID), nil
}

func (r *EmailVerificationTokenRepository) selectLatestByUser(userID int64) *entity.EmailVerificationToken {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var latest *entity.EmailVerificationToken
	for _, token := range r.tokens {
		if token.UserID != userID {
			continue
		}
		if latest == nil || token.CreatedAt.After(latest.CreatedAt) || (token.CreatedAt.Equal(latest.CreatedAt) && token.TokenHash > latest.TokenHash) {
			latest = token
		}
	}
	return cloneEmailVerificationToken(latest)
}

func (r *EmailVerificationTokenRepository) InvalidateByUser(ctx context.Context, userID int64) error {
	_ = ctx
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.invalidateByUser(userID)
}

func (r *EmailVerificationTokenRepository) invalidateByUser(userID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for key, token := range r.tokens {
		if token.UserID != userID {
			continue
		}
		delete(r.tokens, key)
	}
	return nil
}

func (r *EmailVerificationTokenRepository) Update(ctx context.Context, token *entity.EmailVerificationToken) error {
	_ = ctx
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.update(token)
}

func (r *EmailVerificationTokenRepository) DeleteExpiredOrConsumedBefore(ctx context.Context, cutoff time.Time, limit int) (int, error) {
	_ = ctx
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.deleteExpiredOrConsumedBefore(cutoff, limit), nil
}

func (r *EmailVerificationTokenRepository) update(token *entity.EmailVerificationToken) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tokens[token.TokenHash] = cloneEmailVerificationToken(token)
	return nil
}

func (r *EmailVerificationTokenRepository) deleteExpiredOrConsumedBefore(cutoff time.Time, limit int) int {
	if limit <= 0 {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	deleted := 0
	for key, token := range r.tokens {
		if deleted >= limit {
			break
		}
		if token.ExpiresAt.After(cutoff) && (token.ConsumedAt == nil || token.ConsumedAt.After(cutoff)) {
			continue
		}
		delete(r.tokens, key)
		deleted++
	}
	return deleted
}

func (r *EmailVerificationTokenRepository) snapshot() emailVerificationTokenRepositoryState {
	r.mu.RLock()
	defer r.mu.RUnlock()
	state := emailVerificationTokenRepositoryState{
		Tokens: make(map[string]*entity.EmailVerificationToken, len(r.tokens)),
	}
	for key, token := range r.tokens {
		state.Tokens[key] = cloneEmailVerificationToken(token)
	}
	return state
}

func (r *EmailVerificationTokenRepository) restore(state emailVerificationTokenRepositoryState) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tokens = make(map[string]*entity.EmailVerificationToken, len(state.Tokens))
	for key, token := range state.Tokens {
		r.tokens[key] = cloneEmailVerificationToken(token)
	}
}

func cloneEmailVerificationToken(token *entity.EmailVerificationToken) *entity.EmailVerificationToken {
	if token == nil {
		return nil
	}
	out := *token
	if token.ConsumedAt != nil {
		consumedAt := *token.ConsumedAt
		out.ConsumedAt = &consumedAt
	}
	if token.DeliveredAt != nil {
		deliveredAt := *token.DeliveredAt
		out.DeliveredAt = &deliveredAt
	}
	return &out
}
