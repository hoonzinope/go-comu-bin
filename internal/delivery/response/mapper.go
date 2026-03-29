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
		Cursor:     list.Cursor,
		HasMore:    list.HasMore,
		NextCursor: list.NextCursor,
	}
}

func UserFromDTO(user *model.User) *User {
	if user == nil {
		return &User{}
	}
	return &User{
		ID:              user.ID,
		UUID:            user.UUID,
		Name:            user.Name,
		Email:           user.Email,
		Guest:           user.Guest,
		GuestStatus:     string(user.GuestStatus),
		EmailVerifiedAt: user.EmailVerifiedAt,
		Role:            user.Role,
		Status:          string(user.Status),
		CreatedAt:       user.CreatedAt,
		UpdatedAt:       user.UpdatedAt,
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
		Cursor:     list.Cursor,
		HasMore:    list.HasMore,
		NextCursor: list.NextCursor,
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
		Cursor:     list.Cursor,
		HasMore:    list.HasMore,
		NextCursor: list.NextCursor,
	}
}

func NotificationListFromDTO(list *model.NotificationList) *NotificationList {
	if list == nil {
		return &NotificationList{}
	}

	items := make([]Notification, 0, len(list.Notifications))
	for _, item := range list.Notifications {
		items = append(items, notificationFromDTO(item))
	}

	return &NotificationList{
		Notifications: items,
		Limit:         list.Limit,
		Cursor:        list.Cursor,
		HasMore:       list.HasMore,
		NextCursor:    list.NextCursor,
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
		UUID:        board.UUID,
		Name:        board.Name,
		Description: board.Description,
		CreatedAt:   board.CreatedAt,
	}
}

func postFromDTO(post model.Post) Post {
	return Post{
		UUID:       post.UUID,
		Title:      post.Title,
		Content:    post.Content,
		AuthorUUID: post.AuthorUUID,
		BoardUUID:  post.BoardUUID,
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
		UUID:       comment.UUID,
		Content:    comment.Content,
		AuthorUUID: comment.AuthorUUID,
		PostUUID:   comment.PostUUID,
		ParentUUID: comment.ParentUUID,
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

func notificationFromDTO(notification model.Notification) Notification {
	return Notification{
		UUID:           notification.UUID,
		Type:           string(notification.Type),
		ActorUUID:      notification.ActorUUID,
		PostUUID:       notification.PostUUID,
		CommentUUID:    notification.CommentUUID,
		ActorName:      notification.ActorName,
		PostTitle:      notification.PostTitle,
		CommentPreview: notification.CommentPreview,
		IsRead:         notification.IsRead,
		TargetKind:     notification.TargetKind,
		MessageKey:     notification.MessageKey,
		MessageArgs: MessageArgs{
			ActorName:      notification.MessageArgs.ActorName,
			PostTitle:      notification.MessageArgs.PostTitle,
			CommentPreview: notification.MessageArgs.CommentPreview,
		},
		ReadAt:    notification.ReadAt,
		CreatedAt: notification.CreatedAt,
	}
}

func reactionFromDTO(reaction model.Reaction) Reaction {
	return Reaction{
		ID:         reaction.ID,
		TargetType: string(reaction.TargetType),
		TargetUUID: reaction.TargetUUID,
		Type:       string(reaction.Type),
		UserUUID:   reaction.UserUUID,
		CreatedAt:  reaction.CreatedAt,
	}
}

func attachmentFromDTO(attachment model.Attachment) Attachment {
	previewURL := attachment.PreviewURL
	if previewURL == "" {
		previewURL = fmt.Sprintf("/api/v1/posts/%s/attachments/%s/preview", attachment.PostUUID, attachment.UUID)
	}
	return Attachment{
		UUID:        attachment.UUID,
		PostUUID:    attachment.PostUUID,
		FileName:    attachment.FileName,
		ContentType: attachment.ContentType,
		SizeBytes:   attachment.SizeBytes,
		FileURL:     fmt.Sprintf("/api/v1/posts/%s/attachments/%s/file", attachment.PostUUID, attachment.UUID),
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
