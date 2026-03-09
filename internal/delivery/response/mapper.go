package response

import (
	"fmt"

	"github.com/hoonzinope/go-comu-bin/internal/application/model"
)

func BoardListFromDTO(list *model.BoardList) *BoardList {
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

func PostListFromDTO(list *model.PostList) *PostList {
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

func PostDetailFromDTO(detail *model.PostDetail) *PostDetail {
	if detail == nil {
		return &PostDetail{}
	}

	attachments := make([]Attachment, 0, len(detail.Attachments))
	for _, attachment := range detail.Attachments {
		attachments = append(attachments, attachmentFromDTO(attachment))
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
		Post:            postPtrFromDTO(detail.Post),
		Tags:            TagsFromDTO(detail.Tags),
		Attachments:     attachments,
		Comments:        comments,
		CommentsHasMore: detail.CommentsHasMore,
		Reactions:       reactions,
	}
}

func CommentListFromDTO(list *model.CommentList) *CommentList {
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

func ReactionsFromDTO(items []model.Reaction) []Reaction {
	out := make([]Reaction, 0, len(items))
	for _, item := range items {
		out = append(out, reactionFromDTO(item))
	}
	return out
}

func AttachmentsFromDTO(items []model.Attachment) []Attachment {
	out := make([]Attachment, 0, len(items))
	for _, item := range items {
		out = append(out, attachmentFromDTO(item))
	}
	return out
}

func TagsFromDTO(items []model.Tag) []Tag {
	out := make([]Tag, 0, len(items))
	for _, item := range items {
		out = append(out, tagFromDTO(item))
	}
	return out
}

func boardFromDTO(board model.Board) Board {
	return Board{
		ID:          board.ID,
		Name:        board.Name,
		Description: board.Description,
		CreatedAt:   board.CreatedAt,
	}
}

func postFromDTO(post model.Post) Post {
	return Post{
		ID:         post.ID,
		Title:      post.Title,
		Content:    post.Content,
		AuthorUUID: post.AuthorUUID,
		BoardID:    post.BoardID,
		CreatedAt:  post.CreatedAt,
		UpdatedAt:  post.UpdatedAt,
	}
}

func postPtrFromDTO(post *model.Post) *Post {
	if post == nil {
		return nil
	}
	out := postFromDTO(*post)
	return &out
}

func commentFromDTO(comment model.Comment) Comment {
	return Comment{
		ID:         comment.ID,
		Content:    comment.Content,
		AuthorUUID: comment.AuthorUUID,
		PostID:     comment.PostID,
		ParentID:   comment.ParentID,
		CreatedAt:  comment.CreatedAt,
	}
}

func commentPtrFromDTO(comment *model.Comment) *Comment {
	if comment == nil {
		return nil
	}
	out := commentFromDTO(*comment)
	return &out
}

func commentDetailFromDTO(detail *model.CommentDetail) CommentDetail {
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

func reactionFromDTO(reaction model.Reaction) Reaction {
	return Reaction{
		ID:         reaction.ID,
		TargetType: string(reaction.TargetType),
		TargetID:   reaction.TargetID,
		Type:       string(reaction.Type),
		UserUUID:   reaction.UserUUID,
		CreatedAt:  reaction.CreatedAt,
	}
}

func attachmentFromDTO(attachment model.Attachment) Attachment {
	previewURL := attachment.PreviewURL
	if previewURL == "" {
		previewURL = fmt.Sprintf("/api/v1/posts/%d/attachments/%d/preview", attachment.PostID, attachment.ID)
	}
	return Attachment{
		ID:          attachment.ID,
		PostID:      attachment.PostID,
		FileName:    attachment.FileName,
		ContentType: attachment.ContentType,
		SizeBytes:   attachment.SizeBytes,
		FileURL:     fmt.Sprintf("/api/v1/posts/%d/attachments/%d/file", attachment.PostID, attachment.ID),
		PreviewURL:  previewURL,
		CreatedAt:   attachment.CreatedAt,
	}
}

func tagFromDTO(tag model.Tag) Tag {
	return Tag{
		ID:        tag.ID,
		Name:      tag.Name,
		CreatedAt: tag.CreatedAt,
	}
}
