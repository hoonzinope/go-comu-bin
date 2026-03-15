package service

import (
	"context"
	"errors"
	"testing"

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

	reportID, err := svc.CreateReport(context.Background(), reporterID, entity.ReportTargetPost, mustPostUUID(t, repositories.post, postID), entity.ReportReasonSpam, "spam")
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
	_, err := svc.CreateReport(context.Background(), reporterID, entity.ReportTargetPost, mustPostUUID(t, repositories.post, postID), entity.ReportReasonSpam, "spam")
	require.NoError(t, err)

	_, err = svc.CreateReport(context.Background(), reporterID, entity.ReportTargetPost, mustPostUUID(t, repositories.post, postID), entity.ReportReasonAbuse, "again")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customerror.ErrReportAlreadyExists))
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

	firstID, err := svc.CreateReport(context.Background(), reporter1, entity.ReportTargetPost, mustPostUUID(t, repositories.post, postID), entity.ReportReasonSpam, "first")
	require.NoError(t, err)
	secondID, err := svc.CreateReport(context.Background(), reporter2, entity.ReportTargetComment, mustCommentUUID(t, repositories.comment, commentID), entity.ReportReasonAbuse, "second")
	require.NoError(t, err)
	require.NoError(t, svc.ResolveReport(context.Background(), adminID, firstID, entity.ReportStatusAccepted, "ok"))

	list, err := svc.GetReports(context.Background(), adminID, nil, 10, 0)
	require.NoError(t, err)
	require.Len(t, list.Reports, 2)
	assert.Equal(t, secondID, list.Reports[0].ID)
	assert.Equal(t, firstID, list.Reports[1].ID)
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

	reportID, err := svc.CreateReport(context.Background(), reporterID, entity.ReportTargetPost, mustPostUUID(t, repositories.post, postID), entity.ReportReasonSpam, "detail")
	require.NoError(t, err)
	require.NoError(t, svc.ResolveReport(context.Background(), adminID, reportID, entity.ReportStatusRejected, "no"))

	resolvedStatus := entity.ReportStatusRejected
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

	reportID, err := svc.CreateReport(context.Background(), reporterID, entity.ReportTargetPost, mustPostUUID(t, repositories.post, postID), entity.ReportReasonSpam, "detail")
	require.NoError(t, err)
	require.NoError(t, svc.ResolveReport(context.Background(), adminID, reportID, entity.ReportStatusAccepted, "ok"))

	err = svc.ResolveReport(context.Background(), adminID, reportID, entity.ReportStatusRejected, "retry")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customerror.ErrInvalidInput))
}
