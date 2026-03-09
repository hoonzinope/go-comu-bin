package inmemory

import (
	"testing"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/application/porttest"
)

func TestPostTagRepositoryContract(t *testing.T) {
	porttest.RunPostTagRepositoryContractTests(t, func() port.PostTagRepository {
		return NewPostTagRepository()
	})
}
