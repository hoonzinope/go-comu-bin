package inmemory

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var _ port.PasswordResetTokenRepository = (*PasswordResetTokenRepository)(nil)

type PasswordResetTokenRepository struct {
	mu          sync.RWMutex
	coordinator *txCoordinator
	tokens      map[string]*entity.PasswordResetToken
}

type passwordResetTokenRepositoryState struct {
	Tokens map[string]*entity.PasswordResetToken
}

func NewPasswordResetTokenRepository() *PasswordResetTokenRepository {
	return &PasswordResetTokenRepository{
		coordinator: newTxCoordinator(),
		tokens:      make(map[string]*entity.PasswordResetToken),
	}
}

func (r *PasswordResetTokenRepository) attachCoordinator(coordinator *txCoordinator) {
	r.coordinator = coordinator
}

func (r *PasswordResetTokenRepository) Save(ctx context.Context, token *entity.PasswordResetToken) error {
	_ = ctx
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.save(token)
}

func (r *PasswordResetTokenRepository) save(token *entity.PasswordResetToken) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tokens[token.TokenHash] = clonePasswordResetToken(token)
	return nil
}

func (r *PasswordResetTokenRepository) SelectByTokenHash(ctx context.Context, tokenHash string) (*entity.PasswordResetToken, error) {
	_ = ctx
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.selectByTokenHash(tokenHash)
}

func (r *PasswordResetTokenRepository) selectByTokenHash(tokenHash string) (*entity.PasswordResetToken, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	token, ok := r.tokens[tokenHash]
	if !ok {
		return nil, nil
	}
	return clonePasswordResetToken(token), nil
}

func (r *PasswordResetTokenRepository) InvalidateByUser(ctx context.Context, userID int64) error {
	_ = ctx
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.invalidateByUser(userID)
}

func (r *PasswordResetTokenRepository) invalidateByUser(userID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	for key, token := range r.tokens {
		if token.UserID != userID || token.IsConsumed() {
			continue
		}
		next := clonePasswordResetToken(token)
		next.Consume(now)
		r.tokens[key] = next
	}
	return nil
}

func (r *PasswordResetTokenRepository) Update(ctx context.Context, token *entity.PasswordResetToken) error {
	_ = ctx
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.update(token)
}

func (r *PasswordResetTokenRepository) update(token *entity.PasswordResetToken) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tokens[token.TokenHash] = clonePasswordResetToken(token)
	return nil
}

func (r *PasswordResetTokenRepository) DeleteExpiredOrConsumedBefore(ctx context.Context, cutoff time.Time, limit int) (int, error) {
	_ = ctx
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.deleteExpiredOrConsumedBefore(cutoff, limit), nil
}

func (r *PasswordResetTokenRepository) deleteExpiredOrConsumedBefore(cutoff time.Time, limit int) int {
	if limit <= 0 {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	type candidate struct {
		tokenHash  string
		eligibleAt time.Time
	}
	candidates := make([]candidate, 0)
	for key, token := range r.tokens {
		switch {
		case token.ConsumedAt != nil && !token.ConsumedAt.After(cutoff):
			candidates = append(candidates, candidate{tokenHash: key, eligibleAt: *token.ConsumedAt})
		case !token.ExpiresAt.After(cutoff):
			candidates = append(candidates, candidate{tokenHash: key, eligibleAt: token.ExpiresAt})
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].eligibleAt.Equal(candidates[j].eligibleAt) {
			return candidates[i].tokenHash < candidates[j].tokenHash
		}
		return candidates[i].eligibleAt.Before(candidates[j].eligibleAt)
	})
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	for _, item := range candidates {
		delete(r.tokens, item.tokenHash)
	}
	return len(candidates)
}

func (r *PasswordResetTokenRepository) snapshot() passwordResetTokenRepositoryState {
	r.mu.RLock()
	defer r.mu.RUnlock()
	state := passwordResetTokenRepositoryState{
		Tokens: make(map[string]*entity.PasswordResetToken, len(r.tokens)),
	}
	for key, token := range r.tokens {
		state.Tokens[key] = clonePasswordResetToken(token)
	}
	return state
}

func (r *PasswordResetTokenRepository) restore(state passwordResetTokenRepositoryState) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tokens = make(map[string]*entity.PasswordResetToken, len(state.Tokens))
	for key, token := range state.Tokens {
		r.tokens[key] = clonePasswordResetToken(token)
	}
}

func clonePasswordResetToken(token *entity.PasswordResetToken) *entity.PasswordResetToken {
	if token == nil {
		return nil
	}
	out := *token
	if token.ConsumedAt != nil {
		consumedAt := *token.ConsumedAt
		out.ConsumedAt = &consumedAt
	}
	return &out
}
