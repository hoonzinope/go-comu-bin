package inmemory

import (
	"testing"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/application/porttest"
)

func TestTagRepositoryContract(t *testing.T) {
	porttest.RunTagRepositoryContractTests(t, func() port.TagRepository {
		return NewTagRepository()
	})
}
