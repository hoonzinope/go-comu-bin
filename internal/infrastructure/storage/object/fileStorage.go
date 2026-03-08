package object

import (
	"bytes"
	"context"
	"io"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var _ port.FileStorage = (*FileStorage)(nil)

type objectClient interface {
	PutObject(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error)
	GetObject(ctx context.Context, bucketName, objectName string, opts minio.GetObjectOptions) (*minio.Object, error)
	RemoveObject(ctx context.Context, bucketName, objectName string, opts minio.RemoveObjectOptions) error
}

type FileStorage struct {
	client objectClient
	bucket string
}

func NewFileStorage(endpoint, bucket, accessKey, secretKey string, useSSL bool) (*FileStorage, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, err
	}
	return &FileStorage{client: client, bucket: bucket}, nil
}

func NewFileStorageWithClient(client objectClient, bucket string) *FileStorage {
	return &FileStorage{client: client, bucket: bucket}
}

func (s *FileStorage) Save(key string, content io.Reader) error {
	data, err := io.ReadAll(content)
	if err != nil {
		return err
	}
	_, err = s.client.PutObject(context.Background(), s.bucket, key, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{})
	return err
}

func (s *FileStorage) Open(key string) (io.ReadCloser, error) {
	object, err := s.client.GetObject(context.Background(), s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	return object, nil
}

func (s *FileStorage) Delete(key string) error {
	return s.client.RemoveObject(context.Background(), s.bucket, key, minio.RemoveObjectOptions{})
}
