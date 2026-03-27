package event

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONEventSerializer_Deserialize_FillsOccurredAtWhenMissing(t *testing.T) {
	s := NewJSONEventSerializer()
	occurredAt := time.Date(2026, 3, 13, 19, 30, 0, 0, time.UTC)
	tests := []struct {
		name      string
		eventName string
		payload   any
		assertAt  func(t *testing.T, event any)
	}{
		{
			name:      "board changed",
			eventName: EventNameBoardChanged,
			payload:   BoardChanged{Operation: "create", BoardID: 1},
			assertAt: func(t *testing.T, event any) {
				got, ok := event.(BoardChanged)
				require.True(t, ok)
				assert.Equal(t, occurredAt, got.At)
			},
		},
		{
			name:      "post changed",
			eventName: EventNamePostChanged,
			payload:   PostChanged{Operation: "update", PostID: 2, BoardID: 3},
			assertAt: func(t *testing.T, event any) {
				got, ok := event.(PostChanged)
				require.True(t, ok)
				assert.Equal(t, occurredAt, got.At)
			},
		},
		{
			name:      "comment changed",
			eventName: EventNameCommentChanged,
			payload:   CommentChanged{Operation: "create", CommentID: 4, PostID: 5},
			assertAt: func(t *testing.T, event any) {
				got, ok := event.(CommentChanged)
				require.True(t, ok)
				assert.Equal(t, occurredAt, got.At)
			},
		},
		{
			name:      "reaction changed",
			eventName: EventNameReactionChanged,
			payload:   ReactionChanged{Operation: "create", TargetType: entity.ReactionTargetPost, TargetID: 6, PostID: 7},
			assertAt: func(t *testing.T, event any) {
				got, ok := event.(ReactionChanged)
				require.True(t, ok)
				assert.Equal(t, occurredAt, got.At)
			},
		},
		{
			name:      "attachment changed",
			eventName: EventNameAttachmentChanged,
			payload:   AttachmentChanged{Operation: "delete", AttachmentID: 8, PostID: 9},
			assertAt: func(t *testing.T, event any) {
				got, ok := event.(AttachmentChanged)
				require.True(t, ok)
				assert.Equal(t, occurredAt, got.At)
			},
		},
		{
			name:      "signup email verification requested",
			eventName: EventNameSignupEmailVerificationRequested,
			payload:   SignupEmailVerificationRequested{MailDeliveryRequested: MailDeliveryRequested{UserID: 1, Email: "alice@example.com", RawToken: "raw-signup", TokenHash: "hash-signup"}},
			assertAt: func(t *testing.T, event any) {
				got, ok := event.(SignupEmailVerificationRequested)
				require.True(t, ok)
				assert.Equal(t, occurredAt, got.At)
				assert.Equal(t, int64(1), got.UserID)
				assert.Equal(t, "alice@example.com", got.Email)
				assert.Equal(t, "raw-signup", got.RawToken)
				assert.Equal(t, "hash-signup", got.TokenHash)
			},
		},
		{
			name:      "email verification resend requested",
			eventName: EventNameEmailVerificationResendRequested,
			payload:   EmailVerificationResendRequested{MailDeliveryRequested: MailDeliveryRequested{UserID: 2, Email: "alice@example.com", RawToken: "raw-resend", TokenHash: "hash-resend"}},
			assertAt: func(t *testing.T, event any) {
				got, ok := event.(EmailVerificationResendRequested)
				require.True(t, ok)
				assert.Equal(t, occurredAt, got.At)
				assert.Equal(t, int64(2), got.UserID)
				assert.Equal(t, "raw-resend", got.RawToken)
				assert.Equal(t, "hash-resend", got.TokenHash)
			},
		},
		{
			name:      "password reset requested",
			eventName: EventNamePasswordResetRequested,
			payload:   PasswordResetRequested{MailDeliveryRequested: MailDeliveryRequested{UserID: 3, Email: "bob@example.com", RawToken: "raw-reset", TokenHash: "hash-reset"}},
			assertAt: func(t *testing.T, event any) {
				got, ok := event.(PasswordResetRequested)
				require.True(t, ok)
				assert.Equal(t, occurredAt, got.At)
				assert.Equal(t, int64(3), got.UserID)
				assert.Equal(t, "bob@example.com", got.Email)
				assert.Equal(t, "raw-reset", got.RawToken)
				assert.Equal(t, "hash-reset", got.TokenHash)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := json.Marshal(tc.payload)
			require.NoError(t, err)

			event, err := s.Deserialize(tc.eventName, raw, occurredAt)
			require.NoError(t, err)
			tc.assertAt(t, event)
		})
	}
}

func TestJSONEventSerializer_Deserialize_UnsupportedEvent(t *testing.T) {
	s := NewJSONEventSerializer()
	_, err := s.Deserialize("unknown.event", []byte(`{}`), time.Now())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported event name")
}
