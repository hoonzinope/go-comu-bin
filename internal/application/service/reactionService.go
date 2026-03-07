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
	if err := s.invalidateReactionCaches(TargetID, TargetType); err != nil {
		return false, err
	}
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
	return s.invalidateReactionCaches(TargetID, TargetType)
}

func (s *ReactionService) GetReactionsByTarget(targetID int64, targetType entity.ReactionTargetType) ([]model.Reaction, error) {
	cacheKey := key.ReactionList(string(targetType), targetID)
	value, err := s.cache.GetOrSetWithTTL(cacheKey, s.cachePolicy.ListTTLSeconds, func() (interface{}, error) {
		reactions, err := s.reactionRepository.GetByTarget(targetID, targetType)
		if err != nil {
			return nil, customError.WrapRepository("select reactions by target", err)
		}
		return mapper.ReactionsFromEntities(reactions), nil
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

func (s *ReactionService) invalidateReactionCaches(targetID int64, targetType entity.ReactionTargetType) error {
	if err := s.cache.Delete(key.ReactionList(string(targetType), targetID)); err != nil {
		return customError.WrapCache("invalidate reaction list", err)
	}
	if targetType == entity.ReactionTargetPost {
		if err := s.cache.Delete(key.PostDetail(targetID)); err != nil {
			return customError.WrapCache("invalidate post detail after post reaction", err)
		}
	}
	if targetType == entity.ReactionTargetComment {
		comment, err := s.commentRepository.SelectCommentByID(targetID)
		if err == nil && comment != nil {
			if deleteErr := s.cache.Delete(key.PostDetail(comment.PostID)); deleteErr != nil {
				return customError.WrapCache("invalidate post detail after comment reaction", deleteErr)
			}
		}
	}
	return nil
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
