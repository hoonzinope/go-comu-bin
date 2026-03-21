package post

import (
	"context"
	svccommon "github.com/hoonzinope/go-comu-bin/internal/application/service/common"
	"log/slog"

	appcache "github.com/hoonzinope/go-comu-bin/internal/application/cache"
	"github.com/hoonzinope/go-comu-bin/internal/application/model"
	"github.com/hoonzinope/go-comu-bin/internal/application/policy"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
)

var (
	commentDefaultLimit = 10
	maxPostTags         = 10
	maxTagLength        = 30
)

const (
	postDeleteBatchSize = 500
	DeleteBatchSize     = postDeleteBatchSize
)

var _ port.PostUseCase = (*PostService)(nil)

type Service = PostService

type PostService struct {
	queryHandler   *postQueryHandler
	commandHandler *postCommandHandler
}

func NewPostService(userRepository port.UserRepository, boardRepository port.BoardRepository, postRepository port.PostRepository, postSearchRepository port.PostSearchRepository, tagRepository port.TagRepository, postTagRepository port.PostTagRepository, attachmentRepository port.AttachmentRepository, commentRepository port.CommentRepository, reactionRepository port.ReactionRepository, unitOfWork port.UnitOfWork, cache port.Cache, cachePolicy appcache.Policy, authorizationPolicy policy.AuthorizationPolicy, logger ...*slog.Logger) *PostService {
	return NewPostServiceWithActionDispatcher(userRepository, boardRepository, postRepository, postSearchRepository, tagRepository, postTagRepository, attachmentRepository, commentRepository, reactionRepository, unitOfWork, cache, nil, cachePolicy, authorizationPolicy, logger...)
}

func NewService(userRepository port.UserRepository, boardRepository port.BoardRepository, postRepository port.PostRepository, postSearchRepository port.PostSearchRepository, tagRepository port.TagRepository, postTagRepository port.PostTagRepository, attachmentRepository port.AttachmentRepository, commentRepository port.CommentRepository, reactionRepository port.ReactionRepository, unitOfWork port.UnitOfWork, cache port.Cache, cachePolicy appcache.Policy, authorizationPolicy policy.AuthorizationPolicy, logger ...*slog.Logger) *Service {
	return NewPostService(userRepository, boardRepository, postRepository, postSearchRepository, tagRepository, postTagRepository, attachmentRepository, commentRepository, reactionRepository, unitOfWork, cache, cachePolicy, authorizationPolicy, logger...)
}

func NewPostServiceWithActionDispatcher(userRepository port.UserRepository, boardRepository port.BoardRepository, postRepository port.PostRepository, postSearchRepository port.PostSearchRepository, tagRepository port.TagRepository, postTagRepository port.PostTagRepository, attachmentRepository port.AttachmentRepository, commentRepository port.CommentRepository, reactionRepository port.ReactionRepository, unitOfWork port.UnitOfWork, cache port.Cache, actionDispatcher port.ActionHookDispatcher, cachePolicy appcache.Policy, authorizationPolicy policy.AuthorizationPolicy, logger ...*slog.Logger) *PostService {
	resolvedLogger := svccommon.ResolveLogger(logger)
	tagCoordinator := newPostTagCoordinator(tagRepository, postTagRepository)
	attachmentCoordinator := newPostAttachmentCoordinator(attachmentRepository)
	deletionWorkflow := newPostDeletionWorkflow(commentRepository, reactionRepository, attachmentCoordinator)
	return &PostService{
		queryHandler:   newPostQueryHandler(userRepository, boardRepository, postRepository, postSearchRepository, tagRepository, postTagRepository, attachmentRepository, commentRepository, reactionRepository, cache, cachePolicy),
		commandHandler: newPostCommandHandler(boardRepository, postRepository, unitOfWork, svccommon.ResolveActionDispatcher(actionDispatcher), authorizationPolicy, resolvedLogger, tagCoordinator, attachmentCoordinator, deletionWorkflow),
	}
}

func NewServiceWithActionDispatcher(userRepository port.UserRepository, boardRepository port.BoardRepository, postRepository port.PostRepository, postSearchRepository port.PostSearchRepository, tagRepository port.TagRepository, postTagRepository port.PostTagRepository, attachmentRepository port.AttachmentRepository, commentRepository port.CommentRepository, reactionRepository port.ReactionRepository, unitOfWork port.UnitOfWork, cache port.Cache, actionDispatcher port.ActionHookDispatcher, cachePolicy appcache.Policy, authorizationPolicy policy.AuthorizationPolicy, logger ...*slog.Logger) *Service {
	return NewPostServiceWithActionDispatcher(userRepository, boardRepository, postRepository, postSearchRepository, tagRepository, postTagRepository, attachmentRepository, commentRepository, reactionRepository, unitOfWork, cache, actionDispatcher, cachePolicy, authorizationPolicy, logger...)
}

func (s *PostService) CreatePost(ctx context.Context, title, content string, tags []string, mentionedUsernames []string, authorID int64, boardUUID string) (string, error) {
	return s.commandHandler.CreatePost(ctx, title, content, tags, mentionedUsernames, authorID, boardUUID)
}

func (s *PostService) CreateDraftPost(ctx context.Context, title, content string, tags []string, mentionedUsernames []string, authorID int64, boardUUID string) (string, error) {
	return s.commandHandler.CreateDraftPost(ctx, title, content, tags, mentionedUsernames, authorID, boardUUID)
}

func (s *PostService) GetPostsList(ctx context.Context, boardUUID string, limit int, cursor string) (*model.PostList, error) {
	return s.queryHandler.GetPostsList(ctx, boardUUID, limit, cursor)
}

func (s *PostService) SearchPosts(ctx context.Context, query string, limit int, cursor string) (*model.PostList, error) {
	return s.queryHandler.SearchPosts(ctx, query, limit, cursor)
}

func (s *PostService) GetPostsByTag(ctx context.Context, tagName string, limit int, cursor string) (*model.PostList, error) {
	return s.queryHandler.GetPostsByTag(ctx, tagName, limit, cursor)
}

func (s *PostService) GetPostDetail(ctx context.Context, postUUID string) (*model.PostDetail, error) {
	return s.queryHandler.GetPostDetail(ctx, postUUID)
}

func (s *PostService) PublishPost(ctx context.Context, postUUID string, authorID int64) error {
	return s.commandHandler.PublishPost(ctx, postUUID, authorID)
}

func (s *PostService) UpdatePost(ctx context.Context, postUUID string, authorID int64, title, content string, tags []string) error {
	return s.commandHandler.UpdatePost(ctx, postUUID, authorID, title, content, tags)
}

func (s *PostService) DeletePost(ctx context.Context, postUUID string, authorID int64) error {
	return s.commandHandler.DeletePost(ctx, postUUID, authorID)
}
