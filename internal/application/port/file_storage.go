package port

import "io"

type FileStorage interface {
	Save(key string, content io.Reader) error
	Delete(key string) error
}
