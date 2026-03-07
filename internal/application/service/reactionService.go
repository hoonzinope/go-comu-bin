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
	userRepository      port.UserRepository
	postRepository      port.PostRepository
	commentRepository   port.CommentRepository
	reactionRepository  port.ReactionRepository
	cache               port.Cache
	cachePolicy         appcache.Policy
}

func NewReactionService(userRepository port.UserRepository, postRepository port.PostRepository, commentRepository port.CommentRepository, reactionRepository port.ReactionRepository, cache port.Cache, cachePolicy appcache.Policy) *ReactionService {
	return &ReactionService{
		userRepository:      userRepository,
		postRepository:      postRepository,
		commentRepository:   commentRepository,
		reactionRepository:  reactionRepository,
		cache:               cache,
		cachePolicy:         cachePolicy,
	}
}

func (s *ReactionService) SetReaction(UserID, TargetID int64, TargetType entity.ReactionTargetType, ReactionType entity.ReactionType) (bool, error) {
	user, err := s.userRepository.SelectUserByID(UserID)
	if err != nil {
		return false, customError.ErrInternalServerError
	}
	if user == nil {
		return false, customError.ErrUserNotFound
	}

	if err := s.ensureTargetExists(TargetID, TargetType); err != nil {
		return false, err
	}

	_, created, changed, err := s.reactionRepository.SetUserTargetReaction(UserID, TargetID, TargetType, ReactionType)
	if err != nil {
		return false, customError.ErrInternalServerError
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
		return customError.ErrInternalServerError
	}
	if user == nil {
		return customError.ErrUserNotFound
	}
	if err := s.ensureTargetExists(TargetID, TargetType); err != nil {
		return err
	}

	deleted, err := s.reactionRepository.DeleteUserTargetReaction(UserID, TargetID, TargetType)
	if err != nil {
		return customError.ErrInternalServerError
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
		reactions, err := s.reactionRepository.GetByTarget(targetID, targetType)
		if err != nil {
			return nil, customError.ErrInternalServerError
		}
		return mapper.ReactionsFromEntities(reactions), nil
	})
	if err != nil {
		return nil, err
	}
	reactions, ok := value.([]model.Reaction)
	if !ok {
		return nil, customError.ErrInternalServerError
	}
	return reactions, nil
}

func (s *ReactionService) invalidateReactionCaches(targetID int64, targetType entity.ReactionTargetType) {
	s.cache.Delete(key.ReactionList(string(targetType), targetID))
	if targetType == entity.ReactionTargetPost {
		s.cache.Delete(key.PostDetail(targetID))
	}
	if targetType == entity.ReactionTargetComment {
		comment, err := s.commentRepository.SelectCommentByID(targetID)
		if err == nil && comment != nil {
			s.cache.Delete(key.PostDetail(comment.PostID))
		}
	}
}

func (s *ReactionService) ensureTargetExists(targetID int64, targetType entity.ReactionTargetType) error {
	switch targetType {
	case entity.ReactionTargetPost:
		post, err := s.postRepository.SelectPostByID(targetID)
		if err != nil {
			return customError.ErrInternalServerError
		}
		if post == nil {
			return customError.ErrPostNotFound
		}
		return nil
	case entity.ReactionTargetComment:
		comment, err := s.commentRepository.SelectCommentByID(targetID)
		if err != nil {
			return customError.ErrInternalServerError
		}
		if comment == nil {
			return customError.ErrCommentNotFound
		}
		return nil
	default:
		return customError.ErrInternalServerError
	}
}
