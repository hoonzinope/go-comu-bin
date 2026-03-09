package inmemory

import (
	"testing"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/application/porttest"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBoardRepositoryContract(t *testing.T) {
	porttest.RunBoardRepositoryContractTests(t, func() port.BoardRepository {
		return NewBoardRepository()
	})
}

func TestBoardRepository_SelectReturnsClone(t *testing.T) {
	repo := NewBoardRepository()
	id, err := repo.Save(entity.NewBoard("free", "desc"))
	require.NoError(t, err)

	selected, err := repo.SelectBoardByID(id)
	require.NoError(t, err)
	require.NotNil(t, selected)

	selected.Update("mutated", "changed")

	again, err := repo.SelectBoardByID(id)
	require.NoError(t, err)
	require.NotNil(t, again)
	assert.Equal(t, "free", again.Name)
}
