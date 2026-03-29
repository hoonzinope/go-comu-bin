package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/application/porttest"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBoardRepository_Contract(t *testing.T) {
	t.Parallel()

	porttest.RunBoardRepositoryContractTests(t, func() port.BoardRepository {
		return NewBoardRepository(openTestSQLiteDB(t))
	})
}

func TestTagRepository_Contract(t *testing.T) {
	t.Parallel()

	porttest.RunTagRepositoryContractTests(t, func() port.TagRepository {
		return NewTagRepository(openTestSQLiteDB(t))
	})
}

func TestPostTagRepository_Contract(t *testing.T) {
	t.Parallel()

	porttest.RunPostTagRepositoryContractTests(t, func() port.PostTagRepository {
		return NewPostTagRepository(openTestSQLiteDB(t))
	})
}

func TestPostRepository_Contract(t *testing.T) {
	t.Parallel()

	porttest.RunPostRepositoryContractTests(t, func() port.PostRepository {
		return NewPostRepository(openTestSQLiteDB(t))
	})
}

func TestCommentRepository_Contract(t *testing.T) {
	t.Parallel()

	porttest.RunCommentRepositoryContractTests(t, func() port.CommentRepository {
		return NewCommentRepository(openTestSQLiteDB(t))
	})
}

func TestReactionRepository_Contract(t *testing.T) {
	t.Parallel()

	porttest.RunReactionRepositoryContractTests(t, func() port.ReactionRepository {
		return NewReactionRepository(openTestSQLiteDB(t))
	})
}

func TestAttachmentRepository_Contract(t *testing.T) {
	t.Parallel()

	porttest.RunAttachmentRepositoryContractTests(t, func() port.AttachmentRepository {
		return NewAttachmentRepository(openTestSQLiteDB(t))
	})
}

func TestReportRepository_Contract(t *testing.T) {
	t.Parallel()

	porttest.RunReportRepositoryContractTests(t, func() port.ReportRepository {
		return NewReportRepository(openTestSQLiteDB(t))
	})
}

func TestPostRepository_SelectPublishedPostsByTagName(t *testing.T) {
	t.Parallel()

	db := openTestSQLiteDB(t)
	boardRepo := NewBoardRepository(db)
	tagRepo := NewTagRepository(db)
	postTagRepo := NewPostTagRepository(db)
	postRepo := NewPostRepository(db)

	boardID := mustSaveBoard(t, boardRepo, entity.NewBoard("free", "desc"))
	authorID := int64(1)
	firstPostID := mustSavePost(t, postRepo, entity.NewPost("first", "body", authorID, boardID))
	secondPostID := mustSavePost(t, postRepo, entity.NewPost("second", "body", authorID, boardID))
	tagID, err := tagRepo.Save(context.Background(), entity.NewTag("go"))
	require.NoError(t, err)
	require.NoError(t, postTagRepo.UpsertActive(context.Background(), firstPostID, tagID))
	require.NoError(t, postTagRepo.UpsertActive(context.Background(), secondPostID, tagID))

	posts, err := postRepo.SelectPublishedPostsByTagName(context.Background(), "go", 10, 0)
	require.NoError(t, err)
	require.Len(t, posts, 2)
	assert.Equal(t, secondPostID, posts[0].ID)
	assert.Equal(t, firstPostID, posts[1].ID)

	posts, err = postRepo.SelectPublishedPostsByTagName(context.Background(), "go", 10, secondPostID)
	require.NoError(t, err)
	require.Len(t, posts, 1)
	assert.Equal(t, firstPostID, posts[0].ID)
}

func TestUserRepository_NilExecutorReturnsRepositoryFailure(t *testing.T) {
	t.Parallel()

	repo := NewUserRepository(nil)
	_, err := repo.Save(context.Background(), entity.NewUser("alice", "pw"))
	require.Error(t, err)
	assert.ErrorIs(t, err, customerror.ErrRepositoryFailure)
	assert.NotErrorIs(t, err, customerror.ErrInternalServerError)
}

func TestPostSearchRepository_SearchPublishedPosts(t *testing.T) {
	t.Parallel()

	db := openTestSQLiteDB(t)
	boardRepo := NewBoardRepository(db)
	tagRepo := NewTagRepository(db)
	postTagRepo := NewPostTagRepository(db)
	postRepo := NewPostRepository(db)
	searchRepo := NewPostSearchRepository(db)

	boardID := mustSaveBoard(t, boardRepo, entity.NewBoard("free", "desc"))
	authorID := int64(1)
	titlePostID := mustSavePost(t, postRepo, entity.NewPost("Go search", "body", authorID, boardID))
	contentPostID := mustSavePost(t, postRepo, entity.NewPost("title", "search token in body", authorID, boardID))
	tagPostID := mustSavePost(t, postRepo, entity.NewPost("other", "body", authorID, boardID))
	tagID, err := tagRepo.Save(context.Background(), entity.NewTag("search"))
	require.NoError(t, err)
	require.NoError(t, postTagRepo.UpsertActive(context.Background(), tagPostID, tagID))
	require.NoError(t, searchRepo.RebuildAll(context.Background()))

	results, err := searchRepo.SearchPublishedPosts(context.Background(), "search", 10, nil)
	require.NoError(t, err)
	require.Len(t, results, 3)
	assert.Equal(t, titlePostID, results[0].Post.ID)
	assert.Equal(t, tagPostID, results[1].Post.ID)
	assert.Equal(t, contentPostID, results[2].Post.ID)

	sameScorePost1 := mustSavePost(t, postRepo, entity.NewPost("alpha beta", "alpha beta", authorID, boardID))
	time.Sleep(time.Millisecond)
	sameScorePost2 := mustSavePost(t, postRepo, entity.NewPost("alpha beta", "alpha beta", authorID, boardID))
	time.Sleep(time.Millisecond)
	sameScorePost3 := mustSavePost(t, postRepo, entity.NewPost("alpha beta", "alpha beta", authorID, boardID))
	require.NoError(t, searchRepo.RebuildAll(context.Background()))

	cursorResults, err := searchRepo.SearchPublishedPosts(context.Background(), "alpha beta", 2, nil)
	require.NoError(t, err)
	require.Len(t, cursorResults, 2)
	assert.Equal(t, sameScorePost3, cursorResults[0].Post.ID)
	assert.Equal(t, sameScorePost2, cursorResults[1].Post.ID)

	cursor := &port.PostSearchCursor{Score: cursorResults[1].Score, PostID: cursorResults[1].Post.ID}
	nextResults, err := searchRepo.SearchPublishedPosts(context.Background(), "alpha beta", 10, cursor)
	require.NoError(t, err)
	require.Len(t, nextResults, 1)
	assert.Equal(t, sameScorePost1, nextResults[0].Post.ID)
}

func TestPostSearchRepository_FTSIndexMaintenance(t *testing.T) {
	t.Parallel()

	db := openTestSQLiteDB(t)
	boardRepo := NewBoardRepository(db)
	tagRepo := NewTagRepository(db)
	postTagRepo := NewPostTagRepository(db)
	postRepo := NewPostRepository(db)
	searchRepo := NewPostSearchRepository(db)

	boardID := mustSaveBoard(t, boardRepo, entity.NewBoard("free", "desc"))
	authorID := int64(1)
	publishedPost := entity.NewPost("Go Search", "search body", authorID, boardID)
	publishedID := mustSavePost(t, postRepo, publishedPost)
	draftID := mustSavePost(t, postRepo, entity.NewDraftPost("Draft Search", "draft body", authorID, boardID))
	deletedID := mustSavePost(t, postRepo, entity.NewPost("Deleted Search", "deleted body", authorID, boardID))
	require.NoError(t, postRepo.Delete(context.Background(), deletedID))

	tagID, err := tagRepo.Save(context.Background(), entity.NewTag("search"))
	require.NoError(t, err)
	require.NoError(t, postTagRepo.UpsertActive(context.Background(), publishedID, tagID))

	require.NoError(t, searchRepo.RebuildAll(context.Background()))

	title, content, tags := mustLoadSearchFTSRow(t, db, publishedID)
	assert.Equal(t, "go search", title)
	assert.Equal(t, "search body", content)
	assert.Equal(t, "search", tags)
	assertSearchFTSRowMissing(t, db, draftID)
	assertSearchFTSRowMissing(t, db, deletedID)

	publishedPost.Title = "New Search Title"
	require.NoError(t, postRepo.Update(context.Background(), publishedPost))
	require.NoError(t, searchRepo.UpsertPost(context.Background(), publishedID))

	title, content, tags = mustLoadSearchFTSRow(t, db, publishedID)
	assert.Equal(t, "new search title", title)
	assert.Equal(t, "search body", content)
	assert.Equal(t, "search", tags)

	require.NoError(t, postRepo.Delete(context.Background(), publishedID))
	require.NoError(t, searchRepo.DeletePost(context.Background(), publishedID))
	assertSearchFTSRowMissing(t, db, publishedID)
}

func TestPostSearchRepository_RebuildAll_PreservesConcurrentUpserts(t *testing.T) {
	db := openTestSQLiteDBWithMaxOpenConns(t, 2)
	boardRepo := NewBoardRepository(db)
	postRepo := NewPostRepository(db)
	searchRepo := NewPostSearchRepository(db)

	boardID := mustSaveBoard(t, boardRepo, entity.NewBoard("free", "desc"))
	authorID := int64(1)
	post := entity.NewPost("old title", "body", authorID, boardID)
	postID := mustSavePost(t, postRepo, post)
	require.NoError(t, searchRepo.UpsertPost(context.Background(), postID))

	loaded := make(chan struct{})
	resume := make(chan struct{})
	searchRepo.afterRebuildLoad = func() {
		close(loaded)
		<-resume
	}

	rebuildDone := make(chan error, 1)
	go func() {
		rebuildDone <- searchRepo.RebuildAll(context.Background())
	}()

	<-loaded

	post.Update("new title", "body")
	updateDone := make(chan error, 1)
	go func() {
		if err := postRepo.Update(context.Background(), post); err != nil {
			updateDone <- err
			return
		}
		updateDone <- searchRepo.UpsertPost(context.Background(), postID)
	}()

	select {
	case updateErr := <-updateDone:
		require.NoError(t, updateErr)
	case <-time.After(time.Second):
		t.Fatal("update did not finish while rebuild was paused")
	}

	close(resume)
	require.NoError(t, <-rebuildDone)

	title, _, _ := mustLoadSearchFTSRow(t, db, postID)
	assert.Equal(t, "new title", title)

	results, err := searchRepo.SearchPublishedPosts(context.Background(), "old title", 10, nil)
	require.NoError(t, err)
	assert.Empty(t, results)
	results, err = searchRepo.SearchPublishedPosts(context.Background(), "new title", 10, nil)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, postID, results[0].Post.ID)
}

func TestPostSearchRepository_RebuildAll_PreservesConcurrentDeletes(t *testing.T) {
	db := openTestSQLiteDBWithMaxOpenConns(t, 2)
	boardRepo := NewBoardRepository(db)
	postRepo := NewPostRepository(db)
	searchRepo := NewPostSearchRepository(db)

	boardID := mustSaveBoard(t, boardRepo, entity.NewBoard("free", "desc"))
	authorID := int64(1)
	post := entity.NewPost("delete title", "body", authorID, boardID)
	postID := mustSavePost(t, postRepo, post)
	require.NoError(t, searchRepo.UpsertPost(context.Background(), postID))

	loaded := make(chan struct{})
	resume := make(chan struct{})
	searchRepo.afterRebuildLoad = func() {
		close(loaded)
		<-resume
	}

	rebuildDone := make(chan error, 1)
	go func() {
		rebuildDone <- searchRepo.RebuildAll(context.Background())
	}()

	<-loaded

	deleteDone := make(chan error, 1)
	go func() {
		if err := postRepo.Delete(context.Background(), postID); err != nil {
			deleteDone <- err
			return
		}
		deleteDone <- searchRepo.DeletePost(context.Background(), postID)
	}()

	select {
	case deleteErr := <-deleteDone:
		require.NoError(t, deleteErr)
	case <-time.After(time.Second):
		t.Fatal("delete did not finish while rebuild was paused")
	}

	close(resume)
	require.NoError(t, <-rebuildDone)

	assertSearchFTSRowMissing(t, db, postID)
}

func TestPostSearchRepository_RebuildAll_PreservesReadConsistency(t *testing.T) {
	db := openTestSQLiteDBWithMaxOpenConns(t, 2)
	boardRepo := NewBoardRepository(db)
	postRepo := NewPostRepository(db)
	searchRepo := NewPostSearchRepository(db)

	boardID := mustSaveBoard(t, boardRepo, entity.NewBoard("free", "desc"))
	authorID := int64(1)
	postID := mustSavePost(t, postRepo, entity.NewPost("read title", "body", authorID, boardID))
	require.NoError(t, searchRepo.UpsertPost(context.Background(), postID))

	loaded := make(chan struct{})
	resume := make(chan struct{})
	searchRepo.afterRebuildLoad = func() {
		close(loaded)
		<-resume
	}

	rebuildDone := make(chan error, 1)
	go func() {
		rebuildDone <- searchRepo.RebuildAll(context.Background())
	}()

	<-loaded

	results, err := searchRepo.SearchPublishedPosts(context.Background(), "read title", 10, nil)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, postID, results[0].Post.ID)

	close(resume)
	require.NoError(t, <-rebuildDone)
}

func TestPostSearchRepository_SearchPublishedPosts_RequiresFTSIndex(t *testing.T) {
	t.Parallel()

	db := openTestSQLiteDB(t)
	boardRepo := NewBoardRepository(db)
	postRepo := NewPostRepository(db)
	searchRepo := NewPostSearchRepository(db)

	boardID := mustSaveBoard(t, boardRepo, entity.NewBoard("free", "desc"))
	authorID := int64(1)
	postID := mustSavePost(t, postRepo, entity.NewPost("Go Search", "body", authorID, boardID))
	require.NoError(t, searchRepo.RebuildAll(context.Background()))

	_, err := db.ExecContext(context.Background(), `DELETE FROM post_search_fts WHERE rowid = ?`, postID)
	require.NoError(t, err)

	results, err := searchRepo.SearchPublishedPosts(context.Background(), "search", 10, nil)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestUnitOfWork_CommitsAndRollsBackBoardAndPostChanges(t *testing.T) {
	t.Parallel()

	db := openTestSQLiteDB(t)
	boardRepo := NewBoardRepository(db)
	postRepo := NewPostRepository(db)
	tagRepo := NewTagRepository(db)
	postTagRepo := NewPostTagRepository(db)
	uow := NewUnitOfWork(db, boardRepo, postRepo, tagRepo, postTagRepo, nil, nil, nil, nil, nil, nil, nil, nil)

	board := entity.NewBoard("free", "desc")
	tag := entity.NewTag("go")
	post := entity.NewPost("title", "content", 1, 0)
	committedPostUUID := post.UUID

	require.NoError(t, uow.WithinTransaction(context.Background(), func(tx port.TxScope) error {
		boardID, err := tx.BoardRepository().Save(context.Background(), board)
		if err != nil {
			return err
		}
		post.BoardID = boardID
		postID, err := tx.PostRepository().Save(context.Background(), post)
		if err != nil {
			return err
		}
		tagID, err := tx.TagRepository().Save(context.Background(), tag)
		if err != nil {
			return err
		}
		return tx.PostTagRepository().UpsertActive(context.Background(), postID, tagID)
	}))
	loadedBoard, err := boardRepo.SelectBoardByUUID(context.Background(), board.UUID)
	require.NoError(t, err)
	require.NotNil(t, loadedBoard)
	loadedPost, err := postRepo.SelectPostByUUIDIncludingUnpublished(context.Background(), committedPostUUID)
	require.NoError(t, err)
	require.NotNil(t, loadedPost)

	rolledBackBoard := entity.NewBoard("rollback", "desc")
	rolledBackPost := entity.NewPost("title", "content", 1, 0)
	rolledBackTag := entity.NewTag("rollback")
	rollbackErr := errors.New("rollback")
	err = uow.WithinTransaction(context.Background(), func(tx port.TxScope) error {
		boardID, innerErr := tx.BoardRepository().Save(context.Background(), rolledBackBoard)
		if innerErr != nil {
			return innerErr
		}
		rolledBackPost.BoardID = boardID
		postID, innerErr := tx.PostRepository().Save(context.Background(), rolledBackPost)
		if innerErr != nil {
			return innerErr
		}
		tagID, innerErr := tx.TagRepository().Save(context.Background(), rolledBackTag)
		if innerErr != nil {
			return innerErr
		}
		if innerErr = tx.PostTagRepository().UpsertActive(context.Background(), postID, tagID); innerErr != nil {
			return innerErr
		}
		return rollbackErr
	})
	require.ErrorIs(t, err, rollbackErr)
	loadedBoard, err = boardRepo.SelectBoardByUUID(context.Background(), rolledBackBoard.UUID)
	require.NoError(t, err)
	assert.Nil(t, loadedBoard)
	loadedPost, err = postRepo.SelectPostByUUIDIncludingUnpublished(context.Background(), rolledBackPost.UUID)
	require.NoError(t, err)
	assert.Nil(t, loadedPost)
}

func TestNotificationRepository_SaveDedupAndReadFlow(t *testing.T) {
	t.Parallel()

	repo := NewNotificationRepository(openTestSQLiteDB(t))

	first := entity.NewNotification(1, 2, entity.NotificationTypeMentioned, 10, 20, "actor", "post", "comment")
	first.DedupKey = "event-1"
	id1, err := repo.Save(context.Background(), first)
	require.NoError(t, err)
	require.NotZero(t, id1)

	dup := entity.NewNotification(1, 2, entity.NotificationTypeMentioned, 10, 20, "actor", "post", "comment")
	dup.DedupKey = "event-1"
	id2, err := repo.Save(context.Background(), dup)
	require.NoError(t, err)
	assert.Equal(t, id1, id2)

	unread, err := repo.CountUnreadByRecipientUserID(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, 1, unread)

	other := entity.NewNotification(1, 3, entity.NotificationTypePostCommented, 11, 21, "actor-2", "post-2", "comment-2")
	id3, err := repo.Save(context.Background(), other)
	require.NoError(t, err)
	require.NotZero(t, id3)

	items, err := repo.SelectByRecipientUserID(context.Background(), 1, 10, 0)
	require.NoError(t, err)
	require.Len(t, items, 2)
	assert.Equal(t, id3, items[0].ID)
	assert.Equal(t, id1, items[1].ID)

	require.NoError(t, repo.MarkRead(context.Background(), id1))
	unread, err = repo.CountUnreadByRecipientUserID(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, 1, unread)

	changed, err := repo.MarkAllReadByRecipientUserID(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, 1, changed)

	unread, err = repo.CountUnreadByRecipientUserID(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, 0, unread)
}

func TestUnitOfWork_CommitsAndRollsBackRemainingRepositories(t *testing.T) {
	t.Parallel()

	db := openTestSQLiteDB(t)
	boardRepo := NewBoardRepository(db)
	postRepo := NewPostRepository(db)
	tagRepo := NewTagRepository(db)
	postTagRepo := NewPostTagRepository(db)
	commentRepo := NewCommentRepository(db)
	reactionRepo := NewReactionRepository(db)
	attachmentRepo := NewAttachmentRepository(db)
	reportRepo := NewReportRepository(db)
	notificationRepo := NewNotificationRepository(db)
	uow := NewUnitOfWork(db, boardRepo, postRepo, tagRepo, postTagRepo, commentRepo, reactionRepo, attachmentRepo, reportRepo, notificationRepo, nil, nil, nil)

	board := entity.NewBoard("free", "desc")
	var committedPostID int64
	var committedCommentID int64
	var committedAttachmentID int64
	var committedReportID int64
	var committedNotificationID int64
	require.NoError(t, uow.WithinTransaction(context.Background(), func(tx port.TxScope) error {
		boardID, err := tx.BoardRepository().Save(context.Background(), board)
		if err != nil {
			return err
		}
		post := entity.NewPost("title", "content", 1, boardID)
		postID, err := tx.PostRepository().Save(context.Background(), post)
		if err != nil {
			return err
		}
		committedPostID = postID
		comment := entity.NewComment("comment", 1, postID, nil)
		commentID, err := tx.CommentRepository().Save(context.Background(), comment)
		if err != nil {
			return err
		}
		committedCommentID = commentID
		_, _, _, err = tx.ReactionRepository().SetUserTargetReaction(context.Background(), 1, postID, entity.ReactionTargetPost, entity.ReactionTypeLike)
		if err != nil {
			return err
		}
		attachment := entity.NewAttachment(postID, "file.png", "image/png", 1, "storage-key")
		attachment.MarkReferenced()
		attachmentID, err := tx.AttachmentRepository().Save(context.Background(), attachment)
		if err != nil {
			return err
		}
		committedAttachmentID = attachmentID
		report := entity.NewReport(entity.ReportTargetPost, postID, 1, entity.ReportReasonSpam, "detail")
		reportID, err := tx.ReportRepository().Save(context.Background(), report)
		if err != nil {
			return err
		}
		committedReportID = reportID
		notification := entity.NewNotification(2, 1, entity.NotificationTypePostCommented, postID, commentID, "actor", "post", "comment")
		notification.DedupKey = "event-1"
		notificationID, err := tx.NotificationRepository().Save(context.Background(), notification)
		if err != nil {
			return err
		}
		committedNotificationID = notificationID
		return nil
	}))

	loadedBoard, err := boardRepo.SelectBoardByUUID(context.Background(), board.UUID)
	require.NoError(t, err)
	require.NotNil(t, loadedBoard)
	loadedPost, err := postRepo.SelectPostByID(context.Background(), committedPostID)
	require.NoError(t, err)
	require.NotNil(t, loadedPost)
	loadedComment, err := commentRepo.SelectCommentByID(context.Background(), committedCommentID)
	require.NoError(t, err)
	require.NotNil(t, loadedComment)
	loadedAttachment, err := attachmentRepo.SelectByID(context.Background(), committedAttachmentID)
	require.NoError(t, err)
	require.NotNil(t, loadedAttachment)
	loadedReport, err := reportRepo.SelectByID(context.Background(), committedReportID)
	require.NoError(t, err)
	require.NotNil(t, loadedReport)
	loadedNotification, err := notificationRepo.SelectByID(context.Background(), committedNotificationID)
	require.NoError(t, err)
	require.NotNil(t, loadedNotification)

	rolledBackBoard := entity.NewBoard("rollback", "desc")
	rollbackErr := errors.New("rollback")
	err = uow.WithinTransaction(context.Background(), func(tx port.TxScope) error {
		boardID, err := tx.BoardRepository().Save(context.Background(), rolledBackBoard)
		if err != nil {
			return err
		}
		post := entity.NewPost("rollback", "content", 1, boardID)
		postID, err := tx.PostRepository().Save(context.Background(), post)
		if err != nil {
			return err
		}
		if _, err := tx.CommentRepository().Save(context.Background(), entity.NewComment("rollback", 1, postID, nil)); err != nil {
			return err
		}
		if _, _, _, err := tx.ReactionRepository().SetUserTargetReaction(context.Background(), 1, postID, entity.ReactionTargetPost, entity.ReactionTypeLike); err != nil {
			return err
		}
		if _, err := tx.AttachmentRepository().Save(context.Background(), entity.NewAttachment(postID, "rollback.png", "image/png", 1, "storage-key")); err != nil {
			return err
		}
		if _, err := tx.ReportRepository().Save(context.Background(), entity.NewReport(entity.ReportTargetPost, postID, 1, entity.ReportReasonSpam, "detail")); err != nil {
			return err
		}
		if _, err := tx.NotificationRepository().Save(context.Background(), entity.NewNotification(2, 1, entity.NotificationTypePostCommented, postID, 0, "actor", "post", "comment")); err != nil {
			return err
		}
		return rollbackErr
	})
	require.ErrorIs(t, err, rollbackErr)
	loadedBoard, err = boardRepo.SelectBoardByUUID(context.Background(), rolledBackBoard.UUID)
	require.NoError(t, err)
	assert.Nil(t, loadedBoard)
}

func mustSaveBoard(t *testing.T, repo *BoardRepository, board *entity.Board) int64 {
	t.Helper()
	id, err := repo.Save(context.Background(), board)
	require.NoError(t, err)
	return id
}

func mustSavePost(t *testing.T, repo *PostRepository, post *entity.Post) int64 {
	t.Helper()
	id, err := repo.Save(context.Background(), post)
	require.NoError(t, err)
	return id
}

func mustLoadSearchFTSRow(t *testing.T, db *sql.DB, postID int64) (string, string, string) {
	t.Helper()
	var title sql.NullString
	var content sql.NullString
	var tags sql.NullString
	err := db.QueryRowContext(context.Background(), `
SELECT title, content, tags
FROM post_search_fts
WHERE rowid = ?
`, postID).Scan(&title, &content, &tags)
	require.NoError(t, err)
	return title.String, content.String, tags.String
}

func assertSearchFTSRowMissing(t *testing.T, db *sql.DB, postID int64) {
	t.Helper()
	var title sql.NullString
	var content sql.NullString
	var tags sql.NullString
	err := db.QueryRowContext(context.Background(), `
SELECT title, content, tags
FROM post_search_fts
WHERE rowid = ?
`, postID).Scan(&title, &content, &tags)
	require.ErrorIs(t, err, sql.ErrNoRows)
}
