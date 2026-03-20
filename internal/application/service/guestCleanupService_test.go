package service

import (
	"context"
	"testing"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/hoonzinope/go-comu-bin/internal/infrastructure/auth"
	cacheInMemory "github.com/hoonzinope/go-comu-bin/internal/infrastructure/cache/inmemory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGuestCleanupService_CleanupGuests_SoftDeletesStaleGuestWithoutSessionOrContent(t *testing.T) {
	repositories := newTestRepositories()
	cache := cacheInMemory.NewInMemoryCache()
	sessionRepository := auth.NewCacheSessionRepository(cache)
	svc := NewGuestCleanupService(repositories.user, repositories.post, repositories.comment, repositories.reaction, repositories.report, sessionRepository, repositories.unitOfWork)

	guest := entity.NewGuest("guest-1", "guest-1@example.invalid", "pw")
	guest.MarkGuestExpired()
	expiredAt := time.Now().Add(-2 * time.Hour)
	guest.GuestExpiredAt = &expiredAt
	guestID, err := repositories.user.Save(context.Background(), guest)
	require.NoError(t, err)

	deletedCount, err := svc.CleanupGuests(context.Background(), time.Now(), time.Hour, time.Hour, 10)
	require.NoError(t, err)
	assert.Equal(t, 1, deletedCount)

	user, err := repositories.user.SelectUserByID(context.Background(), guestID)
	require.NoError(t, err)
	assert.Nil(t, user)
}

func TestGuestCleanupService_CleanupGuests_SkipsGuestWithActiveSession(t *testing.T) {
	repositories := newTestRepositories()
	cache := cacheInMemory.NewInMemoryCache()
	sessionRepository := auth.NewCacheSessionRepository(cache)
	svc := NewGuestCleanupService(repositories.user, repositories.post, repositories.comment, repositories.reaction, repositories.report, sessionRepository, repositories.unitOfWork)

	guest := entity.NewGuest("guest-1", "guest-1@example.invalid", "pw")
	guest.MarkGuestActive()
	activatedAt := time.Now().Add(-2 * time.Hour)
	guest.GuestActivatedAt = &activatedAt
	guestID, err := repositories.user.Save(context.Background(), guest)
	require.NoError(t, err)

	tokenProvider := auth.NewJwtTokenProvider("test-secret")
	token, err := tokenProvider.IdToToken(guestID)
	require.NoError(t, err)
	require.NoError(t, sessionRepository.Save(context.Background(), guestID, token, tokenProvider.TTLSeconds()))

	deletedCount, err := svc.CleanupGuests(context.Background(), time.Now(), time.Hour, time.Hour, 10)
	require.NoError(t, err)
	assert.Equal(t, 0, deletedCount)

	user, err := repositories.user.SelectUserByID(context.Background(), guestID)
	require.NoError(t, err)
	require.NotNil(t, user)
	assert.True(t, user.IsGuest())
}

func TestGuestCleanupService_CleanupGuests_SkipsGuestWithPosts(t *testing.T) {
	repositories := newTestRepositories()
	cache := cacheInMemory.NewInMemoryCache()
	sessionRepository := auth.NewCacheSessionRepository(cache)
	svc := NewGuestCleanupService(repositories.user, repositories.post, repositories.comment, repositories.reaction, repositories.report, sessionRepository, repositories.unitOfWork)

	guest := entity.NewGuest("guest-1", "guest-1@example.invalid", "pw")
	guest.MarkGuestActive()
	activatedAt := time.Now().Add(-2 * time.Hour)
	guest.GuestActivatedAt = &activatedAt
	guestID, err := repositories.user.Save(context.Background(), guest)
	require.NoError(t, err)
	boardID := seedBoard(repositories.board, "free", "desc")
	seedPost(repositories.post, guestID, boardID, "title", "content")

	deletedCount, err := svc.CleanupGuests(context.Background(), time.Now(), time.Hour, time.Hour, 10)
	require.NoError(t, err)
	assert.Equal(t, 0, deletedCount)

	user, err := repositories.user.SelectUserByID(context.Background(), guestID)
	require.NoError(t, err)
	require.NotNil(t, user)
	assert.True(t, user.IsGuest())
}

func TestGuestCleanupService_CleanupGuests_DeletesGuestWithOnlyDeletedContent(t *testing.T) {
	repositories := newTestRepositories()
	cache := cacheInMemory.NewInMemoryCache()
	sessionRepository := auth.NewCacheSessionRepository(cache)
	svc := NewGuestCleanupService(repositories.user, repositories.post, repositories.comment, repositories.reaction, repositories.report, sessionRepository, repositories.unitOfWork)

	guest := entity.NewGuest("guest-1", "guest-1@example.invalid", "pw")
	guest.MarkGuestActive()
	activatedAt := time.Now().Add(-2 * time.Hour)
	guest.GuestActivatedAt = &activatedAt
	guestID, err := repositories.user.Save(context.Background(), guest)
	require.NoError(t, err)

	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, guestID, boardID, "title", "content")
	commentID := seedComment(repositories.comment, guestID, postID, "comment")

	post, err := repositories.post.SelectPostByIDIncludingUnpublished(context.Background(), postID)
	require.NoError(t, err)
	require.NotNil(t, post)
	post.SoftDelete()
	require.NoError(t, repositories.post.Update(context.Background(), post))

	comment, err := repositories.comment.SelectCommentByID(context.Background(), commentID)
	require.NoError(t, err)
	require.NotNil(t, comment)
	comment.SoftDelete()
	require.NoError(t, repositories.comment.Update(context.Background(), comment))

	deletedCount, err := svc.CleanupGuests(context.Background(), time.Now(), time.Hour, time.Hour, 10)
	require.NoError(t, err)
	assert.Equal(t, 1, deletedCount)

	user, err := repositories.user.SelectUserByID(context.Background(), guestID)
	require.NoError(t, err)
	assert.Nil(t, user)
}
