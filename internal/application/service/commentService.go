package service

import (
	"github.com/hoonzinope/go-comu-bin/internal/application"
	appcache "github.com/hoonzinope/go-comu-bin/internal/application/cache"
	"github.com/hoonzinope/go-comu-bin/internal/application/cache/key"
	"github.com/hoonzinope/go-comu-bin/internal/application/policy"
	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
	"github.com/hoonzinope/go-comu-bin/internal/domain/dto"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var _ application.CommentUseCase = (*CommentService)(nil)

type CommentService struct {
	userRepository      application.UserRepository
	postRepository      application.PostRepository
	commentRepository   application.CommentRepository
	cache               application.Cache
	cachePolicy         appcache.Policy
	authorizationPolicy policy.AuthorizationPolicy
}

func NewCommentService(userRepository application.UserRepository, postRepository application.PostRepository, commentRepository application.CommentRepository, cache application.Cache, cachePolicy appcache.Policy) *CommentService {
	return &CommentService{
		userRepository:      userRepository,
		postRepository:      postRepository,
		commentRepository:   commentRepository,
		cache:               cache,
		cachePolicy:         cachePolicy,
		authorizationPolicy: policy.NewRoleAuthorizationPolicy(),
	}
}

func (s *CommentService) CreateComment(content string, authorID, postID int64) (int64, error) {
	// 댓글 생성 로직 구현
	user, err := s.userRepository.SelectUserByID(authorID) // user 존재 여부 확인
	if err != nil {
		return 0, customError.ErrInternalServerError
	}
	if user == nil {
		return 0, customError.ErrUserNotFound
	}
	post, err := s.postRepository.SelectPostByID(postID) // post 존재 여부 확인
	if err != nil {
		return 0, customError.ErrInternalServerError
	}
	if post == nil {
		return 0, customError.ErrPostNotFound
	}
	newComment := entity.NewComment(content, authorID, postID, nil)
	commentID, err := s.commentRepository.Save(newComment)
	if err != nil {
		return 0, customError.ErrInternalServerError
	}
	s.cache.DeleteByPrefix(key.CommentListPrefix(postID))
	s.cache.Delete(key.PostDetail(postID))
	return commentID, nil
}

func (s *CommentService) GetCommentsByPost(postID int64, limit int, lastID int64) (*dto.CommentList, error) {
	cacheKey := key.CommentList(postID, limit, lastID)
	value, err := s.cache.GetOrSetWithTTL(cacheKey, s.cachePolicy.ListTTLSeconds, func() (interface{}, error) {
		// 커서 기반 페이지네이션을 위해 1개 더 조회한다.
		fetchLimit := limit
		if limit > 0 {
			fetchLimit = limit + 1
		}

		comments, err := s.commentRepository.SelectComments(postID, fetchLimit, lastID)
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
	comment, err := s.commentRepository.SelectCommentByID(id) // comment 존재 여부 확인
	if err != nil {
		return customError.ErrInternalServerError
	}
	if comment == nil {
		return customError.ErrCommentNotFound
	}
	requester, err := s.userRepository.SelectUserByID(authorID)
	if requester == nil || err != nil {
		return customError.ErrUserNotFound
	}
	if err := s.authorizationPolicy.OwnerOrAdmin(requester, comment.AuthorID); err != nil {
		return err
	}
	comment.Update(content)
	err = s.commentRepository.Update(comment)
	if err != nil {
		return customError.ErrInternalServerError
	}
	s.cache.DeleteByPrefix(key.CommentListPrefix(comment.PostID))
	s.cache.Delete(key.PostDetail(comment.PostID))
	return nil
}

func (s *CommentService) DeleteComment(id, authorID int64) error {
	// 댓글 삭제 로직 구현
	comment, err := s.commentRepository.SelectCommentByID(id) // comment 존재 여부 확인
	if err != nil {
		return customError.ErrInternalServerError
	}
	if comment == nil {
		return customError.ErrCommentNotFound
	}
	requester, err := s.userRepository.SelectUserByID(authorID)
	if requester == nil || err != nil {
		return customError.ErrUserNotFound
	}
	if err := s.authorizationPolicy.OwnerOrAdmin(requester, comment.AuthorID); err != nil {
		return err
	}
	err = s.commentRepository.Delete(comment.ID)
	if err != nil {
		return customError.ErrInternalServerError
	}
	s.cache.DeleteByPrefix(key.CommentListPrefix(comment.PostID))
	s.cache.Delete(key.PostDetail(comment.PostID))
	s.cache.Delete(key.ReactionList("comment", comment.ID))
	return nil
}
