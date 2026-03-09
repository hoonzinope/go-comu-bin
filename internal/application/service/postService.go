package service

import (
	"regexp"
	"sort"
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
	maxPostTags         = 10
	maxTagLength        = 30
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
	cachePolicy          appcache.Policy
	authorizationPolicy  policy.AuthorizationPolicy
}

var attachmentEmbedPattern = regexp.MustCompile(`!\[[^\]]*]\(attachment://([0-9]+)\)`)

func NewPostService(userRepository port.UserRepository, boardRepository port.BoardRepository, postRepository port.PostRepository, tagRepository port.TagRepository, postTagRepository port.PostTagRepository, attachmentRepository port.AttachmentRepository, commentRepository port.CommentRepository, reactionRepository port.ReactionRepository, unitOfWork port.UnitOfWork, cache port.Cache, cachePolicy appcache.Policy, authorizationPolicy policy.AuthorizationPolicy) *PostService {
	return &PostService{
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
		cachePolicy:          cachePolicy,
		authorizationPolicy:  authorizationPolicy,
	}
}

func (s *PostService) CreatePost(title, content string, tags []string, authorID, boardID int64) (int64, error) {
	return s.createPost(title, content, tags, authorID, boardID, false)
}

func (s *PostService) CreateDraftPost(title, content string, tags []string, authorID, boardID int64) (int64, error) {
	return s.createPost(title, content, tags, authorID, boardID, true)
}

func (s *PostService) createPost(title, content string, tags []string, authorID, boardID int64, draft bool) (int64, error) {
	if strings.TrimSpace(title) == "" || strings.TrimSpace(content) == "" {
		return 0, customError.ErrInvalidInput
	}
	normalizedTags, err := normalizeTags(tags)
	if err != nil {
		return 0, err
	}
	if len(extractAttachmentRefIDs(content)) > 0 {
		return 0, customError.ErrInvalidInput
	}
	var newPost *entity.Post
	if draft {
		newPost = entity.NewDraftPost(title, content, authorID, boardID)
	} else {
		newPost = entity.NewPost(title, content, authorID, boardID)
	}

	var postID int64
	err = s.unitOfWork.WithinTransaction(func(tx port.TxScope) error {
		user, err := tx.UserRepository().SelectUserByID(authorID)
		if err != nil {
			return customError.WrapRepository("select user by id for create post", err)
		}
		if user == nil {
			return customError.ErrUserNotFound
		}
		if err := s.authorizationPolicy.CanWrite(user); err != nil {
			return err
		}
		board, err := tx.BoardRepository().SelectBoardByID(boardID)
		if err != nil {
			return customError.WrapRepository("select board by id for create post", err)
		}
		if board == nil {
			return customError.ErrBoardNotFound
		}
		var saveErr error
		postID, saveErr = tx.PostRepository().Save(newPost)
		if saveErr != nil {
			return customError.WrapRepository("save post", saveErr)
		}
		return s.upsertPostTags(tx, postID, normalizedTags)
	})
	if err != nil {
		return 0, err
	}
	if !draft {
		bestEffortCacheDeleteByPrefix(s.cache, key.PostListPrefix(boardID), "invalidate post list after create post")
		s.invalidateTagPostListCaches(normalizedTags)
	}
	return postID, nil
}

func (s *PostService) GetPostsList(boardID int64, limit int, lastID int64) (*model.PostList, error) {
	if err := requirePositiveLimit(limit); err != nil {
		return nil, err
	}
	cacheKey := key.PostList(boardID, limit, lastID)
	value, err := s.cache.GetOrSetWithTTL(cacheKey, s.cachePolicy.ListTTLSeconds, func() (interface{}, error) {
		board, err := s.boardRepository.SelectBoardByID(boardID)
		if err != nil {
			return nil, customError.WrapRepository("select board by id for post list", err)
		}
		if board == nil {
			return nil, customError.ErrBoardNotFound
		}

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
		if len(posts) > limit {
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

func (s *PostService) GetPostsByTag(tagName string, limit int, lastID int64) (*model.PostList, error) {
	if err := requirePositiveLimit(limit); err != nil {
		return nil, err
	}
	normalizedName := normalizeTagName(tagName)
	if normalizedName == "" || len(normalizedName) > maxTagLength {
		return nil, customError.ErrInvalidInput
	}

	cacheKey := key.TagPostList(normalizedName, limit, lastID)
	value, err := s.cache.GetOrSetWithTTL(cacheKey, s.cachePolicy.ListTTLSeconds, func() (interface{}, error) {
		return s.loadPublishedPostsByTag(normalizedName, limit, lastID)
	})
	if err != nil {
		return nil, normalizeCacheLoadError("load tag post list cache", err)
	}
	list, ok := value.(*model.PostList)
	if !ok {
		return nil, customError.Mark(customError.ErrCacheFailure, "decode tag post list cache payload")
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
		tags, err := s.tagsForPost(post.ID)
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

		return &model.PostDetail{
			Post:            postModel,
			Tags:            tags,
			Attachments:     attachments,
			Comments:        commentDetails,
			CommentsHasMore: commentsHasMore,
			Reactions:       reactionModels,
		}, nil
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
	var boardID int64
	var postID int64
	var currentTags []string
	err := s.unitOfWork.WithinTransaction(func(tx port.TxScope) error {
		post, err := tx.PostRepository().SelectPostByIDIncludingUnpublished(id)
		if err != nil {
			return customError.WrapRepository("select post by id including unpublished for publish post", err)
		}
		if post == nil {
			return customError.ErrPostNotFound
		}
		requester, err := tx.UserRepository().SelectUserByID(authorID)
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
		if err := s.validateAttachmentRefsWithRepo(tx.AttachmentRepository(), post.ID, post.Content); err != nil {
			return err
		}
		currentTags, err = s.activeTagNamesByPostIDTx(tx, post.ID)
		if err != nil {
			return err
		}
		if syncErr := s.syncPostAttachmentOrphans(tx.AttachmentRepository(), post.ID, post.Content); syncErr != nil {
			return syncErr
		}
		publishedPost := *post
		publishedPost.Publish()
		if updateErr := tx.PostRepository().Update(&publishedPost); updateErr != nil {
			return customError.WrapRepository("publish post", updateErr)
		}
		boardID = post.BoardID
		postID = post.ID
		return nil
	})
	if err != nil {
		return err
	}
	bestEffortCacheDeleteByPrefix(s.cache, key.PostListPrefix(boardID), "invalidate post list after publish post")
	bestEffortCacheDelete(s.cache, key.PostDetail(postID), "invalidate post detail after publish post")
	s.invalidateTagPostListCaches(currentTags)
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

func (s *PostService) UpdatePost(id, authorID int64, title, content string, tags []string) error {
	if strings.TrimSpace(title) == "" || strings.TrimSpace(content) == "" {
		return customError.ErrInvalidInput
	}
	normalizedTags, err := normalizeTags(tags)
	if err != nil {
		return err
	}

	var postID, boardID int64
	var currentTagNames []string
	err = s.unitOfWork.WithinTransaction(func(tx port.TxScope) error {
		post, err := tx.PostRepository().SelectPostByIDIncludingUnpublished(id)
		if err != nil {
			return customError.WrapRepository("select post by id for update post", err)
		}
		if post == nil {
			return customError.ErrPostNotFound
		}
		requester, err := tx.UserRepository().SelectUserByID(authorID)
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
		if err := s.validateAttachmentRefsWithRepo(tx.AttachmentRepository(), post.ID, content); err != nil {
			return err
		}
		currentTagNames, err = s.activeTagNamesByPostIDTx(tx, post.ID)
		if err != nil {
			return err
		}
		if syncErr := s.syncPostAttachmentOrphans(tx.AttachmentRepository(), post.ID, content); syncErr != nil {
			return syncErr
		}
		updatedPost := *post
		updatedPost.Update(title, content)
		if updateErr := tx.PostRepository().Update(&updatedPost); updateErr != nil {
			return customError.WrapRepository("update post", updateErr)
		}
		if err := s.syncPostTags(tx, post.ID, normalizedTags); err != nil {
			return err
		}
		postID = post.ID
		boardID = post.BoardID
		return nil
	})
	if err != nil {
		return err
	}
	bestEffortCacheDelete(s.cache, key.PostDetail(postID), "invalidate post detail after update post")
	bestEffortCacheDeleteByPrefix(s.cache, key.PostListPrefix(boardID), "invalidate post list after update post")
	s.invalidateTagPostListCaches(unionTagNames(currentTagNames, normalizedTags))
	return nil
}

func (s *PostService) DeletePost(id, authorID int64) error {
	var postID, boardID int64
	var currentTagNames []string
	var commentIDs []int64
	err := s.unitOfWork.WithinTransaction(func(tx port.TxScope) error {
		post, err := tx.PostRepository().SelectPostByIDIncludingUnpublished(id)
		if err != nil {
			return customError.WrapRepository("select post by id for delete post", err)
		}
		if post == nil {
			return customError.ErrPostNotFound
		}
		requester, err := tx.UserRepository().SelectUserByID(authorID)
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
		currentTagNames, err = s.activeTagNamesByPostIDTx(tx, post.ID)
		if err != nil {
			return err
		}
		comments, err := tx.CommentRepository().SelectComments(post.ID, int(^uint(0)>>1), 0)
		if err != nil {
			return customError.WrapRepository("select comments for delete post", err)
		}
		commentIDs = commentIDs[:0]
		for _, comment := range comments {
			commentIDs = append(commentIDs, comment.ID)
		}
		if deleteErr := tx.PostRepository().Delete(post.ID); deleteErr != nil {
			return customError.WrapRepository("delete post", deleteErr)
		}
		if deleteErr := tx.PostTagRepository().SoftDeleteByPostID(post.ID); deleteErr != nil {
			return customError.WrapRepository("soft delete post tags", deleteErr)
		}
		for _, comment := range comments {
			if _, reactionErr := tx.ReactionRepository().DeleteByTarget(comment.ID, entity.ReactionTargetComment); reactionErr != nil {
				return customError.WrapRepository("delete post comment reactions", reactionErr)
			}
			if deleteErr := tx.CommentRepository().Delete(comment.ID); deleteErr != nil {
				return customError.WrapRepository("soft delete post comments", deleteErr)
			}
		}
		if orphanErr := s.orphanPostAttachments(tx.AttachmentRepository(), post.ID); orphanErr != nil {
			return orphanErr
		}
		if _, reactionErr := tx.ReactionRepository().DeleteByTarget(post.ID, entity.ReactionTargetPost); reactionErr != nil {
			return customError.WrapRepository("delete post reactions", reactionErr)
		}
		postID = post.ID
		boardID = post.BoardID
		return nil
	})
	if err != nil {
		return err
	}
	bestEffortCacheDelete(s.cache, key.PostDetail(postID), "invalidate post detail after delete post")
	bestEffortCacheDeleteByPrefix(s.cache, key.PostListPrefix(boardID), "invalidate post list after delete post")
	bestEffortCacheDeleteByPrefix(s.cache, key.CommentListPrefix(postID), "invalidate comment list after delete post")
	for _, commentID := range commentIDs {
		bestEffortCacheDelete(s.cache, key.ReactionList(string(entity.ReactionTargetComment), commentID), "invalidate comment reaction list after delete post")
	}
	bestEffortCacheDelete(s.cache, key.ReactionList(string(entity.ReactionTargetPost), postID), "invalidate post reaction list after delete post")
	s.invalidateTagPostListCaches(currentTagNames)
	return nil
}

func (s *PostService) tagsForPost(postID int64) ([]model.Tag, error) {
	relations, err := s.postTagRepository.SelectActiveByPostID(postID)
	if err != nil {
		return nil, customError.WrapRepository("select active tags by post id", err)
	}
	if len(relations) == 0 {
		return []model.Tag{}, nil
	}

	tagIDs := make([]int64, 0, len(relations))
	for _, relation := range relations {
		tagIDs = append(tagIDs, relation.TagID)
	}
	tags, err := s.tagRepository.SelectByIDs(tagIDs)
	if err != nil {
		return nil, customError.WrapRepository("select tags by ids", err)
	}
	sort.Slice(tags, func(i, j int) bool {
		return tags[i].Name < tags[j].Name
	})
	return mapper.TagsFromEntities(tags), nil
}

func (s *PostService) activeTagNamesByPostID(postID int64) ([]string, error) {
	tags, err := s.tagsForPost(postID)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(tags))
	for _, tag := range tags {
		names = append(names, tag.Name)
	}
	return names, nil
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
	relations, err := tx.PostTagRepository().SelectActiveByPostID(postID)
	if err != nil {
		return nil, customError.WrapRepository("select active tags by post id", err)
	}
	if len(relations) == 0 {
		return []model.Tag{}, nil
	}
	tagIDs := make([]int64, 0, len(relations))
	for _, relation := range relations {
		tagIDs = append(tagIDs, relation.TagID)
	}
	tags, err := tx.TagRepository().SelectByIDs(tagIDs)
	if err != nil {
		return nil, customError.WrapRepository("select tags by ids", err)
	}
	sort.Slice(tags, func(i, j int) bool {
		return tags[i].Name < tags[j].Name
	})
	return mapper.TagsFromEntities(tags), nil
}

func (s *PostService) loadPublishedPostsByTag(normalizedName string, limit int, lastID int64) (*model.PostList, error) {
	tag, err := s.tagRepository.SelectByName(normalizedName)
	if err != nil {
		return nil, customError.WrapRepository("select tag by name for post list", err)
	}
	if tag == nil {
		return nil, customError.ErrTagNotFound
	}

	publishedPosts, err := s.postRepository.SelectPublishedPostsByTagName(normalizedName, limit+1, lastID)
	if err != nil {
		return nil, customError.WrapRepository("select published posts by tag name", err)
	}
	hasMore := false
	var nextLastID *int64
	if len(publishedPosts) > limit {
		hasMore = true
		publishedPosts = publishedPosts[:limit]
	}
	if hasMore && len(publishedPosts) > 0 {
		next := publishedPosts[len(publishedPosts)-1].ID
		nextLastID = &next
	}
	postModels, err := s.postsFromEntities(publishedPosts)
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
}

func (s *PostService) syncPostTags(tx port.TxScope, postID int64, normalizedTags []string) error {
	currentRelations, err := tx.PostTagRepository().SelectActiveByPostID(postID)
	if err != nil {
		return customError.WrapRepository("select active post tags for sync", err)
	}

	targetTagIDs := make(map[int64]struct{}, len(normalizedTags))
	for _, tagName := range normalizedTags {
		tagID, resolveErr := s.getOrCreateTagID(tx, tagName)
		if resolveErr != nil {
			return resolveErr
		}
		targetTagIDs[tagID] = struct{}{}
		if upsertErr := tx.PostTagRepository().UpsertActive(postID, tagID); upsertErr != nil {
			return customError.WrapRepository("upsert active post tag", upsertErr)
		}
	}
	for _, relation := range currentRelations {
		if _, ok := targetTagIDs[relation.TagID]; ok {
			continue
		}
		if deleteErr := tx.PostTagRepository().SoftDelete(postID, relation.TagID); deleteErr != nil {
			return customError.WrapRepository("soft delete post tag", deleteErr)
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
		if err := tx.PostTagRepository().UpsertActive(postID, tagID); err != nil {
			return customError.WrapRepository("upsert post tag relation", err)
		}
	}
	return nil
}

func (s *PostService) getOrCreateTagID(tx port.TxScope, tagName string) (int64, error) {
	tag, err := tx.TagRepository().SelectByName(tagName)
	if err != nil {
		return 0, customError.WrapRepository("select tag by name", err)
	}
	if tag != nil {
		return tag.ID, nil
	}
	tagID, err := tx.TagRepository().Save(entity.NewTag(tagName))
	if err != nil {
		return 0, customError.WrapRepository("save tag", err)
	}
	return tagID, nil
}

func (s *PostService) invalidateTagPostListCaches(tagNames []string) {
	for _, tagName := range tagNames {
		bestEffortCacheDeleteByPrefix(s.cache, key.TagPostListPrefix(tagName), "invalidate tag post list")
	}
}

func normalizeTags(tags []string) ([]string, error) {
	if len(tags) > maxPostTags {
		return nil, customError.ErrInvalidInput
	}
	seen := make(map[string]struct{}, len(tags))
	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		normalized := normalizeTagName(tag)
		if normalized == "" || len(normalized) > maxTagLength {
			return nil, customError.ErrInvalidInput
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	if len(out) > maxPostTags {
		return nil, customError.ErrInvalidInput
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

func (s *PostService) validateAttachmentRefs(postID int64, content string) error {
	return s.validateAttachmentRefsWithRepo(s.attachmentRepository, postID, content)
}

func (s *PostService) validateAttachmentRefsWithRepo(repo port.AttachmentRepository, postID int64, content string) error {
	for _, attachmentID := range extractAttachmentRefIDs(content) {
		attachment, err := repo.SelectByID(attachmentID)
		if err != nil {
			return customError.WrapRepository("select attachment by id for validate post attachments", err)
		}
		if attachment == nil || attachment.PostID != postID || attachment.IsPendingDelete() {
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
		if item.IsOrphaned() || item.IsPendingDelete() {
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

func (s *PostService) syncPostAttachmentOrphans(repo port.AttachmentRepository, postID int64, content string) error {
	items, err := repo.SelectByPostID(postID)
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
		if err := repo.Update(item); err != nil {
			return customError.WrapRepository("update attachment orphan state", err)
		}
	}
	return nil
}

func (s *PostService) orphanPostAttachments(repo port.AttachmentRepository, postID int64) error {
	items, err := repo.SelectByPostID(postID)
	if err != nil {
		return customError.WrapRepository("select attachments by post id for delete post", err)
	}
	for _, item := range items {
		item.MarkOrphaned()
		if err := repo.Update(item); err != nil {
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
