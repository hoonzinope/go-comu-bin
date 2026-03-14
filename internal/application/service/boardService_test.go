package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/cache/key"
	"github.com/hoonzinope/go-comu-bin/internal/application/cache/testutil"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBoardService_CreateBoard_ForbiddenForNonAdmin(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "user", "pw", "user")
	svc := NewBoardService(repositories.user, repositories.board, repositories.post, repositories.unitOfWork, newTestCache(), newTestCachePolicy(), newTestAuthorizationPolicy())

	_, err := svc.CreateBoard(context.Background(), userID, "free", "desc")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customerror.ErrForbidden))
}

func TestBoardService_CreateBoard_SuccessForAdmin(t *testing.T) {
	repositories := newTestRepositories()
	adminID := seedUser(repositories.user, "admin", "pw", "admin")
	svc := NewBoardService(repositories.user, repositories.board, repositories.post, repositories.unitOfWork, newTestCache(), newTestCachePolicy(), newTestAuthorizationPolicy())

	boardID, err := svc.CreateBoard(context.Background(), adminID, "free", "desc")
	require.NoError(t, err)
	assert.NotZero(t, boardID)
}

func TestBoardService_CreateBoard_InvalidInput(t *testing.T) {
	repositories := newTestRepositories()
	adminID := seedUser(repositories.user, "admin", "pw", "admin")
	svc := NewBoardService(repositories.user, repositories.board, repositories.post, repositories.unitOfWork, newTestCache(), newTestCachePolicy(), newTestAuthorizationPolicy())

	_, err := svc.CreateBoard(context.Background(), adminID, " ", "desc")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customerror.ErrInvalidInput))
}

func TestBoardService_GetBoards_Success(t *testing.T) {
	repositories := newTestRepositories()
	seedBoard(repositories.board, "b1", "d1")
	seedBoard(repositories.board, "b2", "d2")
	svc := NewBoardService(repositories.user, repositories.board, repositories.post, repositories.unitOfWork, newTestCache(), newTestCachePolicy(), newTestAuthorizationPolicy())

	list, err := svc.GetBoards(context.Background(), 10, 0)
	require.NoError(t, err)
	assert.Len(t, list.Boards, 2)
}

func TestBoardService_GetBoards_ExcludesHiddenBoards(t *testing.T) {
	repositories := newTestRepositories()
	adminID := seedUser(repositories.user, "admin", "pw", "admin")
	visibleBoardID := seedBoard(repositories.board, "visible", "d1")
	hiddenBoardID := seedBoard(repositories.board, "hidden", "d2")
	svc := NewBoardService(repositories.user, repositories.board, repositories.post, repositories.unitOfWork, newTestCache(), newTestCachePolicy(), newTestAuthorizationPolicy())

	require.NoError(t, svc.SetBoardVisibility(context.Background(), hiddenBoardID, adminID, true))

	list, err := svc.GetBoards(context.Background(), 10, 0)
	require.NoError(t, err)
	require.Len(t, list.Boards, 1)
	assert.Equal(t, visibleBoardID, list.Boards[0].ID)
}

func TestBoardService_GetBoards_InvalidLimit(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewBoardService(repositories.user, repositories.board, repositories.post, repositories.unitOfWork, newTestCache(), newTestCachePolicy(), newTestAuthorizationPolicy())

	_, err := svc.GetBoards(context.Background(), 0, 0)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customerror.ErrInvalidInput))

	_, err = svc.GetBoards(context.Background(), maxPageLimit+1, 0)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customerror.ErrInvalidInput))
}

func TestBoardService_GetBoards_HasMoreAndNextCursor(t *testing.T) {
	repositories := newTestRepositories()
	seedBoard(repositories.board, "b1", "d1")
	seedBoard(repositories.board, "b2", "d2")
	seedBoard(repositories.board, "b3", "d3")
	svc := NewBoardService(repositories.user, repositories.board, repositories.post, repositories.unitOfWork, newTestCache(), newTestCachePolicy(), newTestAuthorizationPolicy())

	list, err := svc.GetBoards(context.Background(), 2, 0)
	require.NoError(t, err)
	require.Len(t, list.Boards, 2)
	assert.True(t, list.HasMore)
	require.NotNil(t, list.NextLastID)
	assert.Equal(t, list.Boards[len(list.Boards)-1].ID, *list.NextLastID)
}

func TestBoardService_GetBoards_HasMoreRemainsCorrectWithHiddenBoardsAtTop(t *testing.T) {
	repositories := newTestRepositories()
	adminID := seedUser(repositories.user, "admin", "pw", "admin")
	_ = seedBoard(repositories.board, "v1", "d1")
	_ = seedBoard(repositories.board, "v2", "d2")
	visibleTopID := seedBoard(repositories.board, "v3", "d3")
	hidden1ID := seedBoard(repositories.board, "h1", "d4")
	hidden2ID := seedBoard(repositories.board, "h2", "d5")
	svc := NewBoardService(repositories.user, repositories.board, repositories.post, repositories.unitOfWork, newTestCache(), newTestCachePolicy(), newTestAuthorizationPolicy())

	require.NoError(t, svc.SetBoardVisibility(context.Background(), hidden1ID, adminID, true))
	require.NoError(t, svc.SetBoardVisibility(context.Background(), hidden2ID, adminID, true))

	list, err := svc.GetBoards(context.Background(), 2, 0)
	require.NoError(t, err)
	require.Len(t, list.Boards, 2)
	assert.Equal(t, visibleTopID, list.Boards[0].ID)
	assert.True(t, list.HasMore)
	require.NotNil(t, list.NextLastID)
	assert.Equal(t, list.Boards[len(list.Boards)-1].ID, *list.NextLastID)
}

func TestBoardService_UpdateDelete_SuccessForAdmin(t *testing.T) {
	repositories := newTestRepositories()
	adminID := seedUser(repositories.user, "admin", "pw", "admin")
	boardID := seedBoard(repositories.board, "free", "desc")
	svc := NewBoardService(repositories.user, repositories.board, repositories.post, repositories.unitOfWork, newTestCache(), newTestCachePolicy(), newTestAuthorizationPolicy())

	require.NoError(t, svc.UpdateBoard(context.Background(), boardID, adminID, "new", "new-desc"))
	require.NoError(t, svc.DeleteBoard(context.Background(), boardID, adminID))
}

func TestBoardService_DeleteBoard_RejectsNonEmptyBoard(t *testing.T) {
	repositories := newTestRepositories()
	adminID := seedUser(repositories.user, "admin", "pw", "admin")
	authorID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	seedPost(repositories.post, authorID, boardID, "title", "content")
	svc := NewBoardService(repositories.user, repositories.board, repositories.post, repositories.unitOfWork, newTestCache(), newTestCachePolicy(), newTestAuthorizationPolicy())

	err := svc.DeleteBoard(context.Background(), boardID, adminID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customerror.ErrBoardNotEmpty))
}

func TestBoardService_SetBoardVisibility_AdminOnly(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "user", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	svc := NewBoardService(repositories.user, repositories.board, repositories.post, repositories.unitOfWork, newTestCache(), newTestCachePolicy(), newTestAuthorizationPolicy())

	err := svc.SetBoardVisibility(context.Background(), boardID, userID, true)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customerror.ErrForbidden))
}

func TestBoardService_CreateBoard_InvalidatesBoardListCache(t *testing.T) {
	repositories := newTestRepositories()
	cache := testutil.NewSpyCache()
	svc := NewBoardServiceWithActionDispatcher(repositories.user, repositories.board, repositories.post, repositories.unitOfWork, cache, newTestActionDispatcher(t, repositories, cache), newTestCachePolicy(), newTestAuthorizationPolicy())

	adminID := seedUser(repositories.user, "admin", "pw", "admin")
	seedBoard(repositories.board, "b1", "d1")

	_, err := svc.GetBoards(context.Background(), 10, 0)
	require.NoError(t, err)
	_, ok, err := cache.Get(context.Background(), key.BoardList(10, 0))
	require.NoError(t, err)
	require.True(t, ok)

	_, err = svc.CreateBoard(context.Background(), adminID, "b2", "d2")
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		_, ok, err = cache.Get(context.Background(), key.BoardList(10, 0))
		require.NoError(t, err)
		return !ok
	}, time.Second, 10*time.Millisecond)
}

func TestBoardService_CreateBoard_Succeeds_WhenInvalidationFails(t *testing.T) {
	repositories := newTestRepositories()
	adminID := seedUser(repositories.user, "admin", "pw", "admin")
	svc := NewBoardService(repositories.user, repositories.board, repositories.post, repositories.unitOfWork, &errorCache{
		deleteByPrefixErr: newCacheFailure(nil),
	}, newTestCachePolicy(), newTestAuthorizationPolicy())

	boardID, err := svc.CreateBoard(context.Background(), adminID, "free", "desc")
	require.NoError(t, err)
	assert.NotZero(t, boardID)

	board, repoErr := repositories.board.SelectBoardByID(context.Background(), boardID)
	require.NoError(t, repoErr)
	assert.NotNil(t, board)
}

func TestBoardService_GetBoards_ReturnsCacheFailure_WhenCacheLoadFails(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewBoardService(repositories.user, repositories.board, repositories.post, repositories.unitOfWork, &errorCache{
		getOrSetWithTTLErr: newCacheFailure(nil),
	}, newTestCachePolicy(), newTestAuthorizationPolicy())

	_, err := svc.GetBoards(context.Background(), 10, 0)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customerror.ErrCacheFailure))
}
