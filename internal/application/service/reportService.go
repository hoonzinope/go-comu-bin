package service

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	appevent "github.com/hoonzinope/go-comu-bin/internal/application/event"
	"github.com/hoonzinope/go-comu-bin/internal/application/model"
	"github.com/hoonzinope/go-comu-bin/internal/application/policy"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

const maxReportReasonDetailLength = 1000
const maxReportResolutionNoteLength = 1000

var _ port.ReportUseCase = (*ReportService)(nil)

type ReportService struct {
	userRepository      port.UserRepository
	postRepository      port.PostRepository
	commentRepository   port.CommentRepository
	reportRepository    port.ReportRepository
	unitOfWork          port.UnitOfWork
	actionDispatcher    port.ActionHookDispatcher
	authorizationPolicy policy.AuthorizationPolicy
	logger              *slog.Logger
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
		userRepository:      userRepository,
		postRepository:      postRepository,
		commentRepository:   commentRepository,
		reportRepository:    reportRepository,
		unitOfWork:          unitOfWork,
		actionDispatcher:    resolveActionDispatcher(actionDispatcher),
		authorizationPolicy: authorizationPolicy,
		logger:              resolveLogger(logger),
	}
}

func (s *ReportService) CreateReport(ctx context.Context, reporterUserID int64, targetType entity.ReportTargetType, targetUUID string, reasonCode entity.ReportReasonCode, reasonDetail string) (int64, error) {
	targetUUID = strings.TrimSpace(targetUUID)
	if targetUUID == "" {
		return 0, customerror.ErrInvalidInput
	}
	if _, ok := entity.ParseReportTargetType(string(targetType)); !ok {
		return 0, customerror.ErrInvalidInput
	}
	if _, ok := entity.ParseReportReasonCode(string(reasonCode)); !ok {
		return 0, customerror.ErrInvalidInput
	}
	reasonDetail = strings.TrimSpace(reasonDetail)
	if len(reasonDetail) > maxReportReasonDetailLength {
		return 0, customerror.ErrInvalidInput
	}
	var reportID int64
	err := s.unitOfWork.WithinTransaction(ctx, func(tx port.TxScope) error {
		txCtx := tx.Context()
		user, err := tx.UserRepository().SelectUserByID(txCtx, reporterUserID)
		if err != nil {
			return customerror.WrapRepository("select user by id for create report", err)
		}
		if user == nil {
			return customerror.ErrUserNotFound
		}
		targetID, err := s.resolveVisibleReportTargetIDTx(tx, user, targetType, targetUUID)
		if err != nil {
			return err
		}
		report := entity.NewReport(targetType, targetID, reporterUserID, reasonCode, reasonDetail)
		reportID, err = tx.ReportRepository().Save(txCtx, report)
		if err != nil {
			if errors.Is(err, customerror.ErrReportAlreadyExists) {
				return customerror.ErrReportAlreadyExists
			}
			return customerror.WrapRepository("save report", err)
		}
		if err := dispatchDomainActions(tx, s.actionDispatcher, appevent.NewReportChanged("created", reportID, string(entity.ReportStatusPending))); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return reportID, nil
}

func (s *ReportService) resolveVisibleReportTargetIDTx(tx port.TxScope, user *entity.User, targetType entity.ReportTargetType, targetUUID string) (int64, error) {
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
		if _, err := ensurePostVisibleForUser(txCtx, tx.PostRepository(), tx.BoardRepository(), user, post.ID, customerror.ErrPostNotFound, "report target"); err != nil {
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
		if _, _, err := ensureCommentTargetVisibleForUser(txCtx, tx.CommentRepository(), tx.PostRepository(), tx.BoardRepository(), user, comment.ID, customerror.ErrCommentNotFound, "report target"); err != nil {
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

func (s *ReportService) GetReports(ctx context.Context, adminID int64, status *entity.ReportStatus, limit int, lastID int64) (*model.ReportList, error) {
	if err := requirePositiveLimit(limit); err != nil {
		return nil, err
	}
	if status != nil {
		if _, ok := entity.ParseReportStatus(string(*status)); !ok {
			return nil, customerror.ErrInvalidInput
		}
	}

	var out *model.ReportList
	err := s.unitOfWork.WithinTransaction(ctx, func(tx port.TxScope) error {
		txCtx := tx.Context()
		admin, err := tx.UserRepository().SelectUserByID(txCtx, adminID)
		if err != nil {
			return customerror.WrapRepository("select admin by id for get reports", err)
		}
		if admin == nil {
			return customerror.ErrUserNotFound
		}
		if err := s.authorizationPolicy.AdminOnly(admin); err != nil {
			return err
		}

		items, err := tx.ReportRepository().SelectList(txCtx, status, limit+1, lastID)
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
		views, err := s.reportsFromEntities(txCtx, items)
		if err != nil {
			return err
		}
		out = &model.ReportList{
			Reports:    views,
			Limit:      limit,
			LastID:     lastID,
			HasMore:    hasMore,
			NextLastID: nextLastID,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (s *ReportService) ResolveReport(ctx context.Context, adminID, reportID int64, status entity.ReportStatus, resolutionNote string) error {
	if reportID <= 0 {
		return customerror.ErrInvalidInput
	}
	if status != entity.ReportStatusAccepted && status != entity.ReportStatusRejected {
		return customerror.ErrInvalidInput
	}
	resolutionNote = strings.TrimSpace(resolutionNote)
	if len(resolutionNote) > maxReportResolutionNoteLength {
		return customerror.ErrInvalidInput
	}

	err := s.unitOfWork.WithinTransaction(ctx, func(tx port.TxScope) error {
		txCtx := tx.Context()
		admin, err := tx.UserRepository().SelectUserByID(txCtx, adminID)
		if err != nil {
			return customerror.WrapRepository("select admin by id for resolve report", err)
		}
		if admin == nil {
			return customerror.ErrUserNotFound
		}
		if err := s.authorizationPolicy.AdminOnly(admin); err != nil {
			return err
		}
		report, err := tx.ReportRepository().SelectByID(txCtx, reportID)
		if err != nil {
			return customerror.WrapRepository("select report by id for resolve report", err)
		}
		if report == nil {
			return customerror.ErrReportNotFound
		}
		if !report.Resolve(status, resolutionNote, adminID) {
			return customerror.ErrInvalidInput
		}
		if err := tx.ReportRepository().Update(txCtx, report); err != nil {
			return customerror.WrapRepository("update report for resolve", err)
		}
		if err := dispatchDomainActions(tx, s.actionDispatcher, appevent.NewReportChanged("resolved", report.ID, string(report.Status))); err != nil {
			return err
		}
		s.logger.Info("admin resolved report", "report_id", report.ID, "status", report.Status, "admin_id", adminID)
		return nil
	})
	return err
}

func (s *ReportService) reportsFromEntities(ctx context.Context, reports []*entity.Report) ([]model.Report, error) {
	if len(reports) == 0 {
		return []model.Report{}, nil
	}
	userIDs := make([]int64, 0, len(reports)*2)
	for _, report := range reports {
		userIDs = append(userIDs, report.ReporterUserID)
		if report.ResolvedBy != nil {
			userIDs = append(userIDs, *report.ResolvedBy)
		}
	}
	userUUIDs, err := userUUIDsByIDs(ctx, s.userRepository, userIDs)
	if err != nil {
		return nil, err
	}
	out := make([]model.Report, 0, len(reports))
	for _, report := range reports {
		reporterUUID, ok := userUUIDs[report.ReporterUserID]
		if !ok {
			return nil, customerror.WrapRepository("select users by ids including deleted", errors.New("reporter not found"))
		}
		view := model.Report{
			ID:             report.ID,
			TargetType:     string(report.TargetType),
			TargetID:       report.TargetID,
			ReporterUserID: report.ReporterUserID,
			ReporterUUID:   reporterUUID,
			ReasonCode:     string(report.ReasonCode),
			ReasonDetail:   report.ReasonDetail,
			Status:         string(report.Status),
			ResolutionNote: report.ResolutionNote,
			ResolvedBy:     report.ResolvedBy,
			ResolvedAt:     report.ResolvedAt,
			CreatedAt:      report.CreatedAt,
			UpdatedAt:      report.UpdatedAt,
		}
		if report.ResolvedBy != nil {
			resolvedByUUID := userUUIDs[*report.ResolvedBy]
			view.ResolvedByUUID = &resolvedByUUID
		}
		out = append(out, view)
	}
	return out, nil
}
