package service

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/cache/key"
	"github.com/hoonzinope/go-comu-bin/internal/application/cache/testutil"
	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostService_UpdatePost_ForbiddenForNonOwnerNonAdmin(t *testing.T) {
	repositories := newTestRepositories()
	ownerID := seedUser(repositories.user, "owner", "pw", "user")
	otherID := seedUser(repositories.user, "other", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, ownerID, boardID, "title", "content")
	svc := newTestPostService(t, repositories, newTestCache())

	err := svc.UpdatePost(context.Background(), postID, otherID, "new-title", "new-content", nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrForbidden))
}

func TestPostService_UpdatePost_AllowedForAdmin(t *testing.T) {
	repositories := newTestRepositories()
	ownerID := seedUser(repositories.user, "owner", "pw", "user")
	adminID := seedUser(repositories.user, "admin", "pw", "admin")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, ownerID, boardID, "title", "content")
	svc := newTestPostService(t, repositories, newTestCache())

	require.NoError(t, svc.UpdatePost(context.Background(), postID, adminID, "new-title", "new-content", nil))
}

func TestPostService_CreateGetListDelete_Success(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "user", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	svc := newTestPostService(t, repositories, newTestCache())

	postID, err := svc.CreatePost(context.Background(), "title", "content", nil, userID, boardID)
	require.NoError(t, err)
	assert.NotZero(t, postID)

	list, err := svc.GetPostsList(context.Background(), boardID, 10, 0)
	require.NoError(t, err)
	assert.Len(t, list.Posts, 1)

	detail, err := svc.GetPostDetail(context.Background(), postID)
	require.NoError(t, err)
	require.NotNil(t, detail.Post)
	assert.Equal(t, postID, detail.Post.ID)

	require.NoError(t, svc.DeletePost(context.Background(), postID, userID))
}

func TestPostService_CreatePost_InvalidInput(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "user", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	svc := newTestPostService(t, repositories, newTestCache())

	_, err := svc.CreatePost(context.Background(), " ", "content", nil, userID, boardID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrInvalidInput))
}

func TestPostService_CreatePost_BlockedForSuspendedUser(t *testing.T) {
	repositories := newTestRepositories()
	user := entity.NewUser("user", "pw")
	user.Suspend("spam", nil)
	userID, err := repositories.user.Save(context.Background(), user)
	require.NoError(t, err)
	boardID := seedBoard(repositories.board, "free", "desc")
	svc := newTestPostService(t, repositories, newTestCache())

	_, err = svc.CreatePost(context.Background(), "title", "content", nil, userID, boardID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrUserSuspended))
}

func TestPostService_GetPostsList_HasMoreAndNextCursor(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "user", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	seedPost(repositories.post, userID, boardID, "title1", "content1")
	seedPost(repositories.post, userID, boardID, "title2", "content2")
	seedPost(repositories.post, userID, boardID, "title3", "content3")
	svc := newTestPostService(t, repositories, newTestCache())

	list, err := svc.GetPostsList(context.Background(), boardID, 2, 0)
	require.NoError(t, err)
	require.Len(t, list.Posts, 2)
	assert.True(t, list.HasMore)
	require.NotNil(t, list.NextLastID)
	assert.Equal(t, list.Posts[len(list.Posts)-1].ID, *list.NextLastID)
}

func TestPostService_GetPostsList_InvalidLimit(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "user", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	seedPost(repositories.post, userID, boardID, "title", "content")
	svc := newTestPostService(t, repositories, newTestCache())

	_, err := svc.GetPostsList(context.Background(), boardID, 0, 0)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrInvalidInput))
}

func TestPostService_GetPostsList_ReturnsBoardNotFound_WhenBoardDeleted(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "user", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	seedPost(repositories.post, userID, boardID, "title", "content")
	require.NoError(t, repositories.board.Delete(context.Background(), boardID))
	svc := newTestPostService(t, repositories, newTestCache())

	_, err := svc.GetPostsList(context.Background(), boardID, 10, 0)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrBoardNotFound))
}

func TestPostService_GetPostDetail_UsesCache(t *testing.T) {
	repositories := newTestRepositories()
	cache := testutil.NewSpyCache()
	postSvc := newTestPostService(t, repositories, cache)

	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")

	detail1, err := postSvc.GetPostDetail(context.Background(), postID)
	require.NoError(t, err)
	require.NotNil(t, detail1.Post)
	assert.Equal(t, "title", detail1.Post.Title)

	detail2, err := postSvc.GetPostDetail(context.Background(), postID)
	require.NoError(t, err)
	require.NotNil(t, detail2.Post)
	assert.Equal(t, "title", detail2.Post.Title)

	assert.Equal(t, 1, cache.LoadCount(key.PostDetail(postID)))
}

func TestPostService_UpdatePost_InvalidatesCaches(t *testing.T) {
	repositories := newTestRepositories()
	cache := testutil.NewSpyCache()
	postSvc := newTestPostService(t, repositories, cache)

	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")

	_, err := postSvc.GetPostDetail(context.Background(), postID)
	require.NoError(t, err)
	_, err = postSvc.GetPostsList(context.Background(), boardID, 10, 0)
	require.NoError(t, err)

	require.NoError(t, postSvc.UpdatePost(context.Background(), postID, userID, "new", "new-content", nil))

	require.Eventually(t, func() bool {
		_, ok, err := cache.Get(context.Background(), key.PostDetail(postID))
		require.NoError(t, err)
		if ok {
			return false
		}
		_, ok, err = cache.Get(context.Background(), key.PostList(boardID, 10, 0))
		require.NoError(t, err)
		return !ok
	}, time.Second, 10*time.Millisecond)
}

func TestPostService_UpdatePost_SucceedsWhenCacheInvalidationFails(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")
	svc := newTestPostService(t, repositories, &errorCache{
		deleteErr:         newCacheFailure(nil),
		deleteByPrefixErr: newCacheFailure(nil),
	})

	err := svc.UpdatePost(context.Background(), postID, userID, "new", "new-content", nil)
	require.NoError(t, err)

	post, repoErr := repositories.post.SelectPostByIDIncludingUnpublished(context.Background(), postID)
	require.NoError(t, repoErr)
	require.NotNil(t, post)
	assert.Equal(t, "new", post.Title)
	assert.Equal(t, "new-content", post.Content)
}

func TestPostService_GetPostDetail_ReturnsCacheFailure_WhenCacheLoadFails(t *testing.T) {
	repositories := newTestRepositories()
	svc := newTestPostService(t, repositories, &errorCache{
		getOrSetWithTTLErr: newCacheFailure(nil),
	})

	_, err := svc.GetPostDetail(context.Background(), 1)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrCacheFailure))
}

func TestPostService_DeletePost_SoftDeletedPostIsNoLongerVisible(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")
	svc := newTestPostService(t, repositories, newTestCache())

	require.NoError(t, svc.DeletePost(context.Background(), postID, userID))

	_, err := svc.GetPostDetail(context.Background(), postID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrPostNotFound))

	list, err := svc.GetPostsList(context.Background(), boardID, 10, 0)
	require.NoError(t, err)
	assert.Empty(t, list.Posts)
}

func TestPostService_CreatePost_NormalizesTagsAndIncludesThemInDetail(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	svc := newTestPostService(t, repositories, newTestCache())

	postID, err := svc.CreatePost(context.Background(), "title", "content", []string{" Go ", "go", "Backend"}, userID, boardID)
	require.NoError(t, err)

	detail, err := svc.GetPostDetail(context.Background(), postID)
	require.NoError(t, err)
	require.Len(t, detail.Tags, 2)
	assert.Equal(t, "backend", detail.Tags[0].Name)
	assert.Equal(t, "go", detail.Tags[1].Name)
}

func TestPostService_UpdatePost_SoftDeletesAndReactivatesTagRelations(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	svc := newTestPostService(t, repositories, newTestCache())

	postID, err := svc.CreatePost(context.Background(), "title", "content", []string{"go", "backend"}, userID, boardID)
	require.NoError(t, err)

	require.NoError(t, svc.UpdatePost(context.Background(), postID, userID, "title", "content", []string{"go"}))

	backendList, err := svc.GetPostsByTag(context.Background(), "backend", 10, 0)
	require.NoError(t, err)
	assert.Empty(t, backendList.Posts)

	require.NoError(t, svc.UpdatePost(context.Background(), postID, userID, "title", "content", []string{"GO", "backend"}))

	backendList, err = svc.GetPostsByTag(context.Background(), "backend", 10, 0)
	require.NoError(t, err)
	require.Len(t, backendList.Posts, 1)
	assert.Equal(t, postID, backendList.Posts[0].ID)
}

func TestPostService_DeletePost_RemovesPostFromTagList(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	svc := newTestPostService(t, repositories, newTestCache())

	postID, err := svc.CreatePost(context.Background(), "title", "content", []string{"go"}, userID, boardID)
	require.NoError(t, err)
	require.NoError(t, svc.DeletePost(context.Background(), postID, userID))

	list, err := svc.GetPostsByTag(context.Background(), "go", 10, 0)
	require.NoError(t, err)
	assert.Empty(t, list.Posts)
}

func TestPostService_GetPostsByTag_ExcludesDraftPosts(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	svc := newTestPostService(t, repositories, newTestCache())

	draftID, err := svc.CreateDraftPost(context.Background(), "draft", "content", []string{"go"}, userID, boardID)
	require.NoError(t, err)
	publishedID, err := svc.CreatePost(context.Background(), "post", "content", []string{"go"}, userID, boardID)
	require.NoError(t, err)
	assert.NotEqual(t, draftID, publishedID)

	list, err := svc.GetPostsByTag(context.Background(), "go", 10, 0)
	require.NoError(t, err)
	require.Len(t, list.Posts, 1)
	assert.Equal(t, publishedID, list.Posts[0].ID)
}

func TestPostService_GetPostsByTag_ExcludesHiddenBoardPosts(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "alice", "pw", "user")
	visibleBoardID := seedBoard(repositories.board, "free", "desc")
	hiddenBoardID := seedBoard(repositories.board, "secret", "desc")
	svc := newTestPostService(t, repositories, newTestCache())

	visiblePostID, err := svc.CreatePost(context.Background(), "visible", "content", []string{"go"}, userID, visibleBoardID)
	require.NoError(t, err)
	_, err = svc.CreatePost(context.Background(), "hidden", "content", []string{"go"}, userID, hiddenBoardID)
	require.NoError(t, err)

	hiddenBoard, err := repositories.board.SelectBoardByID(context.Background(), hiddenBoardID)
	require.NoError(t, err)
	require.NotNil(t, hiddenBoard)
	hiddenBoard.SetHidden(true)
	require.NoError(t, repositories.board.Update(context.Background(), hiddenBoard))

	list, err := svc.GetPostsByTag(context.Background(), "go", 10, 0)
	require.NoError(t, err)
	require.Len(t, list.Posts, 1)
	assert.Equal(t, visiblePostID, list.Posts[0].ID)
}

func TestPostService_GetPostsByTag_PaginationIgnoresHiddenBoards(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "alice", "pw", "user")
	visibleBoardID := seedBoard(repositories.board, "free", "desc")
	hiddenBoardID := seedBoard(repositories.board, "secret", "desc")
	svc := newTestPostService(t, repositories, newTestCache())

	visibleOlderID, err := svc.CreatePost(context.Background(), "visible-older", "content", []string{"go"}, userID, visibleBoardID)
	require.NoError(t, err)
	_, err = svc.CreatePost(context.Background(), "hidden-older", "content", []string{"go"}, userID, hiddenBoardID)
	require.NoError(t, err)
	visibleNewerID, err := svc.CreatePost(context.Background(), "visible-newer", "content", []string{"go"}, userID, visibleBoardID)
	require.NoError(t, err)
	_, err = svc.CreatePost(context.Background(), "hidden-newer", "content", []string{"go"}, userID, hiddenBoardID)
	require.NoError(t, err)

	hiddenBoard, err := repositories.board.SelectBoardByID(context.Background(), hiddenBoardID)
	require.NoError(t, err)
	require.NotNil(t, hiddenBoard)
	hiddenBoard.SetHidden(true)
	require.NoError(t, repositories.board.Update(context.Background(), hiddenBoard))

	firstPage, err := svc.GetPostsByTag(context.Background(), "go", 1, 0)
	require.NoError(t, err)
	require.Len(t, firstPage.Posts, 1)
	assert.Equal(t, visibleNewerID, firstPage.Posts[0].ID)
	assert.True(t, firstPage.HasMore)
	require.NotNil(t, firstPage.NextLastID)
	assert.Equal(t, visibleNewerID, *firstPage.NextLastID)

	secondPage, err := svc.GetPostsByTag(context.Background(), "go", 1, *firstPage.NextLastID)
	require.NoError(t, err)
	require.Len(t, secondPage.Posts, 1)
	assert.Equal(t, visibleOlderID, secondPage.Posts[0].ID)
	assert.False(t, secondPage.HasMore)
	assert.Nil(t, secondPage.NextLastID)
}

func TestPostService_CreatePost_InvalidTags(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	svc := newTestPostService(t, repositories, newTestCache())

	_, err := svc.CreatePost(context.Background(), "title", "content", []string{" "}, userID, boardID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrInvalidInput))
}

func TestPostService_DeletePost_OrphansAttachmentsAndSoftDeletesComments(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")
	commentID := seedComment(repositories.comment, userID, postID, "comment")
	attachmentID, err := repositories.attachment.Save(context.Background(), entity.NewAttachment(postID, "a.png", "image/png", 10, "posts/1/a.png"))
	require.NoError(t, err)
	attachment, err := repositories.attachment.SelectByID(context.Background(), attachmentID)
	require.NoError(t, err)
	require.NotNil(t, attachment)
	attachment.MarkReferenced()
	require.NoError(t, repositories.attachment.Update(context.Background(), attachment))
	svc := newTestPostService(t, repositories, newTestCache())

	require.NoError(t, svc.DeletePost(context.Background(), postID, userID))

	comment, err := repositories.comment.SelectCommentByID(context.Background(), commentID)
	require.NoError(t, err)
	assert.Nil(t, comment)

	attachment, err = repositories.attachment.SelectByID(context.Background(), attachmentID)
	require.NoError(t, err)
	require.NotNil(t, attachment)
	assert.True(t, attachment.IsOrphaned())
}

func TestPostService_DeletePost_RemovesStoredReactionsForPostAndComments(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")
	commentID := seedComment(repositories.comment, userID, postID, "comment")
	_, _, _, err := repositories.reaction.SetUserTargetReaction(context.Background(), userID, postID, entity.ReactionTargetPost, entity.ReactionTypeLike)
	require.NoError(t, err)
	_, _, _, err = repositories.reaction.SetUserTargetReaction(context.Background(), userID, commentID, entity.ReactionTargetComment, entity.ReactionTypeLike)
	require.NoError(t, err)
	svc := newTestPostService(t, repositories, newTestCache())

	require.NoError(t, svc.DeletePost(context.Background(), postID, userID))

	postReactions, err := repositories.reaction.GetByTarget(context.Background(), postID, entity.ReactionTargetPost)
	require.NoError(t, err)
	assert.Empty(t, postReactions)
	commentReactions, err := repositories.reaction.GetByTarget(context.Background(), commentID, entity.ReactionTargetComment)
	require.NoError(t, err)
	assert.Empty(t, commentReactions)
}

func TestPostService_CreateDraftPost_DoesNotAppearInPublicList(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	svc := newTestPostService(t, repositories, newTestCache())

	postID, err := svc.CreateDraftPost(context.Background(), "draft-title", "draft-content", nil, userID, boardID)
	require.NoError(t, err)
	assert.NotZero(t, postID)

	list, err := svc.GetPostsList(context.Background(), boardID, 10, 0)
	require.NoError(t, err)
	assert.Empty(t, list.Posts)
}

func TestPostService_PublishPost_MakesDraftVisible(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	svc := newTestPostService(t, repositories, newTestCache())

	postID, err := svc.CreateDraftPost(context.Background(), "draft-title", "draft-content", nil, userID, boardID)
	require.NoError(t, err)

	err = svc.PublishPost(context.Background(), postID, userID)
	require.NoError(t, err)

	list, err := svc.GetPostsList(context.Background(), boardID, 10, 0)
	require.NoError(t, err)
	require.Len(t, list.Posts, 1)
	assert.Equal(t, postID, list.Posts[0].ID)
}

func TestPostService_UpdatePost_RejectsForeignAttachmentReference(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedDraftPost(repositories.post, userID, boardID, "title", "content")
	otherPostID := seedDraftPost(repositories.post, userID, boardID, "title2", "content2")
	foreignAttachmentID, err := repositories.attachment.Save(context.Background(), entity.NewAttachment(otherPostID, "a.png", "image/png", 10, "posts/2/a.png"))
	require.NoError(t, err)
	svc := newTestPostService(t, repositories, newTestCache())

	err = svc.UpdatePost(context.Background(), postID, userID, "title", "body ![a](attachment://"+strconv.FormatInt(foreignAttachmentID, 10)+")", nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrInvalidInput))
}

func TestPostService_PublishPost_RejectsMissingAttachmentReference(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedDraftPost(repositories.post, userID, boardID, "title", "body ![a](attachment://999)")
	svc := newTestPostService(t, repositories, newTestCache())

	err := svc.PublishPost(context.Background(), postID, userID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrInvalidInput))
}

func TestPostService_GetPostDetail_IncludesAttachments(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "body")
	attachment := entity.NewAttachment(postID, "a.png", "image/png", 10, "posts/1/a.png")
	attachment.MarkReferenced()
	_, err := repositories.attachment.Save(context.Background(), attachment)
	require.NoError(t, err)
	svc := newTestPostService(t, repositories, newTestCache())

	detail, err := svc.GetPostDetail(context.Background(), postID)
	require.NoError(t, err)
	require.Len(t, detail.Attachments, 1)
	assert.Equal(t, "a.png", detail.Attachments[0].FileName)
}

func TestPostService_GetPostDetail_ExposesCommentPreviewHasMore(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "body")
	for i := 0; i < 11; i++ {
		seedComment(repositories.comment, userID, postID, "comment")
	}
	svc := newTestPostService(t, repositories, newTestCache())

	detail, err := svc.GetPostDetail(context.Background(), postID)
	require.NoError(t, err)
	require.Len(t, detail.Comments, 10)
	assert.True(t, detail.CommentsHasMore)
}

func TestPostDetailQuery_Load_Success(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "body")
	seedComment(repositories.comment, userID, postID, "comment")

	query := newPostDetailQuery(
		repositories.user,
		repositories.post,
		repositories.tag,
		repositories.postTag,
		repositories.attachment,
		repositories.comment,
		repositories.reaction,
	)

	detail, err := query.Load(context.Background(), postID)
	require.NoError(t, err)
	require.NotNil(t, detail)
	require.NotNil(t, detail.Post)
	assert.Equal(t, postID, detail.Post.ID)
	assert.Len(t, detail.Comments, 1)
}

func TestPostService_UpdatePost_MarksUnusedAttachmentsAsOrphaned(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedDraftPost(repositories.post, userID, boardID, "title", "content")
	referenced := entity.NewAttachment(postID, "a.png", "image/png", 10, "posts/1/a.png")
	referenced.MarkReferenced()
	referencedID, err := repositories.attachment.Save(context.Background(), referenced)
	require.NoError(t, err)
	unusedID, err := repositories.attachment.Save(context.Background(), entity.NewAttachment(postID, "b.png", "image/png", 10, "posts/1/b.png"))
	require.NoError(t, err)
	svc := newTestPostService(t, repositories, newTestCache())

	err = svc.UpdatePost(context.Background(), postID, userID, "title", "body ![a](attachment://"+strconv.FormatInt(referencedID, 10)+")", nil)
	require.NoError(t, err)

	referencedAfter, err := repositories.attachment.SelectByID(context.Background(), referencedID)
	require.NoError(t, err)
	unusedAfter, err := repositories.attachment.SelectByID(context.Background(), unusedID)
	require.NoError(t, err)
	assert.False(t, referencedAfter.IsOrphaned())
	assert.True(t, unusedAfter.IsOrphaned())
}
