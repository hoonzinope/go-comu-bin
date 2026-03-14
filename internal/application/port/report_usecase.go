package port

import "context"

import "github.com/hoonzinope/go-comu-bin/internal/application/model"
import "github.com/hoonzinope/go-comu-bin/internal/domain/entity"

type ReportUseCase interface {
	CreateReport(ctx context.Context, reporterUserID int64, targetType entity.ReportTargetType, targetID int64, reasonCode entity.ReportReasonCode, reasonDetail string) (int64, error)
	GetReports(ctx context.Context, adminID int64, status *entity.ReportStatus, limit int, lastID int64) (*model.ReportList, error)
	ResolveReport(ctx context.Context, adminID, reportID int64, status entity.ReportStatus, resolutionNote string) error
}

