package event

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	inmemory "github.com/hoonzinope/go-comu-bin/internal/infrastructure/persistence/inmemory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingEmailVerificationMailer struct {
	calls []struct {
		email     string
		token     string
		expiresAt time.Time
	}
	err error
}

func (m *recordingEmailVerificationMailer) SendEmailVerification(ctx context.Context, email, token string, expiresAt time.Time) error {
	_ = ctx
	m.calls = append(m.calls, struct {
		email     string
		token     string
		expiresAt time.Time
	}{email: email, token: token, expiresAt: expiresAt})
	return m.err
}

type recordingPasswordResetMailer struct {
	calls []struct {
		email     string
		token     string
		expiresAt time.Time
	}
	err error
}

func (m *recordingPasswordResetMailer) SendPasswordReset(ctx context.Context, email, token string, expiresAt time.Time) error {
	_ = ctx
	m.calls = append(m.calls, struct {
		email     string
		token     string
		expiresAt time.Time
	}{email: email, token: token, expiresAt: expiresAt})
	return m.err
}

type failingEmailVerificationTokenRepository struct {
	base      port.EmailVerificationTokenRepository
	updateErr error
}

func (r *failingEmailVerificationTokenRepository) Save(ctx context.Context, token *entity.EmailVerificationToken) error {
	return r.base.Save(ctx, token)
}

func (r *failingEmailVerificationTokenRepository) SelectByTokenHash(ctx context.Context, tokenHash string) (*entity.EmailVerificationToken, error) {
	return r.base.SelectByTokenHash(ctx, tokenHash)
}

func (r *failingEmailVerificationTokenRepository) SelectLatestByUser(ctx context.Context, userID int64) (*entity.EmailVerificationToken, error) {
	return r.base.SelectLatestByUser(ctx, userID)
}

func (r *failingEmailVerificationTokenRepository) InvalidateByUser(ctx context.Context, userID int64) error {
	return r.base.InvalidateByUser(ctx, userID)
}

func (r *failingEmailVerificationTokenRepository) Update(ctx context.Context, token *entity.EmailVerificationToken) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	return r.base.Update(ctx, token)
}

func (r *failingEmailVerificationTokenRepository) DeleteExpiredOrConsumedBefore(ctx context.Context, cutoff time.Time, limit int) (int, error) {
	return r.base.DeleteExpiredOrConsumedBefore(ctx, cutoff, limit)
}

type failingPasswordResetTokenRepository struct {
	base      port.PasswordResetTokenRepository
	updateErr error
}

func (r *failingPasswordResetTokenRepository) Save(ctx context.Context, token *entity.PasswordResetToken) error {
	return r.base.Save(ctx, token)
}

func (r *failingPasswordResetTokenRepository) SelectByTokenHash(ctx context.Context, tokenHash string) (*entity.PasswordResetToken, error) {
	return r.base.SelectByTokenHash(ctx, tokenHash)
}

func (r *failingPasswordResetTokenRepository) SelectLatestByUser(ctx context.Context, userID int64) (*entity.PasswordResetToken, error) {
	return r.base.SelectLatestByUser(ctx, userID)
}

func (r *failingPasswordResetTokenRepository) InvalidateByUser(ctx context.Context, userID int64) error {
	return r.base.InvalidateByUser(ctx, userID)
}

func (r *failingPasswordResetTokenRepository) Update(ctx context.Context, token *entity.PasswordResetToken) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	return r.base.Update(ctx, token)
}

func (r *failingPasswordResetTokenRepository) DeleteExpiredOrConsumedBefore(ctx context.Context, cutoff time.Time, limit int) (int, error) {
	return r.base.DeleteExpiredOrConsumedBefore(ctx, cutoff, limit)
}

func TestMailDeliveryHandler_Handle_SendsSignupVerificationAndActivatesExactToken(t *testing.T) {
	verificationRepo := inmemory.NewEmailVerificationTokenRepository()
	pending := entity.NewEmailVerificationToken(10, "hash-signup", time.Now().Add(time.Hour))
	pending.Consume(time.Now())
	require.NoError(t, verificationRepo.Save(context.Background(), pending))
	mailer := &recordingEmailVerificationMailer{}
	handler := NewMailDeliveryHandler(mailer, &recordingPasswordResetMailer{}, verificationRepo, inmemory.NewPasswordResetTokenRepository(), "")

	err := handler.Handle(context.Background(), NewSignupEmailVerificationRequested(10, "alice@example.com", "raw-signup", "hash-signup", time.Now().Add(time.Hour)))
	require.NoError(t, err)
	require.Len(t, mailer.calls, 1)
	assert.Equal(t, "alice@example.com", mailer.calls[0].email)
	assert.Equal(t, "raw-signup", mailer.calls[0].token)
	activated, err := verificationRepo.SelectByTokenHash(context.Background(), "hash-signup")
	require.NoError(t, err)
	require.NotNil(t, activated)
	assert.True(t, activated.IsUsable(time.Now()))
}

func TestMailDeliveryHandler_Handle_SendsPasswordResetAndActivatesExactToken(t *testing.T) {
	resetRepo := inmemory.NewPasswordResetTokenRepository()
	pending := entity.NewPasswordResetToken(11, "hash-reset", time.Now().Add(time.Hour))
	pending.Consume(time.Now())
	require.NoError(t, resetRepo.Save(context.Background(), pending))
	mailer := &recordingPasswordResetMailer{}
	handler := NewMailDeliveryHandler(&recordingEmailVerificationMailer{}, mailer, inmemory.NewEmailVerificationTokenRepository(), resetRepo, "")

	err := handler.Handle(context.Background(), NewPasswordResetRequested(11, "bob@example.com", "raw-reset", "hash-reset", time.Now().Add(time.Hour)))
	require.NoError(t, err)
	require.Len(t, mailer.calls, 1)
	assert.Equal(t, "bob@example.com", mailer.calls[0].email)
	assert.Equal(t, "raw-reset", mailer.calls[0].token)
	activated, err := resetRepo.SelectByTokenHash(context.Background(), "hash-reset")
	require.NoError(t, err)
	require.NotNil(t, activated)
	assert.True(t, activated.IsUsable(time.Now()))
}

func TestMailDeliveryHandler_Handle_TreatsMissingTokenAsSuccess(t *testing.T) {
	mailer := &recordingEmailVerificationMailer{}
	handler := NewMailDeliveryHandler(mailer, &recordingPasswordResetMailer{}, inmemory.NewEmailVerificationTokenRepository(), inmemory.NewPasswordResetTokenRepository(), "")

	err := handler.Handle(context.Background(), NewEmailVerificationResendRequested(12, "alice@example.com", "raw-resend", "missing-hash", time.Now().Add(time.Hour)))
	require.NoError(t, err)
	require.Len(t, mailer.calls, 1)
}

func TestMailDeliveryHandler_Handle_ReturnsRepositoryFailure(t *testing.T) {
	verificationRepo := &failingEmailVerificationTokenRepository{
		base:      inmemory.NewEmailVerificationTokenRepository(),
		updateErr: errors.New("update failed"),
	}
	pending := entity.NewEmailVerificationToken(13, "hash-fail", time.Now().Add(time.Hour))
	pending.Consume(time.Now())
	require.NoError(t, verificationRepo.base.Save(context.Background(), pending))
	mailer := &recordingEmailVerificationMailer{}
	handler := NewMailDeliveryHandler(mailer, &recordingPasswordResetMailer{}, verificationRepo, inmemory.NewPasswordResetTokenRepository(), "")

	err := handler.Handle(context.Background(), NewSignupEmailVerificationRequested(13, "alice@example.com", "raw-fail", "hash-fail", time.Now().Add(time.Hour)))
	require.Error(t, err)
	assert.ErrorIs(t, err, customerror.ErrRepositoryFailure)
	assert.Contains(t, err.Error(), "activate email verification token after mail send")
	require.Len(t, mailer.calls, 1)
}

func TestMailDeliveryHandler_Handle_ReturnsMailSendFailure(t *testing.T) {
	mailer := &recordingEmailVerificationMailer{err: errors.New("smtp down")}
	handler := NewMailDeliveryHandler(mailer, &recordingPasswordResetMailer{}, inmemory.NewEmailVerificationTokenRepository(), inmemory.NewPasswordResetTokenRepository(), "")

	err := handler.Handle(context.Background(), NewSignupEmailVerificationRequested(14, "alice@example.com", "raw", "hash", time.Now().Add(time.Hour)))
	require.Error(t, err)
	assert.ErrorIs(t, err, customerror.ErrMailDeliveryFailure)
	assert.Contains(t, err.Error(), "send email verification mail")
}

func TestMailDeliveryHandler_Handle_IgnoresUnknownEvent(t *testing.T) {
	handler := NewMailDeliveryHandler(&recordingEmailVerificationMailer{}, &recordingPasswordResetMailer{}, inmemory.NewEmailVerificationTokenRepository(), inmemory.NewPasswordResetTokenRepository(), "")
	err := handler.Handle(context.Background(), nil)
	require.NoError(t, err)
}

func TestMailDeliveryHandler_Handle_AlreadyActiveTokenIsNoOp(t *testing.T) {
	verificationRepo := inmemory.NewEmailVerificationTokenRepository()
	active := entity.NewEmailVerificationToken(15, "hash-active", time.Now().Add(time.Hour))
	require.NoError(t, verificationRepo.Save(context.Background(), active))
	mailer := &recordingEmailVerificationMailer{}
	handler := NewMailDeliveryHandler(mailer, &recordingPasswordResetMailer{}, verificationRepo, inmemory.NewPasswordResetTokenRepository(), "")

	err := handler.Handle(context.Background(), NewSignupEmailVerificationRequested(15, "alice@example.com", "raw-active", "hash-active", time.Now().Add(time.Hour)))
	require.NoError(t, err)
	require.Len(t, mailer.calls, 1)
	loaded, err := verificationRepo.SelectByTokenHash(context.Background(), "hash-active")
	require.NoError(t, err)
	require.NotNil(t, loaded)
	assert.False(t, loaded.IsConsumed())
}

func TestMailDeliveryHandler_Handle_NoOpWhenHandlerOrDependenciesMissing(t *testing.T) {
	var handler *MailDeliveryHandler
	require.NoError(t, handler.Handle(context.Background(), NewSignupEmailVerificationRequested(1, "alice@example.com", "raw", "hash", time.Now().Add(time.Hour))))

	handler = NewMailDeliveryHandler(nil, nil, nil, nil, "")
	require.NoError(t, handler.Handle(context.Background(), NewPasswordResetRequested(1, "bob@example.com", "raw", "hash", time.Now().Add(time.Hour))))
}
