package localfs

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
)

var _ port.FileStorage = (*FileStorage)(nil)

type FileStorage struct {
	rootDir string
}

func NewFileStorage(rootDir string) *FileStorage {
	return &FileStorage{rootDir: rootDir}
}

func (s *FileStorage) Save(ctx context.Context, key string, content io.Reader) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	fullPath, err := s.resolve(key)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	dir := filepath.Dir(fullPath)
	file, err := os.CreateTemp(dir, ".upload-*")
	if err != nil {
		return err
	}
	tempPath := file.Name()
	defer func() {
		_ = file.Close()
		if tempPath != "" {
			_ = os.Remove(tempPath)
		}
	}()
	_, err = io.Copy(file, &contextReader{ctx: ctx, reader: content})
	if err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, fullPath); err != nil {
		return err
	}
	tempPath = ""
	return nil
}

func (s *FileStorage) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	fullPath, err := s.resolve(key)
	if err != nil {
		return nil, err
	}
	return os.Open(fullPath)
}

func (s *FileStorage) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	fullPath, err := s.resolve(key)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	err = os.Remove(fullPath)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (s *FileStorage) resolve(key string) (string, error) {
	cleanKey := filepath.Clean(key)
	if cleanKey == "." || cleanKey == "" {
		return "", fmt.Errorf("invalid storage key")
	}
	if strings.HasPrefix(cleanKey, "..") || filepath.IsAbs(cleanKey) {
		return "", fmt.Errorf("invalid storage key")
	}
	return filepath.Join(s.rootDir, cleanKey), nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}
