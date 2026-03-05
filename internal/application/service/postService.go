package service

import (
	"fmt"

	"github.com/hoonzinope/go-comu-bin/internal/application"
	"github.com/hoonzinope/go-comu-bin/internal/application/policy"
	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
	"github.com/hoonzinope/go-comu-bin/internal/domain/dto"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var (
	commentDefaultLimit = 10
)

var _ application.PostUseCase = (*PostService)(nil)

type PostService struct {
	repository          application.Repository
	cache               application.Cache
	authorizationPolicy policy.AuthorizationPolicy
}

func NewPostService(repository application.Repository, caches ...application.Cache) *PostService {
	return &PostService{
		repository:          repository,
		cache:               resolveCache(caches),
		authorizationPolicy: policy.NewRoleAuthorizationPolicy(),
	}
}

func (s *PostService) CreatePost(title, content string, authorID, boardID int64) (int64, error) {
	// 게시글 생성 로직 구현
	user, err := s.repository.UserRepository.SelectUserByID(authorID) // user 존재 여부 확인
	if user == nil || err != nil {
		return 0, customError.ErrInternalServerError
	}
	board, err := s.repository.BoardRepository.SelectBoardByID(boardID) // board 존재 여부 확인
	if board == nil || err != nil {
		return 0, customError.ErrInternalServerError
	}
	newPost := entity.NewPost(title, content, authorID, boardID)
	postID, err := s.repository.PostRepository.Save(newPost)
	if err != nil {
		return 0, customError.ErrInternalServerError
	}
	s.cache.DeleteByPrefix(fmt.Sprintf("posts:list:board:%d:", boardID))
	return postID, nil
}

func (s *PostService) GetPostsList(boardID int64, limit int, lastID int64) (*dto.PostList, error) {
	cacheKey := fmt.Sprintf("posts:list:board:%d:limit:%d:last:%d", boardID, limit, lastID)
	value, err := s.cache.GetOrSetWithTTL(cacheKey, listCacheTTLSeconds, func() (interface{}, error) {
		// 커서 기반 페이지네이션을 위해 1개 더 조회한다.
		fetchLimit := limit
		if limit > 0 {
			fetchLimit = limit + 1
		}

		posts, err := s.repository.PostRepository.SelectPosts(boardID, fetchLimit, lastID)
		if err != nil {
			return nil, customError.ErrInternalServerError
		}

		hasMore := false
		var nextLastID *int64
		if limit >= 0 && len(posts) > limit {
			hasMore = true
			posts = posts[:limit]
		}
		if hasMore && len(posts) > 0 {
			next := posts[len(posts)-1].ID
			nextLastID = &next
		}

		return &dto.PostList{
			Posts:      posts,
			Limit:      limit,
			LastID:     lastID,
			HasMore:    hasMore,
			NextLastID: nextLastID,
		}, nil
	})
	if err != nil {
		return nil, err
	}
	list, ok := value.(*dto.PostList)
	if !ok {
		return nil, customError.ErrInternalServerError
	}
	return list, nil
}

func (s *PostService) GetPostDetail(id int64) (*dto.PostDetail, error) {
	cacheKey := fmt.Sprintf("posts:detail:%d", id)
	value, err := s.cache.GetOrSetWithTTL(cacheKey, detailCacheTTLSeconds, func() (interface{}, error) {
		post, err := s.repository.PostRepository.SelectPostByID(id)
		if post == nil || err != nil {
			return nil, customError.ErrInternalServerError
		}
		reactions, err := s.repository.ReactionRepository.GetByTarget(post.ID, "post")
		if err != nil {
			return nil, customError.ErrInternalServerError
		}
		comments, err := s.repository.CommentRepository.SelectComments(post.ID, commentDefaultLimit, 0) // 댓글은 최대 10개까지 조회
		commentDetails := make([]*dto.CommentDetail, len(comments))
		if err != nil {
			return nil, customError.ErrInternalServerError
		}
		for i, comment := range comments {
			commentReactions, err := s.repository.ReactionRepository.GetByTarget(comment.ID, "comment")
			if err != nil {
				return nil, customError.ErrInternalServerError
			}
			commentDetails[i] = &dto.CommentDetail{
				Comment:   comment,
				Reactions: commentReactions,
			}
		}
		postDetail := &dto.PostDetail{
			Post:      post,
			Comments:  commentDetails,
			Reactions: reactions,
		}
		return postDetail, nil
	})
	if err != nil {
		return nil, err
	}
	detail, ok := value.(*dto.PostDetail)
	if !ok {
		return nil, customError.ErrInternalServerError
	}
	return detail, nil
}

func (s *PostService) UpdatePost(id, authorID int64, title, content string) error {
	// 게시글 수정 로직 구현
	post, err := s.repository.PostRepository.SelectPostByID(id) // post 존재 여부 확인
	if post == nil || err != nil {
		return customError.ErrInternalServerError
	}
	requester, err := s.repository.UserRepository.SelectUserByID(authorID)
	if requester == nil || err != nil {
		return customError.ErrUserNotFound
	}
	if err := s.authorizationPolicy.OwnerOrAdmin(requester, post.AuthorID); err != nil {
		return err
	}
	post.Update(title, content)
	err = s.repository.PostRepository.Update(post)
	if err != nil {
		return customError.ErrInternalServerError
	}
	s.cache.Delete(fmt.Sprintf("posts:detail:%d", post.ID))
	s.cache.DeleteByPrefix(fmt.Sprintf("posts:list:board:%d:", post.BoardID))
	return nil
}

func (s *PostService) DeletePost(id, authorID int64) error {
	// 게시글 삭제 로직 구현
	post, err := s.repository.PostRepository.SelectPostByID(id) // post 존재 여부 확인
	if post == nil || err != nil {
		return customError.ErrInternalServerError
	}
	requester, err := s.repository.UserRepository.SelectUserByID(authorID)
	if requester == nil || err != nil {
		return customError.ErrUserNotFound
	}
	if err := s.authorizationPolicy.OwnerOrAdmin(requester, post.AuthorID); err != nil {
		return err
	}
	err = s.repository.PostRepository.Delete(post.ID)
	if err != nil {
		return customError.ErrInternalServerError
	}
	s.cache.Delete(fmt.Sprintf("posts:detail:%d", post.ID))
	s.cache.DeleteByPrefix(fmt.Sprintf("posts:list:board:%d:", post.BoardID))
	s.cache.DeleteByPrefix(fmt.Sprintf("comments:list:post:%d:", post.ID))
	s.cache.Delete(fmt.Sprintf("reactions:list:post:%d", post.ID))
	return nil
}
