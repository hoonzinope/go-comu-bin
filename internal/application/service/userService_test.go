package service

import (
	"context"
	"errors"
	"testing"
	"time"

	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserService_SignUp_Success(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewUserService(repositories.user, newTestPasswordHasher(), repositories.unitOfWork)

	result, err := svc.SignUp(context.Background(), "alice", "alice@example.com", "pw")
	require.NoError(t, err)
	assert.Equal(t, "ok", result)

	user, err := repositories.user.SelectUserByEmail(context.Background(), "alice@example.com")
	require.NoError(t, err)
	require.NotNil(t, user)
	assert.Equal(t, "alice", user.Name)
}

func TestUserService_SignUp_SendsEmailVerificationWhenConfigured(t *testing.T) {
	repositories := newTestRepositories()
	mailer := newRecordingEmailVerificationMailSender()
	issuer := &fixedEmailVerificationTokenIssuer{tokens: []string{"verify-token-1"}}
	svc := NewUserServiceWithEmailVerification(repositories.user, newTestPasswordHasher(), repositories.unitOfWork, repositories.emailVerification, issuer, mailer, 30*time.Minute)

	result, err := svc.SignUp(context.Background(), "alice", "alice@example.com", "pw")
	require.NoError(t, err)
	assert.Equal(t, "ok", result)
	require.Len(t, mailer.sent, 1)
	assert.Equal(t, "alice@example.com", mailer.sent[0].email)
	assert.Equal(t, "verify-token-1", mailer.sent[0].token)

	user, err := repositories.user.SelectUserByEmail(context.Background(), "alice@example.com")
	require.NoError(t, err)
	require.NotNil(t, user)
	assert.False(t, user.IsEmailVerified())

	saved, err := repositories.emailVerification.SelectByTokenHash(context.Background(), testHashEmailVerificationToken("verify-token-1"))
	require.NoError(t, err)
	require.NotNil(t, saved)
	assert.True(t, saved.IsUsable(time.Now()))
}

func TestUserService_SignUp_DoesNotSendVerificationEmailWhenCommitFails(t *testing.T) {
	repositories := newTestRepositories()
	mailer := newRecordingEmailVerificationMailSender()
	issuer := &fixedEmailVerificationTokenIssuer{tokens: []string{"verify-token-1"}}
	svc := NewUserServiceWithEmailVerification(
		repositories.user,
		newTestPasswordHasher(),
		failingCommitUnitOfWork{
			scope: &accountTestTxScope{
				user:              repositories.user,
				emailVerification: repositories.emailVerification,
			},
			err: errors.New("commit failed"),
		},
		repositories.emailVerification,
		issuer,
		mailer,
		30*time.Minute,
	)

	_, err := svc.SignUp(context.Background(), "alice", "alice@example.com", "pw")
	require.Error(t, err)
	require.Empty(t, mailer.sent)
}

func TestUserService_SignUp_RollsBackCreatedUserWhenMailSendFails(t *testing.T) {
	repositories := newTestRepositories()
	mailer := newRecordingEmailVerificationMailSender()
	mailer.err = errors.New("send failed")
	issuer := &fixedEmailVerificationTokenIssuer{tokens: []string{"verify-token-1"}}
	svc := NewUserServiceWithEmailVerification(
		repositories.user,
		newTestPasswordHasher(),
		repositories.unitOfWork,
		repositories.emailVerification,
		issuer,
		mailer,
		30*time.Minute,
	)

	_, err := svc.SignUp(context.Background(), "alice", "alice@example.com", "pw")
	require.Error(t, err)
	assert.ErrorIs(t, err, customerror.ErrInternalServerError)
	require.Len(t, mailer.sent, 1)

	user, err := repositories.user.SelectUserByEmail(context.Background(), "alice@example.com")
	require.NoError(t, err)
	assert.Nil(t, user)

	saved, err := repositories.emailVerification.SelectByTokenHash(context.Background(), testHashEmailVerificationToken("verify-token-1"))
	require.NoError(t, err)
	require.NotNil(t, saved)
	assert.True(t, saved.IsConsumed())
	assert.False(t, saved.IsUsable(time.Now()))
}

func TestUserService_SignUp_DeletesUserWhenVerificationActivationFails(t *testing.T) {
	repositories := newTestRepositories()
	mailer := newRecordingEmailVerificationMailSender()
	issuer := &fixedEmailVerificationTokenIssuer{tokens: []string{"verify-token-1"}}
	svc := NewUserServiceWithEmailVerification(
		repositories.user,
		newTestPasswordHasher(),
		repositories.unitOfWork,
		&failingEmailVerificationTokenRepository{
			base:          repositories.emailVerification,
			updateErr:     errors.New("activation failed"),
		},
		issuer,
		mailer,
		30*time.Minute,
	)

	_, err := svc.SignUp(context.Background(), "alice", "alice@example.com", "pw")
	require.Error(t, err)
	assert.ErrorIs(t, err, customerror.ErrInternalServerError)
	assert.Contains(t, err.Error(), "activation failed")

	user, err := repositories.user.SelectUserByEmail(context.Background(), "alice@example.com")
	require.NoError(t, err)
	assert.Nil(t, user)

	saved, err := repositories.emailVerification.SelectByTokenHash(context.Background(), testHashEmailVerificationToken("verify-token-1"))
	require.NoError(t, err)
	require.NotNil(t, saved)
	assert.True(t, saved.IsConsumed())
	assert.False(t, saved.IsUsable(time.Now()))
}

func TestUserService_IssueGuestAccount_Success(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewUserService(repositories.user, newTestPasswordHasher(), repositories.unitOfWork)

	userID, err := svc.IssueGuestAccount(context.Background())
	require.NoError(t, err)
	assert.NotZero(t, userID)

	user, err := repositories.user.SelectUserByID(context.Background(), userID)
	require.NoError(t, err)
	require.NotNil(t, user)
	assert.True(t, user.IsGuest())
	assert.Equal(t, entity.GuestStatusPending, user.GuestStatus)
	assert.NotNil(t, user.GuestIssuedAt)
	assert.Nil(t, user.GuestActivatedAt)
	assert.Nil(t, user.GuestExpiredAt)
	assert.NotEmpty(t, user.Email)
	assert.NotEmpty(t, user.Password)
}

func TestUserService_IssueGuestAccount_GeneratesUniqueIdentity(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewUserService(repositories.user, newTestPasswordHasher(), repositories.unitOfWork)

	firstID, err := svc.IssueGuestAccount(context.Background())
	require.NoError(t, err)
	secondID, err := svc.IssueGuestAccount(context.Background())
	require.NoError(t, err)
	require.NotEqual(t, firstID, secondID)

	first, err := repositories.user.SelectUserByID(context.Background(), firstID)
	require.NoError(t, err)
	second, err := repositories.user.SelectUserByID(context.Background(), secondID)
	require.NoError(t, err)
	require.NotNil(t, first)
	require.NotNil(t, second)
	assert.NotEqual(t, first.Name, second.Name)
	assert.NotEqual(t, first.Email, second.Email)
}

func TestUserService_SignUp_Duplicate(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewUserService(repositories.user, newTestPasswordHasher(), repositories.unitOfWork)
	_, _ = svc.SignUp(context.Background(), "alice", "alice@example.com", "pw")

	_, err := svc.SignUp(context.Background(), "alice", "alice@example.com", "pw2")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customerror.ErrUserAlreadyExists))
}

func TestUserService_SignUp_RejectsDuplicateEmail(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewUserService(repositories.user, newTestPasswordHasher(), repositories.unitOfWork)
	_, err := svc.SignUp(context.Background(), "alice", "alice@example.com", "pw")
	require.NoError(t, err)

	_, err = svc.SignUp(context.Background(), "bob", "alice@example.com", "pw")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customerror.ErrUserAlreadyExists))
}

func TestUserService_SignUp_InvalidEmail(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewUserService(repositories.user, newTestPasswordHasher(), repositories.unitOfWork)

	_, err := svc.SignUp(context.Background(), "alice", "not-an-email", "pw")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customerror.ErrInvalidInput))
}

func TestUserService_SignUp_TrimsUsernameBeforePersist(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewUserService(repositories.user, newTestPasswordHasher(), repositories.unitOfWork)

	_, err := svc.SignUp(context.Background(), " alice ", "alice@example.com", "pw")
	require.NoError(t, err)

	user, err := repositories.user.SelectUserByUsername(context.Background(), "alice")
	require.NoError(t, err)
	require.NotNil(t, user)
	assert.Equal(t, "alice", user.Name)
}

func TestUserService_SignUp_DuplicateAfterWhitespaceNormalization(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewUserService(repositories.user, newTestPasswordHasher(), repositories.unitOfWork)
	_, err := svc.SignUp(context.Background(), "alice", "alice@example.com", "pw")
	require.NoError(t, err)

	_, err = svc.SignUp(context.Background(), " alice ", "alice@example.com", "pw2")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customerror.ErrUserAlreadyExists))
}

func TestUserService_SignUp_InvalidInput(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewUserService(repositories.user, newTestPasswordHasher(), repositories.unitOfWork)

	_, err := svc.SignUp(context.Background(), " ", "alice@example.com", "pw")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customerror.ErrInvalidInput))
}

func TestUserService_DeleteMe_InvalidCredential(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewUserService(repositories.user, newTestPasswordHasher(), repositories.unitOfWork)
	_, _ = svc.SignUp(context.Background(), "alice", "alice@example.com", "pw")
	user, err := repositories.user.SelectUserByUsername(context.Background(), "alice")
	require.NoError(t, err)
	require.NotNil(t, user)

	err = svc.DeleteMe(context.Background(), user.ID, "wrong")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customerror.ErrInvalidCredential))
}

func TestUserService_DeleteMe_Success(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewUserService(repositories.user, newTestPasswordHasher(), repositories.unitOfWork)
	_, _ = svc.SignUp(context.Background(), "alice", "alice@example.com", "pw")
	user, err := repositories.user.SelectUserByUsername(context.Background(), "alice")
	require.NoError(t, err)
	require.NotNil(t, user)

	require.NoError(t, svc.DeleteMe(context.Background(), user.ID, "pw"))
}

func TestUserService_DeleteMe_RejectsGuestUser(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewUserService(repositories.user, newTestPasswordHasher(), repositories.unitOfWork)
	guest := entity.NewGuest("guest-1", "guest-1@example.invalid", "hashed-secret")
	guestID, err := repositories.user.Save(context.Background(), guest)
	require.NoError(t, err)

	err = svc.DeleteMe(context.Background(), guestID, "")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customerror.ErrForbidden))
}

func TestUserService_DeleteMe_UserNotFound(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewUserService(repositories.user, newTestPasswordHasher(), repositories.unitOfWork)

	err := svc.DeleteMe(context.Background(), 999, "pw")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customerror.ErrUserNotFound))
}

func TestUserService_DeleteMe_SucceedsEvenWhenUserHasPostsCommentsAndReactions(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewUserService(repositories.user, newTestPasswordHasher(), repositories.unitOfWork)
	_, _ = svc.SignUp(context.Background(), "alice", "alice@example.com", "pw")
	_, _ = svc.SignUp(context.Background(), "bob", "bob@example.com", "pw")
	alice, err := repositories.user.SelectUserByUsername(context.Background(), "alice")
	require.NoError(t, err)
	require.NotNil(t, alice)
	bob, err := repositories.user.SelectUserByUsername(context.Background(), "bob")
	require.NoError(t, err)
	require.NotNil(t, bob)

	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, bob.ID, boardID, "title", "content")
	seedComment(repositories.comment, alice.ID, postID, "comment")
	_, _, _, err = repositories.reaction.SetUserTargetReaction(context.Background(), alice.ID, postID, "post", "like")
	require.NoError(t, err)

	err = svc.DeleteMe(context.Background(), alice.ID, "pw")
	require.NoError(t, err)
}

func TestUserService_DeleteMe_AllowsReuseOfUsernameAfterSoftDelete(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewUserService(repositories.user, newTestPasswordHasher(), repositories.unitOfWork)
	_, _ = svc.SignUp(context.Background(), "alice", "alice@example.com", "pw")
	user, err := repositories.user.SelectUserByUsername(context.Background(), "alice")
	require.NoError(t, err)
	require.NotNil(t, user)

	require.NoError(t, svc.DeleteMe(context.Background(), user.ID, "pw"))

	_, err = svc.SignUp(context.Background(), "alice", "alice@example.com", "pw2")
	require.NoError(t, err)
}

func TestUserService_DeleteMe_InvalidatesCredentialsAfterSoftDelete(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewUserService(repositories.user, newTestPasswordHasher(), repositories.unitOfWork)
	_, _ = svc.SignUp(context.Background(), "alice", "alice@example.com", "pw")
	user, err := repositories.user.SelectUserByUsername(context.Background(), "alice")
	require.NoError(t, err)
	require.NotNil(t, user)

	require.NoError(t, svc.DeleteMe(context.Background(), user.ID, "pw"))

	_, err = svc.VerifyCredentials(context.Background(), "alice", "pw")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customerror.ErrInvalidCredential))
}

func TestUserService_VerifyCredentials_RejectsGuestUser(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewUserService(repositories.user, newTestPasswordHasher(), repositories.unitOfWork)
	guest := entity.NewGuest("guest-1", "guest-1@example.invalid", "pw")
	_, err := repositories.user.Save(context.Background(), guest)
	require.NoError(t, err)

	_, err = svc.VerifyCredentials(context.Background(), guest.Name, "pw")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customerror.ErrInvalidCredential))
}

func TestUserService_UpgradeGuest_Success(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewUserService(repositories.user, newTestPasswordHasher(), repositories.unitOfWork)
	guest := entity.NewGuest("guest-1", "guest-1@example.invalid", "pw")
	guestID, err := repositories.user.Save(context.Background(), guest)
	require.NoError(t, err)
	before, err := repositories.user.SelectUserByID(context.Background(), guestID)
	require.NoError(t, err)
	require.NotNil(t, before)

	err = svc.UpgradeGuest(context.Background(), guestID, "alice", "alice@example.com", "newpw")
	require.NoError(t, err)

	after, err := repositories.user.SelectUserByID(context.Background(), guestID)
	require.NoError(t, err)
	require.NotNil(t, after)
	assert.Equal(t, before.UUID, after.UUID)
	assert.Equal(t, guestID, after.ID)
	assert.Equal(t, "alice", after.Name)
	assert.Equal(t, "alice@example.com", after.Email)
	assert.False(t, after.IsGuest())
	assert.Equal(t, entity.GuestStatus(""), after.GuestStatus)
	assert.Nil(t, after.GuestIssuedAt)
	assert.Nil(t, after.GuestActivatedAt)
	assert.Nil(t, after.GuestExpiredAt)
	assert.NotEqual(t, "newpw", after.Password)
}

func TestUserService_UpgradeGuest_RejectsNonGuest(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewUserService(repositories.user, newTestPasswordHasher(), repositories.unitOfWork)
	userID := seedUser(repositories.user, "alice", "pw", "user")

	err := svc.UpgradeGuest(context.Background(), userID, "alice2", "alice2@example.com", "newpw")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customerror.ErrInvalidInput))
}

func TestUserService_UpgradeGuest_RejectsDuplicateIdentity(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewUserService(repositories.user, newTestPasswordHasher(), repositories.unitOfWork)
	_, err := svc.SignUp(context.Background(), "alice", "alice@example.com", "pw")
	require.NoError(t, err)
	guest := entity.NewGuest("guest-1", "guest-1@example.invalid", "pw")
	guestID, err := repositories.user.Save(context.Background(), guest)
	require.NoError(t, err)

	err = svc.UpgradeGuest(context.Background(), guestID, "alice", "alice@example.com", "newpw")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customerror.ErrUserAlreadyExists))
}

func TestUserService_VerifyCredentials_UserNotFound(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewUserService(repositories.user, newTestPasswordHasher(), repositories.unitOfWork)

	_, err := svc.VerifyCredentials(context.Background(), "nope", "pw")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customerror.ErrInvalidCredential))
}

func TestUserService_VerifyCredentials_WrongPassword(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewUserService(repositories.user, newTestPasswordHasher(), repositories.unitOfWork)
	_, _ = svc.SignUp(context.Background(), "alice", "alice@example.com", "pw")

	_, err := svc.VerifyCredentials(context.Background(), "alice", "wrong")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customerror.ErrInvalidCredential))
}

func TestUserService_VerifyCredentials_TrimsUsername(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewUserService(repositories.user, newTestPasswordHasher(), repositories.unitOfWork)
	_, err := svc.SignUp(context.Background(), "alice", "alice@example.com", "pw")
	require.NoError(t, err)

	userID, err := svc.VerifyCredentials(context.Background(), " alice ", "pw")
	require.NoError(t, err)
	assert.NotZero(t, userID)
}

func TestUserService_SuspendUser_Success(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewUserService(repositories.user, newTestPasswordHasher(), repositories.unitOfWork)
	adminID := seedUser(repositories.user, "admin", "pw", "admin")
	targetID := seedUser(repositories.user, "alice", "pw", "user")
	target, err := repositories.user.SelectUserByID(context.Background(), targetID)
	require.NoError(t, err)
	require.NotNil(t, target)

	err = svc.SuspendUser(context.Background(), adminID, target.UUID, "spam", "7d")
	require.NoError(t, err)

	target, err = repositories.user.SelectUserByID(context.Background(), targetID)
	require.NoError(t, err)
	require.NotNil(t, target)
	assert.True(t, target.IsSuspended())
	assert.Equal(t, "spam", target.SuspensionReason)
	require.NotNil(t, target.SuspendedUntil)
}

func TestUserService_SuspendUser_ForbiddenForNonAdmin(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewUserService(repositories.user, newTestPasswordHasher(), repositories.unitOfWork)
	userID := seedUser(repositories.user, "user", "pw", "user")
	targetID := seedUser(repositories.user, "alice", "pw", "user")
	target, err := repositories.user.SelectUserByID(context.Background(), targetID)
	require.NoError(t, err)
	require.NotNil(t, target)

	err = svc.SuspendUser(context.Background(), userID, target.UUID, "spam", "7d")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customerror.ErrForbidden))
}

func TestUserService_UnsuspendUser_Success(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewUserService(repositories.user, newTestPasswordHasher(), repositories.unitOfWork)
	adminID := seedUser(repositories.user, "admin", "pw", "admin")
	targetID := seedUser(repositories.user, "alice", "pw", "user")
	target, err := repositories.user.SelectUserByID(context.Background(), targetID)
	require.NoError(t, err)
	require.NotNil(t, target)

	require.NoError(t, svc.SuspendUser(context.Background(), adminID, target.UUID, "spam", "unlimited"))
	require.NoError(t, svc.UnsuspendUser(context.Background(), adminID, target.UUID))

	target, err = repositories.user.SelectUserByID(context.Background(), targetID)
	require.NoError(t, err)
	require.NotNil(t, target)
	assert.False(t, target.IsSuspended())
	assert.Equal(t, "", target.SuspensionReason)
	assert.Nil(t, target.SuspendedUntil)
}

func TestUserService_GetUserSuspension_Success(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewUserService(repositories.user, newTestPasswordHasher(), repositories.unitOfWork)
	adminID := seedUser(repositories.user, "admin", "pw", "admin")
	target := entity.NewUser("alice", "pw")
	until := time.Now().Add(7 * 24 * time.Hour)
	target.Suspend("spam", &until)
	targetID, err := repositories.user.Save(context.Background(), target)
	require.NoError(t, err)
	target, err = repositories.user.SelectUserByID(context.Background(), targetID)
	require.NoError(t, err)
	require.NotNil(t, target)

	view, err := svc.GetUserSuspension(context.Background(), adminID, target.UUID)
	require.NoError(t, err)
	require.NotNil(t, view)
	assert.Equal(t, target.UUID, view.UserUUID)
	assert.Equal(t, entity.UserStatusSuspended, view.Status)
	assert.Equal(t, "spam", view.Reason)
	require.NotNil(t, view.SuspendedUntil)
}

func TestUserService_GetUserSuspension_ForbiddenForNonAdmin(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewUserService(repositories.user, newTestPasswordHasher(), repositories.unitOfWork)
	userID := seedUser(repositories.user, "user", "pw", "user")
	targetID := seedUser(repositories.user, "alice", "pw", "user")
	target, err := repositories.user.SelectUserByID(context.Background(), targetID)
	require.NoError(t, err)
	require.NotNil(t, target)

	_, err = svc.GetUserSuspension(context.Background(), userID, target.UUID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customerror.ErrForbidden))
}
