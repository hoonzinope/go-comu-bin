package porttest

import (
	"testing"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func RunAttachmentRepositoryContractTests(t *testing.T, newRepository func() port.AttachmentRepository) {
	t.Helper()

	t.Run("select cleanup candidates includes pending delete before cutoff", func(t *testing.T) {
		repo := newRepository()

		pending := entity.NewAttachment(10, "pending.png", "image/png", 10, "pending.png")
		pending.MarkReferenced()
		oldPendingTime := time.Now().Add(-2 * time.Hour)
		pending.MarkPendingDeleteAt(oldPendingTime)
		pendingID, err := repo.Save(pending)
		require.NoError(t, err)

		live := entity.NewAttachment(10, "live.png", "image/png", 10, "live.png")
		live.MarkReferenced()
		_, err = repo.Save(live)
		require.NoError(t, err)

		items, err := repo.SelectCleanupCandidatesBefore(time.Now().Add(-time.Hour), 10)
		require.NoError(t, err)
		require.Len(t, items, 1)
		assert.Equal(t, pendingID, items[0].ID)
	})

	t.Run("select cleanup candidates orders by oldest eligible timestamp first", func(t *testing.T) {
		repo := newRepository()

		orphan := entity.NewAttachment(10, "orphan.png", "image/png", 10, "orphan.png")
		orphanTime := time.Now().Add(-3 * time.Hour)
		orphan.OrphanedAt = &orphanTime
		orphanID, err := repo.Save(orphan)
		require.NoError(t, err)

		pending := entity.NewAttachment(10, "pending.png", "image/png", 10, "pending.png")
		pending.MarkReferenced()
		pendingTime := time.Now().Add(-2 * time.Hour)
		pending.MarkPendingDeleteAt(pendingTime)
		pendingID, err := repo.Save(pending)
		require.NoError(t, err)

		items, err := repo.SelectCleanupCandidatesBefore(time.Now().Add(-time.Hour), 10)
		require.NoError(t, err)
		require.Len(t, items, 2)
		assert.Equal(t, []int64{orphanID, pendingID}, []int64{items[0].ID, items[1].ID})
	})

	t.Run("select cleanup candidates respects limit", func(t *testing.T) {
		repo := newRepository()

		first := entity.NewAttachment(10, "first.png", "image/png", 10, "first.png")
		firstTime := time.Now().Add(-3 * time.Hour)
		first.OrphanedAt = &firstTime
		_, err := repo.Save(first)
		require.NoError(t, err)

		second := entity.NewAttachment(10, "second.png", "image/png", 10, "second.png")
		secondTime := time.Now().Add(-2 * time.Hour)
		second.OrphanedAt = &secondTime
		_, err = repo.Save(second)
		require.NoError(t, err)

		items, err := repo.SelectCleanupCandidatesBefore(time.Now().Add(-time.Hour), 1)
		require.NoError(t, err)
		require.Len(t, items, 1)
		assert.Equal(t, "first.png", items[0].FileName)
	})
}
