package service

import (
	"fmt"

	"github.com/hoonzinope/go-comu-bin/internal/application"
	"github.com/hoonzinope/go-comu-bin/internal/application/policy"
	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
	"github.com/hoonzinope/go-comu-bin/internal/domain/dto"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var _ application.CommentUseCase = (*CommentService)(nil)

type CommentService struct {
	repository          application.Repository
	cache               application.Cache
	authorizationPolicy policy.AuthorizationPolicy
}

func NewCommentService(repository application.Repository, caches ...application.Cache) *CommentService {
	return &CommentService{
		repository:          repository,
		cache:               resolveCache(caches),
		authorizationPolicy: policy.NewRoleAuthorizationPolicy(),
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
	newComment := entity.NewComment(content, authorID, postID, nil)
	commentID, err := s.repository.CommentRepository.Save(newComment)
	if err != nil {
		return 0, customError.ErrInternalServerError
	}
	s.cache.DeleteByPrefix(fmt.Sprintf("comments:list:post:%d:", postID))
	s.cache.Delete(fmt.Sprintf("posts:detail:%d", postID))
	return commentID, nil
}

func (s *CommentService) GetCommentsByPost(postID int64, limit int, lastID int64) (*dto.CommentList, error) {
	cacheKey := fmt.Sprintf("comments:list:post:%d:limit:%d:last:%d", postID, limit, lastID)
	value, err := s.cache.GetOrSetWithTTL(cacheKey, listCacheTTLSeconds, func() (interface{}, error) {
		// 커서 기반 페이지네이션을 위해 1개 더 조회한다.
		fetchLimit := limit
		if limit > 0 {
			fetchLimit = limit + 1
		}

		comments, err := s.repository.CommentRepository.SelectComments(postID, fetchLimit, lastID)
		if err != nil {
			return nil, customError.ErrInternalServerError
		}

		hasMore := false
		var nextLastID *int64
		if limit >= 0 && len(comments) > limit {
			hasMore = true
			comments = comments[:limit]
		}
		if hasMore && len(comments) > 0 {
			next := comments[len(comments)-1].ID
			nextLastID = &next
		}

		return &dto.CommentList{
			Comments:   comments,
			Limit:      limit,
			LastID:     lastID,
			HasMore:    hasMore,
			NextLastID: nextLastID,
		}, nil
	})
	if err != nil {
		return nil, err
	}
	list, ok := value.(*dto.CommentList)
	if !ok {
		return nil, customError.ErrInternalServerError
	}
	return list, nil
}

func (s *CommentService) UpdateComment(id, authorID int64, content string) error {
	// 댓글 수정 로직 구현
	comment, err := s.repository.CommentRepository.SelectCommentByID(id) // comment 존재 여부 확인
	if comment == nil || err != nil {
		return customError.ErrInternalServerError
	}
	requester, err := s.repository.UserRepository.SelectUserByID(authorID)
	if requester == nil || err != nil {
		return customError.ErrUserNotFound
	}
	if err := s.authorizationPolicy.OwnerOrAdmin(requester, comment.AuthorID); err != nil {
		return err
	}
	comment.Update(content)
	err = s.repository.CommentRepository.Update(comment)
	if err != nil {
		return customError.ErrInternalServerError
	}
	s.cache.DeleteByPrefix(fmt.Sprintf("comments:list:post:%d:", comment.PostID))
	s.cache.Delete(fmt.Sprintf("posts:detail:%d", comment.PostID))
	return nil
}

func (s *CommentService) DeleteComment(id, authorID int64) error {
	// 댓글 삭제 로직 구현
	comment, err := s.repository.CommentRepository.SelectCommentByID(id) // comment 존재 여부 확인
	if comment == nil || err != nil {
		return customError.ErrInternalServerError
	}
	requester, err := s.repository.UserRepository.SelectUserByID(authorID)
	if requester == nil || err != nil {
		return customError.ErrUserNotFound
	}
	if err := s.authorizationPolicy.OwnerOrAdmin(requester, comment.AuthorID); err != nil {
		return err
	}
	err = s.repository.CommentRepository.Delete(comment.ID)
	if err != nil {
		return customError.ErrInternalServerError
	}
	s.cache.DeleteByPrefix(fmt.Sprintf("comments:list:post:%d:", comment.PostID))
	s.cache.Delete(fmt.Sprintf("posts:detail:%d", comment.PostID))
	s.cache.Delete(fmt.Sprintf("reactions:list:comment:%d", comment.ID))
	return nil
}
