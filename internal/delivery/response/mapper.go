package response

import (
	"github.com/hoonzinope/go-comu-bin/internal/domain/dto"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

func BoardListFromDTO(list *dto.BoardList) *BoardList {
	if list == nil {
		return &BoardList{}
	}

	boards := make([]Board, 0, len(list.Boards))
	for _, board := range list.Boards {
		boards = append(boards, boardFromEntity(board))
	}

	return &BoardList{
		Boards: boards,
		Limit:  list.Limit,
		Offset: list.Offset,
	}
}

func PostListFromDTO(list *dto.PostList) *PostList {
	if list == nil {
		return &PostList{}
	}

	posts := make([]Post, 0, len(list.Posts))
	for _, post := range list.Posts {
		posts = append(posts, postFromEntity(post))
	}

	return &PostList{
		Posts:  posts,
		Limit:  list.Limit,
		Offset: list.Offset,
	}
}

func PostDetailFromDTO(detail *dto.PostDetail) *PostDetail {
	if detail == nil {
		return &PostDetail{}
	}

	comments := make([]CommentDetail, 0, len(detail.Comments))
	for _, comment := range detail.Comments {
		comments = append(comments, commentDetailFromDTO(comment))
	}

	reactions := make([]Reaction, 0, len(detail.Reactions))
	for _, reaction := range detail.Reactions {
		reactions = append(reactions, reactionFromEntity(reaction))
	}

	return &PostDetail{
		Post:      postPtrFromEntity(detail.Post),
		Comments:  comments,
		Reactions: reactions,
	}
}

func CommentListFromDTO(list *dto.CommentList) *CommentList {
	if list == nil {
		return &CommentList{}
	}

	comments := make([]Comment, 0, len(list.Comments))
	for _, comment := range list.Comments {
		comments = append(comments, commentFromEntity(comment))
	}

	return &CommentList{
		Comments: comments,
		Limit:    list.Limit,
		Offset:   list.Offset,
	}
}

func ReactionsFromEntities(items []*entity.Reaction) []Reaction {
	out := make([]Reaction, 0, len(items))
	for _, reaction := range items {
		out = append(out, reactionFromEntity(reaction))
	}
	return out
}

func boardFromEntity(board *entity.Board) Board {
	if board == nil {
		return Board{}
	}
	return Board{
		ID:          board.ID,
		Name:        board.Name,
		Description: board.Description,
		CreatedAt:   board.CreatedAt,
	}
}

func postFromEntity(post *entity.Post) Post {
	if post == nil {
		return Post{}
	}
	return Post{
		ID:        post.ID,
		Title:     post.Title,
		Content:   post.Content,
		AuthorID:  post.AuthorID,
		BoardID:   post.BoardID,
		CreatedAt: post.CreatedAt,
		UpdatedAt: post.UpdatedAt,
	}
}

func postPtrFromEntity(post *entity.Post) *Post {
	if post == nil {
		return nil
	}
	out := postFromEntity(post)
	return &out
}

func commentFromEntity(comment *entity.Comment) Comment {
	if comment == nil {
		return Comment{}
	}
	return Comment{
		ID:        comment.ID,
		Content:   comment.Content,
		AuthorID:  comment.AuthorID,
		PostID:    comment.PostID,
		ParentID:  comment.ParentID,
		CreatedAt: comment.CreatedAt,
	}
}

func commentPtrFromEntity(comment *entity.Comment) *Comment {
	if comment == nil {
		return nil
	}
	out := commentFromEntity(comment)
	return &out
}

func commentDetailFromDTO(detail *dto.CommentDetail) CommentDetail {
	if detail == nil {
		return CommentDetail{}
	}
	reactions := make([]Reaction, 0, len(detail.Reactions))
	for _, reaction := range detail.Reactions {
		reactions = append(reactions, reactionFromEntity(reaction))
	}
	return CommentDetail{
		Comment:   commentPtrFromEntity(detail.Comment),
		Reactions: reactions,
	}
}

func reactionFromEntity(reaction *entity.Reaction) Reaction {
	if reaction == nil {
		return Reaction{}
	}
	return Reaction{
		ID:         reaction.ID,
		TargetType: reaction.TargetType,
		TargetID:   reaction.TargetID,
		Type:       reaction.Type,
		UserID:     reaction.UserID,
		CreatedAt:  reaction.CreatedAt,
	}
}
