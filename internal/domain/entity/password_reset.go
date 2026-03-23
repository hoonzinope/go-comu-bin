package entity

import "time"

type PasswordResetToken struct {
	UserID     int64
	TokenHash  string
	CreatedAt  time.Time
	ExpiresAt  time.Time
	ConsumedAt *time.Time
}

func NewPasswordResetToken(userID int64, tokenHash string, expiresAt time.Time) *PasswordResetToken {
	return &PasswordResetToken{
		UserID:    userID,
		TokenHash: tokenHash,
		CreatedAt: time.Now(),
		ExpiresAt: expiresAt,
	}
}

func (t *PasswordResetToken) IsExpired(now time.Time) bool {
	return !t.ExpiresAt.After(now)
}

func (t *PasswordResetToken) IsConsumed() bool {
	return t.ConsumedAt != nil
}

func (t *PasswordResetToken) IsUsable(now time.Time) bool {
	return t != nil && !t.IsConsumed() && !t.IsExpired(now)
}

func (t *PasswordResetToken) Consume(now time.Time) {
	t.ConsumedAt = &now
}
