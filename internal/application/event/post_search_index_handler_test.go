package event

import (
	"context"
	"testing"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type spyPostSearchIndexer struct {
	rebuildCalls int
	upserted     []int64
	deleted      []int64
}

func (s *spyPostSearchIndexer) RebuildAll(context.Context) error {
	s.rebuildCalls++
	return nil
}

func (s *spyPostSearchIndexer) UpsertPost(context.Context, int64) error {
	return nil
}

func (s *spyPostSearchIndexer) DeletePost(context.Context, int64) error {
	return nil
}

func (s *spyPostSearchIndexer) HandlePostChanged(ctx context.Context, postID int64) error {
	return s.UpsertPost(ctx, postID)
}

func TestPostSearchIndexHandler_PostChangedRoutesToIndexer(t *testing.T) {
	indexer := &capturingPostSearchIndexer{}
	handler := NewPostSearchIndexHandler(indexer)
	publishedAt := time.Now()

	require.NoError(t, handler.Handle(context.Background(), NewPostChanged("created", 11, 2, &publishedAt, []string{"go"}, nil)))
	require.NoError(t, handler.Handle(context.Background(), NewPostChanged("updated", 12, 2, &publishedAt, []string{"go"}, nil)))
	require.NoError(t, handler.Handle(context.Background(), NewPostChanged("published", 13, 2, &publishedAt, []string{"go"}, nil)))
	require.NoError(t, handler.Handle(context.Background(), NewPostChanged("deleted", 14, 2, &publishedAt, []string{"go"}, nil)))

	assert.Equal(t, []int64{11, 12, 13}, indexer.upserted)
	assert.Equal(t, []int64{14}, indexer.deleted)
}

func TestPostSearchIndexHandler_IgnoresOtherEvents(t *testing.T) {
	indexer := &capturingPostSearchIndexer{}
	handler := NewPostSearchIndexHandler(indexer)

	require.NoError(t, handler.Handle(context.Background(), NewBoardChanged("updated", 1)))

	assert.Empty(t, indexer.upserted)
	assert.Empty(t, indexer.deleted)
}

func TestPostSearchIndexHandler_HandlesNilInputs(t *testing.T) {
	var nilHandler *PostSearchIndexHandler
	require.NoError(t, nilHandler.Handle(context.Background(), nil))

	handler := NewPostSearchIndexHandler(nil)
	require.NoError(t, handler.Handle(context.Background(), nil))
}

type capturingPostSearchIndexer struct {
	upserted []int64
	deleted  []int64
}

func (s *capturingPostSearchIndexer) RebuildAll(context.Context) error { return nil }

func (s *capturingPostSearchIndexer) UpsertPost(_ context.Context, postID int64) error {
	s.upserted = append(s.upserted, postID)
	return nil
}

func (s *capturingPostSearchIndexer) DeletePost(_ context.Context, postID int64) error {
	s.deleted = append(s.deleted, postID)
	return nil
}

var _ port.PostSearchIndexer = (*capturingPostSearchIndexer)(nil)
