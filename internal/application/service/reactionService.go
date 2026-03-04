package service

import (
	"github.com/hoonzinope/go-comu-bin/internal/application"
	"github.com/hoonzinope/go-comu-bin/internal/application/policy"
	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var _ application.ReactionUseCase = (*ReactionService)(nil)

type ReactionService struct {
	repository          application.Repository
	authorizationPolicy policy.AuthorizationPolicy
}

func NewReactionService(repository application.Repository) *ReactionService {
	return &ReactionService{
		repository:          repository,
		authorizationPolicy: policy.NewRoleAuthorizationPolicy(),
	}
}

func (s *ReactionService) AddReaction(UserID, TargetID int64, TargetType string, ReactionType string) error {
	// 리액션 추가 로직 구현
	user, err := s.repository.UserRepository.SelectUserByID(UserID) // user 존재 여부 확인
	if user == nil || err != nil {
		return customError.ErrInternalServerError
	}
	newReaction := &entity.Reaction{}
	switch TargetType {
	case "post":
		post, err := s.repository.PostRepository.SelectPostByID(TargetID) // post 존재 여부 확인
		if post == nil || err != nil {
			return customError.ErrInternalServerError
		}
		newReaction.NewReaction(TargetType, TargetID, ReactionType, UserID)
	case "comment":
		comment, err := s.repository.CommentRepository.SelectCommentByID(TargetID) // comment 존재 여부 확인
		if comment == nil || err != nil {
			return customError.ErrInternalServerError
		}
		newReaction.NewReaction(TargetType, TargetID, ReactionType, UserID)
	default:
		return customError.ErrInternalServerError
	}
	err = s.repository.ReactionRepository.Add(newReaction)
	if err != nil {
		return customError.ErrInternalServerError
	}
	return nil
}

func (s *ReactionService) RemoveReaction(UserID, ID int64) error {
	// 리액션 제거 로직 구현
	user, err := s.repository.UserRepository.SelectUserByID(UserID) // user 존재 여부 확인
	if user == nil || err != nil {
		return customError.ErrInternalServerError
	}
	removeReaction, err := s.repository.ReactionRepository.GetByID(ID)
	if removeReaction == nil || err != nil {
		return customError.ErrInternalServerError
	}
	if err := s.authorizationPolicy.OwnerOrAdmin(user, removeReaction.UserID); err != nil {
		return err
	}
	err = s.repository.ReactionRepository.Remove(removeReaction)
	if err != nil {
		return customError.ErrInternalServerError
	}
	return nil
}

func (s *ReactionService) GetReactionsByTarget(targetID int64, targetType string) ([]*entity.Reaction, error) {
	reactions, err := s.repository.ReactionRepository.GetByTarget(targetID, targetType)
	if err != nil {
		return nil, customError.ErrInternalServerError
	}
	return reactions, nil
}
