package service

import (
	"errors"
	"testing"

	"github.com/hoonzinope/go-comu-bin/internal/application/cache/key"
	"github.com/hoonzinope/go-comu-bin/internal/application/cache/testutil"
	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBoardService_CreateBoard_ForbiddenForNonAdmin(t *testing.T) {
	repository := newTestRepository()
	userID := seedUser(repository, "user", "pw", "user")
	svc := NewBoardService(repository, newTestCache(), newTestCachePolicy())

	_, err := svc.CreateBoard(userID, "free", "desc")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrForbidden))
}

func TestBoardService_CreateBoard_SuccessForAdmin(t *testing.T) {
	repository := newTestRepository()
	adminID := seedUser(repository, "admin", "pw", "admin")
	svc := NewBoardService(repository, newTestCache(), newTestCachePolicy())

	boardID, err := svc.CreateBoard(adminID, "free", "desc")
	require.NoError(t, err)
	assert.NotZero(t, boardID)
}

func TestBoardService_GetBoards_Success(t *testing.T) {
	repository := newTestRepository()
	seedBoard(repository, "b1", "d1")
	seedBoard(repository, "b2", "d2")
	svc := NewBoardService(repository, newTestCache(), newTestCachePolicy())

	list, err := svc.GetBoards(10, 0)
	require.NoError(t, err)
	assert.Len(t, list.Boards, 2)
}

func TestBoardService_GetBoards_HasMoreAndNextCursor(t *testing.T) {
	repository := newTestRepository()
	seedBoard(repository, "b1", "d1")
	seedBoard(repository, "b2", "d2")
	seedBoard(repository, "b3", "d3")
	svc := NewBoardService(repository, newTestCache(), newTestCachePolicy())

	list, err := svc.GetBoards(2, 0)
	require.NoError(t, err)
	require.Len(t, list.Boards, 2)
	assert.True(t, list.HasMore)
	require.NotNil(t, list.NextLastID)
	assert.Equal(t, list.Boards[len(list.Boards)-1].ID, *list.NextLastID)
}

func TestBoardService_UpdateDelete_SuccessForAdmin(t *testing.T) {
	repository := newTestRepository()
	adminID := seedUser(repository, "admin", "pw", "admin")
	boardID := seedBoard(repository, "free", "desc")
	svc := NewBoardService(repository, newTestCache(), newTestCachePolicy())

	require.NoError(t, svc.UpdateBoard(boardID, adminID, "new", "new-desc"))
	require.NoError(t, svc.DeleteBoard(boardID, adminID))
}

func TestBoardService_CreateBoard_InvalidatesBoardListCache(t *testing.T) {
	repo := newTestRepository()
	cache := testutil.NewSpyCache()
	svc := NewBoardService(repo, cache, newTestCachePolicy())

	adminID := seedUser(repo, "admin", "pw", "admin")
	seedBoard(repo, "b1", "d1")

	_, err := svc.GetBoards(10, 0)
	require.NoError(t, err)
	_, ok := cache.Get(key.BoardList(10, 0))
	require.True(t, ok)

	_, err = svc.CreateBoard(adminID, "b2", "d2")
	require.NoError(t, err)

	_, ok = cache.Get(key.BoardList(10, 0))
	assert.False(t, ok)
}
