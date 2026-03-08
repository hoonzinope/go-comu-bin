package service

import (
	"regexp"
	"strconv"
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
	userRepository       port.UserRepository
	boardRepository      port.BoardRepository
	postRepository       port.PostRepository
	attachmentRepository port.AttachmentRepository
	commentRepository    port.CommentRepository
	reactionRepository   port.ReactionRepository
	cache                port.Cache
	cachePolicy          appcache.Policy
	authorizationPolicy  policy.AuthorizationPolicy
}

var attachmentEmbedPattern = regexp.MustCompile(`!\[[^\]]*]\(attachment://([0-9]+)\)`)

func NewPostService(userRepository port.UserRepository, boardRepository port.BoardRepository, postRepository port.PostRepository, attachmentRepository port.AttachmentRepository, commentRepository port.CommentRepository, reactionRepository port.ReactionRepository, cache port.Cache, cachePolicy appcache.Policy, authorizationPolicy policy.AuthorizationPolicy) *PostService {
	return &PostService{
		userRepository:       userRepository,
		boardRepository:      boardRepository,
		postRepository:       postRepository,
		attachmentRepository: attachmentRepository,
		commentRepository:    commentRepository,
		reactionRepository:   reactionRepository,
		cache:                cache,
		cachePolicy:          cachePolicy,
		authorizationPolicy:  authorizationPolicy,
	}
}

func (s *PostService) CreatePost(title, content string, authorID, boardID int64) (int64, error) {
	return s.createPost(title, content, authorID, boardID, false)
}

func (s *PostService) CreateDraftPost(title, content string, authorID, boardID int64) (int64, error) {
	return s.createPost(title, content, authorID, boardID, true)
}

func (s *PostService) createPost(title, content string, authorID, boardID int64, draft bool) (int64, error) {
	if strings.TrimSpace(title) == "" || strings.TrimSpace(content) == "" {
		return 0, customError.ErrInvalidInput
	}
	if len(extractAttachmentRefIDs(content)) > 0 {
		return 0, customError.ErrInvalidInput
	}
	user, err := s.userRepository.SelectUserByID(authorID) // user 존재 여부 확인
	if err != nil {
		return 0, customError.WrapRepository("select user by id for create post", err)
	}
	if user == nil {
		return 0, customError.ErrUserNotFound
	}
	if err := s.authorizationPolicy.CanWrite(user); err != nil {
		return 0, err
	}
	board, err := s.boardRepository.SelectBoardByID(boardID) // board 존재 여부 확인
	if err != nil {
		return 0, customError.WrapRepository("select board by id for create post", err)
	}
	if board == nil {
		return 0, customError.ErrBoardNotFound
	}
	var newPost *entity.Post
	if draft {
		newPost = entity.NewDraftPost(title, content, authorID, boardID)
	} else {
		newPost = entity.NewPost(title, content, authorID, boardID)
	}
	postID, err := s.postRepository.Save(newPost)
	if err != nil {
		return 0, customError.WrapRepository("save post", err)
	}
	if !draft {
		bestEffortCacheDeleteByPrefix(s.cache, key.PostListPrefix(boardID), "invalidate post list after create post")
	}
	return postID, nil
}

func (s *PostService) GetPostsList(boardID int64, limit int, lastID int64) (*model.PostList, error) {
	cacheKey := key.PostList(boardID, limit, lastID)
	value, err := s.cache.GetOrSetWithTTL(cacheKey, s.cachePolicy.ListTTLSeconds, func() (interface{}, error) {
		board, err := s.boardRepository.SelectBoardByID(boardID)
		if err != nil {
			return nil, customError.WrapRepository("select board by id for post list", err)
		}
		if board == nil {
			return nil, customError.ErrBoardNotFound
		}

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

		postModels, err := s.postsFromEntities(posts)
		if err != nil {
			return nil, err
		}

		return &model.PostList{
			Posts:      postModels,
			Limit:      limit,
			LastID:     lastID,
			HasMore:    hasMore,
			NextLastID: nextLastID,
		}, nil
	})
	if err != nil {
		return nil, normalizeCacheLoadError("load post list cache", err)
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
		comments, commentsHasMore, err := s.visibleCommentsForDetail(post.ID, commentDefaultLimit)
		if err != nil {
			return nil, err
		}
		commentDetails := make([]*model.CommentDetail, len(comments))
		for i, comment := range comments {
			commentReactions, err := s.reactionRepository.GetByTarget(comment.ID, entity.ReactionTargetComment)
			if err != nil {
				return nil, customError.WrapRepository("select comment reactions for post detail", err)
			}
			commentModel, err := s.commentFromEntity(comment)
			if err != nil {
				return nil, err
			}
			commentReactionModels, err := s.reactionsFromEntities(commentReactions)
			if err != nil {
				return nil, err
			}
			commentDetails[i] = &model.CommentDetail{
				Comment:   commentModel,
				Reactions: commentReactionModels,
			}
		}
		postModel, err := s.postFromEntity(post)
		if err != nil {
			return nil, err
		}
		attachmentEntities, err := s.attachmentRepository.SelectByPostID(post.ID)
		if err != nil {
			return nil, customError.WrapRepository("select attachments for post detail", err)
		}
		attachments := attachmentsFromEntities(attachmentEntities)
		reactionModels, err := s.reactionsFromEntities(reactions)
		if err != nil {
			return nil, err
		}
		postDetail := &model.PostDetail{
			Post:            postModel,
			Attachments:     attachments,
			Comments:        commentDetails,
			CommentsHasMore: commentsHasMore,
			Reactions:       reactionModels,
		}
		return postDetail, nil
	})
	if err != nil {
		return nil, normalizeCacheLoadError("load post detail cache", err)
	}
	detail, ok := value.(*model.PostDetail)
	if !ok {
		return nil, customError.Mark(customError.ErrCacheFailure, "decode post detail cache payload")
	}
	return detail, nil
}

func (s *PostService) PublishPost(id, authorID int64) error {
	post, err := s.postRepository.SelectPostByIDIncludingUnpublished(id)
	if err != nil {
		return customError.WrapRepository("select post by id including unpublished for publish post", err)
	}
	if post == nil {
		return customError.ErrPostNotFound
	}
	requester, err := s.userRepository.SelectUserByID(authorID)
	if err != nil {
		return customError.WrapRepository("select user by id for publish post", err)
	}
	if requester == nil {
		return customError.ErrUserNotFound
	}
	if err := s.authorizationPolicy.CanWrite(requester); err != nil {
		return err
	}
	if err := s.authorizationPolicy.OwnerOrAdmin(requester, post.AuthorID); err != nil {
		return err
	}
	if err := s.validateAttachmentRefs(post.ID, post.Content); err != nil {
		return err
	}
	if err := s.syncPostAttachmentOrphans(post.ID, post.Content); err != nil {
		return err
	}
	post.Publish()
	if err := s.postRepository.Update(post); err != nil {
		return customError.WrapRepository("publish post", err)
	}
	bestEffortCacheDeleteByPrefix(s.cache, key.PostListPrefix(post.BoardID), "invalidate post list after publish post")
	bestEffortCacheDelete(s.cache, key.PostDetail(post.ID), "invalidate post detail after publish post")
	return nil
}

func (s *PostService) postsFromEntities(posts []*entity.Post) ([]model.Post, error) {
	out := make([]model.Post, 0, len(posts))
	for _, post := range posts {
		postModel, err := s.postModelFromEntity(post)
		if err != nil {
			return nil, err
		}
		out = append(out, postModel)
	}
	return out, nil
}

func (s *PostService) postFromEntity(post *entity.Post) (*model.Post, error) {
	postModel, err := s.postModelFromEntity(post)
	if err != nil {
		return nil, err
	}
	return &postModel, nil
}

func (s *PostService) postModelFromEntity(post *entity.Post) (model.Post, error) {
	authorUUID, err := userUUIDByID(s.userRepository, post.AuthorID)
	if err != nil {
		return model.Post{}, err
	}
	out := mapper.PostFromEntity(post)
	out.AuthorUUID = authorUUID
	return out, nil
}

func (s *PostService) commentFromEntity(comment *entity.Comment) (*model.Comment, error) {
	authorUUID, err := userUUIDByID(s.userRepository, comment.AuthorID)
	if err != nil {
		return nil, err
	}
	out := mapper.CommentFromEntity(comment)
	out.AuthorUUID = authorUUID
	return &out, nil
}

func (s *PostService) reactionsFromEntities(reactions []*entity.Reaction) ([]model.Reaction, error) {
	out := make([]model.Reaction, 0, len(reactions))
	for _, reaction := range reactions {
		userUUID, err := userUUIDByID(s.userRepository, reaction.UserID)
		if err != nil {
			return nil, err
		}
		reactionModel := mapper.ReactionFromEntity(reaction)
		reactionModel.UserUUID = userUUID
		out = append(out, reactionModel)
	}
	return out, nil
}

func (s *PostService) UpdatePost(id, authorID int64, title, content string) error {
	// 게시글 수정 로직 구현
	if strings.TrimSpace(title) == "" || strings.TrimSpace(content) == "" {
		return customError.ErrInvalidInput
	}
	post, err := s.postRepository.SelectPostByIDIncludingUnpublished(id) // post 존재 여부 확인
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
	if err := s.authorizationPolicy.CanWrite(requester); err != nil {
		return err
	}
	if err := s.authorizationPolicy.OwnerOrAdmin(requester, post.AuthorID); err != nil {
		return err
	}
	if err := s.validateAttachmentRefs(post.ID, content); err != nil {
		return err
	}
	if err := s.syncPostAttachmentOrphans(post.ID, content); err != nil {
		return err
	}
	post.Update(title, content)
	err = s.postRepository.Update(post)
	if err != nil {
		return customError.WrapRepository("update post", err)
	}
	bestEffortCacheDelete(s.cache, key.PostDetail(post.ID), "invalidate post detail after update post")
	bestEffortCacheDeleteByPrefix(s.cache, key.PostListPrefix(post.BoardID), "invalidate post list after update post")
	return nil
}

func (s *PostService) DeletePost(id, authorID int64) error {
	// 게시글 삭제 로직 구현
	post, err := s.postRepository.SelectPostByIDIncludingUnpublished(id) // post 존재 여부 확인
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
	if err := s.authorizationPolicy.CanWrite(requester); err != nil {
		return err
	}
	if err := s.authorizationPolicy.OwnerOrAdmin(requester, post.AuthorID); err != nil {
		return err
	}
	err = s.postRepository.Delete(post.ID)
	if err != nil {
		return customError.WrapRepository("delete post", err)
	}
	comments, err := s.commentRepository.SelectComments(post.ID, int(^uint(0)>>1), 0)
	if err != nil {
		return customError.WrapRepository("select comments for delete post", err)
	}
	for _, comment := range comments {
		if _, err := s.reactionRepository.DeleteByTarget(comment.ID, entity.ReactionTargetComment); err != nil {
			return customError.WrapRepository("delete post comment reactions", err)
		}
		if err := s.commentRepository.Delete(comment.ID); err != nil {
			return customError.WrapRepository("soft delete post comments", err)
		}
		bestEffortCacheDelete(s.cache, key.ReactionList(string(entity.ReactionTargetComment), comment.ID), "invalidate comment reaction list after delete post")
	}
	if err := s.orphanPostAttachments(post.ID); err != nil {
		return err
	}
	if _, err := s.reactionRepository.DeleteByTarget(post.ID, entity.ReactionTargetPost); err != nil {
		return customError.WrapRepository("delete post reactions", err)
	}
	bestEffortCacheDelete(s.cache, key.PostDetail(post.ID), "invalidate post detail after delete post")
	bestEffortCacheDeleteByPrefix(s.cache, key.PostListPrefix(post.BoardID), "invalidate post list after delete post")
	bestEffortCacheDeleteByPrefix(s.cache, key.CommentListPrefix(post.ID), "invalidate comment list after delete post")
	bestEffortCacheDelete(s.cache, key.ReactionList(string(entity.ReactionTargetPost), post.ID), "invalidate post reaction list after delete post")
	return nil
}

func (s *PostService) validateAttachmentRefs(postID int64, content string) error {
	for _, attachmentID := range extractAttachmentRefIDs(content) {
		attachment, err := s.attachmentRepository.SelectByID(attachmentID)
		if err != nil {
			return customError.WrapRepository("select attachment by id for validate post attachments", err)
		}
		if attachment == nil || attachment.PostID != postID {
			return customError.ErrInvalidInput
		}
	}
	return nil
}

func extractAttachmentRefIDs(content string) []int64 {
	matches := attachmentEmbedPattern.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[int64]struct{}, len(matches))
	out := make([]int64, 0, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		id, err := strconv.ParseInt(match[1], 10, 64)
		if err != nil {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func attachmentsFromEntities(items []*entity.Attachment) []model.Attachment {
	out := make([]model.Attachment, 0, len(items))
	for _, item := range items {
		if item.IsOrphaned() {
			continue
		}
		out = append(out, model.Attachment{
			ID:          item.ID,
			PostID:      item.PostID,
			FileName:    item.FileName,
			ContentType: item.ContentType,
			SizeBytes:   item.SizeBytes,
			StorageKey:  item.StorageKey,
			CreatedAt:   item.CreatedAt,
		})
	}
	return out
}

func (s *PostService) syncPostAttachmentOrphans(postID int64, content string) error {
	items, err := s.attachmentRepository.SelectByPostID(postID)
	if err != nil {
		return customError.WrapRepository("select attachments by post id for sync orphans", err)
	}
	refIDs := make(map[int64]struct{})
	for _, id := range extractAttachmentRefIDs(content) {
		refIDs[id] = struct{}{}
	}
	for _, item := range items {
		if _, ok := refIDs[item.ID]; ok {
			item.MarkReferenced()
		} else {
			item.MarkOrphaned()
		}
		if err := s.attachmentRepository.Update(item); err != nil {
			return customError.WrapRepository("update attachment orphan state", err)
		}
	}
	return nil
}

func (s *PostService) orphanPostAttachments(postID int64) error {
	items, err := s.attachmentRepository.SelectByPostID(postID)
	if err != nil {
		return customError.WrapRepository("select attachments by post id for delete post", err)
	}
	for _, item := range items {
		item.MarkOrphaned()
		if err := s.attachmentRepository.Update(item); err != nil {
			return customError.WrapRepository("orphan attachments for delete post", err)
		}
	}
	return nil
}

func (s *PostService) visibleCommentsForDetail(postID int64, limit int) ([]*entity.Comment, bool, error) {
	comments, err := s.commentRepository.SelectCommentsIncludingDeleted(postID)
	if err != nil {
		return nil, false, customError.WrapRepository("select comments for post detail including deleted", err)
	}
	filtered := filterVisibleComments(comments, 0)
	hasMore := false
	if limit > 0 && len(filtered) > limit {
		hasMore = true
		filtered = filtered[:limit]
	}
	return filtered, hasMore, nil
}
