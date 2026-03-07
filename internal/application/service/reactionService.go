package service

import (
	"github.com/hoonzinope/go-comu-bin/internal/application"
	appcache "github.com/hoonzinope/go-comu-bin/internal/application/cache"
	"github.com/hoonzinope/go-comu-bin/internal/application/cache/key"
	"github.com/hoonzinope/go-comu-bin/internal/application/policy"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
	"github.com/hoonzinope/go-comu-bin/internal/domain/dto"
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
	authorizationPolicy policy.AuthorizationPolicy
}

func NewReactionService(userRepository port.UserRepository, postRepository port.PostRepository, commentRepository port.CommentRepository, reactionRepository port.ReactionRepository, cache port.Cache, cachePolicy appcache.Policy) *ReactionService {
	return &ReactionService{
		userRepository:      userRepository,
		postRepository:      postRepository,
		commentRepository:   commentRepository,
		reactionRepository:  reactionRepository,
		cache:               cache,
		cachePolicy:         cachePolicy,
		authorizationPolicy: policy.NewRoleAuthorizationPolicy(),
	}
}

func (s *ReactionService) AddReaction(UserID, TargetID int64, TargetType string, ReactionType string) error {
	// 리액션 추가 로직 구현
	user, err := s.userRepository.SelectUserByID(UserID) // user 존재 여부 확인
	if err != nil {
		return customError.ErrInternalServerError
	}
	if user == nil {
		return customError.ErrUserNotFound
	}
	var newReaction *entity.Reaction
	switch TargetType {
	case "post":
		post, err := s.postRepository.SelectPostByID(TargetID) // post 존재 여부 확인
		if err != nil {
			return customError.ErrInternalServerError
		}
		if post == nil {
			return customError.ErrPostNotFound
		}
		newReaction = entity.NewReaction(TargetType, TargetID, ReactionType, UserID)
	case "comment":
		comment, err := s.commentRepository.SelectCommentByID(TargetID) // comment 존재 여부 확인
		if err != nil {
			return customError.ErrInternalServerError
		}
		if comment == nil {
			return customError.ErrCommentNotFound
		}
		newReaction = entity.NewReaction(TargetType, TargetID, ReactionType, UserID)
		s.cache.Delete(key.PostDetail(comment.PostID))
	default:
		return customError.ErrInternalServerError
	}
	err = s.reactionRepository.Add(newReaction)
	if err != nil {
		return customError.ErrInternalServerError
	}
	s.cache.Delete(key.ReactionList(TargetType, TargetID))
	if TargetType == "post" {
		s.cache.Delete(key.PostDetail(TargetID))
	}
	return nil
}

func (s *ReactionService) RemoveReaction(UserID, ID int64) error {
	// 리액션 제거 로직 구현
	user, err := s.userRepository.SelectUserByID(UserID) // user 존재 여부 확인
	if err != nil {
		return customError.ErrInternalServerError
	}
	if user == nil {
		return customError.ErrUserNotFound
	}
	removeReaction, err := s.reactionRepository.GetByID(ID)
	if err != nil {
		return customError.ErrInternalServerError
	}
	if removeReaction == nil {
		return customError.ErrReactionNotFound
	}
	if err := s.authorizationPolicy.OwnerOrAdmin(user, removeReaction.UserID); err != nil {
		return err
	}
	err = s.reactionRepository.Remove(removeReaction)
	if err != nil {
		return customError.ErrInternalServerError
	}
	s.cache.Delete(key.ReactionList(removeReaction.TargetType, removeReaction.TargetID))
	if removeReaction.TargetType == "post" {
		s.cache.Delete(key.PostDetail(removeReaction.TargetID))
	}
	if removeReaction.TargetType == "comment" {
		comment, err := s.commentRepository.SelectCommentByID(removeReaction.TargetID)
		if err == nil && comment != nil {
			s.cache.Delete(key.PostDetail(comment.PostID))
		}
	}
	return nil
}

func (s *ReactionService) GetReactionsByTarget(targetID int64, targetType string) ([]dto.Reaction, error) {
	cacheKey := key.ReactionList(targetType, targetID)
	value, err := s.cache.GetOrSetWithTTL(cacheKey, s.cachePolicy.ListTTLSeconds, func() (interface{}, error) {
		reactions, err := s.reactionRepository.GetByTarget(targetID, targetType)
		if err != nil {
			return nil, customError.ErrInternalServerError
		}
		return application.ReactionsDTOFromEntities(reactions), nil
	})
	if err != nil {
		return nil, err
	}
	reactions, ok := value.([]dto.Reaction)
	if !ok {
		return nil, customError.ErrInternalServerError
	}
	return reactions, nil
}
