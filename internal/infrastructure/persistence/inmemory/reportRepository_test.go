package inmemory

import (
	"context"
	"testing"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/application/porttest"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReportRepositoryContract(t *testing.T) {
	porttest.RunReportRepositoryContractTests(t, func() port.ReportRepository {
		return NewReportRepository()
	})
}

func TestReportRepository_SelectReturnsClone(t *testing.T) {
	repo := NewReportRepository()
	id, err := repo.Save(context.Background(), entity.NewReport(entity.ReportTargetPost, 1, 1, entity.ReportReasonSpam, "detail"))
	require.NoError(t, err)

	selected, err := repo.SelectByID(context.Background(), id)
	require.NoError(t, err)
	require.NotNil(t, selected)
	selected.ReasonDetail = "mutated"

	again, err := repo.SelectByID(context.Background(), id)
	require.NoError(t, err)
	require.NotNil(t, again)
	assert.Equal(t, "detail", again.ReasonDetail)
}

