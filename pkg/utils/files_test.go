package utils

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
)

func TestCopyFile(t *testing.T) {
	src, _ := os.CreateTemp("", "src-*.txt")
	dest, _ := os.CreateTemp("", "dest-*.txt")
	_, _ = src.Write([]byte("Hello World!"))
	_ = src.Close()
	err := CopyFile(src.Name(), dest.Name())
	require.NoError(t, err)
	assert.FileExists(t, dest.Name())
	destBytes, err := os.ReadFile(dest.Name())
	require.NoError(t, err)
	assert.Equal(t, string(destBytes), "Hello World!")
}

func TestCopyFileToDir(t *testing.T) {
	src, _ := os.CreateTemp("", "src-*.txt")
	destDir, _ := os.MkdirTemp("", "dest-*")
	_, _ = src.Write([]byte("Hello World!"))
	_ = src.Close()
	dest, err := CopyFileToDir(src.Name(), destDir)
	require.NoError(t, err)
	assert.FileExists(t, dest)
	destBytes, err := os.ReadFile(dest)
	require.NoError(t, err)
	assert.Equal(t, string(destBytes), "Hello World!")
}

func TestListFiles(t *testing.T) {
	files, err := ListFiles(filepath.Join("..", "..", "test", "testdata", "fixtures", "multi-dc"), "k8s*.yaml")
	require.NoError(t, err)
	require.Len(t, files, 1)
	assert.Equal(t, files[0], "../../test/testdata/fixtures/multi-dc/k8ssandra.yaml")
}

func TestReadLines(t *testing.T) {
	src, _ := os.CreateTemp("", "src-*.txt")
	_, _ = src.Write([]byte("Hello\nWorld!"))
	_ = src.Close()
	lines, err := ReadLines(src.Name())
	require.NoError(t, err)
	assert.Equal(t, []string{"Hello", "World!"}, lines)
}
