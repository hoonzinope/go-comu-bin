package inmemory

import (
	"context"
	"testing"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostSearchStore_RebuildAll_PreservesUpdatesAfterRebuildStart(t *testing.T) {
	tagRepo := NewTagRepository()
	postTagRepo := NewPostTagRepository()
	postRepo := NewPostRepository(tagRepo, postTagRepo)
	searchStore := NewPostSearchStore(postRepo, tagRepo, postTagRepo)

	post := testPost("old title", "body", 1, 1)
	_, err := postRepo.Save(context.Background(), post)
	require.NoError(t, err)
	require.NoError(t, searchStore.RebuildAll(context.Background()))

	rebuildLoaded := make(chan struct{})
	rebuildResume := make(chan struct{})
	searchStore.afterRebuildLoad = func() {
		close(rebuildLoaded)
		<-rebuildResume
	}
	defer func() {
		searchStore.afterRebuildLoad = nil
	}()

	post.Title = "new title"
	require.NoError(t, postRepo.Update(context.Background(), post))

	rebuildDone := make(chan error, 1)
	go func() {
		rebuildDone <- searchStore.RebuildAll(context.Background())
	}()

	<-rebuildLoaded

	require.NoError(t, searchStore.UpsertPost(context.Background(), post.ID))

	close(rebuildResume)
	require.NoError(t, <-rebuildDone)

	results, err := searchStore.SearchPublishedPosts(context.Background(), "new", 10, nil)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, post.ID, results[0].Post.ID)

	oldResults, err := searchStore.SearchPublishedPosts(context.Background(), "old", 10, nil)
	require.NoError(t, err)
	assert.Empty(t, oldResults)
}

func TestPostSearchRepository_SearchPublishedPosts_RanksByFieldWeightAndPhraseBoost(t *testing.T) {
	tagRepo := NewTagRepository()
	postTagRepo := NewPostTagRepository()
	postRepo := NewPostRepository(tagRepo, postTagRepo)
	searchStore := NewPostSearchStore(postRepo, tagRepo, postTagRepo)

	contentOnly := testPost("title", "go search tokens appear in content", 1, 1)
	_, err := postRepo.Save(context.Background(), contentOnly)
	require.NoError(t, err)

	tagOnly := testPost("title", "body", 1, 1)
	_, err = postRepo.Save(context.Background(), tagOnly)
	require.NoError(t, err)

	titlePhrase := testPost("go search", "body", 1, 1)
	_, err = postRepo.Save(context.Background(), titlePhrase)
	require.NoError(t, err)

	tagID, err := tagRepo.Save(context.Background(), entity.NewTag("go"))
	require.NoError(t, err)
	searchTagID, err := tagRepo.Save(context.Background(), entity.NewTag("search"))
	require.NoError(t, err)
	require.NoError(t, postTagRepo.UpsertActive(context.Background(), tagOnly.ID, tagID))
	require.NoError(t, postTagRepo.UpsertActive(context.Background(), tagOnly.ID, searchTagID))
	require.NoError(t, searchStore.RebuildAll(context.Background()))

	results, err := searchStore.SearchPublishedPosts(context.Background(), "go search", 10, nil)
	require.NoError(t, err)
	require.Len(t, results, 3)
	assert.Equal(t, titlePhrase.ID, results[0].Post.ID)
	assert.Equal(t, tagOnly.ID, results[1].Post.ID)
	assert.Equal(t, contentOnly.ID, results[2].Post.ID)
	assert.Greater(t, results[0].Score, results[1].Score)
	assert.Greater(t, results[1].Score, results[2].Score)
}

func TestPostSearchRepository_SearchPublishedPosts_StripsDiacritics(t *testing.T) {
	tagRepo := NewTagRepository()
	postTagRepo := NewPostTagRepository()
	postRepo := NewPostRepository(tagRepo, postTagRepo)
	searchStore := NewPostSearchStore(postRepo, tagRepo, postTagRepo)

	post := testPost("Café search", "body", 1, 1)
	_, err := postRepo.Save(context.Background(), post)
	require.NoError(t, err)
	require.NoError(t, searchStore.RebuildAll(context.Background()))

	results, err := searchStore.SearchPublishedPosts(context.Background(), "cafe", 10, nil)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, post.ID, results[0].Post.ID)
}

func TestPostSearchStore_ConstructorsAndAttachBoardRepository(t *testing.T) {
	tagRepo := NewTagRepository()
	postTagRepo := NewPostTagRepository()
	postRepo := NewPostRepository(tagRepo, postTagRepo)
	boardRepo := NewBoardRepository()

	store := NewPostSearchStore(postRepo, tagRepo, postTagRepo)
	compat := NewPostSearchRepository(postRepo, tagRepo, postTagRepo)

	require.NotNil(t, store)
	require.NotNil(t, compat)
	store.AttachBoardRepository(boardRepo)
	assert.Equal(t, boardRepo, store.boardRepository)
}

func TestPostSearchRepository_SearchPublishedPosts_ExcludesDraftDeletedAndHonorsCursor(t *testing.T) {
	tagRepo := NewTagRepository()
	postTagRepo := NewPostTagRepository()
	postRepo := NewPostRepository(tagRepo, postTagRepo)
	searchStore := NewPostSearchStore(postRepo, tagRepo, postTagRepo)

	first := testPost("alpha beta", "alpha beta", 1, 1)
	_, err := postRepo.Save(context.Background(), first)
	require.NoError(t, err)
	second := testPost("alpha beta", "alpha beta", 1, 1)
	_, err = postRepo.Save(context.Background(), second)
	require.NoError(t, err)
	draft := entity.NewDraftPost("alpha beta", "alpha beta", 1, 1)
	_, err = postRepo.Save(context.Background(), draft)
	require.NoError(t, err)
	deleted := testPost("alpha beta", "alpha beta", 1, 1)
	deletedID, err := postRepo.Save(context.Background(), deleted)
	require.NoError(t, err)
	require.NoError(t, postRepo.Delete(context.Background(), deletedID))
	require.NoError(t, searchStore.RebuildAll(context.Background()))

	results, err := searchStore.SearchPublishedPosts(context.Background(), "alpha beta", 2, nil)
	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Equal(t, second.ID, results[0].Post.ID)
	assert.Equal(t, first.ID, results[1].Post.ID)

	cursor := &port.PostSearchCursor{Score: results[0].Score, PostID: results[0].Post.ID}
	next, err := searchStore.SearchPublishedPosts(context.Background(), "alpha beta", 2, cursor)
	require.NoError(t, err)
	require.Len(t, next, 1)
	assert.Equal(t, first.ID, next[0].Post.ID)
}

func TestPostSearchRepository_UpsertAndDeletePost_UpdatesIndex(t *testing.T) {
	tagRepo := NewTagRepository()
	postTagRepo := NewPostTagRepository()
	postRepo := NewPostRepository(tagRepo, postTagRepo)
	searchStore := NewPostSearchStore(postRepo, tagRepo, postTagRepo)

	post := testPost("go search", "body", 1, 1)
	_, err := postRepo.Save(context.Background(), post)
	require.NoError(t, err)
	require.NoError(t, searchStore.UpsertPost(context.Background(), post.ID))

	results, err := searchStore.SearchPublishedPosts(context.Background(), "go search", 10, nil)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, post.ID, results[0].Post.ID)

	require.NoError(t, postRepo.Delete(context.Background(), post.ID))
	require.NoError(t, searchStore.DeletePost(context.Background(), post.ID))

	results, err = searchStore.SearchPublishedPosts(context.Background(), "go search", 10, nil)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestPostSearchRepository_UpsertPost_ReplacesExistingDocumentInIndex(t *testing.T) {
	tagRepo := NewTagRepository()
	postTagRepo := NewPostTagRepository()
	postRepo := NewPostRepository(tagRepo, postTagRepo)
	searchStore := NewPostSearchStore(postRepo, tagRepo, postTagRepo)

	post := testPost("go search", "body", 1, 1)
	_, err := postRepo.Save(context.Background(), post)
	require.NoError(t, err)
	require.NoError(t, searchStore.UpsertPost(context.Background(), post.ID))

	initialResults, err := searchStore.SearchPublishedPosts(context.Background(), "go search", 10, nil)
	require.NoError(t, err)
	require.Len(t, initialResults, 1)
	assert.Equal(t, post.ID, initialResults[0].Post.ID)

	post.Title = "new topic"
	post.Content = "updated body"
	require.NoError(t, postRepo.Update(context.Background(), post))
	require.NoError(t, searchStore.UpsertPost(context.Background(), post.ID))

	oldResults, err := searchStore.SearchPublishedPosts(context.Background(), "go search", 10, nil)
	require.NoError(t, err)
	assert.Empty(t, oldResults)

	newResults, err := searchStore.SearchPublishedPosts(context.Background(), "new topic", 10, nil)
	require.NoError(t, err)
	require.Len(t, newResults, 1)
	assert.Equal(t, post.ID, newResults[0].Post.ID)
}

func TestPostSearchStore_HandlesNilAndMissingInputs(t *testing.T) {
	var nilStore *PostSearchStore

	results, err := nilStore.SearchPublishedPosts(context.Background(), "go", 10, nil)
	require.NoError(t, err)
	assert.Empty(t, results)
	require.NoError(t, nilStore.RebuildAll(context.Background()))
	require.NoError(t, nilStore.UpsertPost(context.Background(), 1))
	require.NoError(t, nilStore.DeletePost(context.Background(), 1))
}

func TestPostSearchStore_UpsertPost_RemovesMissingAndDraftDocuments(t *testing.T) {
	tagRepo := NewTagRepository()
	postTagRepo := NewPostTagRepository()
	postRepo := NewPostRepository(tagRepo, postTagRepo)
	searchStore := NewPostSearchStore(postRepo, tagRepo, postTagRepo)

	post := testPost("go search", "body", 1, 1)
	_, err := postRepo.Save(context.Background(), post)
	require.NoError(t, err)
	require.NoError(t, searchStore.UpsertPost(context.Background(), post.ID))

	results, err := searchStore.SearchPublishedPosts(context.Background(), "go", 10, nil)
	require.NoError(t, err)
	require.Len(t, results, 1)

	require.NoError(t, postRepo.Delete(context.Background(), post.ID))
	require.NoError(t, searchStore.UpsertPost(context.Background(), post.ID))

	results, err = searchStore.SearchPublishedPosts(context.Background(), "go", 10, nil)
	require.NoError(t, err)
	assert.Empty(t, results)

	draft := entity.NewDraftPost("go draft", "body", 1, 1)
	_, err = postRepo.Save(context.Background(), draft)
	require.NoError(t, err)
	require.NoError(t, searchStore.UpsertPost(context.Background(), draft.ID))

	results, err = searchStore.SearchPublishedPosts(context.Background(), "go", 10, nil)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestPostSearchStore_SearchPublishedPosts_EmptyQueryAndCursorBoundaries(t *testing.T) {
	tagRepo := NewTagRepository()
	postTagRepo := NewPostTagRepository()
	postRepo := NewPostRepository(tagRepo, postTagRepo)
	searchStore := NewPostSearchStore(postRepo, tagRepo, postTagRepo)

	first := testPost("go", "go", 1, 1)
	_, err := postRepo.Save(context.Background(), first)
	require.NoError(t, err)
	second := testPost("go", "go", 1, 1)
	_, err = postRepo.Save(context.Background(), second)
	require.NoError(t, err)
	require.NoError(t, searchStore.RebuildAll(context.Background()))

	emptyResults, err := searchStore.SearchPublishedPosts(context.Background(), "   ", 10, nil)
	require.NoError(t, err)
	assert.Empty(t, emptyResults)

	results, err := searchStore.SearchPublishedPosts(context.Background(), "go", 10, nil)
	require.NoError(t, err)
	require.Len(t, results, 2)

	tooHighCursor := &port.PostSearchCursor{Score: results[0].Score + 1, PostID: results[0].Post.ID}
	afterHigh, err := searchStore.SearchPublishedPosts(context.Background(), "go", 10, tooHighCursor)
	require.NoError(t, err)
	assert.Len(t, afterHigh, 2)

	equalTopCursor := &port.PostSearchCursor{Score: results[0].Score, PostID: results[0].Post.ID}
	afterEqualTop, err := searchStore.SearchPublishedPosts(context.Background(), "go", 10, equalTopCursor)
	require.NoError(t, err)
	require.Len(t, afterEqualTop, 1)
	assert.Equal(t, first.ID, afterEqualTop[0].Post.ID)

	lowestCursor := &port.PostSearchCursor{Score: results[1].Score, PostID: results[1].Post.ID}
	afterLowest, err := searchStore.SearchPublishedPosts(context.Background(), "go", 10, lowestCursor)
	require.NoError(t, err)
	assert.Empty(t, afterLowest)
}
