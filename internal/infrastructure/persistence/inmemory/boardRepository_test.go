package inmemory

import (
	"testing"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/application/porttest"
)

func TestBoardRepositoryContract(t *testing.T) {
	porttest.RunBoardRepositoryContractTests(t, func() port.BoardRepository {
		return NewBoardRepository()
	})
}
