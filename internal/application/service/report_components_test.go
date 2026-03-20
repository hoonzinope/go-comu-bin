package service

import (
	"context"
	"testing"

	"github.com/hoonzinope/go-comu-bin/internal/application/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReportQueryHandler_GetReports(t *testing.T) {
	repositories := newTestRepositories()
	adminID := seedUser(repositories.user, "admin", "pw", "admin")
	reporterID := seedUser(repositories.user, "reporter", "pw", "user")
	authorID := seedUser(repositories.user, "author", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, authorID, boardID, "title", "content")
	svc := NewReportServiceWithActionDispatcher(repositories.user, repositories.post, repositories.comment, repositories.report, repositories.unitOfWork, resolveActionDispatcher(nil), newTestAuthorizationPolicy())
	_, err := svc.CreateReport(context.Background(), reporterID, model.ReportTargetPost, mustPostUUID(t, repositories.post, postID), model.ReportReasonSpam, "spam")
	require.NoError(t, err)

	handler := newReportQueryHandler(repositories.user, repositories.post, repositories.comment, repositories.report, repositories.unitOfWork, newTestAuthorizationPolicy())
	list, err := handler.GetReports(context.Background(), adminID, nil, 10, 0)
	require.NoError(t, err)
	require.Len(t, list.Reports, 1)
	assert.Equal(t, "spam", list.Reports[0].ReasonCode)
}

func TestReportCommandHandler_ResolveReport(t *testing.T) {
	repositories := newTestRepositories()
	adminID := seedUser(repositories.user, "admin", "pw", "admin")
	reporterID := seedUser(repositories.user, "reporter", "pw", "user")
	authorID := seedUser(repositories.user, "author", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, authorID, boardID, "title", "content")

	handler := newReportCommandHandler(repositories.user, repositories.post, repositories.comment, repositories.report, repositories.unitOfWork, resolveActionDispatcher(nil), newTestAuthorizationPolicy(), resolveLogger(nil))
	reportID, err := handler.CreateReport(context.Background(), reporterID, model.ReportTargetPost, mustPostUUID(t, repositories.post, postID), model.ReportReasonSpam, "detail")
	require.NoError(t, err)
	require.NoError(t, handler.ResolveReport(context.Background(), adminID, reportID, model.ReportStatusAccepted, "ok"))
}
