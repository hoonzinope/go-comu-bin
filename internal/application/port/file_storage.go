package port

import "io"

type FileStorage interface {
	Save(key string, content io.Reader) error
	Open(key string) (io.ReadCloser, error)
	Delete(key string) error
}
