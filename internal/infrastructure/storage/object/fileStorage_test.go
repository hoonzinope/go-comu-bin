package object

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeObjectClient struct {
	putBucket string
	putKey    string
	putBody   string
	putSize   int64
	putReader io.Reader
	getBucket string
	getKey    string
	delBucket string
	delKey    string
}

func (f *fakeObjectClient) PutObject(_ context.Context, bucketName, objectName string, reader io.Reader, objectSize int64, _ minio.PutObjectOptions) (minio.UploadInfo, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return minio.UploadInfo{}, err
	}
	f.putBucket = bucketName
	f.putKey = objectName
	f.putBody = string(data)
	f.putSize = objectSize
	f.putReader = reader
	return minio.UploadInfo{}, nil
}

func (f *fakeObjectClient) GetObject(_ context.Context, bucketName, objectName string, _ minio.GetObjectOptions) (*minio.Object, error) {
	f.getBucket = bucketName
	f.getKey = objectName
	return nil, nil
}

func (f *fakeObjectClient) RemoveObject(_ context.Context, bucketName, objectName string, _ minio.RemoveObjectOptions) error {
	f.delBucket = bucketName
	f.delKey = objectName
	return nil
}

func TestFileStorage_SaveAndDelete(t *testing.T) {
	client := &fakeObjectClient{}
	storage := NewFileStorageWithClient(client, "attachments")

	require.NoError(t, storage.Save("posts/1/a.png", bytes.NewReader([]byte("hello"))))
	assert.Equal(t, "attachments", client.putBucket)
	assert.Equal(t, "posts/1/a.png", client.putKey)
	assert.Equal(t, "hello", client.putBody)

	require.NoError(t, storage.Delete("posts/1/a.png"))
	assert.Equal(t, "attachments", client.delBucket)
	assert.Equal(t, "posts/1/a.png", client.delKey)
}

func TestFileStorage_Save_UsesSizedReaderWithoutCopyWhenAvailable(t *testing.T) {
	client := &fakeObjectClient{}
	storage := NewFileStorageWithClient(client, "attachments")
	reader := bytes.NewReader([]byte("hello"))

	require.NoError(t, storage.Save("posts/1/a.png", reader))

	assert.Same(t, reader, client.putReader)
	assert.EqualValues(t, 5, client.putSize)
}
