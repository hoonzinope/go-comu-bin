package service

import (
	"strings"

	appcache "github.com/hoonzinope/go-comu-bin/internal/application/cache"
	"github.com/hoonzinope/go-comu-bin/internal/application/cache/key"
	"github.com/hoonzinope/go-comu-bin/internal/application/mapper"
	"github.com/hoonzinope/go-comu-bin/internal/application/model"
	"github.com/hoonzinope/go-comu-bin/internal/application/policy"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var _ port.CommentUseCase = (*CommentService)(nil)

type CommentService struct {
	userRepository      port.UserRepository
	postRepository      port.PostRepository
	commentRepository   port.CommentRepository
	cache               port.Cache
	cachePolicy         appcache.Policy
	authorizationPolicy policy.AuthorizationPolicy
}

func NewCommentService(userRepository port.UserRepository, postRepository port.PostRepository, commentRepository port.CommentRepository, cache port.Cache, cachePolicy appcache.Policy, authorizationPolicy policy.AuthorizationPolicy) *CommentService {
	return &CommentService{
		userRepository:      userRepository,
		postRepository:      postRepository,
		commentRepository:   commentRepository,
		cache:               cache,
		cachePolicy:         cachePolicy,
		authorizationPolicy: authorizationPolicy,
	}
}

func (s *CommentService) CreateComment(content string, authorID, postID int64) (int64, error) {
	// 댓글 생성 로직 구현
	if strings.TrimSpace(content) == "" {
		return 0, customError.ErrInvalidInput
	}
	user, err := s.userRepository.SelectUserByID(authorID) // user 존재 여부 확인
	if err != nil {
		return 0, customError.WrapRepository("select user by id for create comment", err)
	}
	if user == nil {
		return 0, customError.ErrUserNotFound
	}
	post, err := s.postRepository.SelectPostByID(postID) // post 존재 여부 확인
	if err != nil {
		return 0, customError.WrapRepository("select post by id for create comment", err)
	}
	if post == nil {
		return 0, customError.ErrPostNotFound
	}
	newComment := entity.NewComment(content, authorID, postID, nil)
	commentID, err := s.commentRepository.Save(newComment)
	if err != nil {
		return 0, customError.WrapRepository("save comment", err)
	}
	if _, err := s.cache.DeleteByPrefix(key.CommentListPrefix(postID)); err != nil {
		return 0, customError.WrapCache("invalidate comment list after create comment", err)
	}
	if err := s.cache.Delete(key.PostDetail(postID)); err != nil {
		return 0, customError.WrapCache("invalidate post detail after create comment", err)
	}
	return commentID, nil
}

func (s *CommentService) GetCommentsByPost(postID int64, limit int, lastID int64) (*model.CommentList, error) {
	cacheKey := key.CommentList(postID, limit, lastID)
	value, err := s.cache.GetOrSetWithTTL(cacheKey, s.cachePolicy.ListTTLSeconds, func() (interface{}, error) {
		// 커서 기반 페이지네이션을 위해 1개 더 조회한다.
		fetchLimit := limit
		if limit > 0 {
			fetchLimit = limit + 1
		}

		comments, err := s.commentRepository.SelectComments(postID, fetchLimit, lastID)
		if err != nil {
			return nil, customError.WrapRepository("select comments by post", err)
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

		return &model.CommentList{
			Comments:   mapper.CommentsFromEntities(comments),
			Limit:      limit,
			LastID:     lastID,
			HasMore:    hasMore,
			NextLastID: nextLastID,
		}, nil
	})
	if err != nil {
		return nil, err
	}
	list, ok := value.(*model.CommentList)
	if !ok {
		return nil, customError.Mark(customError.ErrCacheFailure, "decode comment list cache payload")
	}
	return list, nil
}

func (s *CommentService) UpdateComment(id, authorID int64, content string) error {
	// 댓글 수정 로직 구현
	if strings.TrimSpace(content) == "" {
		return customError.ErrInvalidInput
	}
	comment, err := s.commentRepository.SelectCommentByID(id) // comment 존재 여부 확인
	if err != nil {
		return customError.WrapRepository("select comment by id for update comment", err)
	}
	if comment == nil {
		return customError.ErrCommentNotFound
	}
	requester, err := s.userRepository.SelectUserByID(authorID)
	if err != nil {
		return customError.WrapRepository("select user by id for update comment", err)
	}
	if requester == nil {
		return customError.ErrUserNotFound
	}
	if err := s.authorizationPolicy.OwnerOrAdmin(requester, comment.AuthorID); err != nil {
		return err
	}
	comment.Update(content)
	err = s.commentRepository.Update(comment)
	if err != nil {
		return customError.WrapRepository("update comment", err)
	}
	if _, err := s.cache.DeleteByPrefix(key.CommentListPrefix(comment.PostID)); err != nil {
		return customError.WrapCache("invalidate comment list after update comment", err)
	}
	if err := s.cache.Delete(key.PostDetail(comment.PostID)); err != nil {
		return customError.WrapCache("invalidate post detail after update comment", err)
	}
	return nil
}

func (s *CommentService) DeleteComment(id, authorID int64) error {
	// 댓글 삭제 로직 구현
	comment, err := s.commentRepository.SelectCommentByID(id) // comment 존재 여부 확인
	if err != nil {
		return customError.WrapRepository("select comment by id for delete comment", err)
	}
	if comment == nil {
		return customError.ErrCommentNotFound
	}
	requester, err := s.userRepository.SelectUserByID(authorID)
	if err != nil {
		return customError.WrapRepository("select user by id for delete comment", err)
	}
	if requester == nil {
		return customError.ErrUserNotFound
	}
	if err := s.authorizationPolicy.OwnerOrAdmin(requester, comment.AuthorID); err != nil {
		return err
	}
	err = s.commentRepository.Delete(comment.ID)
	if err != nil {
		return customError.WrapRepository("delete comment", err)
	}
	if _, err := s.cache.DeleteByPrefix(key.CommentListPrefix(comment.PostID)); err != nil {
		return customError.WrapCache("invalidate comment list after delete comment", err)
	}
	if err := s.cache.Delete(key.PostDetail(comment.PostID)); err != nil {
		return customError.WrapCache("invalidate post detail after delete comment", err)
	}
	if err := s.cache.Delete(key.ReactionList(string(entity.ReactionTargetComment), comment.ID)); err != nil {
		return customError.WrapCache("invalidate comment reaction list after delete comment", err)
	}
	return nil
}
