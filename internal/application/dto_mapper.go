package application

import (
	"github.com/hoonzinope/go-comu-bin/internal/domain/dto"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

func BoardDTOFromEntity(board *entity.Board) dto.Board {
	return dto.Board{
		ID:          board.ID,
		Name:        board.Name,
		Description: board.Description,
		CreatedAt:   board.CreatedAt,
	}
}

func BoardsDTOFromEntities(items []*entity.Board) []dto.Board {
	out := make([]dto.Board, 0, len(items))
	for _, item := range items {
		out = append(out, BoardDTOFromEntity(item))
	}
	return out
}

func PostDTOFromEntity(post *entity.Post) dto.Post {
	return dto.Post{
		ID:        post.ID,
		Title:     post.Title,
		Content:   post.Content,
		AuthorID:  post.AuthorID,
		BoardID:   post.BoardID,
		CreatedAt: post.CreatedAt,
		UpdatedAt: post.UpdatedAt,
	}
}

func PostPtrDTOFromEntity(post *entity.Post) *dto.Post {
	if post == nil {
		return nil
	}
	out := PostDTOFromEntity(post)
	return &out
}

func PostsDTOFromEntities(items []*entity.Post) []dto.Post {
	out := make([]dto.Post, 0, len(items))
	for _, item := range items {
		out = append(out, PostDTOFromEntity(item))
	}
	return out
}

func CommentDTOFromEntity(comment *entity.Comment) dto.Comment {
	return dto.Comment{
		ID:        comment.ID,
		Content:   comment.Content,
		AuthorID:  comment.AuthorID,
		PostID:    comment.PostID,
		ParentID:  comment.ParentID,
		CreatedAt: comment.CreatedAt,
	}
}

func CommentPtrDTOFromEntity(comment *entity.Comment) *dto.Comment {
	if comment == nil {
		return nil
	}
	out := CommentDTOFromEntity(comment)
	return &out
}

func CommentsDTOFromEntities(items []*entity.Comment) []dto.Comment {
	out := make([]dto.Comment, 0, len(items))
	for _, item := range items {
		out = append(out, CommentDTOFromEntity(item))
	}
	return out
}

func ReactionDTOFromEntity(reaction *entity.Reaction) dto.Reaction {
	return dto.Reaction{
		ID:         reaction.ID,
		TargetType: reaction.TargetType,
		TargetID:   reaction.TargetID,
		Type:       reaction.Type,
		UserID:     reaction.UserID,
		CreatedAt:  reaction.CreatedAt,
	}
}

func ReactionsDTOFromEntities(items []*entity.Reaction) []dto.Reaction {
	out := make([]dto.Reaction, 0, len(items))
	for _, item := range items {
		out = append(out, ReactionDTOFromEntity(item))
	}
	return out
}
