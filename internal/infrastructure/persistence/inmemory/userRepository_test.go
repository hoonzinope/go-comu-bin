package inmemory

import (
	"testing"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/application/porttest"
)

func TestUserRepositoryContract(t *testing.T) {
	porttest.RunUserRepositoryContractTests(t, func() port.UserRepository {
		return NewUserRepository()
	})
}
