package service

import (
	"log/slog"

	appcache "github.com/hoonzinope/go-comu-bin/internal/application/cache"
	"github.com/hoonzinope/go-comu-bin/internal/application/policy"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	commentsvc "github.com/hoonzinope/go-comu-bin/internal/application/service/comment"
)

type CommentService = commentsvc.Service

func NewCommentService(userRepository port.UserRepository, boardRepository port.BoardRepository, postRepository port.PostRepository, commentRepository port.CommentRepository, reactionRepository port.ReactionRepository, unitOfWork port.UnitOfWork, cache port.Cache, cachePolicy appcache.Policy, authorizationPolicy policy.AuthorizationPolicy, logger ...*slog.Logger) *CommentService {
	return commentsvc.NewService(userRepository, boardRepository, postRepository, commentRepository, reactionRepository, unitOfWork, cache, cachePolicy, authorizationPolicy, logger...)
}

func NewCommentServiceWithActionDispatcher(userRepository port.UserRepository, boardRepository port.BoardRepository, postRepository port.PostRepository, commentRepository port.CommentRepository, reactionRepository port.ReactionRepository, unitOfWork port.UnitOfWork, cache port.Cache, actionDispatcher port.ActionHookDispatcher, cachePolicy appcache.Policy, authorizationPolicy policy.AuthorizationPolicy, logger ...*slog.Logger) *CommentService {
	return commentsvc.NewServiceWithActionDispatcher(userRepository, boardRepository, postRepository, commentRepository, reactionRepository, unitOfWork, cache, actionDispatcher, cachePolicy, authorizationPolicy, logger...)
}
