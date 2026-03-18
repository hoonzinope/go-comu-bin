package port

import "context"

import "github.com/hoonzinope/go-comu-bin/internal/application/model"

type ReportUseCase interface {
	CreateReport(ctx context.Context, reporterUserID int64, targetType model.ReportTargetType, targetUUID string, reasonCode model.ReportReasonCode, reasonDetail string) (int64, error)
	GetReports(ctx context.Context, adminID int64, status *model.ReportStatus, limit int, lastID int64) (*model.ReportList, error)
	ResolveReport(ctx context.Context, adminID, reportID int64, status model.ReportStatus, resolutionNote string) error
}
