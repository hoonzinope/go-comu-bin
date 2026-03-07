package response

import (
	"github.com/hoonzinope/go-comu-bin/internal/domain/dto"
)

func BoardListFromDTO(list *dto.BoardList) *BoardList {
	if list == nil {
		return &BoardList{}
	}

	boards := make([]Board, 0, len(list.Boards))
	for _, board := range list.Boards {
		boards = append(boards, boardFromDTO(board))
	}

	return &BoardList{
		Boards:     boards,
		Limit:      list.Limit,
		LastID:     list.LastID,
		HasMore:    list.HasMore,
		NextLastID: list.NextLastID,
	}
}

func PostListFromDTO(list *dto.PostList) *PostList {
	if list == nil {
		return &PostList{}
	}

	posts := make([]Post, 0, len(list.Posts))
	for _, post := range list.Posts {
		posts = append(posts, postFromDTO(post))
	}

	return &PostList{
		Posts:      posts,
		Limit:      list.Limit,
		LastID:     list.LastID,
		HasMore:    list.HasMore,
		NextLastID: list.NextLastID,
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
		reactions = append(reactions, reactionFromDTO(reaction))
	}

	return &PostDetail{
		Post:      postPtrFromDTO(detail.Post),
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
		comments = append(comments, commentFromDTO(comment))
	}

	return &CommentList{
		Comments:   comments,
		Limit:      list.Limit,
		LastID:     list.LastID,
		HasMore:    list.HasMore,
		NextLastID: list.NextLastID,
	}
}

func ReactionsFromDTO(items []dto.Reaction) []Reaction {
	out := make([]Reaction, 0, len(items))
	for _, item := range items {
		out = append(out, reactionFromDTO(item))
	}
	return out
}

func boardFromDTO(board dto.Board) Board {
	return Board{
		ID:          board.ID,
		Name:        board.Name,
		Description: board.Description,
		CreatedAt:   board.CreatedAt,
	}
}

func postFromDTO(post dto.Post) Post {
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

func postPtrFromDTO(post *dto.Post) *Post {
	if post == nil {
		return nil
	}
	out := postFromDTO(*post)
	return &out
}

func commentFromDTO(comment dto.Comment) Comment {
	return Comment{
		ID:        comment.ID,
		Content:   comment.Content,
		AuthorID:  comment.AuthorID,
		PostID:    comment.PostID,
		ParentID:  comment.ParentID,
		CreatedAt: comment.CreatedAt,
	}
}

func commentPtrFromDTO(comment *dto.Comment) *Comment {
	if comment == nil {
		return nil
	}
	out := commentFromDTO(*comment)
	return &out
}

func commentDetailFromDTO(detail *dto.CommentDetail) CommentDetail {
	if detail == nil {
		return CommentDetail{}
	}
	reactions := make([]Reaction, 0, len(detail.Reactions))
	for _, reaction := range detail.Reactions {
		reactions = append(reactions, reactionFromDTO(reaction))
	}
	return CommentDetail{
		Comment:   commentPtrFromDTO(detail.Comment),
		Reactions: reactions,
	}
}

func reactionFromDTO(reaction dto.Reaction) Reaction {
	return Reaction{
		ID:         reaction.ID,
		TargetType: reaction.TargetType,
		TargetID:   reaction.TargetID,
		Type:       reaction.Type,
		UserID:     reaction.UserID,
		CreatedAt:  reaction.CreatedAt,
	}
}
