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

type reportCommandHandler struct {
	userRepository      port.UserRepository
	postRepository      port.PostRepository
	commentRepository   port.CommentRepository
	reportRepository    port.ReportRepository
	unitOfWork          port.UnitOfWork
	actionDispatcher    port.ActionHookDispatcher
	authorizationPolicy policy.AuthorizationPolicy
	logger              *slog.Logger
}

func newReportCommandHandler(userRepository port.UserRepository, postRepository port.PostRepository, commentRepository port.CommentRepository, reportRepository port.ReportRepository, unitOfWork port.UnitOfWork, actionDispatcher port.ActionHookDispatcher, authorizationPolicy policy.AuthorizationPolicy, logger *slog.Logger) *reportCommandHandler {
	return &reportCommandHandler{userRepository: userRepository, postRepository: postRepository, commentRepository: commentRepository, reportRepository: reportRepository, unitOfWork: unitOfWork, actionDispatcher: actionDispatcher, authorizationPolicy: authorizationPolicy, logger: logger}
}

func (h *reportCommandHandler) CreateReport(ctx context.Context, reporterUserID int64, targetType model.ReportTargetType, targetUUID string, reasonCode model.ReportReasonCode, reasonDetail string) (int64, error) {
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
		if err := dispatchDomainActions(tx, h.actionDispatcher, appevent.NewReportChanged("created", reportID, string(entity.ReportStatusPending))); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return reportID, nil
}

func (h *reportCommandHandler) ResolveReport(ctx context.Context, adminID, reportID int64, status model.ReportStatus, resolutionNote string) error {
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
		admin, err := tx.UserRepository().SelectUserByID(txCtx, adminID)
		if err != nil {
			return customerror.WrapRepository("select admin by id for resolve report", err)
		}
		if admin == nil {
			return customerror.ErrUserNotFound
		}
		if err := h.authorizationPolicy.AdminOnly(admin); err != nil {
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
		if err := dispatchDomainActions(tx, h.actionDispatcher, appevent.NewReportChanged("resolved", report.ID, string(report.Status))); err != nil {
			return err
		}
		h.logger.Info("admin resolved report", "report_id", report.ID, "status", report.Status, "admin_id", adminID)
		return nil
	})
}

func (h *reportCommandHandler) resolveVisibleReportTargetIDTx(tx port.TxScope, user *entity.User, targetType entity.ReportTargetType, targetUUID string) (int64, error) {
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
