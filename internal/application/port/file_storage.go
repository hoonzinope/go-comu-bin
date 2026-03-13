package port

import (
	"context"
	"io"
)

type FileStorage interface {
	Save(ctx context.Context, key string, content io.Reader) error
	Open(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
}
