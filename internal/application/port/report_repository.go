package port

import (
	"context"

	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

type ReportRepository interface {
	Save(ctx context.Context, report *entity.Report) (int64, error)
	SelectByID(ctx context.Context, id int64) (*entity.Report, error)
	SelectByReporterAndTarget(ctx context.Context, reporterUserID int64, targetType entity.ReportTargetType, targetID int64) (*entity.Report, error)
	SelectList(ctx context.Context, status *entity.ReportStatus, limit int, lastID int64) ([]*entity.Report, error)
	Update(ctx context.Context, report *entity.Report) error
}

