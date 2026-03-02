package service

import (
	"github.com/hoonzinope/go-comu-bin/internal/application"
	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
	"github.com/hoonzinope/go-comu-bin/internal/domain/dto"
)

type CommentService struct {
	repository application.Repository
}

func NewCommentService(repository application.Repository) *CommentService {
	return &CommentService{
		repository: repository,
	}
}

func (s *CommentService) CreateComment(content string, authorID, postID int64) (int64, error) {
	// 댓글 생성 로직 구현
	user, err := s.repository.UserRepository.SelectUserByID(authorID) // user 존재 여부 확인
	if user == nil || err != nil {
		return 0, customError.ErrInternalServerError
	}
	post, err := s.repository.PostRepository.SelectPostByID(postID) // post 존재 여부 확인
	if post == nil || err != nil {
		return 0, customError.ErrInternalServerError
	}
	commentID, err := s.repository.CommentRepository.SaveComment(content, authorID, postID)
	if err != nil {
		return 0, customError.ErrInternalServerError
	}
	return commentID, nil
}

func (s *CommentService) GetCommentsByPost(postID int64, limit, offset int) ([]*dto.CommentDetail, error) {
	// 댓글 목록 조회 로직 구현
	comments, err := s.repository.CommentRepository.SelectCommentsByPostID(postID, limit, offset)
	if err != nil {
		return nil, customError.ErrInternalServerError
	}
	commentDetails := make([]*dto.CommentDetail, len(comments))
	for i, comment := range comments {
		reactions, err := s.repository.ReactionRepository.GetReactionsByTarget(comment.ID, "comment")
		if err != nil {
			return nil, customError.ErrInternalServerError
		}
		commentDetails[i] = &dto.CommentDetail{
			Comment:   comment,
			Reactions: reactions,
		}
	}
	return commentDetails, nil
}

func (s *CommentService) UpdateComment(id, authorID int64, content string) error {
	// 댓글 수정 로직 구현
	comment, err := s.repository.CommentRepository.SelectCommentByID(id) // comment 존재 여부 확인
	if comment == nil || err != nil {
		return customError.ErrInternalServerError
	}
	if comment.AuthorID != authorID {
		return customError.ErrInternalServerError
	}
	err = s.repository.CommentRepository.UpdateComment(id, content)
	if err != nil {
		return customError.ErrInternalServerError
	}
	return nil
}

func (s *CommentService) DeleteComment(id, authorID int64) error {
	// 댓글 삭제 로직 구현
	comment, err := s.repository.CommentRepository.SelectCommentByID(id) // comment 존재 여부 확인
	if comment == nil || err != nil {
		return customError.ErrInternalServerError
	}
	if comment.AuthorID != authorID {
		return customError.ErrInternalServerError
	}
	err = s.repository.CommentRepository.DeleteComment(id)
	if err != nil {
		return customError.ErrInternalServerError
	}
	return nil
}
