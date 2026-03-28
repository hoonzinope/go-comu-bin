package sqlite

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func BenchmarkOutboxRepository_SelectByIDParallel(b *testing.B) {
	for _, maxOpenConns := range []int{1, 2, 4, 10} {
		b.Run(fmt.Sprintf("max_open_conns=%d", maxOpenConns), func(b *testing.B) {
			db := openBenchmarkSQLiteDB(b, maxOpenConns)
			repo := seedOutboxBenchmarkRepository(b, db)
			ctx := context.Background()
			before := db.Stats()

			b.ReportAllocs()
			b.ResetTimer()

			var firstErr error
			var mu sync.Mutex
			var busyCount int64
			b.SetParallelism(4)
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					_, err := repo.SelectByID(ctx, "outbox-bench-1")
					if err == nil {
						continue
					}
					if isSQLiteBusyError(err) {
						atomic.AddInt64(&busyCount, 1)
						continue
					}
					mu.Lock()
					if firstErr == nil {
						firstErr = err
					}
					mu.Unlock()
					return
				}
			})

			if firstErr != nil {
				b.Fatal(firstErr)
			}
			b.ReportMetric(float64(atomic.LoadInt64(&busyCount))/float64(b.N), "busy/op")
			reportDBWaitMetrics(b, before, db.Stats())
		})
	}
}

func BenchmarkOutboxRepository_MarkRetryParallel(b *testing.B) {
	for _, maxOpenConns := range []int{1, 2, 4, 10} {
		b.Run(fmt.Sprintf("max_open_conns=%d", maxOpenConns), func(b *testing.B) {
			db := openBenchmarkSQLiteDB(b, maxOpenConns)
			repo := seedOutboxBenchmarkRepository(b, db)
			ctx := context.Background()
			before := db.Stats()

			b.ReportAllocs()
			b.ResetTimer()

			var firstErr error
			var mu sync.Mutex
			var busyCount int64
			b.SetParallelism(4)
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					if err := repo.MarkRetry(ctx, "outbox-bench-1", time.Now().Add(time.Second), "bench"); err != nil {
						if isSQLiteBusyError(err) {
							atomic.AddInt64(&busyCount, 1)
							continue
						}
						mu.Lock()
						if firstErr == nil {
							firstErr = err
						}
						mu.Unlock()
						return
					}
				}
			})

			if firstErr != nil {
				b.Fatal(firstErr)
			}
			b.ReportMetric(float64(atomic.LoadInt64(&busyCount))/float64(b.N), "busy/op")
			reportDBWaitMetrics(b, before, db.Stats())
		})
	}
}
