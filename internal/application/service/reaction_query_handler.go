package service

import (
	"context"
	"errors"

	appcache "github.com/hoonzinope/go-comu-bin/internal/application/cache"
	"github.com/hoonzinope/go-comu-bin/internal/application/cache/key"
	"github.com/hoonzinope/go-comu-bin/internal/application/mapper"
	"github.com/hoonzinope/go-comu-bin/internal/application/model"
	"github.com/hoonzinope/go-comu-bin/internal/application/policy"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

type reactionQueryHandler struct {
	userRepository     port.UserRepository
	boardRepository    port.BoardRepository
	postRepository     port.PostRepository
	commentRepository  port.CommentRepository
	reactionRepository port.ReactionRepository
	cache              port.Cache
	cachePolicy        appcache.Policy
}

func newReactionQueryHandler(userRepository port.UserRepository, boardRepository port.BoardRepository, postRepository port.PostRepository, commentRepository port.CommentRepository, reactionRepository port.ReactionRepository, cache port.Cache, cachePolicy appcache.Policy) *reactionQueryHandler {
	return &reactionQueryHandler{userRepository: userRepository, boardRepository: boardRepository, postRepository: postRepository, commentRepository: commentRepository, reactionRepository: reactionRepository, cache: cache, cachePolicy: cachePolicy}
}

func (h *reactionQueryHandler) GetReactionsByTarget(ctx context.Context, targetUUID string, targetType model.ReactionTargetType) ([]model.Reaction, error) {
	entityTargetType, ok := targetType.ToEntity()
	if !ok {
		return nil, customerror.ErrInvalidInput
	}
	targetID, err := h.resolveTargetID(ctx, targetUUID, entityTargetType)
	if err != nil {
		return nil, err
	}
	cacheKey := key.ReactionList(string(entityTargetType), targetID)
	value, err := h.cache.GetOrSetWithTTL(ctx, cacheKey, h.cachePolicy.ListTTLSeconds, func(ctx context.Context) (interface{}, error) {
		if err := h.ensureTargetExists(ctx, nil, targetID, entityTargetType); err != nil {
			return nil, err
		}
		reactions, err := h.reactionRepository.GetByTarget(ctx, targetID, entityTargetType)
		if err != nil {
			return nil, customerror.WrapRepository("select reactions by target", err)
		}
		reactionModels, err := h.reactionsFromEntities(ctx, targetUUID, reactions)
		if err != nil {
			return nil, err
		}
		return reactionModels, nil
	})
	if err != nil {
		return nil, normalizeCacheLoadError("load reaction list cache", err)
	}
	reactions, ok := value.([]model.Reaction)
	if !ok {
		return nil, customerror.Mark(customerror.ErrCacheFailure, "decode reaction list cache payload")
	}
	return reactions, nil
}

func (h *reactionQueryHandler) reactionsFromEntities(ctx context.Context, targetUUID string, reactions []*entity.Reaction) ([]model.Reaction, error) {
	userUUIDs, err := userUUIDsForReactions(ctx, h.userRepository, reactions)
	if err != nil {
		return nil, err
	}
	out := make([]model.Reaction, 0, len(reactions))
	for _, reaction := range reactions {
		userUUID, ok := userUUIDs[reaction.UserID]
		if !ok {
			return nil, customerror.WrapRepository("select users by ids including deleted", errors.New("reaction user not found"))
		}
		reactionModel := mapper.ReactionFromEntity(reaction)
		reactionModel.TargetUUID = targetUUID
		reactionModel.UserUUID = userUUID
		out = append(out, reactionModel)
	}
	return out, nil
}

func (h *reactionQueryHandler) resolveTargetID(ctx context.Context, targetUUID string, targetType entity.ReactionTargetType) (int64, error) {
	switch targetType {
	case entity.ReactionTargetPost:
		post, err := h.postRepository.SelectPostByUUID(ctx, targetUUID)
		if err != nil {
			return 0, customerror.WrapRepository("select post by uuid for reaction target", err)
		}
		if post == nil {
			return 0, customerror.ErrPostNotFound
		}
		return post.ID, nil
	case entity.ReactionTargetComment:
		comment, err := h.commentRepository.SelectCommentByUUID(ctx, targetUUID)
		if err != nil {
			return 0, customerror.WrapRepository("select comment by uuid for reaction target", err)
		}
		if comment == nil {
			return 0, customerror.ErrCommentNotFound
		}
		return comment.ID, nil
	default:
		return 0, customerror.ErrInvalidInput
	}
}

func (h *reactionQueryHandler) ensureTargetExists(ctx context.Context, user *entity.User, targetID int64, targetType entity.ReactionTargetType) error {
	switch targetType {
	case entity.ReactionTargetPost:
		_, err := policy.EnsurePostVisibleForUser(ctx, h.postRepository, h.boardRepository, user, targetID, customerror.ErrBoardNotFound, "reaction target")
		return err
	case entity.ReactionTargetComment:
		_, _, err := policy.EnsureCommentTargetVisibleForUser(ctx, h.commentRepository, h.postRepository, h.boardRepository, user, targetID, customerror.ErrBoardNotFound, "reaction target")
		return err
	default:
		return customerror.ErrInternalServerError
	}
}
