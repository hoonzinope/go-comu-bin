package inmemory

import (
	"context"
	"sort"
	"sync"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var _ port.ReportRepository = (*ReportRepository)(nil)

type ReportRepository struct {
	mu          sync.RWMutex
	coordinator *txCoordinator
	reportDB    struct {
		ID   int64
		Data map[int64]*entity.Report
	}
}

type reportRepositoryState struct {
	ID   int64
	Data map[int64]*entity.Report
}

func NewReportRepository() *ReportRepository {
	return &ReportRepository{
		coordinator: newTxCoordinator(),
		reportDB: struct {
			ID   int64
			Data map[int64]*entity.Report
		}{
			Data: make(map[int64]*entity.Report),
		},
	}
}

func (r *ReportRepository) attachCoordinator(coordinator *txCoordinator) {
	r.coordinator = coordinator
}

func (r *ReportRepository) Save(ctx context.Context, report *entity.Report) (int64, error) {
	_ = ctx
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.save(report)
}

func (r *ReportRepository) save(report *entity.Report) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, existing := range r.reportDB.Data {
		if existing.ReporterUserID == report.ReporterUserID && existing.TargetType == report.TargetType && existing.TargetID == report.TargetID {
			return 0, customerror.ErrReportAlreadyExists
		}
	}
	r.reportDB.ID++
	saved := cloneReport(report)
	saved.ID = r.reportDB.ID
	r.reportDB.Data[saved.ID] = saved
	report.ID = saved.ID
	return saved.ID, nil
}

func (r *ReportRepository) SelectByID(ctx context.Context, id int64) (*entity.Report, error) {
	_ = ctx
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.selectByID(id)
}

func (r *ReportRepository) selectByID(id int64) (*entity.Report, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return cloneReport(r.reportDB.Data[id]), nil
}

func (r *ReportRepository) SelectByReporterAndTarget(ctx context.Context, reporterUserID int64, targetType entity.ReportTargetType, targetID int64) (*entity.Report, error) {
	_ = ctx
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.selectByReporterAndTarget(reporterUserID, targetType, targetID)
}

func (r *ReportRepository) selectByReporterAndTarget(reporterUserID int64, targetType entity.ReportTargetType, targetID int64) (*entity.Report, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, report := range r.reportDB.Data {
		if report.ReporterUserID == reporterUserID && report.TargetType == targetType && report.TargetID == targetID {
			return cloneReport(report), nil
		}
	}
	return nil, nil
}

func (r *ReportRepository) SelectList(ctx context.Context, status *entity.ReportStatus, limit int, lastID int64) ([]*entity.Report, error) {
	_ = ctx
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.selectList(status, limit, lastID)
}

func (r *ReportRepository) selectList(status *entity.ReportStatus, limit int, lastID int64) ([]*entity.Report, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if limit <= 0 {
		return []*entity.Report{}, nil
	}
	reports := make([]*entity.Report, 0, len(r.reportDB.Data))
	for _, report := range r.reportDB.Data {
		if status != nil && report.Status != *status {
			continue
		}
		reports = append(reports, cloneReport(report))
	}
	sort.Slice(reports, func(i, j int) bool {
		leftPending := reports[i].Status == entity.ReportStatusPending
		rightPending := reports[j].Status == entity.ReportStatusPending
		if leftPending != rightPending {
			return leftPending
		}
		return reports[i].ID > reports[j].ID
	})

	start := 0
	if lastID > 0 {
		start = len(reports)
		for idx, report := range reports {
			if report.ID == lastID {
				start = idx + 1
				break
			}
		}
	}
	if start >= len(reports) {
		return []*entity.Report{}, nil
	}
	end := start + limit
	if end > len(reports) {
		end = len(reports)
	}
	return reports[start:end], nil
}

func (r *ReportRepository) Update(ctx context.Context, report *entity.Report) error {
	_ = ctx
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.update(report)
}

func (r *ReportRepository) update(report *entity.Report) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.reportDB.Data[report.ID]; !exists {
		return nil
	}
	r.reportDB.Data[report.ID] = cloneReport(report)
	return nil
}

func (r *ReportRepository) snapshot() reportRepositoryState {
	r.mu.RLock()
	defer r.mu.RUnlock()

	state := reportRepositoryState{
		ID:   r.reportDB.ID,
		Data: make(map[int64]*entity.Report, len(r.reportDB.Data)),
	}
	for id, report := range r.reportDB.Data {
		state.Data[id] = cloneReport(report)
	}
	return state
}

func (r *ReportRepository) restore(state reportRepositoryState) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.reportDB.ID = state.ID
	r.reportDB.Data = make(map[int64]*entity.Report, len(state.Data))
	for id, report := range state.Data {
		r.reportDB.Data[id] = cloneReport(report)
	}
}

func cloneReport(report *entity.Report) *entity.Report {
	if report == nil {
		return nil
	}
	out := *report
	if report.ResolvedBy != nil {
		resolvedBy := *report.ResolvedBy
		out.ResolvedBy = &resolvedBy
	}
	if report.ResolvedAt != nil {
		resolvedAt := *report.ResolvedAt
		out.ResolvedAt = &resolvedAt
	}
	return &out
}
