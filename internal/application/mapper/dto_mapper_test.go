package mapper

import (
	"testing"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/stretchr/testify/assert"
)

func TestEntityMappers(t *testing.T) {
	now := time.Unix(10, 0)

	board := BoardFromEntity(&entity.Board{ID: 1, Name: "free", Description: "desc", CreatedAt: now})
	post := PostFromEntity(&entity.Post{ID: 2, Title: "t", Content: "c", AuthorID: 3, BoardID: 4, CreatedAt: now, UpdatedAt: now})
	comment := CommentFromEntity(&entity.Comment{ID: 5, Content: "nice", AuthorID: 6, PostID: 2, CreatedAt: now})
	reaction := ReactionFromEntity(&entity.Reaction{ID: 7, TargetType: entity.ReactionTargetPost, TargetID: 2, Type: entity.ReactionTypeLike, UserID: 6, CreatedAt: now})
	tag := TagFromEntity(&entity.Tag{ID: 8, Name: "go", CreatedAt: now})

	assert.Equal(t, int64(1), board.ID)
	assert.Equal(t, int64(2), post.ID)
	assert.Equal(t, int64(5), comment.ID)
	assert.Equal(t, entity.ReactionTypeLike, reaction.Type)
	assert.Equal(t, "go", tag.Name)
	assert.Len(t, BoardsFromEntities([]*entity.Board{{ID: 1}, {ID: 2}}), 2)
	assert.Len(t, PostsFromEntities([]*entity.Post{{ID: 1}}), 1)
	assert.Len(t, CommentsFromEntities([]*entity.Comment{{ID: 1}}), 1)
	assert.Len(t, ReactionsFromEntities([]*entity.Reaction{{ID: 1}}), 1)
	assert.Len(t, TagsFromEntities([]*entity.Tag{{ID: 1}}), 1)
}
