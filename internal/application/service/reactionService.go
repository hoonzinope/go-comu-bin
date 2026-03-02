package service

import (
	"github.com/hoonzinope/go-comu-bin/internal/application"
	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
)

type ReactionService struct {
	repository application.Repository
}

func NewReactionService(repository application.Repository) *ReactionService {
	return &ReactionService{
		repository: repository,
	}
}

func (s *ReactionService) AddReaction(UserID, TargetID int64, TargetType string, ReactionType string) error {
	// 리액션 추가 로직 구현
	user, err := s.repository.UserRepository.SelectUserByID(UserID) // user 존재 여부 확인
	if user == nil || err != nil {
		return customError.ErrInternalServerError
	}
	var targetExists bool
	switch TargetType {
	case "post":
		post, err := s.repository.PostRepository.SelectPostByID(TargetID) // post 존재 여부 확인
		if post == nil || err != nil {
			return customError.ErrInternalServerError
		}
		targetExists = true
	case "comment":
		comment, err := s.repository.CommentRepository.SelectCommentByID(TargetID) // comment 존재 여부 확인
		if comment == nil || err != nil {
			return customError.ErrInternalServerError
		}
		targetExists = true
	default:
		return customError.ErrInternalServerError
	}
	if !targetExists {
		return customError.ErrInternalServerError
	}
	err = s.repository.ReactionRepository.AddReaction(UserID, TargetID, TargetType, ReactionType)
	if err != nil {
		return customError.ErrInternalServerError
	}
	return nil
}

func (s *ReactionService) RemoveReaction(UserID, TargetID int64, TargetType string) error {
	// 리액션 제거 로직 구현
	user, err := s.repository.UserRepository.SelectUserByID(UserID) // user 존재 여부 확인
	if user == nil || err != nil {
		return customError.ErrInternalServerError
	}
	var targetExists bool
	switch TargetType {
	case "post":
		post, err := s.repository.PostRepository.SelectPostByID(TargetID) // post 존재 여부 확인
		if post == nil || err != nil {
			return customError.ErrInternalServerError
		}
		targetExists = true
	case "comment":
		comment, err := s.repository.CommentRepository.SelectCommentByID(TargetID) // comment 존재 여부 확인
		if comment == nil || err != nil {
			return customError.ErrInternalServerError
		}
		targetExists = true
	default:
		return customError.ErrInternalServerError
	}
	if !targetExists {
		return customError.ErrInternalServerError
	}
	err = s.repository.ReactionRepository.RemoveReaction(UserID, TargetID, TargetType)
	if err != nil {
		return customError.ErrInternalServerError
	}
	return nil
}
