package service

import (
	"context"
	"testing"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	postsvc "github.com/hoonzinope/go-comu-bin/internal/application/service/post"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostTagCoordinator_SyncPostTags_ReactivatesAndDeletesRelations(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")
	coordinator := postsvc.NewTagCoordinator(repositories.tag, repositories.postTag)

	require.NoError(t, repositories.unitOfWork.WithinTransaction(context.Background(), func(tx port.TxScope) error {
		return coordinator.UpsertPostTags(tx, postID, []string{"go", "backend"})
	}))
	require.NoError(t, repositories.unitOfWork.WithinTransaction(context.Background(), func(tx port.TxScope) error {
		return coordinator.SyncPostTags(tx, postID, []string{"go"})
	}))
	require.NoError(t, repositories.unitOfWork.WithinTransaction(context.Background(), func(tx port.TxScope) error {
		return coordinator.SyncPostTags(tx, postID, []string{"go", "backend"})
	}))

	var tagNames []string
	require.NoError(t, repositories.unitOfWork.WithinTransaction(context.Background(), func(tx port.TxScope) error {
		var err error
		tagNames, err = coordinator.ActiveTagNamesByPostIDTx(tx, postID)
		return err
	}))
	assert.Equal(t, []string{"backend", "go"}, tagNames)
}

func TestPostAttachmentCoordinator_ValidateAttachmentRefsWithRepo(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedDraftPost(repositories.post, userID, boardID, "title", "content")
	attachmentID, err := repositories.attachment.Save(context.Background(), entity.NewAttachment(postID, "a.png", "image/png", 10, "posts/1/a.png"))
	require.NoError(t, err)
	coordinator := postsvc.NewAttachmentCoordinator(repositories.attachment)

	err = coordinator.ValidateAttachmentRefs(context.Background(), postID, "body ![a](attachment://"+mustAttachmentUUID(t, repositories.attachment, attachmentID)+")")
	require.NoError(t, err)
}

func TestPostDeletionWorkflow_DeletePostCommentsInBatches_RemovesCommentReactions(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")
	commentID := seedComment(repositories.comment, userID, postID, "comment")
	_, _, _, err := repositories.reaction.SetUserTargetReaction(context.Background(), userID, commentID, entity.ReactionTargetComment, entity.ReactionTypeLike)
	require.NoError(t, err)
	workflow := postsvc.NewDeletionWorkflow(repositories.comment, repositories.reaction, postsvc.NewAttachmentCoordinator(repositories.attachment))

	var deletedCommentIDs []int64
	require.NoError(t, repositories.unitOfWork.WithinTransaction(context.Background(), func(tx port.TxScope) error {
		deletedCommentIDs, err = workflow.DeletePostArtifacts(tx, postID)
		return err
	}))

	assert.Equal(t, []int64{commentID}, deletedCommentIDs)
	comments, err := repositories.comment.SelectComments(context.Background(), postID, 10, 0)
	require.NoError(t, err)
	assert.Empty(t, comments)
	reactions, err := repositories.reaction.GetByTarget(context.Background(), commentID, entity.ReactionTargetComment)
	require.NoError(t, err)
	assert.Empty(t, reactions)
}

func TestPostQueryHandler_GetPostDetail(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "body")
	query := postsvc.NewQueryHandler(
		repositories.user,
		repositories.board,
		repositories.post,
		repositories.postSearch,
		repositories.postRanking,
		repositories.tag,
		repositories.postTag,
		repositories.attachment,
		repositories.comment,
		repositories.reaction,
		newTestCache(),
		newTestCachePolicy(),
	)

	detail, err := query.GetPostDetail(context.Background(), mustPostUUID(t, repositories.post, postID))
	require.NoError(t, err)
	require.NotNil(t, detail)
	assert.Equal(t, mustPostUUID(t, repositories.post, postID), detail.Post.UUID)
}

func TestPostQueryHandler_GetPostsByTag(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "body")
	require.NoError(t, repositories.unitOfWork.WithinTransaction(context.Background(), func(tx port.TxScope) error {
		return postsvc.NewTagCoordinator(repositories.tag, repositories.postTag).UpsertPostTags(tx, postID, []string{"go"})
	}))
	query := postsvc.NewQueryHandler(
		repositories.user,
		repositories.board,
		repositories.post,
		repositories.postSearch,
		repositories.postRanking,
		repositories.tag,
		repositories.postTag,
		repositories.attachment,
		repositories.comment,
		repositories.reaction,
		newTestCache(),
		newTestCachePolicy(),
	)

	list, err := query.GetPostsByTag(context.Background(), "go", "", "", 10, "")
	require.NoError(t, err)
	require.NotNil(t, list)
	require.Len(t, list.Posts, 1)
	assert.Equal(t, mustPostUUID(t, repositories.post, postID), list.Posts[0].UUID)
}
