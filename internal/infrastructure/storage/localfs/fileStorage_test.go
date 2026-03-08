package localfs

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileStorage_SaveAndDelete(t *testing.T) {
	rootDir := t.TempDir()
	storage := NewFileStorage(rootDir)

	require.NoError(t, storage.Save("posts/1/a.txt", strings.NewReader("hello")))

	reader, err := storage.Open("posts/1/a.txt")
	require.NoError(t, err)
	defer reader.Close()
	opened, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, "hello", string(opened))

	data, err := os.ReadFile(filepath.Join(rootDir, "posts/1/a.txt"))
	require.NoError(t, err)
	assert.Equal(t, "hello", string(data))

	require.NoError(t, storage.Delete("posts/1/a.txt"))

	_, err = os.Stat(filepath.Join(rootDir, "posts/1/a.txt"))
	assert.True(t, os.IsNotExist(err))
}

func TestFileStorage_Save_RejectsPathTraversal(t *testing.T) {
	rootDir := t.TempDir()
	storage := NewFileStorage(rootDir)

	err := storage.Save("../escape.txt", strings.NewReader("hello"))
	require.Error(t, err)
}
