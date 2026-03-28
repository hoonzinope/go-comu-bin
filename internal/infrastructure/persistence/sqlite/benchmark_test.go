package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/stretchr/testify/require"
)

func openBenchmarkSQLiteDB(b *testing.B, maxOpenConns int) *sql.DB {
	b.Helper()

	tempDir := b.TempDir()
	db, err := Open(context.Background(), Options{
		Path:         filepath.Join(tempDir, "bench.db"),
		MaxOpenConns: maxOpenConns,
	})
	require.NoError(b, err)
	b.Cleanup(func() {
		require.NoError(b, db.Close())
	})
	return db
}

func reportDBWaitMetrics(b *testing.B, before, after sql.DBStats) {
	b.Helper()

	if b.N <= 0 {
		return
	}
	waitCountDelta := after.WaitCount - before.WaitCount
	waitDurationDelta := after.WaitDuration - before.WaitDuration
	b.ReportMetric(float64(waitCountDelta)/float64(b.N), "waits/op")
	b.ReportMetric(float64(waitDurationDelta.Microseconds())/float64(b.N), "wait_us/op")
}

func seedSearchBenchmarkRepository(b *testing.B, db *sql.DB) *PostSearchRepository {
	b.Helper()

	boardRepo := NewBoardRepository(db)
	tagRepo := NewTagRepository(db)
	postTagRepo := NewPostTagRepository(db)
	postRepo := NewPostRepository(db)
	searchRepo := NewPostSearchRepository(db)

	boardID, err := boardRepo.Save(context.Background(), entity.NewBoard("bench", "bench"))
	require.NoError(b, err)
	goTagID, err := tagRepo.Save(context.Background(), entity.NewTag("go"))
	require.NoError(b, err)
	searchTagID, err := tagRepo.Save(context.Background(), entity.NewTag("search"))
	require.NoError(b, err)

	for i := 0; i < 96; i++ {
		postID, err := postRepo.Save(context.Background(), entity.NewPost(
			"go search title "+itoa(i),
			"go search body "+itoa(i),
			1,
			boardID,
		))
		require.NoError(b, err)
		require.NoError(b, postTagRepo.UpsertActive(context.Background(), postID, goTagID))
		if i%3 == 0 {
			require.NoError(b, postTagRepo.UpsertActive(context.Background(), postID, searchTagID))
		}
	}
	require.NoError(b, searchRepo.RebuildAll(context.Background()))
	return searchRepo
}

func seedOutboxBenchmarkRepository(b *testing.B, db *sql.DB) *OutboxRepository {
	b.Helper()

	repo := NewOutboxRepository(db)
	now := time.Now().UTC()
	require.NoError(b, repo.Append(context.Background(), port.OutboxMessage{
		ID:            "outbox-bench-1",
		EventName:     "post.changed",
		Payload:       []byte(`{"id":1}`),
		OccurredAt:    now,
		NextAttemptAt: now,
		Status:        port.OutboxStatusPending,
	}))
	return repo
}

func itoa(value int) string {
	return strconv.Itoa(value)
}

func isSQLiteBusyError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "database is locked") || strings.Contains(msg, "sqlite_busy") || strings.Contains(msg, "database is busy")
}
