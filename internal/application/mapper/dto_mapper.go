package mapper

import (
	"github.com/hoonzinope/go-comu-bin/internal/application/model"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

func UserFromEntity(user *entity.User) model.User {
	return model.User{
		ID:              user.ID,
		UUID:            user.UUID,
		Name:            user.Name,
		Email:           user.Email,
		Guest:           user.Guest,
		GuestStatus:     user.GuestStatus,
		EmailVerifiedAt: user.EmailVerifiedAt,
		Role:            user.Role,
		Status:          user.Status,
		CreatedAt:       user.CreatedAt,
		UpdatedAt:       user.UpdatedAt,
	}
}

func BoardFromEntity(board *entity.Board) model.Board {
	return model.Board{
		UUID:        board.UUID,
		Name:        board.Name,
		Description: board.Description,
		Hidden:      board.Hidden,
		CreatedAt:   board.CreatedAt,
	}
}

func BoardsFromEntities(items []*entity.Board) []model.Board {
	out := make([]model.Board, 0, len(items))
	for _, item := range items {
		out = append(out, BoardFromEntity(item))
	}
	return out
}

func PostFromEntity(post *entity.Post) model.Post {
	return model.Post{
		UUID:      post.UUID,
		Title:     post.Title,
		Content:   post.Content,
		AuthorID:  post.AuthorID,
		CreatedAt: post.CreatedAt,
		UpdatedAt: post.UpdatedAt,
	}
}

func PostsFromEntities(items []*entity.Post) []model.Post {
	out := make([]model.Post, 0, len(items))
	for _, item := range items {
		out = append(out, PostFromEntity(item))
	}
	return out
}

func CommentFromEntity(comment *entity.Comment) model.Comment {
	return model.Comment{
		UUID:      comment.UUID,
		Content:   comment.Content,
		AuthorID:  comment.AuthorID,
		ParentID:  comment.ParentID,
		CreatedAt: comment.CreatedAt,
	}
}

func CommentsFromEntities(items []*entity.Comment) []model.Comment {
	out := make([]model.Comment, 0, len(items))
	for _, item := range items {
		out = append(out, CommentFromEntity(item))
	}
	return out
}

func ReactionFromEntity(reaction *entity.Reaction) model.Reaction {
	return model.Reaction{
		ID:         reaction.ID,
		TargetType: reaction.TargetType,
		Type:       reaction.Type,
		UserID:     reaction.UserID,
		CreatedAt:  reaction.CreatedAt,
	}
}

func TagFromEntity(tag *entity.Tag) model.Tag {
	return model.Tag{
		ID:        tag.ID,
		Name:      tag.Name,
		CreatedAt: tag.CreatedAt,
	}
}

func TagsFromEntities(items []*entity.Tag) []model.Tag {
	out := make([]model.Tag, 0, len(items))
	for _, item := range items {
		out = append(out, TagFromEntity(item))
	}
	return out
}

func ReactionsFromEntities(items []*entity.Reaction) []model.Reaction {
	out := make([]model.Reaction, 0, len(items))
	for _, item := range items {
		out = append(out, ReactionFromEntity(item))
	}
	return out
}
