package port

import "context"

type PostSearchIndexer interface {
	RebuildAll(ctx context.Context) error
	UpsertPost(ctx context.Context, postID int64) error
	DeletePost(ctx context.Context, postID int64) error
}
