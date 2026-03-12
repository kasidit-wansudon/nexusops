package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCacheCreatesDirectories(t *testing.T) {
	tmp := t.TempDir()
	basePath := filepath.Join(tmp, "cache")

	c, err := NewCache(basePath, 1<<30) // 1 GB
	require.NoError(t, err)
	require.NotNil(t, c)

	// Verify subdirectories were created
	_, err = os.Stat(filepath.Join(basePath, "data"))
	assert.NoError(t, err)
	_, err = os.Stat(filepath.Join(basePath, "meta"))
	assert.NoError(t, err)
}

func TestSaveAndRestore(t *testing.T) {
	tmp := t.TempDir()
	basePath := filepath.Join(tmp, "cache")
	c, err := NewCache(basePath, 1<<30)
	require.NoError(t, err)

	// Create a source file to cache
	srcDir := filepath.Join(tmp, "src")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))
	srcFile := filepath.Join(srcDir, "build.txt")
	require.NoError(t, os.WriteFile(srcFile, []byte("build output data"), 0o644))

	// Save to cache
	err = c.Save("build-key-1", []string{srcFile})
	require.NoError(t, err)

	// Restore to a different directory
	destDir := filepath.Join(tmp, "dest")
	require.NoError(t, os.MkdirAll(destDir, 0o755))

	hit, err := c.Restore("build-key-1", destDir)
	require.NoError(t, err)
	assert.True(t, hit)

	// Cache miss
	hit, err = c.Restore("nonexistent-key", destDir)
	require.NoError(t, err)
	assert.False(t, hit)
}

func TestInvalidateAndGetStats(t *testing.T) {
	tmp := t.TempDir()
	basePath := filepath.Join(tmp, "cache")
	c, err := NewCache(basePath, 1<<30)
	require.NoError(t, err)

	// Create and cache a file
	srcFile := filepath.Join(tmp, "data.txt")
	require.NoError(t, os.WriteFile(srcFile, []byte("content"), 0o644))
	require.NoError(t, c.Save("key-1", []string{srcFile}))

	stats := c.GetStats()
	assert.Equal(t, 1, stats.EntryCount)
	assert.Greater(t, stats.TotalSize, int64(0))

	// Invalidate
	err = c.Invalidate("key-1")
	require.NoError(t, err)

	stats = c.GetStats()
	assert.Equal(t, 0, stats.EntryCount)
	assert.Equal(t, int64(0), stats.TotalSize)

	// Invalidate nonexistent key is a no-op
	err = c.Invalidate("nonexistent")
	require.NoError(t, err)
}

func TestPrune(t *testing.T) {
	tmp := t.TempDir()
	basePath := filepath.Join(tmp, "cache")
	c, err := NewCache(basePath, 1<<30)
	require.NoError(t, err)

	srcFile := filepath.Join(tmp, "data.txt")
	require.NoError(t, os.WriteFile(srcFile, []byte("content"), 0o644))
	require.NoError(t, c.Save("old-key", []string{srcFile}))

	// Artificially age the entry
	c.entries["old-key"].LastUsedAt = time.Now().Add(-2 * time.Hour)

	require.NoError(t, os.WriteFile(srcFile, []byte("new content"), 0o644))
	require.NoError(t, c.Save("new-key", []string{srcFile}))

	err = c.Prune(1 * time.Hour)
	require.NoError(t, err)

	stats := c.GetStats()
	assert.Equal(t, 1, stats.EntryCount) // only "new-key" remains
}

func TestGenerateKey(t *testing.T) {
	key1 := GenerateKey("go", "1.22", "linux")
	key2 := GenerateKey("go", "1.22", "linux")
	key3 := GenerateKey("go", "1.22", "darwin")

	assert.Equal(t, key1, key2, "same inputs should produce the same key")
	assert.NotEqual(t, key1, key3, "different inputs should produce different keys")
	assert.Len(t, key1, 64) // SHA-256 hex is 64 chars
}

func TestHitRate(t *testing.T) {
	tmp := t.TempDir()
	basePath := filepath.Join(tmp, "cache")
	c, err := NewCache(basePath, 1<<30)
	require.NoError(t, err)

	srcFile := filepath.Join(tmp, "data.txt")
	require.NoError(t, os.WriteFile(srcFile, []byte("content"), 0o644))
	require.NoError(t, c.Save("key-1", []string{srcFile}))

	destDir := filepath.Join(tmp, "dest")
	require.NoError(t, os.MkdirAll(destDir, 0o755))

	// Hit
	_, _ = c.Restore("key-1", destDir)
	// Miss
	_, _ = c.Restore("miss-key", destDir)

	stats := c.GetStats()
	assert.InDelta(t, 0.5, stats.HitRate, 0.01) // 1 hit, 1 miss = 50%
}
