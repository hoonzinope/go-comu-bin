package service

import (
	"context"
	"errors"
	"sort"

	"github.com/hoonzinope/go-comu-bin/internal/application/mapper"
	"github.com/hoonzinope/go-comu-bin/internal/application/model"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

type postDetailQuery struct {
	userRepository       port.UserRepository
	boardRepository      port.BoardRepository
	postRepository       port.PostRepository
	tagRepository        port.TagRepository
	postTagRepository    port.PostTagRepository
	attachmentRepository port.AttachmentRepository
	commentRepository    port.CommentRepository
	reactionRepository   port.ReactionRepository
}

func newPostDetailQuery(userRepository port.UserRepository, boardRepository port.BoardRepository, postRepository port.PostRepository, tagRepository port.TagRepository, postTagRepository port.PostTagRepository, attachmentRepository port.AttachmentRepository, commentRepository port.CommentRepository, reactionRepository port.ReactionRepository) *postDetailQuery {
	return &postDetailQuery{
		userRepository:       userRepository,
		boardRepository:      boardRepository,
		postRepository:       postRepository,
		tagRepository:        tagRepository,
		postTagRepository:    postTagRepository,
		attachmentRepository: attachmentRepository,
		commentRepository:    commentRepository,
		reactionRepository:   reactionRepository,
	}
}

func (q *postDetailQuery) Load(ctx context.Context, id int64) (*model.PostDetail, error) {
	post, err := q.postRepository.SelectPostByID(ctx, id)
	if err != nil {
		return nil, customerror.WrapRepository("select post by id for post detail", err)
	}
	if post == nil {
		return nil, customerror.ErrPostNotFound
	}
	board, err := q.boardRepository.SelectBoardByID(ctx, post.BoardID)
	if err != nil {
		return nil, customerror.WrapRepository("select board by id for post detail", err)
	}
	if board == nil {
		return nil, customerror.ErrBoardNotFound
	}

	postReactions, err := q.reactionRepository.GetByTarget(ctx, post.ID, entity.ReactionTargetPost)
	if err != nil {
		return nil, customerror.WrapRepository("select post reactions for post detail", err)
	}
	comments, commentsHasMore, err := q.visibleCommentsForDetail(ctx, post.ID, commentDefaultLimit)
	if err != nil {
		return nil, err
	}
	commentIDs := make([]int64, 0, len(comments))
	for _, comment := range comments {
		commentIDs = append(commentIDs, comment.ID)
	}
	commentReactionsByID, err := q.reactionRepository.GetByTargets(ctx, commentIDs, entity.ReactionTargetComment)
	if err != nil {
		return nil, customerror.WrapRepository("select comment reactions for post detail", err)
	}
	userUUIDs, err := userUUIDsForPostDetail(ctx, q.userRepository, post, comments, postReactions, commentReactionsByID)
	if err != nil {
		return nil, err
	}

	commentDetails := make([]*model.CommentDetail, len(comments))
	parentUUIDs, err := commentParentUUIDsByID(ctx, q.commentRepository, comments)
	if err != nil {
		return nil, err
	}
	for i, comment := range comments {
		commentModel, err := commentModelFromEntity(comment, post.UUID, userUUIDs, parentUUIDs)
		if err != nil {
			return nil, err
		}
		commentReactionModels, err := reactionsFromEntitiesWithTargetUUID(commentReactionsByID[comment.ID], comment.UUID, userUUIDs)
		if err != nil {
			return nil, err
		}
		commentDetails[i] = &model.CommentDetail{
			Comment:   commentModel,
			Reactions: commentReactionModels,
		}
	}

	postModel, err := postModelFromEntity(post, board.UUID, userUUIDs)
	if err != nil {
		return nil, err
	}
	tags, err := tagsForPost(ctx, q.postTagRepository, q.tagRepository, post.ID)
	if err != nil {
		return nil, err
	}
	attachmentEntities, err := q.attachmentRepository.SelectByPostID(ctx, post.ID)
	if err != nil {
		return nil, customerror.WrapRepository("select attachments for post detail", err)
	}
	reactionModels, err := reactionsFromEntitiesWithTargetUUID(postReactions, post.UUID, userUUIDs)
	if err != nil {
		return nil, err
	}

	return &model.PostDetail{
		Post:            &postModel,
		Tags:            tags,
		Attachments:     attachmentsFromEntities(post.UUID, attachmentEntities),
		Comments:        commentDetails,
		CommentsHasMore: commentsHasMore,
		Reactions:       reactionModels,
	}, nil
}

func (q *postDetailQuery) visibleCommentsForDetail(ctx context.Context, postID int64, limit int) ([]*entity.Comment, bool, error) {
	comments, err := q.commentRepository.SelectVisibleComments(ctx, postID, limit+1, 0)
	if err != nil {
		return nil, false, customerror.WrapRepository("select visible comments for post detail", err)
	}
	hasMore := false
	if limit > 0 && len(comments) > limit {
		hasMore = true
		comments = comments[:limit]
	}
	return comments, hasMore, nil
}

func userUUIDsForPostDetail(ctx context.Context, userRepository port.UserRepository, post *entity.Post, comments []*entity.Comment, postReactions []*entity.Reaction, commentReactionsByID map[int64][]*entity.Reaction) (map[int64]string, error) {
	userIDs := []int64{post.AuthorID}
	for _, comment := range comments {
		userIDs = append(userIDs, comment.AuthorID)
	}
	for _, reaction := range postReactions {
		userIDs = append(userIDs, reaction.UserID)
	}
	for _, reactions := range commentReactionsByID {
		for _, reaction := range reactions {
			userIDs = append(userIDs, reaction.UserID)
		}
	}
	return userUUIDsByIDs(ctx, userRepository, userIDs)
}

func postModelFromEntity(post *entity.Post, boardUUID string, authorUUIDs map[int64]string) (model.Post, error) {
	authorUUID, ok := authorUUIDs[post.AuthorID]
	if !ok {
		return model.Post{}, customerror.WrapRepository("select users by ids including deleted", errors.New("post author not found"))
	}
	out := mapper.PostFromEntity(post)
	out.AuthorUUID = authorUUID
	out.BoardUUID = boardUUID
	return out, nil
}

func reactionsFromEntitiesWithTargetUUID(reactions []*entity.Reaction, targetUUID string, userUUIDs map[int64]string) ([]model.Reaction, error) {
	out := make([]model.Reaction, 0, len(reactions))
	for _, reaction := range reactions {
		userUUID, ok := userUUIDs[reaction.UserID]
		if !ok {
			return nil, customerror.WrapRepository("select users by ids including deleted", errors.New("reaction user not found"))
		}
		reactionModel := mapper.ReactionFromEntity(reaction)
		reactionModel.TargetUUID = targetUUID
		reactionModel.UserUUID = userUUID
		out = append(out, reactionModel)
	}
	return out, nil
}

func tagsForPost(ctx context.Context, postTagRepository port.PostTagRepository, tagRepository port.TagRepository, postID int64) ([]model.Tag, error) {
	relations, err := postTagRepository.SelectActiveByPostID(ctx, postID)
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
	tags, err := tagRepository.SelectByIDs(ctx, tagIDs)
	if err != nil {
		return nil, customerror.WrapRepository("select tags by ids", err)
	}
	sortTagsByName(tags)
	return mapper.TagsFromEntities(tags), nil
}

func sortTagsByName(tags []*entity.Tag) {
	sort.Slice(tags, func(i, j int) bool {
		return tags[i].Name < tags[j].Name
	})
}
