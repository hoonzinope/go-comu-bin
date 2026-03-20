package service

import (
	"context"
	"log/slog"

	"github.com/hoonzinope/go-comu-bin/internal/application/model"
	"github.com/hoonzinope/go-comu-bin/internal/application/policy"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
)

var _ port.ReportUseCase = (*ReportService)(nil)

type ReportService struct {
	queryHandler   *reportQueryHandler
	commandHandler *reportCommandHandler
}

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
	return &ReportService{
		queryHandler:   newReportQueryHandler(userRepository, postRepository, commentRepository, reportRepository, unitOfWork, authorizationPolicy),
		commandHandler: newReportCommandHandler(userRepository, postRepository, commentRepository, reportRepository, unitOfWork, resolveActionDispatcher(actionDispatcher), authorizationPolicy, resolveLogger(logger)),
	}
}

func (s *ReportService) CreateReport(ctx context.Context, reporterUserID int64, targetType model.ReportTargetType, targetUUID string, reasonCode model.ReportReasonCode, reasonDetail string) (int64, error) {
	return s.commandHandler.CreateReport(ctx, reporterUserID, targetType, targetUUID, reasonCode, reasonDetail)
}

func (s *ReportService) GetReports(ctx context.Context, adminID int64, status *model.ReportStatus, limit int, lastID int64) (*model.ReportList, error) {
	return s.queryHandler.GetReports(ctx, adminID, status, limit, lastID)
}

func (s *ReportService) ResolveReport(ctx context.Context, adminID, reportID int64, status model.ReportStatus, resolutionNote string) error {
	return s.commandHandler.ResolveReport(ctx, adminID, reportID, status, resolutionNote)
}
