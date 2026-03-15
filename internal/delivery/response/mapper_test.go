package response

import (
	"testing"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/model"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostDetailFromDTO(t *testing.T) {
	now := time.Unix(10, 0)
	view := &model.PostDetail{
		Post: &model.Post{
			UUID:       "post-uuid",
			Title:      "hello",
			Content:    "world",
			AuthorUUID: "user-uuid",
			BoardUUID:  "board-uuid",
			CreatedAt:  now,
			UpdatedAt:  now,
		},
		Tags: []model.Tag{{ID: 3, Name: "go", CreatedAt: now}},
		Attachments: []model.Attachment{{
			UUID:        "attachment-uuid",
			PostUUID:    "post-uuid",
			FileName:    "a.png",
			ContentType: "image/png",
			SizeBytes:   5,
			CreatedAt:   now,
		}},
		Comments: []*model.CommentDetail{{
			Comment: &model.Comment{
				UUID:       "comment-uuid",
				Content:    "nice",
				AuthorUUID: "commenter-uuid",
				PostUUID:   "post-uuid",
				CreatedAt:  now,
			},
			Reactions: []model.Reaction{{
				ID:         6,
				TargetType: entity.ReactionTargetComment,
				TargetUUID: "comment-uuid",
				Type:       entity.ReactionTypeLike,
				UserUUID:   "reactor-uuid",
				CreatedAt:  now,
			}},
		}},
		CommentsHasMore: true,
		Reactions: []model.Reaction{{
			ID:         7,
			TargetType: entity.ReactionTargetPost,
			TargetUUID: "post-uuid",
			Type:       entity.ReactionTypeLike,
			UserUUID:   "reactor-uuid",
			CreatedAt:  now,
		}},
	}

	resp := PostDetailFromDTO(view)
	require.NotNil(t, resp)
	assert.Equal(t, "post-uuid", resp.Post.UUID)
	assert.Equal(t, "user-uuid", resp.Post.AuthorUUID)
	assert.Equal(t, "/api/v1/posts/post-uuid/attachments/attachment-uuid/file", resp.Attachments[0].FileURL)
	assert.Equal(t, "/api/v1/posts/post-uuid/attachments/attachment-uuid/preview", resp.Attachments[0].PreviewURL)
	assert.Equal(t, "reactor-uuid", resp.Reactions[0].UserUUID)
	assert.Equal(t, "commenter-uuid", resp.Comments[0].Comment.AuthorUUID)
	assert.True(t, resp.CommentsHasMore)
}

func TestAttachmentFromDTOUsesProvidedPreviewURL(t *testing.T) {
	item := attachmentFromDTO(model.Attachment{
		UUID:        "attachment-uuid",
		PostUUID:    "post-uuid",
		FileName:    "a.png",
		ContentType: "image/png",
		SizeBytes:   5,
		PreviewURL:  "/custom-preview",
	})

	assert.Equal(t, "/custom-preview", item.PreviewURL)
}

func TestListMappers(t *testing.T) {
	now := time.Unix(20, 0)
	boardList := BoardListFromDTO(&model.BoardList{
		Boards:     []model.Board{{UUID: "board-uuid", Name: "free", Description: "desc", CreatedAt: now}},
		Limit:      10,
		Cursor:     "cursor-5",
		HasMore:    true,
		NextCursor: stringPtr("cursor-3"),
	})
	postList := PostListFromDTO(&model.PostList{
		Posts:      []model.Post{{UUID: "post-uuid", Title: "t", Content: "c", AuthorUUID: "u", BoardUUID: "board-uuid", CreatedAt: now, UpdatedAt: now}},
		Limit:      10,
		Cursor:     "cursor-4",
		HasMore:    true,
		NextCursor: stringPtr("cursor-2"),
	})
	commentList := CommentListFromDTO(&model.CommentList{
		Comments:   []model.Comment{{UUID: "comment-uuid", Content: "nice", AuthorUUID: "u", PostUUID: "post-uuid", CreatedAt: now}},
		Limit:      10,
		Cursor:     "cursor-1",
		HasMore:    false,
		NextCursor: nil,
	})

	assert.Len(t, boardList.Boards, 1)
	assert.Len(t, postList.Posts, 1)
	assert.Len(t, commentList.Comments, 1)
	assert.Len(t, ReactionsFromDTO([]model.Reaction{{ID: 1, TargetType: entity.ReactionTargetPost, Type: entity.ReactionTypeLike}}), 1)
	assert.Len(t, AttachmentsFromDTO([]model.Attachment{{UUID: "attachment-uuid", PostUUID: "post-uuid", FileName: "a.png"}}), 1)
	assert.Equal(t, "go", TagsFromDTO([]model.Tag{{ID: 1, Name: "go"}})[0].Name)
	assert.Equal(t, "cursor-3", *boardList.NextCursor)
	assert.Equal(t, "cursor-2", *postList.NextCursor)
	assert.Equal(t, "free", boardList.Boards[0].Name)
}

func stringPtr(v string) *string {
	return &v
}
