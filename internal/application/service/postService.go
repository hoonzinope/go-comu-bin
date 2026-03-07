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

var (
	commentDefaultLimit = 10
)

var _ port.PostUseCase = (*PostService)(nil)

type PostService struct {
	userRepository      port.UserRepository
	boardRepository     port.BoardRepository
	postRepository      port.PostRepository
	commentRepository   port.CommentRepository
	reactionRepository  port.ReactionRepository
	cache               port.Cache
	cachePolicy         appcache.Policy
	authorizationPolicy policy.AuthorizationPolicy
}

func NewPostService(userRepository port.UserRepository, boardRepository port.BoardRepository, postRepository port.PostRepository, commentRepository port.CommentRepository, reactionRepository port.ReactionRepository, cache port.Cache, cachePolicy appcache.Policy, authorizationPolicy policy.AuthorizationPolicy) *PostService {
	return &PostService{
		userRepository:      userRepository,
		boardRepository:     boardRepository,
		postRepository:      postRepository,
		commentRepository:   commentRepository,
		reactionRepository:  reactionRepository,
		cache:               cache,
		cachePolicy:         cachePolicy,
		authorizationPolicy: authorizationPolicy,
	}
}

func (s *PostService) CreatePost(title, content string, authorID, boardID int64) (int64, error) {
	// 게시글 생성 로직 구현
	if strings.TrimSpace(title) == "" || strings.TrimSpace(content) == "" {
		return 0, customError.ErrInvalidInput
	}
	user, err := s.userRepository.SelectUserByID(authorID) // user 존재 여부 확인
	if err != nil {
		return 0, customError.WrapRepository("select user by id for create post", err)
	}
	if user == nil {
		return 0, customError.ErrUserNotFound
	}
	board, err := s.boardRepository.SelectBoardByID(boardID) // board 존재 여부 확인
	if err != nil {
		return 0, customError.WrapRepository("select board by id for create post", err)
	}
	if board == nil {
		return 0, customError.ErrBoardNotFound
	}
	newPost := entity.NewPost(title, content, authorID, boardID)
	postID, err := s.postRepository.Save(newPost)
	if err != nil {
		return 0, customError.WrapRepository("save post", err)
	}
	if _, err := s.cache.DeleteByPrefix(key.PostListPrefix(boardID)); err != nil {
		return 0, customError.WrapCache("invalidate post list after create post", err)
	}
	return postID, nil
}

func (s *PostService) GetPostsList(boardID int64, limit int, lastID int64) (*model.PostList, error) {
	cacheKey := key.PostList(boardID, limit, lastID)
	value, err := s.cache.GetOrSetWithTTL(cacheKey, s.cachePolicy.ListTTLSeconds, func() (interface{}, error) {
		// 커서 기반 페이지네이션을 위해 1개 더 조회한다.
		fetchLimit := limit
		if limit > 0 {
			fetchLimit = limit + 1
		}

		posts, err := s.postRepository.SelectPosts(boardID, fetchLimit, lastID)
		if err != nil {
			return nil, customError.WrapRepository("select posts by board", err)
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

		return &model.PostList{
			Posts:      mapper.PostsFromEntities(posts),
			Limit:      limit,
			LastID:     lastID,
			HasMore:    hasMore,
			NextLastID: nextLastID,
		}, nil
	})
	if err != nil {
		return nil, err
	}
	list, ok := value.(*model.PostList)
	if !ok {
		return nil, customError.Mark(customError.ErrCacheFailure, "decode post list cache payload")
	}
	return list, nil
}

func (s *PostService) GetPostDetail(id int64) (*model.PostDetail, error) {
	cacheKey := key.PostDetail(id)
	value, err := s.cache.GetOrSetWithTTL(cacheKey, s.cachePolicy.DetailTTLSeconds, func() (interface{}, error) {
		post, err := s.postRepository.SelectPostByID(id)
		if err != nil {
			return nil, customError.WrapRepository("select post by id for post detail", err)
		}
		if post == nil {
			return nil, customError.ErrPostNotFound
		}
		reactions, err := s.reactionRepository.GetByTarget(post.ID, entity.ReactionTargetPost)
		if err != nil {
			return nil, customError.WrapRepository("select post reactions for post detail", err)
		}
		comments, err := s.commentRepository.SelectComments(post.ID, commentDefaultLimit, 0) // 댓글은 최대 10개까지 조회
		commentDetails := make([]*model.CommentDetail, len(comments))
		if err != nil {
			return nil, customError.WrapRepository("select comments for post detail", err)
		}
		for i, comment := range comments {
			commentReactions, err := s.reactionRepository.GetByTarget(comment.ID, entity.ReactionTargetComment)
			if err != nil {
				return nil, customError.WrapRepository("select comment reactions for post detail", err)
			}
			commentDetails[i] = &model.CommentDetail{
				Comment:   mapper.CommentPtrFromEntity(comment),
				Reactions: mapper.ReactionsFromEntities(commentReactions),
			}
		}
		postDetail := &model.PostDetail{
			Post:      mapper.PostPtrFromEntity(post),
			Comments:  commentDetails,
			Reactions: mapper.ReactionsFromEntities(reactions),
		}
		return postDetail, nil
	})
	if err != nil {
		return nil, err
	}
	detail, ok := value.(*model.PostDetail)
	if !ok {
		return nil, customError.Mark(customError.ErrCacheFailure, "decode post detail cache payload")
	}
	return detail, nil
}

func (s *PostService) UpdatePost(id, authorID int64, title, content string) error {
	// 게시글 수정 로직 구현
	if strings.TrimSpace(title) == "" || strings.TrimSpace(content) == "" {
		return customError.ErrInvalidInput
	}
	post, err := s.postRepository.SelectPostByID(id) // post 존재 여부 확인
	if err != nil {
		return customError.WrapRepository("select post by id for update post", err)
	}
	if post == nil {
		return customError.ErrPostNotFound
	}
	requester, err := s.userRepository.SelectUserByID(authorID)
	if err != nil {
		return customError.WrapRepository("select user by id for update post", err)
	}
	if requester == nil {
		return customError.ErrUserNotFound
	}
	if err := s.authorizationPolicy.OwnerOrAdmin(requester, post.AuthorID); err != nil {
		return err
	}
	post.Update(title, content)
	err = s.postRepository.Update(post)
	if err != nil {
		return customError.WrapRepository("update post", err)
	}
	if err := s.cache.Delete(key.PostDetail(post.ID)); err != nil {
		return customError.WrapCache("invalidate post detail after update post", err)
	}
	if _, err := s.cache.DeleteByPrefix(key.PostListPrefix(post.BoardID)); err != nil {
		return customError.WrapCache("invalidate post list after update post", err)
	}
	return nil
}

func (s *PostService) DeletePost(id, authorID int64) error {
	// 게시글 삭제 로직 구현
	post, err := s.postRepository.SelectPostByID(id) // post 존재 여부 확인
	if err != nil {
		return customError.WrapRepository("select post by id for delete post", err)
	}
	if post == nil {
		return customError.ErrPostNotFound
	}
	requester, err := s.userRepository.SelectUserByID(authorID)
	if err != nil {
		return customError.WrapRepository("select user by id for delete post", err)
	}
	if requester == nil {
		return customError.ErrUserNotFound
	}
	if err := s.authorizationPolicy.OwnerOrAdmin(requester, post.AuthorID); err != nil {
		return err
	}
	err = s.postRepository.Delete(post.ID)
	if err != nil {
		return customError.WrapRepository("delete post", err)
	}
	if err := s.cache.Delete(key.PostDetail(post.ID)); err != nil {
		return customError.WrapCache("invalidate post detail after delete post", err)
	}
	if _, err := s.cache.DeleteByPrefix(key.PostListPrefix(post.BoardID)); err != nil {
		return customError.WrapCache("invalidate post list after delete post", err)
	}
	if _, err := s.cache.DeleteByPrefix(key.CommentListPrefix(post.ID)); err != nil {
		return customError.WrapCache("invalidate comment list after delete post", err)
	}
	if err := s.cache.Delete(key.ReactionList(string(entity.ReactionTargetPost), post.ID)); err != nil {
		return customError.WrapCache("invalidate post reaction list after delete post", err)
	}
	return nil
}
