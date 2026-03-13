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
	_ = ctx
	fullPath, err := s.resolve(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return err
	}
	file, err := os.Create(fullPath)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, content)
	return err
}

func (s *FileStorage) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	_ = ctx
	fullPath, err := s.resolve(key)
	if err != nil {
		return nil, err
	}
	return os.Open(fullPath)
}

func (s *FileStorage) Delete(ctx context.Context, key string) error {
	_ = ctx
	fullPath, err := s.resolve(key)
	if err != nil {
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
