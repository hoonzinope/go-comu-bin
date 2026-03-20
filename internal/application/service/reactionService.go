package service

import (
	"log/slog"

	appcache "github.com/hoonzinope/go-comu-bin/internal/application/cache"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	reactionsvc "github.com/hoonzinope/go-comu-bin/internal/application/service/reaction"
)

type ReactionService = reactionsvc.Service

func NewReactionService(userRepository port.UserRepository, boardRepository port.BoardRepository, postRepository port.PostRepository, commentRepository port.CommentRepository, reactionRepository port.ReactionRepository, unitOfWork port.UnitOfWork, cache port.Cache, cachePolicy appcache.Policy, logger ...*slog.Logger) *ReactionService {
	return reactionsvc.NewService(userRepository, boardRepository, postRepository, commentRepository, reactionRepository, unitOfWork, cache, cachePolicy, logger...)
}

func NewReactionServiceWithActionDispatcher(userRepository port.UserRepository, boardRepository port.BoardRepository, postRepository port.PostRepository, commentRepository port.CommentRepository, reactionRepository port.ReactionRepository, unitOfWork port.UnitOfWork, cache port.Cache, actionDispatcher port.ActionHookDispatcher, cachePolicy appcache.Policy, logger ...*slog.Logger) *ReactionService {
	return reactionsvc.NewServiceWithActionDispatcher(userRepository, boardRepository, postRepository, commentRepository, reactionRepository, unitOfWork, cache, actionDispatcher, cachePolicy, logger...)
}
