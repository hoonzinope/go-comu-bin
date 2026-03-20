package service

import (
	"context"
	"log/slog"

	appcache "github.com/hoonzinope/go-comu-bin/internal/application/cache"
	"github.com/hoonzinope/go-comu-bin/internal/application/model"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
)

var _ port.ReactionUseCase = (*ReactionService)(nil)

type ReactionService struct {
	queryHandler   *reactionQueryHandler
	commandHandler *reactionCommandHandler
}

func NewReactionService(userRepository port.UserRepository, boardRepository port.BoardRepository, postRepository port.PostRepository, commentRepository port.CommentRepository, reactionRepository port.ReactionRepository, unitOfWork port.UnitOfWork, cache port.Cache, cachePolicy appcache.Policy, logger ...*slog.Logger) *ReactionService {
	return NewReactionServiceWithActionDispatcher(userRepository, boardRepository, postRepository, commentRepository, reactionRepository, unitOfWork, cache, nil, cachePolicy, logger...)
}

func NewReactionServiceWithActionDispatcher(userRepository port.UserRepository, boardRepository port.BoardRepository, postRepository port.PostRepository, commentRepository port.CommentRepository, reactionRepository port.ReactionRepository, unitOfWork port.UnitOfWork, cache port.Cache, actionDispatcher port.ActionHookDispatcher, cachePolicy appcache.Policy, logger ...*slog.Logger) *ReactionService {
	queryHandler := newReactionQueryHandler(userRepository, boardRepository, postRepository, commentRepository, reactionRepository, cache, cachePolicy)
	return &ReactionService{
		queryHandler:   queryHandler,
		commandHandler: newReactionCommandHandler(userRepository, boardRepository, postRepository, commentRepository, reactionRepository, unitOfWork, resolveActionDispatcher(actionDispatcher), queryHandler),
	}
}

func (s *ReactionService) SetReaction(ctx context.Context, userID int64, targetUUID string, targetType model.ReactionTargetType, reactionType model.ReactionType) (bool, error) {
	return s.commandHandler.SetReaction(ctx, userID, targetUUID, targetType, reactionType)
}

func (s *ReactionService) DeleteReaction(ctx context.Context, userID int64, targetUUID string, targetType model.ReactionTargetType) error {
	return s.commandHandler.DeleteReaction(ctx, userID, targetUUID, targetType)
}

func (s *ReactionService) GetReactionsByTarget(ctx context.Context, targetUUID string, targetType model.ReactionTargetType) ([]model.Reaction, error) {
	return s.queryHandler.GetReactionsByTarget(ctx, targetUUID, targetType)
}
