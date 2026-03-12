package cache

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCache(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "cache")

	c, err := NewCache(cachePath, 1024*1024)
	require.NoError(t, err)
	assert.NotNil(t, c)
	assert.Equal(t, cachePath, c.basePath)
	assert.Equal(t, int64(1024*1024), c.maxSize)

	// Verify directories were created.
	_, err = os.Stat(filepath.Join(cachePath, "data"))
	assert.NoError(t, err)
	_, err = os.Stat(filepath.Join(cachePath, "meta"))
	assert.NoError(t, err)
}

func TestPutAndGet(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "cache")
	c, err := NewCache(cachePath, 10*1024*1024)
	require.NoError(t, err)

	// Create a file to cache.
	srcFile := filepath.Join(dir, "myfile.txt")
	require.NoError(t, os.WriteFile(srcFile, []byte("cached content"), 0o644))

	// Save to cache.
	err = c.Save("test-key", []string{srcFile})
	require.NoError(t, err)

	// The entry should be tracked.
	assert.Contains(t, c.entries, "test-key")
	assert.Equal(t, "test-key", c.entries["test-key"].Key)

	// Restore to a new directory.
	restoreDir := filepath.Join(dir, "restored")
	require.NoError(t, os.MkdirAll(restoreDir, 0o755))

	found, err := c.Restore("test-key", restoreDir)
	require.NoError(t, err)
	assert.True(t, found)

	// Miss on a key that does not exist.
	found, err = c.Restore("nonexistent-key", restoreDir)
	require.NoError(t, err)
	assert.False(t, found)
}

func TestCacheHasKey(t *testing.T) {
	dir := t.TempDir()
	c, err := NewCache(filepath.Join(dir, "cache"), 10*1024*1024)
	require.NoError(t, err)

	srcFile := filepath.Join(dir, "data.txt")
	require.NoError(t, os.WriteFile(srcFile, []byte("some data"), 0o644))

	err = c.Save("my-key", []string{srcFile})
	require.NoError(t, err)

	// Key should be present in entries.
	_, exists := c.entries["my-key"]
	assert.True(t, exists)

	// A key that was never saved should not be present.
	_, exists = c.entries["missing-key"]
	assert.False(t, exists)
}

func TestCacheDelete(t *testing.T) {
	dir := t.TempDir()
	c, err := NewCache(filepath.Join(dir, "cache"), 10*1024*1024)
	require.NoError(t, err)

	srcFile := filepath.Join(dir, "data.txt")
	require.NoError(t, os.WriteFile(srcFile, []byte("data to delete"), 0o644))

	err = c.Save("del-key", []string{srcFile})
	require.NoError(t, err)

	_, exists := c.entries["del-key"]
	assert.True(t, exists)

	// Invalidate (delete) the entry.
	err = c.Invalidate("del-key")
	require.NoError(t, err)

	_, exists = c.entries["del-key"]
	assert.False(t, exists)

	// Restore should miss after invalidation.
	restoreDir := filepath.Join(dir, "restored")
	require.NoError(t, os.MkdirAll(restoreDir, 0o755))
	found, err := c.Restore("del-key", restoreDir)
	require.NoError(t, err)
	assert.False(t, found)
}

func TestCacheStats(t *testing.T) {
	dir := t.TempDir()
	c, err := NewCache(filepath.Join(dir, "cache"), 10*1024*1024)
	require.NoError(t, err)

	// Initially empty.
	stats := c.GetStats()
	assert.Equal(t, 0, stats.EntryCount)
	assert.Equal(t, int64(0), stats.TotalSize)
	assert.Equal(t, float64(0), stats.HitRate)

	// Save an entry.
	srcFile := filepath.Join(dir, "data.txt")
	require.NoError(t, os.WriteFile(srcFile, []byte("stats test data"), 0o644))
	err = c.Save("stats-key", []string{srcFile})
	require.NoError(t, err)

	stats = c.GetStats()
	assert.Equal(t, 1, stats.EntryCount)
	assert.Greater(t, stats.TotalSize, int64(0))

	// Trigger a hit.
	restoreDir := filepath.Join(dir, "restored")
	require.NoError(t, os.MkdirAll(restoreDir, 0o755))
	_, err = c.Restore("stats-key", restoreDir)
	require.NoError(t, err)

	// Trigger a miss.
	_, err = c.Restore("no-such-key", restoreDir)
	require.NoError(t, err)

	stats = c.GetStats()
	// 1 hit, 1 miss => hitRate = 0.5
	assert.Equal(t, float64(0.5), stats.HitRate)
}

func TestGenerateCacheKey(t *testing.T) {
	// Deterministic: same inputs produce the same key.
	key1 := GenerateKey("go", "1.22", "linux")
	key2 := GenerateKey("go", "1.22", "linux")
	assert.Equal(t, key1, key2)

	// Different inputs produce different keys.
	key3 := GenerateKey("go", "1.21", "linux")
	assert.NotEqual(t, key1, key3)

	// Key is a hex-encoded SHA-256 (64 characters).
	assert.Len(t, key1, 64)
}
