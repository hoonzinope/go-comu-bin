package entity

import "time"

type EmailVerificationToken struct {
	UserID     int64
	TokenHash  string
	CreatedAt  time.Time
	ExpiresAt  time.Time
	ConsumedAt *time.Time
}

func NewEmailVerificationToken(userID int64, tokenHash string, expiresAt time.Time) *EmailVerificationToken {
	return &EmailVerificationToken{
		UserID:    userID,
		TokenHash: tokenHash,
		CreatedAt: time.Now(),
		ExpiresAt: expiresAt,
	}
}

func (t *EmailVerificationToken) IsExpired(now time.Time) bool {
	return !t.ExpiresAt.After(now)
}

func (t *EmailVerificationToken) IsConsumed() bool {
	return t.ConsumedAt != nil
}

func (t *EmailVerificationToken) IsUsable(now time.Time) bool {
	return t != nil && !t.IsConsumed() && !t.IsExpired(now)
}

func (t *EmailVerificationToken) Consume(now time.Time) {
	t.ConsumedAt = &now
}
