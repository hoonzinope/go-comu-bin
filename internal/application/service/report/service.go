package report

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	appevent "github.com/hoonzinope/go-comu-bin/internal/application/event"
	"github.com/hoonzinope/go-comu-bin/internal/application/model"
	"github.com/hoonzinope/go-comu-bin/internal/application/policy"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	svccommon "github.com/hoonzinope/go-comu-bin/internal/application/service/common"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

const maxReportReasonDetailLength = 1000
const maxReportResolutionNoteLength = 1000

var _ port.ReportUseCase = (*Service)(nil)

type Service struct {
	queryHandler   *QueryHandler
	commandHandler *CommandHandler
}

func NewServiceWithActionDispatcher(userRepository port.UserRepository, postRepository port.PostRepository, commentRepository port.CommentRepository, reportRepository port.ReportRepository, unitOfWork port.UnitOfWork, actionDispatcher port.ActionHookDispatcher, authorizationPolicy policy.AuthorizationPolicy, logger ...*slog.Logger) *Service {
	return &Service{
		queryHandler:   NewQueryHandler(userRepository, postRepository, commentRepository, reportRepository, unitOfWork, authorizationPolicy),
		commandHandler: NewCommandHandler(userRepository, postRepository, commentRepository, reportRepository, unitOfWork, svccommon.ResolveActionDispatcher(actionDispatcher), authorizationPolicy, svccommon.ResolveLogger(logger)),
	}
}

func (s *Service) CreateReport(ctx context.Context, reporterUserID int64, targetType model.ReportTargetType, targetUUID string, reasonCode model.ReportReasonCode, reasonDetail string) (int64, error) {
	return s.commandHandler.CreateReport(ctx, reporterUserID, targetType, targetUUID, reasonCode, reasonDetail)
}

func (s *Service) GetReports(ctx context.Context, adminID int64, status *model.ReportStatus, limit int, lastID int64) (*model.ReportList, error) {
	return s.queryHandler.GetReports(ctx, adminID, status, limit, lastID)
}

func (s *Service) ResolveReport(ctx context.Context, adminID, reportID int64, status model.ReportStatus, resolutionNote string) error {
	return s.commandHandler.ResolveReport(ctx, adminID, reportID, status, resolutionNote)
}

type QueryHandler struct {
	userRepository      port.UserRepository
	postRepository      port.PostRepository
	commentRepository   port.CommentRepository
	reportRepository    port.ReportRepository
	unitOfWork          port.UnitOfWork
	authorizationPolicy policy.AuthorizationPolicy
}

func NewQueryHandler(userRepository port.UserRepository, postRepository port.PostRepository, commentRepository port.CommentRepository, reportRepository port.ReportRepository, unitOfWork port.UnitOfWork, authorizationPolicy policy.AuthorizationPolicy) *QueryHandler {
	return &QueryHandler{userRepository: userRepository, postRepository: postRepository, commentRepository: commentRepository, reportRepository: reportRepository, unitOfWork: unitOfWork, authorizationPolicy: authorizationPolicy}
}

func (h *QueryHandler) GetReports(ctx context.Context, adminID int64, status *model.ReportStatus, limit int, lastID int64) (*model.ReportList, error) {
	if err := svccommon.RequirePositiveLimit(limit); err != nil {
		return nil, err
	}
	var entityStatus *entity.ReportStatus
	if status != nil {
		parsed, ok := status.ToEntity()
		if !ok {
			return nil, customerror.ErrInvalidInput
		}
		entityStatus = &parsed
	}
	var out *model.ReportList
	err := h.unitOfWork.WithinTransaction(ctx, func(tx port.TxScope) error {
		txCtx := tx.Context()
		if _, err := svccommon.RequireAdminUser(txCtx, tx.UserRepository(), h.authorizationPolicy, adminID, "get reports"); err != nil {
			return err
		}
		items, err := tx.ReportRepository().SelectList(txCtx, entityStatus, limit+1, lastID)
		if err != nil {
			return customerror.WrapRepository("select report list", err)
		}
		hasMore := false
		var nextLastID *int64
		if len(items) > limit {
			hasMore = true
			items = items[:limit]
		}
		if hasMore && len(items) > 0 {
			next := items[len(items)-1].ID
			nextLastID = &next
		}
		views, err := h.reportsFromEntities(txCtx, items)
		if err != nil {
			return err
		}
		out = &model.ReportList{Reports: views, Limit: limit, LastID: lastID, HasMore: hasMore, NextLastID: nextLastID}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (h *QueryHandler) reportsFromEntities(ctx context.Context, reports []*entity.Report) ([]model.Report, error) {
	if len(reports) == 0 {
		return []model.Report{}, nil
	}
	userIDs := make([]int64, 0, len(reports)*2)
	postIDs := make([]int64, 0, len(reports))
	commentIDs := make([]int64, 0, len(reports))
	for _, report := range reports {
		userIDs = append(userIDs, report.ReporterUserID)
		if report.ResolvedBy != nil {
			userIDs = append(userIDs, *report.ResolvedBy)
		}
		switch report.TargetType {
		case entity.ReportTargetPost:
			postIDs = append(postIDs, report.TargetID)
		case entity.ReportTargetComment:
			commentIDs = append(commentIDs, report.TargetID)
		}
	}
	userUUIDs, err := svccommon.UserUUIDsByIDs(ctx, h.userRepository, userIDs)
	if err != nil {
		return nil, err
	}
	postUUIDs, err := h.postRepository.SelectPostUUIDsByIDsIncludingDeleted(ctx, postIDs)
	if err != nil {
		return nil, customerror.WrapRepository("select post uuids by ids including deleted", err)
	}
	commentUUIDs, err := h.commentRepository.SelectCommentUUIDsByIDsIncludingDeleted(ctx, commentIDs)
	if err != nil {
		return nil, customerror.WrapRepository("select comment uuids by ids including deleted", err)
	}
	out := make([]model.Report, 0, len(reports))
	for _, report := range reports {
		reporterUUID, ok := userUUIDs[report.ReporterUserID]
		if !ok {
			return nil, customerror.WrapRepository("select users by ids including deleted", errors.New("reporter not found"))
		}
		targetUUID, err := reportTargetUUID(report, postUUIDs, commentUUIDs)
		if err != nil {
			return nil, err
		}
		view := model.Report{ID: report.ID, TargetType: string(report.TargetType), TargetUUID: targetUUID, ReporterUUID: reporterUUID, ReasonCode: string(report.ReasonCode), ReasonDetail: report.ReasonDetail, Status: string(report.Status), ResolutionNote: report.ResolutionNote, ResolvedAt: report.ResolvedAt, CreatedAt: report.CreatedAt, UpdatedAt: report.UpdatedAt}
		if report.ResolvedBy != nil {
			resolvedByUUID := userUUIDs[*report.ResolvedBy]
			view.ResolvedByUUID = &resolvedByUUID
		}
		out = append(out, view)
	}
	return out, nil
}

func reportTargetUUID(report *entity.Report, postUUIDs, commentUUIDs map[int64]string) (string, error) {
	switch report.TargetType {
	case entity.ReportTargetPost:
		targetUUID, ok := postUUIDs[report.TargetID]
		if !ok {
			return "", customerror.WrapRepository("select post uuids by ids including deleted", errors.New("report target post not found"))
		}
		return targetUUID, nil
	case entity.ReportTargetComment:
		targetUUID, ok := commentUUIDs[report.TargetID]
		if !ok {
			return "", customerror.WrapRepository("select comment uuids by ids including deleted", errors.New("report target comment not found"))
		}
		return targetUUID, nil
	default:
		return "", customerror.ErrInvalidInput
	}
}

type CommandHandler struct {
	unitOfWork          port.UnitOfWork
	actionDispatcher    port.ActionHookDispatcher
	authorizationPolicy policy.AuthorizationPolicy
	logger              *slog.Logger
}

func NewCommandHandler(_ port.UserRepository, _ port.PostRepository, _ port.CommentRepository, _ port.ReportRepository, unitOfWork port.UnitOfWork, actionDispatcher port.ActionHookDispatcher, authorizationPolicy policy.AuthorizationPolicy, logger *slog.Logger) *CommandHandler {
	return &CommandHandler{unitOfWork: unitOfWork, actionDispatcher: actionDispatcher, authorizationPolicy: authorizationPolicy, logger: logger}
}

func (h *CommandHandler) CreateReport(ctx context.Context, reporterUserID int64, targetType model.ReportTargetType, targetUUID string, reasonCode model.ReportReasonCode, reasonDetail string) (int64, error) {
	targetUUID = strings.TrimSpace(targetUUID)
	if targetUUID == "" {
		return 0, customerror.ErrInvalidInput
	}
	entityTargetType, ok := targetType.ToEntity()
	if !ok {
		return 0, customerror.ErrInvalidInput
	}
	entityReasonCode, ok := reasonCode.ToEntity()
	if !ok {
		return 0, customerror.ErrInvalidInput
	}
	reasonDetail = strings.TrimSpace(reasonDetail)
	if len(reasonDetail) > maxReportReasonDetailLength {
		return 0, customerror.ErrInvalidInput
	}
	var reportID int64
	err := h.unitOfWork.WithinTransaction(ctx, func(tx port.TxScope) error {
		txCtx := tx.Context()
		user, err := tx.UserRepository().SelectUserByID(txCtx, reporterUserID)
		if err != nil {
			return customerror.WrapRepository("select user by id for create report", err)
		}
		if user == nil {
			return customerror.ErrUserNotFound
		}
		if err := policy.ForbidGuest(user); err != nil {
			return err
		}
		if err := h.authorizationPolicy.CanWrite(user); err != nil {
			return err
		}
		targetID, err := h.resolveVisibleReportTargetIDTx(tx, user, entityTargetType, targetUUID)
		if err != nil {
			return err
		}
		report := entity.NewReport(entityTargetType, targetID, reporterUserID, entityReasonCode, reasonDetail)
		reportID, err = tx.ReportRepository().Save(txCtx, report)
		if err != nil {
			if errors.Is(err, customerror.ErrReportAlreadyExists) {
				return customerror.ErrReportAlreadyExists
			}
			return customerror.WrapRepository("save report", err)
		}
		if err := svccommon.DispatchDomainActions(tx, h.actionDispatcher, appevent.NewReportChanged("created", reportID, string(entity.ReportStatusPending))); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return reportID, nil
}

func (h *CommandHandler) ResolveReport(ctx context.Context, adminID, reportID int64, status model.ReportStatus, resolutionNote string) error {
	if reportID <= 0 {
		return customerror.ErrInvalidInput
	}
	entityStatus, ok := status.ToEntity()
	if !ok {
		return customerror.ErrInvalidInput
	}
	if entityStatus != entity.ReportStatusAccepted && entityStatus != entity.ReportStatusRejected {
		return customerror.ErrInvalidInput
	}
	resolutionNote = strings.TrimSpace(resolutionNote)
	if len(resolutionNote) > maxReportResolutionNoteLength {
		return customerror.ErrInvalidInput
	}
	return h.unitOfWork.WithinTransaction(ctx, func(tx port.TxScope) error {
		txCtx := tx.Context()
		if _, err := svccommon.RequireAdminUser(txCtx, tx.UserRepository(), h.authorizationPolicy, adminID, "resolve report"); err != nil {
			return err
		}
		report, err := tx.ReportRepository().SelectByID(txCtx, reportID)
		if err != nil {
			return customerror.WrapRepository("select report by id for resolve report", err)
		}
		if report == nil {
			return customerror.ErrReportNotFound
		}
		if !report.Resolve(entityStatus, resolutionNote, adminID) {
			return customerror.ErrInvalidInput
		}
		if err := tx.ReportRepository().Update(txCtx, report); err != nil {
			return customerror.WrapRepository("update report for resolve", err)
		}
		if err := svccommon.DispatchDomainActions(tx, h.actionDispatcher, appevent.NewReportChanged("resolved", report.ID, string(report.Status))); err != nil {
			return err
		}
		h.logger.Info("admin resolved report", "report_id", report.ID, "status", report.Status, "admin_id", adminID)
		return nil
	})
}

func (h *CommandHandler) resolveVisibleReportTargetIDTx(tx port.TxScope, user *entity.User, targetType entity.ReportTargetType, targetUUID string) (int64, error) {
	txCtx := tx.Context()
	switch targetType {
	case entity.ReportTargetPost:
		post, err := tx.PostRepository().SelectPostByUUID(txCtx, targetUUID)
		if err != nil {
			return 0, customerror.WrapRepository("select post by uuid for report target", err)
		}
		if post == nil {
			return 0, customerror.ErrPostNotFound
		}
		if _, err := policy.EnsurePostVisibleForUser(txCtx, tx.PostRepository(), tx.BoardRepository(), user, post.ID, customerror.ErrPostNotFound, "report target"); err != nil {
			if errors.Is(err, customerror.ErrPostNotFound) {
				return 0, customerror.ErrPostNotFound
			}
			return 0, err
		}
		return post.ID, nil
	case entity.ReportTargetComment:
		comment, err := tx.CommentRepository().SelectCommentByUUID(txCtx, targetUUID)
		if err != nil {
			return 0, customerror.WrapRepository("select comment by uuid for report target", err)
		}
		if comment == nil {
			return 0, customerror.ErrCommentNotFound
		}
		if _, _, err := policy.EnsureCommentTargetVisibleForUser(txCtx, tx.CommentRepository(), tx.PostRepository(), tx.BoardRepository(), user, comment.ID, customerror.ErrCommentNotFound, "report target"); err != nil {
			if errors.Is(err, customerror.ErrCommentNotFound) {
				return 0, customerror.ErrCommentNotFound
			}
			return 0, err
		}
		return comment.ID, nil
	default:
		return 0, customerror.ErrInvalidInput
	}
}
