package comment

import (
	"context"
	svccommon "github.com/hoonzinope/go-comu-bin/internal/application/service/common"
	"log/slog"

	appcache "github.com/hoonzinope/go-comu-bin/internal/application/cache"
	"github.com/hoonzinope/go-comu-bin/internal/application/model"
	"github.com/hoonzinope/go-comu-bin/internal/application/policy"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
)

var _ port.CommentUseCase = (*CommentService)(nil)

type Service = CommentService

type CommentService struct {
	queryHandler   *commentQueryHandler
	commandHandler *commentCommandHandler
}

func NewCommentService(userRepository port.UserRepository, boardRepository port.BoardRepository, postRepository port.PostRepository, commentRepository port.CommentRepository, reactionRepository port.ReactionRepository, unitOfWork port.UnitOfWork, cache port.Cache, cachePolicy appcache.Policy, authorizationPolicy policy.AuthorizationPolicy, logger ...*slog.Logger) *CommentService {
	return NewCommentServiceWithActionDispatcher(userRepository, boardRepository, postRepository, commentRepository, reactionRepository, unitOfWork, cache, nil, cachePolicy, authorizationPolicy, logger...)
}

func NewService(userRepository port.UserRepository, boardRepository port.BoardRepository, postRepository port.PostRepository, commentRepository port.CommentRepository, reactionRepository port.ReactionRepository, unitOfWork port.UnitOfWork, cache port.Cache, cachePolicy appcache.Policy, authorizationPolicy policy.AuthorizationPolicy, logger ...*slog.Logger) *Service {
	return NewCommentService(userRepository, boardRepository, postRepository, commentRepository, reactionRepository, unitOfWork, cache, cachePolicy, authorizationPolicy, logger...)
}

func NewCommentServiceWithActionDispatcher(userRepository port.UserRepository, boardRepository port.BoardRepository, postRepository port.PostRepository, commentRepository port.CommentRepository, reactionRepository port.ReactionRepository, unitOfWork port.UnitOfWork, cache port.Cache, actionDispatcher port.ActionHookDispatcher, cachePolicy appcache.Policy, authorizationPolicy policy.AuthorizationPolicy, logger ...*slog.Logger) *CommentService {
	return &CommentService{
		queryHandler:   newCommentQueryHandler(userRepository, boardRepository, postRepository, commentRepository, cache, cachePolicy),
		commandHandler: newCommentCommandHandler(unitOfWork, svccommon.ResolveActionDispatcher(actionDispatcher), authorizationPolicy),
	}
}

func NewServiceWithActionDispatcher(userRepository port.UserRepository, boardRepository port.BoardRepository, postRepository port.PostRepository, commentRepository port.CommentRepository, reactionRepository port.ReactionRepository, unitOfWork port.UnitOfWork, cache port.Cache, actionDispatcher port.ActionHookDispatcher, cachePolicy appcache.Policy, authorizationPolicy policy.AuthorizationPolicy, logger ...*slog.Logger) *Service {
	return NewCommentServiceWithActionDispatcher(userRepository, boardRepository, postRepository, commentRepository, reactionRepository, unitOfWork, cache, actionDispatcher, cachePolicy, authorizationPolicy, logger...)
}

func (s *CommentService) CreateComment(ctx context.Context, content string, authorID int64, postUUID string, parentUUID *string) (string, error) {
	return s.commandHandler.CreateComment(ctx, content, authorID, postUUID, parentUUID)
}

func (s *CommentService) GetCommentsByPost(ctx context.Context, postUUID string, limit int, cursor string) (*model.CommentList, error) {
	return s.queryHandler.GetCommentsByPost(ctx, postUUID, limit, cursor)
}

func (s *CommentService) UpdateComment(ctx context.Context, commentUUID string, authorID int64, content string) error {
	return s.commandHandler.UpdateComment(ctx, commentUUID, authorID, content)
}

func (s *CommentService) DeleteComment(ctx context.Context, commentUUID string, authorID int64) error {
	return s.commandHandler.DeleteComment(ctx, commentUUID, authorID)
}
