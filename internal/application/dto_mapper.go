package application

import (
	"github.com/hoonzinope/go-comu-bin/internal/application/model"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

func BoardDTOFromEntity(board *entity.Board) model.Board {
	return model.Board{
		ID:          board.ID,
		Name:        board.Name,
		Description: board.Description,
		CreatedAt:   board.CreatedAt,
	}
}

func BoardsDTOFromEntities(items []*entity.Board) []model.Board {
	out := make([]model.Board, 0, len(items))
	for _, item := range items {
		out = append(out, BoardDTOFromEntity(item))
	}
	return out
}

func PostDTOFromEntity(post *entity.Post) model.Post {
	return model.Post{
		ID:        post.ID,
		Title:     post.Title,
		Content:   post.Content,
		AuthorID:  post.AuthorID,
		BoardID:   post.BoardID,
		CreatedAt: post.CreatedAt,
		UpdatedAt: post.UpdatedAt,
	}
}

func PostPtrDTOFromEntity(post *entity.Post) *model.Post {
	if post == nil {
		return nil
	}
	out := PostDTOFromEntity(post)
	return &out
}

func PostsDTOFromEntities(items []*entity.Post) []model.Post {
	out := make([]model.Post, 0, len(items))
	for _, item := range items {
		out = append(out, PostDTOFromEntity(item))
	}
	return out
}

func CommentDTOFromEntity(comment *entity.Comment) model.Comment {
	return model.Comment{
		ID:        comment.ID,
		Content:   comment.Content,
		AuthorID:  comment.AuthorID,
		PostID:    comment.PostID,
		ParentID:  comment.ParentID,
		CreatedAt: comment.CreatedAt,
	}
}

func CommentPtrDTOFromEntity(comment *entity.Comment) *model.Comment {
	if comment == nil {
		return nil
	}
	out := CommentDTOFromEntity(comment)
	return &out
}

func CommentsDTOFromEntities(items []*entity.Comment) []model.Comment {
	out := make([]model.Comment, 0, len(items))
	for _, item := range items {
		out = append(out, CommentDTOFromEntity(item))
	}
	return out
}

func ReactionDTOFromEntity(reaction *entity.Reaction) model.Reaction {
	return model.Reaction{
		ID:         reaction.ID,
		TargetType: reaction.TargetType,
		TargetID:   reaction.TargetID,
		Type:       reaction.Type,
		UserID:     reaction.UserID,
		CreatedAt:  reaction.CreatedAt,
	}
}

func ReactionsDTOFromEntities(items []*entity.Reaction) []model.Reaction {
	out := make([]model.Reaction, 0, len(items))
	for _, item := range items {
		out = append(out, ReactionDTOFromEntity(item))
	}
	return out
}
