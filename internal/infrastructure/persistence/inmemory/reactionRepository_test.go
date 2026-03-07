package inmemory

import (
	"testing"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/application/porttest"
)

func TestReactionRepositoryContract(t *testing.T) {
	porttest.RunReactionRepositoryContractTests(t, func() port.ReactionRepository {
		return NewReactionRepository()
	})
}
