package service

import (
	appcache "github.com/hoonzinope/go-comu-bin/internal/application/cache"
	"github.com/hoonzinope/go-comu-bin/internal/application/cache/key"
	"github.com/hoonzinope/go-comu-bin/internal/application/mapper"
	"github.com/hoonzinope/go-comu-bin/internal/application/model"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var _ port.ReactionUseCase = (*ReactionService)(nil)

type ReactionService struct {
	userRepository     port.UserRepository
	postRepository     port.PostRepository
	commentRepository  port.CommentRepository
	reactionRepository port.ReactionRepository
	cache              port.Cache
	cachePolicy        appcache.Policy
}

func NewReactionService(userRepository port.UserRepository, postRepository port.PostRepository, commentRepository port.CommentRepository, reactionRepository port.ReactionRepository, cache port.Cache, cachePolicy appcache.Policy) *ReactionService {
	return &ReactionService{
		userRepository:     userRepository,
		postRepository:     postRepository,
		commentRepository:  commentRepository,
		reactionRepository: reactionRepository,
		cache:              cache,
		cachePolicy:        cachePolicy,
	}
}

func (s *ReactionService) SetReaction(UserID, TargetID int64, TargetType entity.ReactionTargetType, ReactionType entity.ReactionType) (bool, error) {
	user, err := s.userRepository.SelectUserByID(UserID)
	if err != nil {
		return false, customError.WrapRepository("select user by id for set reaction", err)
	}
	if user == nil {
		return false, customError.ErrUserNotFound
	}

	if err := s.ensureTargetExists(TargetID, TargetType); err != nil {
		return false, err
	}

	_, created, changed, err := s.reactionRepository.SetUserTargetReaction(UserID, TargetID, TargetType, ReactionType)
	if err != nil {
		return false, customError.WrapRepository("set user target reaction", err)
	}
	if !created && !changed {
		return false, nil
	}
	s.invalidateReactionCaches(TargetID, TargetType)
	return created, nil
}

func (s *ReactionService) DeleteReaction(UserID, TargetID int64, TargetType entity.ReactionTargetType) error {
	user, err := s.userRepository.SelectUserByID(UserID)
	if err != nil {
		return customError.WrapRepository("select user by id for delete reaction", err)
	}
	if user == nil {
		return customError.ErrUserNotFound
	}
	if err := s.ensureTargetExists(TargetID, TargetType); err != nil {
		return err
	}

	deleted, err := s.reactionRepository.DeleteUserTargetReaction(UserID, TargetID, TargetType)
	if err != nil {
		return customError.WrapRepository("delete user target reaction", err)
	}
	if !deleted {
		return nil
	}
	s.invalidateReactionCaches(TargetID, TargetType)
	return nil
}

func (s *ReactionService) GetReactionsByTarget(targetID int64, targetType entity.ReactionTargetType) ([]model.Reaction, error) {
	cacheKey := key.ReactionList(string(targetType), targetID)
	value, err := s.cache.GetOrSetWithTTL(cacheKey, s.cachePolicy.ListTTLSeconds, func() (interface{}, error) {
		if err := s.ensureTargetExists(targetID, targetType); err != nil {
			return nil, err
		}
		reactions, err := s.reactionRepository.GetByTarget(targetID, targetType)
		if err != nil {
			return nil, customError.WrapRepository("select reactions by target", err)
		}
		reactionModels, err := s.reactionsFromEntities(reactions)
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
		return nil, customError.Mark(customError.ErrCacheFailure, "decode reaction list cache payload")
	}
	return reactions, nil
}

func (s *ReactionService) reactionsFromEntities(reactions []*entity.Reaction) ([]model.Reaction, error) {
	out := make([]model.Reaction, 0, len(reactions))
	for _, reaction := range reactions {
		userUUID, err := userUUIDByID(s.userRepository, reaction.UserID)
		if err != nil {
			return nil, err
		}
		reactionModel := mapper.ReactionFromEntity(reaction)
		reactionModel.UserUUID = userUUID
		out = append(out, reactionModel)
	}
	return out, nil
}

func (s *ReactionService) invalidateReactionCaches(targetID int64, targetType entity.ReactionTargetType) {
	bestEffortCacheDelete(s.cache, key.ReactionList(string(targetType), targetID), "invalidate reaction list")
	if targetType == entity.ReactionTargetPost {
		bestEffortCacheDelete(s.cache, key.PostDetail(targetID), "invalidate post detail after post reaction")
	}
	if targetType == entity.ReactionTargetComment {
		comment, err := s.commentRepository.SelectCommentByID(targetID)
		if err == nil && comment != nil {
			bestEffortCacheDelete(s.cache, key.PostDetail(comment.PostID), "invalidate post detail after comment reaction")
		}
	}
}

func (s *ReactionService) ensureTargetExists(targetID int64, targetType entity.ReactionTargetType) error {
	switch targetType {
	case entity.ReactionTargetPost:
		post, err := s.postRepository.SelectPostByID(targetID)
		if err != nil {
			return customError.WrapRepository("select post by id for ensure reaction target", err)
		}
		if post == nil {
			return customError.ErrPostNotFound
		}
		return nil
	case entity.ReactionTargetComment:
		comment, err := s.commentRepository.SelectCommentByID(targetID)
		if err != nil {
			return customError.WrapRepository("select comment by id for ensure reaction target", err)
		}
		if comment == nil {
			return customError.ErrCommentNotFound
		}
		return nil
	default:
		return customError.ErrInternalServerError
	}
}
