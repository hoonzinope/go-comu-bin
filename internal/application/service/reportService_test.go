package service

import (
	"context"
	"errors"
	"testing"

	"github.com/hoonzinope/go-comu-bin/internal/application/model"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReportService_CreateReport_Success(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewReportServiceWithActionDispatcher(
		repositories.user,
		repositories.post,
		repositories.comment,
		repositories.report,
		repositories.unitOfWork,
		newTestActionDispatcher(t, repositories, newTestCache()),
		newTestAuthorizationPolicy(),
	)

	reporterID := seedUser(repositories.user, "reporter", "pw", "user")
	authorID := seedUser(repositories.user, "author", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, authorID, boardID, "title", "content")

	reportID, err := svc.CreateReport(context.Background(), reporterID, model.ReportTargetPost, mustPostUUID(t, repositories.post, postID), model.ReportReasonSpam, "spam")
	require.NoError(t, err)
	assert.NotZero(t, reportID)
}

func TestReportService_CreateReport_RejectsDuplicate(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewReportServiceWithActionDispatcher(
		repositories.user,
		repositories.post,
		repositories.comment,
		repositories.report,
		repositories.unitOfWork,
		newTestActionDispatcher(t, repositories, newTestCache()),
		newTestAuthorizationPolicy(),
	)

	reporterID := seedUser(repositories.user, "reporter", "pw", "user")
	authorID := seedUser(repositories.user, "author", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, authorID, boardID, "title", "content")
	_, err := svc.CreateReport(context.Background(), reporterID, model.ReportTargetPost, mustPostUUID(t, repositories.post, postID), model.ReportReasonSpam, "spam")
	require.NoError(t, err)

	_, err = svc.CreateReport(context.Background(), reporterID, model.ReportTargetPost, mustPostUUID(t, repositories.post, postID), model.ReportReasonAbuse, "again")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customerror.ErrReportAlreadyExists))
}

func TestReportService_CreateReport_HiddenBoardBlockedForNonAdmin(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewReportServiceWithActionDispatcher(
		repositories.user,
		repositories.post,
		repositories.comment,
		repositories.report,
		repositories.unitOfWork,
		newTestActionDispatcher(t, repositories, newTestCache()),
		newTestAuthorizationPolicy(),
	)

	reporterID := seedUser(repositories.user, "reporter", "pw", "user")
	authorID := seedUser(repositories.user, "author", "pw", "user")
	boardID := seedBoard(repositories.board, "hidden", "desc")
	postID := seedPost(repositories.post, authorID, boardID, "title", "content")
	board, err := repositories.board.SelectBoardByID(context.Background(), boardID)
	require.NoError(t, err)
	require.NotNil(t, board)
	board.SetHidden(true)
	require.NoError(t, repositories.board.Update(context.Background(), board))

	_, err = svc.CreateReport(context.Background(), reporterID, model.ReportTargetPost, mustPostUUID(t, repositories.post, postID), model.ReportReasonSpam, "spam")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customerror.ErrPostNotFound))
}

func TestReportService_CreateReport_BlockedForGuestUser(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewReportServiceWithActionDispatcher(
		repositories.user,
		repositories.post,
		repositories.comment,
		repositories.report,
		repositories.unitOfWork,
		newTestActionDispatcher(t, repositories, newTestCache()),
		newTestAuthorizationPolicy(),
	)

	guest := entity.NewGuest("guest-1", "guest-1@example.invalid", "pw")
	reporterID, err := repositories.user.Save(context.Background(), guest)
	require.NoError(t, err)
	authorID := seedUser(repositories.user, "author", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, authorID, boardID, "title", "content")

	_, err = svc.CreateReport(context.Background(), reporterID, model.ReportTargetPost, mustPostUUID(t, repositories.post, postID), model.ReportReasonSpam, "spam")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customerror.ErrForbidden))
}

func TestReportService_GetReports_AdminOnly(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewReportServiceWithActionDispatcher(
		repositories.user,
		repositories.post,
		repositories.comment,
		repositories.report,
		repositories.unitOfWork,
		newTestActionDispatcher(t, repositories, newTestCache()),
		newTestAuthorizationPolicy(),
	)

	userID := seedUser(repositories.user, "user", "pw", "user")
	_, err := svc.GetReports(context.Background(), userID, nil, 10, 0)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customerror.ErrForbidden))
}

func TestReportService_GetReports_PendingFirst(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewReportServiceWithActionDispatcher(
		repositories.user,
		repositories.post,
		repositories.comment,
		repositories.report,
		repositories.unitOfWork,
		newTestActionDispatcher(t, repositories, newTestCache()),
		newTestAuthorizationPolicy(),
	)

	adminID := seedUser(repositories.user, "admin", "pw", "admin")
	reporter1 := seedUser(repositories.user, "r1", "pw", "user")
	reporter2 := seedUser(repositories.user, "r2", "pw", "user")
	authorID := seedUser(repositories.user, "author", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, authorID, boardID, "title", "content")
	commentID := seedComment(repositories.comment, authorID, postID, "reply")

	firstID, err := svc.CreateReport(context.Background(), reporter1, model.ReportTargetPost, mustPostUUID(t, repositories.post, postID), model.ReportReasonSpam, "first")
	require.NoError(t, err)
	secondID, err := svc.CreateReport(context.Background(), reporter2, model.ReportTargetComment, mustCommentUUID(t, repositories.comment, commentID), model.ReportReasonAbuse, "second")
	require.NoError(t, err)
	require.NoError(t, svc.ResolveReport(context.Background(), adminID, firstID, model.ReportStatusAccepted, "ok"))

	list, err := svc.GetReports(context.Background(), adminID, nil, 10, 0)
	require.NoError(t, err)
	require.Len(t, list.Reports, 2)
	assert.Equal(t, secondID, list.Reports[0].ID)
	assert.Equal(t, firstID, list.Reports[1].ID)
}

func TestReportService_GetReports_IncludesDeletedPostTargetUUID(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewReportServiceWithActionDispatcher(
		repositories.user,
		repositories.post,
		repositories.comment,
		repositories.report,
		repositories.unitOfWork,
		newTestActionDispatcher(t, repositories, newTestCache()),
		newTestAuthorizationPolicy(),
	)

	adminID := seedUser(repositories.user, "admin", "pw", "admin")
	reporterID := seedUser(repositories.user, "reporter", "pw", "user")
	authorID := seedUser(repositories.user, "author", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, authorID, boardID, "title", "content")
	postUUID := mustPostUUID(t, repositories.post, postID)

	_, err := svc.CreateReport(context.Background(), reporterID, model.ReportTargetPost, postUUID, model.ReportReasonSpam, "detail")
	require.NoError(t, err)
	require.NoError(t, repositories.post.Delete(context.Background(), postID))

	list, err := svc.GetReports(context.Background(), adminID, nil, 10, 0)
	require.NoError(t, err)
	require.Len(t, list.Reports, 1)
	assert.Equal(t, postUUID, list.Reports[0].TargetUUID)
}

func TestReportService_ResolveReport_Success(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewReportServiceWithActionDispatcher(
		repositories.user,
		repositories.post,
		repositories.comment,
		repositories.report,
		repositories.unitOfWork,
		newTestActionDispatcher(t, repositories, newTestCache()),
		newTestAuthorizationPolicy(),
	)

	adminID := seedUser(repositories.user, "admin", "pw", "admin")
	reporterID := seedUser(repositories.user, "reporter", "pw", "user")
	authorID := seedUser(repositories.user, "author", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, authorID, boardID, "title", "content")

	reportID, err := svc.CreateReport(context.Background(), reporterID, model.ReportTargetPost, mustPostUUID(t, repositories.post, postID), model.ReportReasonSpam, "detail")
	require.NoError(t, err)
	require.NoError(t, svc.ResolveReport(context.Background(), adminID, reportID, model.ReportStatusRejected, "no"))

	resolvedStatus := model.ReportStatusRejected
	list, err := svc.GetReports(context.Background(), adminID, &resolvedStatus, 10, 0)
	require.NoError(t, err)
	require.Len(t, list.Reports, 1)
	assert.Equal(t, "rejected", list.Reports[0].Status)
}

func TestReportService_ResolveReport_RejectsAlreadyResolved(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewReportServiceWithActionDispatcher(
		repositories.user,
		repositories.post,
		repositories.comment,
		repositories.report,
		repositories.unitOfWork,
		newTestActionDispatcher(t, repositories, newTestCache()),
		newTestAuthorizationPolicy(),
	)

	adminID := seedUser(repositories.user, "admin", "pw", "admin")
	reporterID := seedUser(repositories.user, "reporter", "pw", "user")
	authorID := seedUser(repositories.user, "author", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, authorID, boardID, "title", "content")

	reportID, err := svc.CreateReport(context.Background(), reporterID, model.ReportTargetPost, mustPostUUID(t, repositories.post, postID), model.ReportReasonSpam, "detail")
	require.NoError(t, err)
	require.NoError(t, svc.ResolveReport(context.Background(), adminID, reportID, model.ReportStatusAccepted, "ok"))

	err = svc.ResolveReport(context.Background(), adminID, reportID, model.ReportStatusRejected, "retry")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customerror.ErrInvalidInput))
}
