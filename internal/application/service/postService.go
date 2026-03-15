package service

import (
	"context"
	"errors"
	"log/slog"
	"regexp"
	"sort"
	"strings"

	appcache "github.com/hoonzinope/go-comu-bin/internal/application/cache"
	"github.com/hoonzinope/go-comu-bin/internal/application/cache/key"
	appevent "github.com/hoonzinope/go-comu-bin/internal/application/event"
	"github.com/hoonzinope/go-comu-bin/internal/application/mapper"
	"github.com/hoonzinope/go-comu-bin/internal/application/model"
	"github.com/hoonzinope/go-comu-bin/internal/application/policy"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var (
	commentDefaultLimit = 10
	maxPostTags         = 10
	maxTagLength        = 30
	postDeleteBatchSize = 500
)

var _ port.PostUseCase = (*PostService)(nil)

type PostService struct {
	userRepository       port.UserRepository
	boardRepository      port.BoardRepository
	postRepository       port.PostRepository
	tagRepository        port.TagRepository
	postTagRepository    port.PostTagRepository
	attachmentRepository port.AttachmentRepository
	commentRepository    port.CommentRepository
	reactionRepository   port.ReactionRepository
	unitOfWork           port.UnitOfWork
	cache                port.Cache
	actionDispatcher     port.ActionHookDispatcher
	cachePolicy          appcache.Policy
	authorizationPolicy  policy.AuthorizationPolicy
	logger               *slog.Logger
	postDetailQuery      *postDetailQuery
}

var attachmentEmbedPattern = regexp.MustCompile(`!\[[^\]]*]\(attachment://([0-9a-fA-F-]+)\)`)

func NewPostService(userRepository port.UserRepository, boardRepository port.BoardRepository, postRepository port.PostRepository, tagRepository port.TagRepository, postTagRepository port.PostTagRepository, attachmentRepository port.AttachmentRepository, commentRepository port.CommentRepository, reactionRepository port.ReactionRepository, unitOfWork port.UnitOfWork, cache port.Cache, cachePolicy appcache.Policy, authorizationPolicy policy.AuthorizationPolicy, logger ...*slog.Logger) *PostService {
	return NewPostServiceWithActionDispatcher(userRepository, boardRepository, postRepository, tagRepository, postTagRepository, attachmentRepository, commentRepository, reactionRepository, unitOfWork, cache, nil, cachePolicy, authorizationPolicy, logger...)
}

func NewPostServiceWithActionDispatcher(userRepository port.UserRepository, boardRepository port.BoardRepository, postRepository port.PostRepository, tagRepository port.TagRepository, postTagRepository port.PostTagRepository, attachmentRepository port.AttachmentRepository, commentRepository port.CommentRepository, reactionRepository port.ReactionRepository, unitOfWork port.UnitOfWork, cache port.Cache, actionDispatcher port.ActionHookDispatcher, cachePolicy appcache.Policy, authorizationPolicy policy.AuthorizationPolicy, logger ...*slog.Logger) *PostService {
	svc := &PostService{
		userRepository:       userRepository,
		boardRepository:      boardRepository,
		postRepository:       postRepository,
		tagRepository:        tagRepository,
		postTagRepository:    postTagRepository,
		attachmentRepository: attachmentRepository,
		commentRepository:    commentRepository,
		reactionRepository:   reactionRepository,
		unitOfWork:           unitOfWork,
		cache:                cache,
		actionDispatcher:     resolveActionDispatcher(actionDispatcher),
		cachePolicy:          cachePolicy,
		authorizationPolicy:  authorizationPolicy,
		logger:               resolveLogger(logger),
	}
	svc.postDetailQuery = newPostDetailQuery(userRepository, boardRepository, postRepository, tagRepository, postTagRepository, attachmentRepository, commentRepository, reactionRepository)
	return svc
}

func (s *PostService) CreatePost(ctx context.Context, title, content string, tags []string, authorID int64, boardUUID string) (string, error) {
	return s.createPost(ctx, title, content, tags, authorID, boardUUID, false)
}

func (s *PostService) CreateDraftPost(ctx context.Context, title, content string, tags []string, authorID int64, boardUUID string) (string, error) {
	return s.createPost(ctx, title, content, tags, authorID, boardUUID, true)
}

func (s *PostService) createPost(ctx context.Context, title, content string, tags []string, authorID int64, boardUUID string, draft bool) (string, error) {
	if strings.TrimSpace(title) == "" || strings.TrimSpace(content) == "" {
		return "", customerror.ErrInvalidInput
	}
	normalizedTags, err := normalizeTags(tags)
	if err != nil {
		return "", err
	}
	if len(extractAttachmentRefIDs(content)) > 0 {
		return "", customerror.ErrInvalidInput
	}
	board, err := s.boardRepository.SelectBoardByUUID(ctx, boardUUID)
	if err != nil {
		return "", customerror.WrapRepository("select board by uuid for create post", err)
	}
	if board == nil {
		return "", customerror.ErrBoardNotFound
	}
	var newPost *entity.Post
	if draft {
		newPost = entity.NewDraftPost(title, content, authorID, board.ID)
	} else {
		newPost = entity.NewPost(title, content, authorID, board.ID)
	}

	var postID int64
	var postUUID string
	err = s.unitOfWork.WithinTransaction(ctx, func(tx port.TxScope) error {
		txCtx := tx.Context()
		user, err := tx.UserRepository().SelectUserByID(txCtx, authorID)
		if err != nil {
			return customerror.WrapRepository("select user by id for create post", err)
		}
		if user == nil {
			return customerror.ErrUserNotFound
		}
		if err := s.authorizationPolicy.CanWrite(user); err != nil {
			return err
		}
		board, err := tx.BoardRepository().SelectBoardByUUID(txCtx, boardUUID)
		if err != nil {
			return customerror.WrapRepository("select board by uuid for create post", err)
		}
		if board == nil {
			return customerror.ErrBoardNotFound
		}
		if err := policy.EnsureBoardVisible(board, user); err != nil {
			return err
		}
		var saveErr error
		postID, saveErr = tx.PostRepository().Save(txCtx, newPost)
		if saveErr != nil {
			return customerror.WrapRepository("save post", saveErr)
		}
		postUUID = newPost.UUID
		if err := s.upsertPostTags(tx, postID, normalizedTags); err != nil {
			return err
		}
		if !draft {
			if err := dispatchDomainActions(tx, s.actionDispatcher, appevent.NewPostChanged("created", postID, board.ID, normalizedTags, nil)); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return postUUID, nil
}

func (s *PostService) GetPostsList(ctx context.Context, boardUUID string, limit int, cursor string) (*model.PostList, error) {
	if err := requirePositiveLimit(limit); err != nil {
		return nil, err
	}
	lastID, err := decodeOpaqueCursor(cursor)
	if err != nil {
		return nil, err
	}
	board, err := s.boardRepository.SelectBoardByUUID(ctx, boardUUID)
	if err != nil {
		return nil, customerror.WrapRepository("select board by uuid for post list", err)
	}
	if board == nil {
		return nil, customerror.ErrBoardNotFound
	}
	cacheKey := key.PostList(board.ID, limit, lastID)
	value, err := s.cache.GetOrSetWithTTL(ctx, cacheKey, s.cachePolicy.ListTTLSeconds, func(ctx context.Context) (interface{}, error) {
		if err := policy.EnsureBoardVisible(board, nil); err != nil {
			return nil, err
		}

		fetchLimit := limit
		if limit > 0 {
			fetchLimit = limit + 1
		}
		posts, err := s.postRepository.SelectPosts(ctx, board.ID, fetchLimit, lastID)
		if err != nil {
			return nil, customerror.WrapRepository("select posts by board", err)
		}

		hasMore := false
		var nextCursor *string
		if len(posts) > limit {
			hasMore = true
			posts = posts[:limit]
		}
		if hasMore && len(posts) > 0 {
			next := encodeOpaqueCursor(posts[len(posts)-1].ID)
			nextCursor = &next
		}

		postModels, err := s.postsFromEntities(ctx, posts)
		if err != nil {
			return nil, err
		}
		return &model.PostList{
			Posts:      postModels,
			Limit:      limit,
			Cursor:     cursor,
			HasMore:    hasMore,
			NextCursor: nextCursor,
		}, nil
	})
	if err != nil {
		return nil, normalizeCacheLoadError("load post list cache", err)
	}
	list, ok := value.(*model.PostList)
	if !ok {
		return nil, customerror.Mark(customerror.ErrCacheFailure, "decode post list cache payload")
	}
	return list, nil
}

func (s *PostService) GetPostsByTag(ctx context.Context, tagName string, limit int, cursor string) (*model.PostList, error) {
	if err := requirePositiveLimit(limit); err != nil {
		return nil, err
	}
	lastID, err := decodeOpaqueCursor(cursor)
	if err != nil {
		return nil, err
	}
	normalizedName := normalizeTagName(tagName)
	if normalizedName == "" || len(normalizedName) > maxTagLength {
		return nil, customerror.ErrInvalidInput
	}

	cacheKey := key.TagPostList(normalizedName, limit, lastID)
	value, err := s.cache.GetOrSetWithTTL(ctx, cacheKey, s.cachePolicy.ListTTLSeconds, func(ctx context.Context) (interface{}, error) {
		return s.loadPublishedPostsByTag(ctx, normalizedName, limit, lastID, cursor)
	})
	if err != nil {
		return nil, normalizeCacheLoadError("load tag post list cache", err)
	}
	list, ok := value.(*model.PostList)
	if !ok {
		return nil, customerror.Mark(customerror.ErrCacheFailure, "decode tag post list cache payload")
	}
	return list, nil
}

func (s *PostService) GetPostDetail(ctx context.Context, postUUID string) (*model.PostDetail, error) {
	post, err := s.postRepository.SelectPostByUUID(ctx, postUUID)
	if err != nil {
		return nil, customerror.WrapRepository("select post by uuid for post detail cache key", err)
	}
	if post == nil {
		return nil, customerror.ErrPostNotFound
	}
	cacheKey := key.PostDetail(post.ID)
	value, err := s.cache.GetOrSetWithTTL(ctx, cacheKey, s.cachePolicy.DetailTTLSeconds, func(ctx context.Context) (interface{}, error) {
		detail, err := s.postDetailQuery.Load(ctx, post.ID)
		if err != nil {
			return nil, err
		}
		board, err := s.boardRepository.SelectBoardByUUID(ctx, detail.Post.BoardUUID)
		if err != nil {
			return nil, customerror.WrapRepository("select board by uuid for post detail visibility", err)
		}
		if err := policy.EnsureBoardVisible(board, nil); err != nil {
			return nil, customerror.ErrPostNotFound
		}
		return detail, nil
	})
	if err != nil {
		return nil, normalizeCacheLoadError("load post detail cache", err)
	}
	detail, ok := value.(*model.PostDetail)
	if !ok {
		return nil, customerror.Mark(customerror.ErrCacheFailure, "decode post detail cache payload")
	}
	return detail, nil
}

func (s *PostService) PublishPost(ctx context.Context, postUUID string, authorID int64) error {
	var boardID int64
	var postID int64
	var currentTags []string
	err := s.unitOfWork.WithinTransaction(ctx, func(tx port.TxScope) error {
		txCtx := tx.Context()
		post, err := tx.PostRepository().SelectPostByUUIDIncludingUnpublished(txCtx, postUUID)
		if err != nil {
			return customerror.WrapRepository("select post by id including unpublished for publish post", err)
		}
		if post == nil {
			return customerror.ErrPostNotFound
		}
		requester, err := tx.UserRepository().SelectUserByID(txCtx, authorID)
		if err != nil {
			return customerror.WrapRepository("select user by id for publish post", err)
		}
		if requester == nil {
			return customerror.ErrUserNotFound
		}
		if err := s.authorizationPolicy.CanWrite(requester); err != nil {
			return err
		}
		if err := s.authorizationPolicy.OwnerOrAdmin(requester, post.AuthorID); err != nil {
			return err
		}
		if err := s.validateAttachmentRefsWithRepo(txCtx, tx.AttachmentRepository(), post.ID, post.Content); err != nil {
			return err
		}
		currentTags, err = s.activeTagNamesByPostIDTx(tx, post.ID)
		if err != nil {
			return err
		}
		if syncErr := s.syncPostAttachmentOrphans(txCtx, tx.AttachmentRepository(), post.ID, post.Content); syncErr != nil {
			return syncErr
		}
		publishedPost := *post
		publishedPost.Publish()
		if updateErr := tx.PostRepository().Update(txCtx, &publishedPost); updateErr != nil {
			return customerror.WrapRepository("publish post", updateErr)
		}
		boardID = post.BoardID
		postID = post.ID
		if err := dispatchDomainActions(tx, s.actionDispatcher, appevent.NewPostChanged("published", postID, boardID, currentTags, nil)); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func (s *PostService) postsFromEntities(ctx context.Context, posts []*entity.Post) ([]model.Post, error) {
	authorUUIDs, err := userUUIDsForPosts(ctx, s.userRepository, posts)
	if err != nil {
		return nil, err
	}
	boardIDs := make([]int64, 0, len(posts))
	seen := make(map[int64]struct{}, len(posts))
	for _, post := range posts {
		if _, ok := seen[post.BoardID]; ok {
			continue
		}
		seen[post.BoardID] = struct{}{}
		boardIDs = append(boardIDs, post.BoardID)
	}
	boardsByID, err := s.boardRepository.SelectBoardsByIDs(ctx, boardIDs)
	if err != nil {
		return nil, customerror.WrapRepository("select boards by ids for posts", err)
	}
	out := make([]model.Post, 0, len(posts))
	for _, post := range posts {
		board := boardsByID[post.BoardID]
		if board == nil {
			return nil, customerror.WrapRepository("select boards by ids including missing", errors.New("post board not found"))
		}
		postModel, err := postModelFromEntity(post, board.UUID, authorUUIDs)
		if err != nil {
			return nil, err
		}
		out = append(out, postModel)
	}
	return out, nil
}

func (s *PostService) postFromEntity(ctx context.Context, post *entity.Post) (*model.Post, error) {
	authorUUIDs, err := userUUIDsByIDs(ctx, s.userRepository, []int64{post.AuthorID})
	if err != nil {
		return nil, err
	}
	board, err := s.boardRepository.SelectBoardByID(ctx, post.BoardID)
	if err != nil {
		return nil, customerror.WrapRepository("select board by id for post model", err)
	}
	if board == nil {
		return nil, customerror.ErrBoardNotFound
	}
	postModel, err := postModelFromEntity(post, board.UUID, authorUUIDs)
	if err != nil {
		return nil, err
	}
	return &postModel, nil
}

func (s *PostService) commentFromEntity(ctx context.Context, comment *entity.Comment) (*model.Comment, error) {
	authorUUIDs, err := userUUIDsByIDs(ctx, s.userRepository, []int64{comment.AuthorID})
	if err != nil {
		return nil, err
	}
	post, err := s.postRepository.SelectPostByIDIncludingUnpublished(ctx, comment.PostID)
	if err != nil {
		return nil, customerror.WrapRepository("select post by id for comment model", err)
	}
	if post == nil {
		return nil, customerror.ErrPostNotFound
	}
	parentUUIDs, err := commentParentUUIDsByID(ctx, s.commentRepository, []*entity.Comment{comment})
	if err != nil {
		return nil, err
	}
	return commentModelFromEntity(comment, post.UUID, authorUUIDs, parentUUIDs)
}

func (s *PostService) reactionsFromEntities(ctx context.Context, reactions []*entity.Reaction) ([]model.Reaction, error) {
	userUUIDs, err := userUUIDsForReactions(ctx, s.userRepository, reactions)
	if err != nil {
		return nil, err
	}
	return reactionsFromEntitiesWithTargetUUID(reactions, "", userUUIDs)
}

func (s *PostService) UpdatePost(ctx context.Context, postUUID string, authorID int64, title, content string, tags []string) error {
	if strings.TrimSpace(title) == "" || strings.TrimSpace(content) == "" {
		return customerror.ErrInvalidInput
	}
	normalizedTags, err := normalizeTags(tags)
	if err != nil {
		return err
	}

	var postID, boardID int64
	var currentTagNames []string
	err = s.unitOfWork.WithinTransaction(ctx, func(tx port.TxScope) error {
		txCtx := tx.Context()
		post, err := tx.PostRepository().SelectPostByUUIDIncludingUnpublished(txCtx, postUUID)
		if err != nil {
			return customerror.WrapRepository("select post by id for update post", err)
		}
		if post == nil {
			return customerror.ErrPostNotFound
		}
		requester, err := tx.UserRepository().SelectUserByID(txCtx, authorID)
		if err != nil {
			return customerror.WrapRepository("select user by id for update post", err)
		}
		if requester == nil {
			return customerror.ErrUserNotFound
		}
		if err := s.authorizationPolicy.CanWrite(requester); err != nil {
			return err
		}
		if err := s.authorizationPolicy.OwnerOrAdmin(requester, post.AuthorID); err != nil {
			return err
		}
		if err := s.validateAttachmentRefsWithRepo(txCtx, tx.AttachmentRepository(), post.ID, content); err != nil {
			return err
		}
		currentTagNames, err = s.activeTagNamesByPostIDTx(tx, post.ID)
		if err != nil {
			return err
		}
		if syncErr := s.syncPostAttachmentOrphans(txCtx, tx.AttachmentRepository(), post.ID, content); syncErr != nil {
			return syncErr
		}
		updatedPost := *post
		updatedPost.Update(title, content)
		if updateErr := tx.PostRepository().Update(txCtx, &updatedPost); updateErr != nil {
			return customerror.WrapRepository("update post", updateErr)
		}
		if err := s.syncPostTags(tx, post.ID, normalizedTags); err != nil {
			return err
		}
		postID = post.ID
		boardID = post.BoardID
		if err := dispatchDomainActions(tx, s.actionDispatcher, appevent.NewPostChanged("updated", postID, boardID, unionTagNames(currentTagNames, normalizedTags), nil)); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func (s *PostService) DeletePost(ctx context.Context, postUUID string, authorID int64) error {
	var postID, boardID int64
	var currentTagNames []string
	err := s.unitOfWork.WithinTransaction(ctx, func(tx port.TxScope) error {
		txCtx := tx.Context()
		post, err := tx.PostRepository().SelectPostByUUIDIncludingUnpublished(txCtx, postUUID)
		if err != nil {
			return customerror.WrapRepository("select post by uuid for delete post", err)
		}
		if post == nil {
			return customerror.ErrPostNotFound
		}
		requester, err := tx.UserRepository().SelectUserByID(txCtx, authorID)
		if err != nil {
			return customerror.WrapRepository("select user by id for delete post", err)
		}
		if requester == nil {
			return customerror.ErrUserNotFound
		}
		if err := s.authorizationPolicy.CanWrite(requester); err != nil {
			return err
		}
		if err := s.authorizationPolicy.OwnerOrAdmin(requester, post.AuthorID); err != nil {
			return err
		}
		currentTagNames, err = s.activeTagNamesByPostIDTx(tx, post.ID)
		if err != nil {
			return err
		}
		if err := s.deletePostCommentsInBatches(tx, post.ID); err != nil {
			return err
		}
		if deleteErr := tx.PostRepository().Delete(txCtx, post.ID); deleteErr != nil {
			return customerror.WrapRepository("delete post", deleteErr)
		}
		if deleteErr := tx.PostTagRepository().SoftDeleteByPostID(txCtx, post.ID); deleteErr != nil {
			return customerror.WrapRepository("soft delete post tags", deleteErr)
		}
		if orphanErr := s.orphanPostAttachments(txCtx, tx.AttachmentRepository(), post.ID); orphanErr != nil {
			return orphanErr
		}
		if _, reactionErr := tx.ReactionRepository().DeleteByTarget(txCtx, post.ID, entity.ReactionTargetPost); reactionErr != nil {
			return customerror.WrapRepository("delete post reactions", reactionErr)
		}
		postID = post.ID
		boardID = post.BoardID
		if err := dispatchDomainActions(tx, s.actionDispatcher, appevent.NewPostChanged("deleted", postID, boardID, currentTagNames, nil)); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func (s *PostService) deletePostCommentsInBatches(tx port.TxScope, postID int64) error {
	txCtx := tx.Context()
	lastID := int64(0)
	for {
		comments, err := tx.CommentRepository().SelectComments(txCtx, postID, postDeleteBatchSize, lastID)
		if err != nil {
			return customerror.WrapRepository("select comments for delete post", err)
		}
		if len(comments) == 0 {
			return nil
		}
		for _, comment := range comments {
			if _, reactionErr := tx.ReactionRepository().DeleteByTarget(txCtx, comment.ID, entity.ReactionTargetComment); reactionErr != nil {
				return customerror.WrapRepository("delete post comment reactions", reactionErr)
			}
			if deleteErr := tx.CommentRepository().Delete(txCtx, comment.ID); deleteErr != nil {
				return customerror.WrapRepository("soft delete post comments", deleteErr)
			}
		}
		if len(comments) < postDeleteBatchSize {
			return nil
		}
		lastID = comments[len(comments)-1].ID
	}
}

func (s *PostService) activeTagNamesByPostIDTx(tx port.TxScope, postID int64) ([]string, error) {
	tags, err := s.tagsForPostTx(tx, postID)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(tags))
	for _, tag := range tags {
		names = append(names, tag.Name)
	}
	return names, nil
}

func (s *PostService) tagsForPostTx(tx port.TxScope, postID int64) ([]model.Tag, error) {
	txCtx := tx.Context()
	relations, err := tx.PostTagRepository().SelectActiveByPostID(txCtx, postID)
	if err != nil {
		return nil, customerror.WrapRepository("select active tags by post id", err)
	}
	if len(relations) == 0 {
		return []model.Tag{}, nil
	}
	tagIDs := make([]int64, 0, len(relations))
	for _, relation := range relations {
		tagIDs = append(tagIDs, relation.TagID)
	}
	tags, err := tx.TagRepository().SelectByIDs(txCtx, tagIDs)
	if err != nil {
		return nil, customerror.WrapRepository("select tags by ids", err)
	}
	sort.Slice(tags, func(i, j int) bool {
		return tags[i].Name < tags[j].Name
	})
	return mapper.TagsFromEntities(tags), nil
}

func (s *PostService) loadPublishedPostsByTag(ctx context.Context, normalizedName string, limit int, lastID int64, cursorValue string) (*model.PostList, error) {
	tag, err := s.tagRepository.SelectByName(ctx, normalizedName)
	if err != nil {
		return nil, customerror.WrapRepository("select tag by name for post list", err)
	}
	if tag == nil {
		return nil, customerror.ErrTagNotFound
	}

	fetchLimit, err := cursorFetchLimit(limit)
	if err != nil {
		return nil, err
	}
	cursor := lastID
	visiblePosts := make([]*entity.Post, 0, fetchLimit)
	boardVisibility := make(map[int64]bool)

	for len(visiblePosts) < fetchLimit {
		publishedPosts, err := s.postRepository.SelectPublishedPostsByTagName(ctx, normalizedName, fetchLimit, cursor)
		if err != nil {
			return nil, customerror.WrapRepository("select published posts by tag name", err)
		}
		if len(publishedPosts) == 0 {
			break
		}

		if err := s.resolveBoardVisibility(ctx, publishedPosts, boardVisibility); err != nil {
			return nil, err
		}

		for _, post := range publishedPosts {
			if boardVisibility[post.BoardID] {
				visiblePosts = append(visiblePosts, post)
				if len(visiblePosts) >= fetchLimit {
					break
				}
			}
		}
		if len(visiblePosts) >= fetchLimit || len(publishedPosts) < fetchLimit {
			break
		}
		cursor = publishedPosts[len(publishedPosts)-1].ID
	}

	hasMore := false
	var nextCursor *string
	if len(visiblePosts) > limit {
		hasMore = true
		visiblePosts = visiblePosts[:limit]
	}
	if hasMore && len(visiblePosts) > 0 {
		next := encodeOpaqueCursor(visiblePosts[len(visiblePosts)-1].ID)
		nextCursor = &next
	}
	postModels, err := s.postsFromEntities(ctx, visiblePosts)
	if err != nil {
		return nil, err
	}
	return &model.PostList{
		Posts:      postModels,
		Limit:      limit,
		Cursor:     cursorValue,
		HasMore:    hasMore,
		NextCursor: nextCursor,
	}, nil
}

func (s *PostService) resolveBoardVisibility(ctx context.Context, posts []*entity.Post, boardVisibility map[int64]bool) error {
	boardIDsToFetch := make(map[int64]struct{}, len(posts))
	for _, post := range posts {
		if _, cached := boardVisibility[post.BoardID]; cached {
			continue
		}
		boardIDsToFetch[post.BoardID] = struct{}{}
	}
	if len(boardIDsToFetch) == 0 {
		return nil
	}
	uncachedBoardIDs := make([]int64, 0, len(boardIDsToFetch))
	for boardID := range boardIDsToFetch {
		uncachedBoardIDs = append(uncachedBoardIDs, boardID)
	}
	boardsByID, err := s.boardRepository.SelectBoardsByIDs(ctx, uncachedBoardIDs)
	if err != nil {
		return customerror.WrapRepository("select boards by ids for tag post visibility", err)
	}
	for _, boardID := range uncachedBoardIDs {
		boardVisibility[boardID] = policy.EnsureBoardVisible(boardsByID[boardID], nil) == nil
	}
	return nil
}

func (s *PostService) syncPostTags(tx port.TxScope, postID int64, normalizedTags []string) error {
	txCtx := tx.Context()
	currentRelations, err := tx.PostTagRepository().SelectActiveByPostID(txCtx, postID)
	if err != nil {
		return customerror.WrapRepository("select active post tags for sync", err)
	}

	targetTagIDs := make(map[int64]struct{}, len(normalizedTags))
	for _, tagName := range normalizedTags {
		tagID, resolveErr := s.getOrCreateTagID(tx, tagName)
		if resolveErr != nil {
			return resolveErr
		}
		targetTagIDs[tagID] = struct{}{}
		if upsertErr := tx.PostTagRepository().UpsertActive(txCtx, postID, tagID); upsertErr != nil {
			return customerror.WrapRepository("upsert active post tag", upsertErr)
		}
	}
	for _, relation := range currentRelations {
		if _, ok := targetTagIDs[relation.TagID]; ok {
			continue
		}
		if deleteErr := tx.PostTagRepository().SoftDelete(txCtx, postID, relation.TagID); deleteErr != nil {
			return customerror.WrapRepository("soft delete post tag", deleteErr)
		}
	}
	return nil
}

func (s *PostService) upsertPostTags(tx port.TxScope, postID int64, normalizedTags []string) error {
	for _, tagName := range normalizedTags {
		tagID, err := s.getOrCreateTagID(tx, tagName)
		if err != nil {
			return err
		}
		if err := tx.PostTagRepository().UpsertActive(tx.Context(), postID, tagID); err != nil {
			return customerror.WrapRepository("upsert post tag relation", err)
		}
	}
	return nil
}

func (s *PostService) getOrCreateTagID(tx port.TxScope, tagName string) (int64, error) {
	tag, err := tx.TagRepository().SelectByName(tx.Context(), tagName)
	if err != nil {
		return 0, customerror.WrapRepository("select tag by name", err)
	}
	if tag != nil {
		return tag.ID, nil
	}
	tagID, err := tx.TagRepository().Save(tx.Context(), entity.NewTag(tagName))
	if err != nil {
		return 0, customerror.WrapRepository("save tag", err)
	}
	return tagID, nil
}

func normalizeTags(tags []string) ([]string, error) {
	if len(tags) > maxPostTags {
		return nil, customerror.ErrInvalidInput
	}
	seen := make(map[string]struct{}, len(tags))
	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		normalized := normalizeTagName(tag)
		if normalized == "" || len(normalized) > maxTagLength {
			return nil, customerror.ErrInvalidInput
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	if len(out) > maxPostTags {
		return nil, customerror.ErrInvalidInput
	}
	sort.Strings(out)
	return out, nil
}

func normalizeTagName(tag string) string {
	return strings.ToLower(strings.TrimSpace(tag))
}

func unionTagNames(left, right []string) []string {
	seen := make(map[string]struct{}, len(left)+len(right))
	out := make([]string, 0, len(left)+len(right))
	for _, item := range append(append([]string{}, left...), right...) {
		if item == "" {
			continue
		}
		if _, exists := seen[item]; exists {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	sort.Strings(out)
	return out
}

func (s *PostService) validateAttachmentRefs(ctx context.Context, postID int64, content string) error {
	return s.validateAttachmentRefsWithRepo(ctx, s.attachmentRepository, postID, content)
}

func (s *PostService) validateAttachmentRefsWithRepo(ctx context.Context, repo port.AttachmentRepository, postID int64, content string) error {
	for _, attachmentUUID := range extractAttachmentRefIDs(content) {
		attachment, err := repo.SelectByUUID(ctx, attachmentUUID)
		if err != nil {
			return customerror.WrapRepository("select attachment by uuid for validate post attachments", err)
		}
		if attachment == nil || attachment.PostID != postID || attachment.IsPendingDelete() {
			return customerror.ErrInvalidInput
		}
	}
	return nil
}

func extractAttachmentRefIDs(content string) []string {
	matches := attachmentEmbedPattern.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(matches))
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		attachmentUUID := strings.TrimSpace(match[1])
		if attachmentUUID == "" {
			continue
		}
		if _, exists := seen[attachmentUUID]; exists {
			continue
		}
		seen[attachmentUUID] = struct{}{}
		out = append(out, attachmentUUID)
	}
	return out
}

func attachmentsFromEntities(postUUID string, items []*entity.Attachment) []model.Attachment {
	out := make([]model.Attachment, 0, len(items))
	for _, item := range items {
		if item.IsOrphaned() || item.IsPendingDelete() {
			continue
		}
		out = append(out, model.Attachment{
			UUID:        item.UUID,
			PostUUID:    postUUID,
			FileName:    item.FileName,
			ContentType: item.ContentType,
			SizeBytes:   item.SizeBytes,
			StorageKey:  item.StorageKey,
			CreatedAt:   item.CreatedAt,
		})
	}
	return out
}

func (s *PostService) syncPostAttachmentOrphans(ctx context.Context, repo port.AttachmentRepository, postID int64, content string) error {
	items, err := repo.SelectByPostID(ctx, postID)
	if err != nil {
		return customerror.WrapRepository("select attachments by post id for sync orphans", err)
	}
	refIDs := make(map[string]struct{})
	for _, attachmentUUID := range extractAttachmentRefIDs(content) {
		refIDs[attachmentUUID] = struct{}{}
	}
	for _, item := range items {
		if _, ok := refIDs[item.UUID]; ok {
			item.MarkReferenced()
		} else {
			item.MarkOrphaned()
		}
		if err := repo.Update(ctx, item); err != nil {
			return customerror.WrapRepository("update attachment orphan state", err)
		}
	}
	return nil
}

func (s *PostService) orphanPostAttachments(ctx context.Context, repo port.AttachmentRepository, postID int64) error {
	items, err := repo.SelectByPostID(ctx, postID)
	if err != nil {
		return customerror.WrapRepository("select attachments by post id for delete post", err)
	}
	for _, item := range items {
		item.MarkOrphaned()
		if err := repo.Update(ctx, item); err != nil {
			return customerror.WrapRepository("orphan attachments for delete post", err)
		}
	}
	return nil
}
