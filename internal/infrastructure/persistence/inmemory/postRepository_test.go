package inmemory

import (
	"testing"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/application/porttest"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostRepositoryContract(t *testing.T) {
	porttest.RunPostRepositoryContractTests(t, func() port.PostRepository {
		return NewPostRepository(nil, nil)
	})
}

func TestPostRepository_FilterByBoardAndPagination(t *testing.T) {
	repo := NewPostRepository(nil, nil)
	_, _ = repo.Save(testPost("p1", "c1", 1, 1))
	_, _ = repo.Save(testPost("p2", "c2", 1, 1))
	_, _ = repo.Save(testPost("p3", "c3", 2, 2))

	posts, err := repo.SelectPosts(1, 10, 0)
	require.NoError(t, err)
	assert.Len(t, posts, 2)
	assert.Equal(t, int64(2), posts[0].ID)
	assert.Equal(t, int64(1), posts[1].ID)
}

func TestPostRepository_SaveSelectUpdateDelete(t *testing.T) {
	repo := NewPostRepository(nil, nil)
	id, err := repo.Save(testPost("title", "content", 1, 1))
	require.NoError(t, err)

	selected, err := repo.SelectPostByID(id)
	require.NoError(t, err)
	require.NotNil(t, selected)
	assert.Equal(t, "title", selected.Title)

	selected.Update("new", "new-content")
	require.NoError(t, repo.Update(selected))

	updated, err := repo.SelectPostByID(id)
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, "new", updated.Title)

	require.NoError(t, repo.Delete(id))
	deleted, err := repo.SelectPostByID(id)
	require.NoError(t, err)
	assert.Nil(t, deleted)
}

func TestPostRepository_Delete_SoftDeletesAndExcludesFromList(t *testing.T) {
	repo := NewPostRepository(nil, nil)
	id, err := repo.Save(testPost("title", "content", 1, 1))
	require.NoError(t, err)

	require.NoError(t, repo.Delete(id))

	selected, err := repo.SelectPostByID(id)
	require.NoError(t, err)
	assert.Nil(t, selected)

	posts, err := repo.SelectPosts(1, 10, 0)
	require.NoError(t, err)
	assert.Empty(t, posts)
}

func TestPostRepository_PaginationCursorAtEnd_ReturnsEmpty(t *testing.T) {
	repo := NewPostRepository(nil, nil)
	_, _ = repo.Save(testPost("p1", "c1", 1, 1))
	_, _ = repo.Save(testPost("p2", "c2", 1, 1))

	posts, err := repo.SelectPosts(1, 10, 1)
	require.NoError(t, err)
	assert.Empty(t, posts)
}

func TestPostRepository_PaginationWithCursor_ReturnsNextChunk(t *testing.T) {
	repo := NewPostRepository(nil, nil)
	_, _ = repo.Save(testPost("p1", "c1", 1, 1))
	_, _ = repo.Save(testPost("p2", "c2", 1, 1))
	_, _ = repo.Save(testPost("p3", "c3", 1, 1))

	posts, err := repo.SelectPosts(1, 10, 3)
	require.NoError(t, err)
	require.Len(t, posts, 2)
	assert.Equal(t, int64(2), posts[0].ID)
	assert.Equal(t, int64(1), posts[1].ID)
}

func TestPostRepository_UpdateDelete_NonExistingID_NoError(t *testing.T) {
	repo := NewPostRepository(nil, nil)
	p := testPost("x", "y", 1, 1)
	p.ID = 999

	require.NoError(t, repo.Update(p))
	require.NoError(t, repo.Delete(999))
}

func TestPostRepository_SelectPublishedPostsByTagName_FiltersBeforePagination(t *testing.T) {
	tagRepo := NewTagRepository()
	postTagRepo := NewPostTagRepository()
	repo := NewPostRepository(tagRepo, postTagRepo)

	tagID, err := tagRepo.Save(entity.NewTag("go"))
	require.NoError(t, err)

	publishedLowID := testPost("published-1", "content", 1, 1)
	_, err = repo.Save(publishedLowID)
	require.NoError(t, err)

	draft := entity.NewDraftPost("draft", "content", 1, 1)
	_, err = repo.Save(draft)
	require.NoError(t, err)

	publishedHighID := testPost("published-2", "content", 1, 1)
	_, err = repo.Save(publishedHighID)
	require.NoError(t, err)

	deleted := testPost("deleted", "content", 1, 1)
	deletedID, err := repo.Save(deleted)
	require.NoError(t, err)
	require.NoError(t, repo.Delete(deletedID))

	require.NoError(t, postTagRepo.UpsertActive(publishedLowID.ID, tagID))
	require.NoError(t, postTagRepo.UpsertActive(draft.ID, tagID))
	require.NoError(t, postTagRepo.UpsertActive(publishedHighID.ID, tagID))
	require.NoError(t, postTagRepo.UpsertActive(deletedID, tagID))

	posts, err := repo.SelectPublishedPostsByTagName("go", 2, 0)
	require.NoError(t, err)
	require.Len(t, posts, 2)
	assert.Equal(t, publishedHighID.ID, posts[0].ID)
	assert.Equal(t, publishedLowID.ID, posts[1].ID)
}

func TestPostRepository_SelectPublishedPostsByTagName_WithoutTagDependenciesErrors(t *testing.T) {
	repo := NewPostRepository(nil, nil)

	posts, err := repo.SelectPublishedPostsByTagName("go", 10, 0)
	require.Error(t, err)
	assert.Nil(t, posts)
}

func TestPostRepository_SelectPublishedPostsByTagName_BlocksWhileTagTransactionLockHeld(t *testing.T) {
	tagRepo := NewTagRepository()
	postTagRepo := NewPostTagRepository()
	repo := NewPostRepository(tagRepo, postTagRepo)
	uow := NewUnitOfWork(
		NewUserRepository(),
		NewBoardRepository(),
		repo,
		tagRepo,
		postTagRepo,
		NewCommentRepository(),
		NewReactionRepository(),
		NewAttachmentRepository(),
		NewOutboxRepository(),
	)

	tagID, err := tagRepo.Save(entity.NewTag("go"))
	require.NoError(t, err)
	post := testPost("published", "content", 1, 1)
	_, err = repo.Save(post)
	require.NoError(t, err)
	require.NoError(t, postTagRepo.UpsertActive(post.ID, tagID))

	txStarted := make(chan struct{})
	txRelease := make(chan struct{})
	txDone := make(chan error, 1)
	go func() {
		err := uow.WithinTransaction(func(tx port.TxScope) error {
			if _, err := tx.TagRepository().Save(entity.NewTag("hold-lock")); err != nil {
				return err
			}
			close(txStarted)
			<-txRelease
			return nil
		})
		txDone <- err
	}()
	<-txStarted

	queryDone := make(chan struct{})
	go func() {
		_, _ = repo.SelectPublishedPostsByTagName("go", 10, 0)
		close(queryDone)
	}()

	select {
	case <-queryDone:
		t.Fatal("tag-based query should block while tag repository tx lock is held")
	case <-time.After(30 * time.Millisecond):
	}

	close(txRelease)
	select {
	case err := <-txDone:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("transaction did not complete")
	}

	select {
	case <-queryDone:
	case <-time.After(time.Second):
		t.Fatal("query did not resume after tx lock release")
	}
}

func TestPostRepository_SelectPublishedPostsByTagName_BlocksWhilePostTagTransactionLockHeld(t *testing.T) {
	tagRepo := NewTagRepository()
	postTagRepo := NewPostTagRepository()
	repo := NewPostRepository(tagRepo, postTagRepo)
	uow := NewUnitOfWork(
		NewUserRepository(),
		NewBoardRepository(),
		repo,
		tagRepo,
		postTagRepo,
		NewCommentRepository(),
		NewReactionRepository(),
		NewAttachmentRepository(),
		NewOutboxRepository(),
	)

	tagID, err := tagRepo.Save(entity.NewTag("go"))
	require.NoError(t, err)
	post := testPost("published", "content", 1, 1)
	_, err = repo.Save(post)
	require.NoError(t, err)
	require.NoError(t, postTagRepo.UpsertActive(post.ID, tagID))

	txStarted := make(chan struct{})
	txRelease := make(chan struct{})
	txDone := make(chan error, 1)
	go func() {
		err := uow.WithinTransaction(func(tx port.TxScope) error {
			if err := tx.PostTagRepository().UpsertActive(post.ID, tagID); err != nil {
				return err
			}
			close(txStarted)
			<-txRelease
			return nil
		})
		txDone <- err
	}()
	<-txStarted

	queryDone := make(chan struct{})
	go func() {
		_, _ = repo.SelectPublishedPostsByTagName("go", 10, 0)
		close(queryDone)
	}()

	select {
	case <-queryDone:
		t.Fatal("tag-based query should block while postTag repository tx lock is held")
	case <-time.After(30 * time.Millisecond):
	}

	close(txRelease)
	select {
	case err := <-txDone:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("transaction did not complete")
	}

	select {
	case <-queryDone:
	case <-time.After(time.Second):
		t.Fatal("query did not resume after tx lock release")
	}
}
