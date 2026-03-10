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
			ID:         1,
			Title:      "hello",
			Content:    "world",
			AuthorUUID: "user-uuid",
			BoardID:    2,
			CreatedAt:  now,
			UpdatedAt:  now,
		},
		Tags: []model.Tag{{ID: 3, Name: "go", CreatedAt: now}},
		Attachments: []model.Attachment{{
			ID:          4,
			PostID:      1,
			FileName:    "a.png",
			ContentType: "image/png",
			SizeBytes:   5,
			CreatedAt:   now,
		}},
		Comments: []*model.CommentDetail{{
			Comment: &model.Comment{
				ID:         5,
				Content:    "nice",
				AuthorUUID: "commenter-uuid",
				PostID:     1,
				CreatedAt:  now,
			},
			Reactions: []model.Reaction{{
				ID:         6,
				TargetType: entity.ReactionTargetComment,
				TargetID:   5,
				Type:       entity.ReactionTypeLike,
				UserUUID:   "reactor-uuid",
				CreatedAt:  now,
			}},
		}},
		CommentsHasMore: true,
		Reactions: []model.Reaction{{
			ID:         7,
			TargetType: entity.ReactionTargetPost,
			TargetID:   1,
			Type:       entity.ReactionTypeLike,
			UserUUID:   "reactor-uuid",
			CreatedAt:  now,
		}},
	}

	resp := PostDetailFromDTO(view)
	require.NotNil(t, resp)
	assert.Equal(t, "user-uuid", resp.Post.AuthorUUID)
	assert.Equal(t, "/api/v1/posts/1/attachments/4/file", resp.Attachments[0].FileURL)
	assert.Equal(t, "/api/v1/posts/1/attachments/4/preview", resp.Attachments[0].PreviewURL)
	assert.Equal(t, "reactor-uuid", resp.Reactions[0].UserUUID)
	assert.Equal(t, "commenter-uuid", resp.Comments[0].Comment.AuthorUUID)
	assert.True(t, resp.CommentsHasMore)
}

func TestAttachmentFromDTOUsesProvidedPreviewURL(t *testing.T) {
	item := attachmentFromDTO(model.Attachment{
		ID:          4,
		PostID:      1,
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
		Boards:     []model.Board{{ID: 1, Name: "free", Description: "desc", CreatedAt: now}},
		Limit:      10,
		LastID:     5,
		HasMore:    true,
		NextLastID: int64Ptr(3),
	})
	postList := PostListFromDTO(&model.PostList{
		Posts:      []model.Post{{ID: 2, Title: "t", Content: "c", AuthorUUID: "u", BoardID: 1, CreatedAt: now, UpdatedAt: now}},
		Limit:      10,
		LastID:     4,
		HasMore:    true,
		NextLastID: int64Ptr(2),
	})
	commentList := CommentListFromDTO(&model.CommentList{
		Comments:   []model.Comment{{ID: 3, Content: "nice", AuthorUUID: "u", PostID: 2, CreatedAt: now}},
		Limit:      10,
		LastID:     1,
		HasMore:    false,
		NextLastID: nil,
	})

	assert.Len(t, boardList.Boards, 1)
	assert.Len(t, postList.Posts, 1)
	assert.Len(t, commentList.Comments, 1)
	assert.Len(t, ReactionsFromDTO([]model.Reaction{{ID: 1, TargetType: entity.ReactionTargetPost, Type: entity.ReactionTypeLike}}), 1)
	assert.Len(t, AttachmentsFromDTO([]model.Attachment{{ID: 1, PostID: 2, FileName: "a.png"}}), 1)
	assert.Equal(t, "go", TagsFromDTO([]model.Tag{{ID: 1, Name: "go"}})[0].Name)
	assert.Equal(t, "free", boardList.Boards[0].Name)
}

func int64Ptr(v int64) *int64 {
	return &v
}
