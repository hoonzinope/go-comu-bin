package service

import (
	"context"
	"errors"
	"testing"
	"time"

	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserService_SignUp_Success(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewUserService(repositories.user, newTestPasswordHasher(), repositories.unitOfWork)

	result, err := svc.SignUp(context.Background(), "alice", "pw")
	require.NoError(t, err)
	assert.Equal(t, "ok", result)
}

func TestUserService_SignUp_Duplicate(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewUserService(repositories.user, newTestPasswordHasher(), repositories.unitOfWork)
	_, _ = svc.SignUp(context.Background(), "alice", "pw")

	_, err := svc.SignUp(context.Background(), "alice", "pw2")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrUserAlreadyExists))
}

func TestUserService_SignUp_TrimsUsernameBeforePersist(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewUserService(repositories.user, newTestPasswordHasher(), repositories.unitOfWork)

	_, err := svc.SignUp(context.Background(), " alice ", "pw")
	require.NoError(t, err)

	user, err := repositories.user.SelectUserByUsername("alice")
	require.NoError(t, err)
	require.NotNil(t, user)
	assert.Equal(t, "alice", user.Name)
}

func TestUserService_SignUp_DuplicateAfterWhitespaceNormalization(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewUserService(repositories.user, newTestPasswordHasher(), repositories.unitOfWork)
	_, err := svc.SignUp(context.Background(), "alice", "pw")
	require.NoError(t, err)

	_, err = svc.SignUp(context.Background(), " alice ", "pw2")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrUserAlreadyExists))
}

func TestUserService_SignUp_InvalidInput(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewUserService(repositories.user, newTestPasswordHasher(), repositories.unitOfWork)

	_, err := svc.SignUp(context.Background(), " ", "pw")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrInvalidInput))
}

func TestUserService_DeleteMe_InvalidCredential(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewUserService(repositories.user, newTestPasswordHasher(), repositories.unitOfWork)
	_, _ = svc.SignUp(context.Background(), "alice", "pw")
	user, err := repositories.user.SelectUserByUsername("alice")
	require.NoError(t, err)
	require.NotNil(t, user)

	err = svc.DeleteMe(context.Background(), user.ID, "wrong")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrInvalidCredential))
}

func TestUserService_DeleteMe_Success(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewUserService(repositories.user, newTestPasswordHasher(), repositories.unitOfWork)
	_, _ = svc.SignUp(context.Background(), "alice", "pw")
	user, err := repositories.user.SelectUserByUsername("alice")
	require.NoError(t, err)
	require.NotNil(t, user)

	require.NoError(t, svc.DeleteMe(context.Background(), user.ID, "pw"))
}

func TestUserService_DeleteMe_UserNotFound(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewUserService(repositories.user, newTestPasswordHasher(), repositories.unitOfWork)

	err := svc.DeleteMe(context.Background(), 999, "pw")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrUserNotFound))
}

func TestUserService_DeleteMe_SucceedsEvenWhenUserHasPostsCommentsAndReactions(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewUserService(repositories.user, newTestPasswordHasher(), repositories.unitOfWork)
	_, _ = svc.SignUp(context.Background(), "alice", "pw")
	_, _ = svc.SignUp(context.Background(), "bob", "pw")
	alice, err := repositories.user.SelectUserByUsername("alice")
	require.NoError(t, err)
	require.NotNil(t, alice)
	bob, err := repositories.user.SelectUserByUsername("bob")
	require.NoError(t, err)
	require.NotNil(t, bob)

	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, bob.ID, boardID, "title", "content")
	seedComment(repositories.comment, alice.ID, postID, "comment")
	_, _, _, err = repositories.reaction.SetUserTargetReaction(alice.ID, postID, "post", "like")
	require.NoError(t, err)

	err = svc.DeleteMe(context.Background(), alice.ID, "pw")
	require.NoError(t, err)
}

func TestUserService_DeleteMe_AllowsReuseOfUsernameAfterSoftDelete(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewUserService(repositories.user, newTestPasswordHasher(), repositories.unitOfWork)
	_, _ = svc.SignUp(context.Background(), "alice", "pw")
	user, err := repositories.user.SelectUserByUsername("alice")
	require.NoError(t, err)
	require.NotNil(t, user)

	require.NoError(t, svc.DeleteMe(context.Background(), user.ID, "pw"))

	_, err = svc.SignUp(context.Background(), "alice", "pw2")
	require.NoError(t, err)
}

func TestUserService_DeleteMe_InvalidatesCredentialsAfterSoftDelete(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewUserService(repositories.user, newTestPasswordHasher(), repositories.unitOfWork)
	_, _ = svc.SignUp(context.Background(), "alice", "pw")
	user, err := repositories.user.SelectUserByUsername("alice")
	require.NoError(t, err)
	require.NotNil(t, user)

	require.NoError(t, svc.DeleteMe(context.Background(), user.ID, "pw"))

	_, err = svc.VerifyCredentials("alice", "pw")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrInvalidCredential))
}

func TestUserService_VerifyCredentials_UserNotFound(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewUserService(repositories.user, newTestPasswordHasher(), repositories.unitOfWork)

	_, err := svc.VerifyCredentials("nope", "pw")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrInvalidCredential))
}

func TestUserService_VerifyCredentials_WrongPassword(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewUserService(repositories.user, newTestPasswordHasher(), repositories.unitOfWork)
	_, _ = svc.SignUp(context.Background(), "alice", "pw")

	_, err := svc.VerifyCredentials("alice", "wrong")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrInvalidCredential))
}

func TestUserService_VerifyCredentials_TrimsUsername(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewUserService(repositories.user, newTestPasswordHasher(), repositories.unitOfWork)
	_, err := svc.SignUp(context.Background(), "alice", "pw")
	require.NoError(t, err)

	userID, err := svc.VerifyCredentials(" alice ", "pw")
	require.NoError(t, err)
	assert.NotZero(t, userID)
}

func TestUserService_SuspendUser_Success(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewUserService(repositories.user, newTestPasswordHasher(), repositories.unitOfWork)
	adminID := seedUser(repositories.user, "admin", "pw", "admin")
	targetID := seedUser(repositories.user, "alice", "pw", "user")
	target, err := repositories.user.SelectUserByID(targetID)
	require.NoError(t, err)
	require.NotNil(t, target)

	err = svc.SuspendUser(context.Background(), adminID, target.UUID, "spam", "7d")
	require.NoError(t, err)

	target, err = repositories.user.SelectUserByID(targetID)
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
	target, err := repositories.user.SelectUserByID(targetID)
	require.NoError(t, err)
	require.NotNil(t, target)

	err = svc.SuspendUser(context.Background(), userID, target.UUID, "spam", "7d")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrForbidden))
}

func TestUserService_UnsuspendUser_Success(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewUserService(repositories.user, newTestPasswordHasher(), repositories.unitOfWork)
	adminID := seedUser(repositories.user, "admin", "pw", "admin")
	targetID := seedUser(repositories.user, "alice", "pw", "user")
	target, err := repositories.user.SelectUserByID(targetID)
	require.NoError(t, err)
	require.NotNil(t, target)

	require.NoError(t, svc.SuspendUser(context.Background(), adminID, target.UUID, "spam", "unlimited"))
	require.NoError(t, svc.UnsuspendUser(context.Background(), adminID, target.UUID))

	target, err = repositories.user.SelectUserByID(targetID)
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
	targetID, err := repositories.user.Save(target)
	require.NoError(t, err)
	target, err = repositories.user.SelectUserByID(targetID)
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
	target, err := repositories.user.SelectUserByID(targetID)
	require.NoError(t, err)
	require.NotNil(t, target)

	_, err = svc.GetUserSuspension(context.Background(), userID, target.UUID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrForbidden))
}
