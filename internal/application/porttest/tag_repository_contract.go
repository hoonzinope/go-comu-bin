package porttest

import (
	"context"
	"testing"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func RunTagRepositoryContractTests(t *testing.T, newRepository func() port.TagRepository) {
	t.Helper()

	t.Run("normalized name stays unique", func(t *testing.T) {
		repo := newRepository()

		id1, err := repo.Save(context.Background(), entity.NewTag("go"))
		require.NoError(t, err)
		id2, err := repo.Save(context.Background(), entity.NewTag("go"))
		require.NoError(t, err)

		assert.Equal(t, id1, id2)
	})
}
