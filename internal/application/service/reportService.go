package service

import (
	"log/slog"

	"github.com/hoonzinope/go-comu-bin/internal/application/policy"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	reportsvc "github.com/hoonzinope/go-comu-bin/internal/application/service/report"
)

type ReportService = reportsvc.Service

func NewReportServiceWithActionDispatcher(
	userRepository port.UserRepository,
	postRepository port.PostRepository,
	commentRepository port.CommentRepository,
	reportRepository port.ReportRepository,
	unitOfWork port.UnitOfWork,
	actionDispatcher port.ActionHookDispatcher,
	authorizationPolicy policy.AuthorizationPolicy,
	logger ...*slog.Logger,
) *ReportService {
	return reportsvc.NewServiceWithActionDispatcher(userRepository, postRepository, commentRepository, reportRepository, unitOfWork, actionDispatcher, authorizationPolicy, logger...)
}
