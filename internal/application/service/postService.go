package service

import (
	"log/slog"

	appcache "github.com/hoonzinope/go-comu-bin/internal/application/cache"
	"github.com/hoonzinope/go-comu-bin/internal/application/policy"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	postsvc "github.com/hoonzinope/go-comu-bin/internal/application/service/post"
)

type PostService = postsvc.Service

func NewPostService(userRepository port.UserRepository, boardRepository port.BoardRepository, postRepository port.PostRepository, postSearchRepository port.PostSearchRepository, tagRepository port.TagRepository, postTagRepository port.PostTagRepository, attachmentRepository port.AttachmentRepository, commentRepository port.CommentRepository, reactionRepository port.ReactionRepository, unitOfWork port.UnitOfWork, cache port.Cache, cachePolicy appcache.Policy, authorizationPolicy policy.AuthorizationPolicy, logger ...*slog.Logger) *PostService {
	return postsvc.NewService(userRepository, boardRepository, postRepository, postSearchRepository, tagRepository, postTagRepository, attachmentRepository, commentRepository, reactionRepository, unitOfWork, cache, cachePolicy, authorizationPolicy, logger...)
}

func NewPostServiceWithActionDispatcher(userRepository port.UserRepository, boardRepository port.BoardRepository, postRepository port.PostRepository, postSearchRepository port.PostSearchRepository, tagRepository port.TagRepository, postTagRepository port.PostTagRepository, attachmentRepository port.AttachmentRepository, commentRepository port.CommentRepository, reactionRepository port.ReactionRepository, unitOfWork port.UnitOfWork, cache port.Cache, actionDispatcher port.ActionHookDispatcher, cachePolicy appcache.Policy, authorizationPolicy policy.AuthorizationPolicy, logger ...*slog.Logger) *PostService {
	return postsvc.NewServiceWithActionDispatcher(userRepository, boardRepository, postRepository, postSearchRepository, tagRepository, postTagRepository, attachmentRepository, commentRepository, reactionRepository, unitOfWork, cache, actionDispatcher, cachePolicy, authorizationPolicy, logger...)
}
