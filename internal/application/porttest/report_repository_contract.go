package porttest

import (
	"context"
	"testing"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func RunReportRepositoryContractTests(t *testing.T, newRepository func() port.ReportRepository) {
	t.Helper()

	t.Run("save select update", func(t *testing.T) {
		repo := newRepository()
		report := entity.NewReport(entity.ReportTargetPost, 10, 1, entity.ReportReasonSpam, "detail")

		id, err := repo.Save(context.Background(), report)
		require.NoError(t, err)
		require.NotZero(t, id)

		selected, err := repo.SelectByID(context.Background(), id)
		require.NoError(t, err)
		require.NotNil(t, selected)
		assert.Equal(t, entity.ReportStatusPending, selected.Status)

		require.True(t, selected.Resolve(entity.ReportStatusAccepted, "ok", 99))
		require.NoError(t, repo.Update(context.Background(), selected))
		updated, err := repo.SelectByID(context.Background(), id)
		require.NoError(t, err)
		require.NotNil(t, updated)
		assert.Equal(t, entity.ReportStatusAccepted, updated.Status)
	})

	t.Run("duplicate reporter target rejected", func(t *testing.T) {
		repo := newRepository()
		_, err := repo.Save(context.Background(), entity.NewReport(entity.ReportTargetPost, 10, 1, entity.ReportReasonSpam, "first"))
		require.NoError(t, err)

		_, err = repo.Save(context.Background(), entity.NewReport(entity.ReportTargetPost, 10, 1, entity.ReportReasonAbuse, "second"))
		require.Error(t, err)
		assert.ErrorIs(t, err, customerror.ErrReportAlreadyExists)
	})

	t.Run("list pending first then latest", func(t *testing.T) {
		repo := newRepository()
		r1 := entity.NewReport(entity.ReportTargetPost, 10, 1, entity.ReportReasonSpam, "a")
		r2 := entity.NewReport(entity.ReportTargetPost, 11, 2, entity.ReportReasonAbuse, "b")
		r3 := entity.NewReport(entity.ReportTargetComment, 12, 3, entity.ReportReasonOther, "c")
		id1, _ := repo.Save(context.Background(), r1)
		id2, _ := repo.Save(context.Background(), r2)
		id3, _ := repo.Save(context.Background(), r3)

		selected2, _ := repo.SelectByID(context.Background(), id2)
		require.True(t, selected2.Resolve(entity.ReportStatusAccepted, "done", 99))
		require.NoError(t, repo.Update(context.Background(), selected2))

		list, err := repo.SelectList(context.Background(), nil, 10, 0)
		require.NoError(t, err)
		require.Len(t, list, 3)
		assert.Equal(t, id3, list[0].ID)
		assert.Equal(t, id1, list[1].ID)
		assert.Equal(t, id2, list[2].ID)
	})

	t.Run("status filter and cursor", func(t *testing.T) {
		repo := newRepository()
		_, _ = repo.Save(context.Background(), entity.NewReport(entity.ReportTargetPost, 10, 1, entity.ReportReasonSpam, "a"))
		_, _ = repo.Save(context.Background(), entity.NewReport(entity.ReportTargetPost, 11, 2, entity.ReportReasonAbuse, "b"))
		r3 := entity.NewReport(entity.ReportTargetComment, 12, 3, entity.ReportReasonOther, "c")
		id3, _ := repo.Save(context.Background(), r3)
		selected3, _ := repo.SelectByID(context.Background(), id3)
		require.True(t, selected3.Resolve(entity.ReportStatusRejected, "no", 99))
		require.NoError(t, repo.Update(context.Background(), selected3))

		status := entity.ReportStatusRejected
		list, err := repo.SelectList(context.Background(), &status, 10, 0)
		require.NoError(t, err)
		require.Len(t, list, 1)
		assert.Equal(t, id3, list[0].ID)
	})
}
