package service

import (
	"context"
	"testing"

	commentsvc "github.com/hoonzinope/go-comu-bin/internal/application/service/comment"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommentParentUUIDsByID_LoadsOnlyReferencedParents(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")
	parentID := seedComment(repositories.comment, userID, postID, "parent")
	replyID := seedCommentWithParent(repositories.comment, userID, postID, "reply", &parentID)
	reply, err := repositories.comment.SelectCommentByID(context.Background(), replyID)
	require.NoError(t, err)
	require.NotNil(t, reply)

	parentUUIDs, err := commentsvc.ParentUUIDsByID(context.Background(), repositories.comment, []*entity.Comment{reply})
	require.NoError(t, err)
	assert.Len(t, parentUUIDs, 1)

	parentMap, err := repositories.comment.SelectCommentUUIDsByIDsIncludingDeleted(context.Background(), []int64{parentID})
	require.NoError(t, err)
	assert.Equal(t, parentMap[parentID], parentUUIDs[parentID])
}
