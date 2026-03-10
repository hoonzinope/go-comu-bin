package service

import (
	"errors"
	"sort"

	"github.com/hoonzinope/go-comu-bin/internal/application/mapper"
	"github.com/hoonzinope/go-comu-bin/internal/application/model"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

type postDetailQuery struct {
	userRepository       port.UserRepository
	postRepository       port.PostRepository
	tagRepository        port.TagRepository
	postTagRepository    port.PostTagRepository
	attachmentRepository port.AttachmentRepository
	commentRepository    port.CommentRepository
	reactionRepository   port.ReactionRepository
}

func newPostDetailQuery(userRepository port.UserRepository, postRepository port.PostRepository, tagRepository port.TagRepository, postTagRepository port.PostTagRepository, attachmentRepository port.AttachmentRepository, commentRepository port.CommentRepository, reactionRepository port.ReactionRepository) *postDetailQuery {
	return &postDetailQuery{
		userRepository:       userRepository,
		postRepository:       postRepository,
		tagRepository:        tagRepository,
		postTagRepository:    postTagRepository,
		attachmentRepository: attachmentRepository,
		commentRepository:    commentRepository,
		reactionRepository:   reactionRepository,
	}
}

func (q *postDetailQuery) Load(id int64) (*model.PostDetail, error) {
	post, err := q.postRepository.SelectPostByID(id)
	if err != nil {
		return nil, customError.WrapRepository("select post by id for post detail", err)
	}
	if post == nil {
		return nil, customError.ErrPostNotFound
	}

	postReactions, err := q.reactionRepository.GetByTarget(post.ID, entity.ReactionTargetPost)
	if err != nil {
		return nil, customError.WrapRepository("select post reactions for post detail", err)
	}
	comments, commentsHasMore, err := q.visibleCommentsForDetail(post.ID, commentDefaultLimit)
	if err != nil {
		return nil, err
	}
	commentIDs := make([]int64, 0, len(comments))
	for _, comment := range comments {
		commentIDs = append(commentIDs, comment.ID)
	}
	commentReactionsByID, err := q.reactionRepository.GetByTargets(commentIDs, entity.ReactionTargetComment)
	if err != nil {
		return nil, customError.WrapRepository("select comment reactions for post detail", err)
	}
	userUUIDs, err := userUUIDsForPostDetail(q.userRepository, post, comments, postReactions, commentReactionsByID)
	if err != nil {
		return nil, err
	}

	commentDetails := make([]*model.CommentDetail, len(comments))
	for i, comment := range comments {
		commentModel, err := commentModelFromEntity(comment, userUUIDs)
		if err != nil {
			return nil, err
		}
		commentReactionModels, err := reactionsFromEntitiesWithUUIDs(commentReactionsByID[comment.ID], userUUIDs)
		if err != nil {
			return nil, err
		}
		commentDetails[i] = &model.CommentDetail{
			Comment:   commentModel,
			Reactions: commentReactionModels,
		}
	}

	postModel, err := postModelFromEntity(post, userUUIDs)
	if err != nil {
		return nil, err
	}
	tags, err := tagsForPost(q.postTagRepository, q.tagRepository, post.ID)
	if err != nil {
		return nil, err
	}
	attachmentEntities, err := q.attachmentRepository.SelectByPostID(post.ID)
	if err != nil {
		return nil, customError.WrapRepository("select attachments for post detail", err)
	}
	reactionModels, err := reactionsFromEntitiesWithUUIDs(postReactions, userUUIDs)
	if err != nil {
		return nil, err
	}

	return &model.PostDetail{
		Post:            &postModel,
		Tags:            tags,
		Attachments:     attachmentsFromEntities(attachmentEntities),
		Comments:        commentDetails,
		CommentsHasMore: commentsHasMore,
		Reactions:       reactionModels,
	}, nil
}

func (q *postDetailQuery) visibleCommentsForDetail(postID int64, limit int) ([]*entity.Comment, bool, error) {
	comments, err := q.commentRepository.SelectVisibleComments(postID, limit+1, 0)
	if err != nil {
		return nil, false, customError.WrapRepository("select visible comments for post detail", err)
	}
	hasMore := false
	if limit > 0 && len(comments) > limit {
		hasMore = true
		comments = comments[:limit]
	}
	return comments, hasMore, nil
}

func userUUIDsForPostDetail(userRepository port.UserRepository, post *entity.Post, comments []*entity.Comment, postReactions []*entity.Reaction, commentReactionsByID map[int64][]*entity.Reaction) (map[int64]string, error) {
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
	return userUUIDsByIDs(userRepository, userIDs)
}

func postModelFromEntity(post *entity.Post, authorUUIDs map[int64]string) (model.Post, error) {
	authorUUID, ok := authorUUIDs[post.AuthorID]
	if !ok {
		return model.Post{}, customError.WrapRepository("select users by ids including deleted", errors.New("post author not found"))
	}
	out := mapper.PostFromEntity(post)
	out.AuthorUUID = authorUUID
	return out, nil
}

func commentModelFromEntity(comment *entity.Comment, authorUUIDs map[int64]string) (*model.Comment, error) {
	authorUUID, ok := authorUUIDs[comment.AuthorID]
	if !ok {
		return nil, customError.WrapRepository("select users by ids including deleted", errors.New("comment author not found"))
	}
	out := mapper.CommentFromEntity(comment)
	out.AuthorUUID = authorUUID
	return &out, nil
}

func reactionsFromEntitiesWithUUIDs(reactions []*entity.Reaction, userUUIDs map[int64]string) ([]model.Reaction, error) {
	out := make([]model.Reaction, 0, len(reactions))
	for _, reaction := range reactions {
		userUUID, ok := userUUIDs[reaction.UserID]
		if !ok {
			return nil, customError.WrapRepository("select users by ids including deleted", errors.New("reaction user not found"))
		}
		reactionModel := mapper.ReactionFromEntity(reaction)
		reactionModel.UserUUID = userUUID
		out = append(out, reactionModel)
	}
	return out, nil
}

func tagsForPost(postTagRepository port.PostTagRepository, tagRepository port.TagRepository, postID int64) ([]model.Tag, error) {
	relations, err := postTagRepository.SelectActiveByPostID(postID)
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
	tags, err := tagRepository.SelectByIDs(tagIDs)
	if err != nil {
		return nil, customError.WrapRepository("select tags by ids", err)
	}
	sortTagsByName(tags)
	return mapper.TagsFromEntities(tags), nil
}

func sortTagsByName(tags []*entity.Tag) {
	sort.Slice(tags, func(i, j int) bool {
		return tags[i].Name < tags[j].Name
	})
}
