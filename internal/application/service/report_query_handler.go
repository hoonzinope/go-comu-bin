package service

import (
	"context"
	"errors"

	"github.com/hoonzinope/go-comu-bin/internal/application/model"
	"github.com/hoonzinope/go-comu-bin/internal/application/policy"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

type reportQueryHandler struct {
	userRepository      port.UserRepository
	postRepository      port.PostRepository
	commentRepository   port.CommentRepository
	reportRepository    port.ReportRepository
	unitOfWork          port.UnitOfWork
	authorizationPolicy policy.AuthorizationPolicy
}

func newReportQueryHandler(userRepository port.UserRepository, postRepository port.PostRepository, commentRepository port.CommentRepository, reportRepository port.ReportRepository, unitOfWork port.UnitOfWork, authorizationPolicy policy.AuthorizationPolicy) *reportQueryHandler {
	return &reportQueryHandler{userRepository: userRepository, postRepository: postRepository, commentRepository: commentRepository, reportRepository: reportRepository, unitOfWork: unitOfWork, authorizationPolicy: authorizationPolicy}
}

func (h *reportQueryHandler) GetReports(ctx context.Context, adminID int64, status *model.ReportStatus, limit int, lastID int64) (*model.ReportList, error) {
	if err := requirePositiveLimit(limit); err != nil {
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
		admin, err := tx.UserRepository().SelectUserByID(txCtx, adminID)
		if err != nil {
			return customerror.WrapRepository("select admin by id for get reports", err)
		}
		if admin == nil {
			return customerror.ErrUserNotFound
		}
		if err := h.authorizationPolicy.AdminOnly(admin); err != nil {
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

func (h *reportQueryHandler) reportsFromEntities(ctx context.Context, reports []*entity.Report) ([]model.Report, error) {
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
	userUUIDs, err := userUUIDsByIDs(ctx, h.userRepository, userIDs)
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
