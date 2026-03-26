package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var _ port.ReportRepository = (*ReportRepository)(nil)

type ReportRepository struct {
	exec sqlExecutor
}

func NewReportRepository(exec sqlExecutor) *ReportRepository {
	return &ReportRepository{exec: exec}
}

func (r *ReportRepository) Save(ctx context.Context, report *entity.Report) (int64, error) {
	if r == nil || r.exec == nil {
		return 0, errors.New("sqlite report repository is not initialized")
	}
	existing, err := r.SelectByReporterAndTarget(ctx, report.ReporterUserID, report.TargetType, report.TargetID)
	if err != nil {
		return 0, err
	}
	if existing != nil {
		return 0, customerror.ErrReportAlreadyExists
	}
	res, err := r.exec.ExecContext(ctx, `
INSERT INTO reports (
    target_type, target_id, reporter_user_id, reason_code, reason_detail, status, resolution_note, resolved_by, resolved_at, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`,
		report.TargetType,
		report.TargetID,
		report.ReporterUserID,
		report.ReasonCode,
		report.ReasonDetail,
		report.Status,
		report.ResolutionNote,
		nullableInt64(report.ResolvedBy),
		timePtrToUnixNano(report.ResolvedAt),
		report.CreatedAt.UnixNano(),
		report.UpdatedAt.UnixNano(),
	)
	if err != nil {
		return 0, fmt.Errorf("save report: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("last insert id for report: %w", err)
	}
	report.ID = id
	return id, nil
}

func (r *ReportRepository) SelectByID(ctx context.Context, id int64) (*entity.Report, error) {
	return r.selectReport(ctx, `
SELECT id, target_type, target_id, reporter_user_id, reason_code, reason_detail, status, resolution_note, resolved_by, resolved_at, created_at, updated_at
FROM reports
WHERE id = ?
LIMIT 1
`, id)
}

func (r *ReportRepository) SelectByReporterAndTarget(ctx context.Context, reporterUserID int64, targetType entity.ReportTargetType, targetID int64) (*entity.Report, error) {
	return r.selectReport(ctx, `
SELECT id, target_type, target_id, reporter_user_id, reason_code, reason_detail, status, resolution_note, resolved_by, resolved_at, created_at, updated_at
FROM reports
WHERE reporter_user_id = ? AND target_type = ? AND target_id = ?
LIMIT 1
`, reporterUserID, targetType, targetID)
}

func (r *ReportRepository) SelectList(ctx context.Context, status *entity.ReportStatus, limit int, lastID int64) ([]*entity.Report, error) {
	items, err := r.selectReports(ctx, `
SELECT id, target_type, target_id, reporter_user_id, reason_code, reason_detail, status, resolution_note, resolved_by, resolved_at, created_at, updated_at
FROM reports
`)
	if err != nil {
		return nil, err
	}
	filtered := make([]*entity.Report, 0, len(items))
	for _, item := range items {
		if status != nil && item.Status != *status {
			continue
		}
		filtered = append(filtered, item)
	}
	sort.Slice(filtered, func(i, j int) bool {
		leftPending := filtered[i].Status == entity.ReportStatusPending
		rightPending := filtered[j].Status == entity.ReportStatusPending
		if leftPending != rightPending {
			return leftPending
		}
		return filtered[i].ID > filtered[j].ID
	})
	start := 0
	if lastID > 0 {
		start = len(filtered)
		for idx, item := range filtered {
			if item.ID == lastID {
				start = idx + 1
				break
			}
		}
	}
	if start >= len(filtered) {
		return []*entity.Report{}, nil
	}
	end := start + limit
	if limit <= 0 {
		return []*entity.Report{}, nil
	}
	if end > len(filtered) {
		end = len(filtered)
	}
	return filtered[start:end], nil
}

func (r *ReportRepository) ExistsByReporterUserID(ctx context.Context, reporterUserID int64) (bool, error) {
	if r == nil || r.exec == nil {
		return false, errors.New("sqlite report repository is not initialized")
	}
	row := r.exec.QueryRowContext(ctx, `SELECT 1 FROM reports WHERE reporter_user_id = ? LIMIT 1`, reporterUserID)
	var found int
	if err := row.Scan(&found); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("exists report by reporter: %w", err)
	}
	return true, nil
}

func (r *ReportRepository) Update(ctx context.Context, report *entity.Report) error {
	if r == nil || r.exec == nil {
		return errors.New("sqlite report repository is not initialized")
	}
	_, err := r.exec.ExecContext(ctx, `
UPDATE reports SET
    target_type = ?,
    target_id = ?,
    reporter_user_id = ?,
    reason_code = ?,
    reason_detail = ?,
    status = ?,
    resolution_note = ?,
    resolved_by = ?,
    resolved_at = ?,
    created_at = ?,
    updated_at = ?
WHERE id = ?
`,
		report.TargetType,
		report.TargetID,
		report.ReporterUserID,
		report.ReasonCode,
		report.ReasonDetail,
		report.Status,
		report.ResolutionNote,
		nullableInt64(report.ResolvedBy),
		timePtrToUnixNano(report.ResolvedAt),
		report.CreatedAt.UnixNano(),
		report.UpdatedAt.UnixNano(),
		report.ID,
	)
	if err != nil {
		return fmt.Errorf("update report: %w", err)
	}
	return nil
}

func (r *ReportRepository) selectReport(ctx context.Context, query string, args ...any) (*entity.Report, error) {
	items, err := r.selectReports(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, nil
	}
	return items[0], nil
}

func (r *ReportRepository) selectReports(ctx context.Context, query string, args ...any) ([]*entity.Report, error) {
	if r == nil || r.exec == nil {
		return nil, errors.New("sqlite report repository is not initialized")
	}
	rows, err := r.exec.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("select reports: %w", err)
	}
	defer rows.Close()
	items := make([]*entity.Report, 0)
	for rows.Next() {
		item, scanErr := scanReport(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("select reports: %w", scanErr)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("select reports: %w", err)
	}
	return items, nil
}

func reportTargetTypeString(value entity.ReportTargetType) string {
	return strings.TrimSpace(string(value))
}
