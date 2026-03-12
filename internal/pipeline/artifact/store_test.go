package artifact

import (
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewStoreCreatesDirectories(t *testing.T) {
	tmp := t.TempDir()
	basePath := filepath.Join(tmp, "artifacts")

	store, err := NewStore(basePath)
	require.NoError(t, err)
	require.NotNil(t, store)

	_, err = os.Stat(basePath)
	assert.NoError(t, err)
	_, err = os.Stat(filepath.Join(basePath, "meta"))
	assert.NoError(t, err)
}

func TestSaveAndGet(t *testing.T) {
	tmp := t.TempDir()
	store, err := NewStore(filepath.Join(tmp, "artifacts"))
	require.NoError(t, err)

	// Create a source file
	srcFile := filepath.Join(tmp, "build.bin")
	content := []byte("compiled binary content here")
	require.NoError(t, os.WriteFile(srcFile, content, 0o644))

	art, err := store.Save("pipeline-1", "compile", srcFile)
	require.NoError(t, err)
	assert.NotEmpty(t, art.ID)
	assert.Equal(t, "pipeline-1", art.PipelineID)
	assert.Equal(t, "compile", art.StepName)
	assert.Equal(t, int64(len(content)), art.Size)
	assert.NotEmpty(t, art.Checksum)
	assert.Len(t, art.Checksum, 64) // SHA-256 hex
	assert.False(t, art.CreatedAt.IsZero())
	assert.True(t, art.ExpiresAt.After(art.CreatedAt))

	// Get metadata
	got, err := store.Get(art.ID)
	require.NoError(t, err)
	assert.Equal(t, art.ID, got.ID)
	assert.Equal(t, art.Checksum, got.Checksum)
	assert.Equal(t, art.Size, got.Size)

	// Get nonexistent
	_, err = store.Get("nonexistent-id")
	assert.Error(t, err)
}

func TestDownloadAndList(t *testing.T) {
	tmp := t.TempDir()
	store, err := NewStore(filepath.Join(tmp, "artifacts"))
	require.NoError(t, err)

	content := []byte("artifact data")
	srcFile := filepath.Join(tmp, "out.dat")
	require.NoError(t, os.WriteFile(srcFile, content, 0o644))

	art, err := store.Save("pipe-a", "test-step", srcFile)
	require.NoError(t, err)

	// Download and read back
	rc, err := store.Download(art.ID)
	require.NoError(t, err)
	defer rc.Close()

	data, err := io.ReadAll(rc)
	require.NoError(t, err)
	assert.Equal(t, content, data)

	// Download nonexistent
	_, err = store.Download("nonexistent")
	assert.Error(t, err)

	// List by pipeline
	// Add another artifact under the same pipeline
	srcFile2 := filepath.Join(tmp, "out2.dat")
	require.NoError(t, os.WriteFile(srcFile2, []byte("second"), 0o644))
	_, err = store.Save("pipe-a", "build-step", srcFile2)
	require.NoError(t, err)

	// Add artifact under different pipeline
	srcFile3 := filepath.Join(tmp, "out3.dat")
	require.NoError(t, os.WriteFile(srcFile3, []byte("third"), 0o644))
	_, err = store.Save("pipe-b", "deploy-step", srcFile3)
	require.NoError(t, err)

	list, err := store.List("pipe-a")
	require.NoError(t, err)
	assert.Len(t, list, 2)

	listB, err := store.List("pipe-b")
	require.NoError(t, err)
	assert.Len(t, listB, 1)

	listEmpty, err := store.List("nonexistent-pipeline")
	require.NoError(t, err)
	assert.Empty(t, listEmpty)
}

func TestDeleteArtifact(t *testing.T) {
	tmp := t.TempDir()
	store, err := NewStore(filepath.Join(tmp, "artifacts"))
	require.NoError(t, err)

	srcFile := filepath.Join(tmp, "file.txt")
	require.NoError(t, os.WriteFile(srcFile, []byte("data"), 0o644))

	art, err := store.Save("pipe-1", "step", srcFile)
	require.NoError(t, err)

	// Verify it exists
	_, err = store.Get(art.ID)
	require.NoError(t, err)

	// Delete
	err = store.Delete(art.ID)
	require.NoError(t, err)

	// Verify deleted
	_, err = store.Get(art.ID)
	assert.Error(t, err)

	// Delete nonexistent is an error
	err = store.Delete("nonexistent")
	assert.Error(t, err)
}

func TestCleanup(t *testing.T) {
	tmp := t.TempDir()
	basePath := filepath.Join(tmp, "artifacts")
	store, err := NewStore(basePath)
	require.NoError(t, err)

	srcFile := filepath.Join(tmp, "file.txt")
	require.NoError(t, os.WriteFile(srcFile, []byte("content"), 0o644))

	art, err := store.Save("pipe-1", "step", srcFile)
	require.NoError(t, err)

	// Artificially expire the artifact by rewriting its metadata
	art.ExpiresAt = time.Now().Add(-1 * time.Hour)
	require.NoError(t, store.saveMeta(art))

	// Add a non-expired artifact
	srcFile2 := filepath.Join(tmp, "file2.txt")
	require.NoError(t, os.WriteFile(srcFile2, []byte("fresh"), 0o644))
	_, err = store.Save("pipe-1", "step2", srcFile2)
	require.NoError(t, err)

	err = store.Cleanup()
	require.NoError(t, err)

	// The expired one should be gone
	_, err = store.Get(art.ID)
	assert.Error(t, err)

	// The fresh one should remain
	list, err := store.List("pipe-1")
	require.NoError(t, err)
	assert.Len(t, list, 1)
}
