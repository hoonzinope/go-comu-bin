package service

import (
	"github.com/hoonzinope/go-comu-bin/internal/application"
	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
	"github.com/hoonzinope/go-comu-bin/internal/domain/dto"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
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
	newComment := &entity.Comment{}
	newComment.NewComment(content, authorID, postID, nil)
	commentID, err := s.repository.CommentRepository.Save(newComment)
	if err != nil {
		return 0, customError.ErrInternalServerError
	}
	return commentID, nil
}

func (s *CommentService) GetCommentsByPost(postID int64, limit, offset int) (*dto.CommentList, error) {
	// 댓글 목록 조회 로직 구현
	comments, err := s.repository.CommentRepository.SelectComments(postID, limit, offset)
	if err != nil {
		return nil, customError.ErrInternalServerError
	}

	return &dto.CommentList{
		Comments: comments,
		Limit:    limit,
		Offset:   offset,
	}, nil
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
	comment.UpdateComment(content)
	err = s.repository.CommentRepository.Update(comment)
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
	err = s.repository.CommentRepository.Delete(comment.ID)
	if err != nil {
		return customError.ErrInternalServerError
	}
	return nil
}
