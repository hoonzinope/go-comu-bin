package inmemory

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

func BenchmarkUnitOfWork_WithinTransactionParallelSaveUser(b *testing.B) {
	userRepository := NewUserRepository()
	boardRepository := NewBoardRepository()
	tagRepository := NewTagRepository()
	postTagRepository := NewPostTagRepository()
	postRepository := NewPostRepository(tagRepository, postTagRepository)
	commentRepository := NewCommentRepository()
	reactionRepository := NewReactionRepository()
	attachmentRepository := NewAttachmentRepository()
	reportRepository := NewReportRepository()
	notificationRepository := NewNotificationRepository()
	outboxRepository := NewOutboxRepository()
	unitOfWork := NewUnitOfWork(
		userRepository,
		boardRepository,
		postRepository,
		tagRepository,
		postTagRepository,
		commentRepository,
		reactionRepository,
		attachmentRepository,
		reportRepository,
		notificationRepository,
		outboxRepository,
	)

	var seq int64
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			id := atomic.AddInt64(&seq, 1)
			_ = unitOfWork.WithinTransaction(context.Background(), func(tx port.TxScope) error {
				user := entity.NewUser(fmt.Sprintf("bench-user-%d", id), "pw")
				_, err := tx.UserRepository().Save(context.Background(), user)
				return err
			})
		}
	})
}
