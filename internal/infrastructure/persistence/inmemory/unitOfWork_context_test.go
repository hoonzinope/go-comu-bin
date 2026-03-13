package inmemory

import (
	"context"
	"testing"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnitOfWork_WithinTransaction_PassesContextToCallback(t *testing.T) {
	tagRepository := NewTagRepository()
	postTagRepository := NewPostTagRepository()
	unitOfWork := NewUnitOfWork(
		NewUserRepository(),
		NewBoardRepository(),
		NewPostRepository(tagRepository, postTagRepository),
		tagRepository,
		postTagRepository,
		NewCommentRepository(),
		NewReactionRepository(),
		NewAttachmentRepository(),
		NewOutboxRepository(),
	)

	type contextKey string
	ctx := context.WithValue(context.Background(), contextKey("request_id"), "req-1")

	var callbackCtx context.Context
	err := unitOfWork.WithinTransaction(ctx, func(tx port.TxScope) error {
		callbackCtx = tx.Context()
		return nil
	})

	require.NoError(t, err)
	require.NotNil(t, callbackCtx)
	assert.Same(t, ctx, callbackCtx)
	assert.Equal(t, "req-1", callbackCtx.Value(contextKey("request_id")))
}
