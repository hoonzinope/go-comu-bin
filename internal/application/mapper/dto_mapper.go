package mapper

import (
	"github.com/hoonzinope/go-comu-bin/internal/application/model"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

func BoardFromEntity(board *entity.Board) model.Board {
	return model.Board{
		ID:          board.ID,
		Name:        board.Name,
		Description: board.Description,
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
		ID:        post.ID,
		Title:     post.Title,
		Content:   post.Content,
		AuthorID:  post.AuthorID,
		BoardID:   post.BoardID,
		CreatedAt: post.CreatedAt,
		UpdatedAt: post.UpdatedAt,
	}
}

func PostPtrFromEntity(post *entity.Post) *model.Post {
	if post == nil {
		return nil
	}
	out := PostFromEntity(post)
	return &out
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
		ID:        comment.ID,
		Content:   comment.Content,
		AuthorID:  comment.AuthorID,
		PostID:    comment.PostID,
		ParentID:  comment.ParentID,
		CreatedAt: comment.CreatedAt,
	}
}

func CommentPtrFromEntity(comment *entity.Comment) *model.Comment {
	if comment == nil {
		return nil
	}
	out := CommentFromEntity(comment)
	return &out
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
		TargetID:   reaction.TargetID,
		Type:       reaction.Type,
		UserID:     reaction.UserID,
		CreatedAt:  reaction.CreatedAt,
	}
}

func ReactionsFromEntities(items []*entity.Reaction) []model.Reaction {
	out := make([]model.Reaction, 0, len(items))
	for _, item := range items {
		out = append(out, ReactionFromEntity(item))
	}
	return out
}
