package artifact

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewStore(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "artifacts")

	store, err := NewStore(storePath)
	require.NoError(t, err)
	assert.NotNil(t, store)
	assert.Equal(t, storePath, store.basePath)

	// Verify directories were created.
	_, err = os.Stat(storePath)
	assert.NoError(t, err)
	_, err = os.Stat(filepath.Join(storePath, "meta"))
	assert.NoError(t, err)
}

func TestSaveAndGet(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(filepath.Join(dir, "store"))
	require.NoError(t, err)

	// Create a temporary source file.
	srcPath := filepath.Join(dir, "build-output.bin")
	content := []byte("hello artifact world")
	require.NoError(t, os.WriteFile(srcPath, content, 0o644))

	// Save the artifact.
	art, err := store.Save("pipeline-1", "build", srcPath)
	require.NoError(t, err)
	assert.NotEmpty(t, art.ID)
	assert.Equal(t, "pipeline-1", art.PipelineID)
	assert.Equal(t, "build", art.StepName)
	assert.Equal(t, int64(len(content)), art.Size)
	assert.NotEmpty(t, art.Checksum)
	assert.False(t, art.CreatedAt.IsZero())
	assert.False(t, art.ExpiresAt.IsZero())

	// Retrieve the artifact by ID.
	got, err := store.Get(art.ID)
	require.NoError(t, err)
	assert.Equal(t, art.ID, got.ID)
	assert.Equal(t, art.PipelineID, got.PipelineID)
	assert.Equal(t, art.Checksum, got.Checksum)
	assert.Equal(t, art.Size, got.Size)

	// Verify the file on disk matches.
	data, err := os.ReadFile(got.Path)
	require.NoError(t, err)
	assert.Equal(t, content, data)
}

func TestListArtifacts(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(filepath.Join(dir, "store"))
	require.NoError(t, err)

	// Create source files and save artifacts for two different pipelines.
	for i, pid := range []string{"pipeline-a", "pipeline-a", "pipeline-b"} {
		srcPath := filepath.Join(dir, "file"+string(rune('0'+i)))
		require.NoError(t, os.WriteFile(srcPath, []byte("data"), 0o644))
		_, err := store.Save(pid, "step", srcPath)
		require.NoError(t, err)
	}

	// List artifacts for pipeline-a only.
	list, err := store.List("pipeline-a")
	require.NoError(t, err)
	assert.Len(t, list, 2)
	for _, a := range list {
		assert.Equal(t, "pipeline-a", a.PipelineID)
	}

	// List artifacts for pipeline-b.
	list, err = store.List("pipeline-b")
	require.NoError(t, err)
	assert.Len(t, list, 1)
	assert.Equal(t, "pipeline-b", list[0].PipelineID)

	// List artifacts for nonexistent pipeline.
	list, err = store.List("pipeline-missing")
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestDeleteArtifact(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(filepath.Join(dir, "store"))
	require.NoError(t, err)

	srcPath := filepath.Join(dir, "deleteme.txt")
	require.NoError(t, os.WriteFile(srcPath, []byte("to delete"), 0o644))

	art, err := store.Save("pipeline-1", "build", srcPath)
	require.NoError(t, err)

	// Verify artifact exists before deletion.
	got, err := store.Get(art.ID)
	require.NoError(t, err)
	assert.Equal(t, art.ID, got.ID)

	// Delete the artifact.
	err = store.Delete(art.ID)
	require.NoError(t, err)

	// Get should fail after deletion.
	_, err = store.Get(art.ID)
	assert.Error(t, err)
}

func TestCleanExpired(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(filepath.Join(dir, "store"))
	require.NoError(t, err)

	// Save an artifact normally.
	srcPath := filepath.Join(dir, "file.txt")
	require.NoError(t, os.WriteFile(srcPath, []byte("content"), 0o644))

	art, err := store.Save("pipeline-1", "build", srcPath)
	require.NoError(t, err)

	// Manually set ExpiresAt to the past and re-save metadata.
	art.ExpiresAt = time.Now().Add(-1 * time.Hour)
	require.NoError(t, store.saveMeta(art))

	// Cleanup should remove the expired artifact.
	err = store.Cleanup()
	require.NoError(t, err)

	// Artifact should no longer be retrievable.
	_, err = store.Get(art.ID)
	assert.Error(t, err)

	// List should be empty.
	list, err := store.List("pipeline-1")
	require.NoError(t, err)
	assert.Empty(t, list)
}
