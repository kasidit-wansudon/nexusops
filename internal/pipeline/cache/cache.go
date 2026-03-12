package cache

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// CacheEntry represents a single cached item with usage tracking.
type CacheEntry struct {
	Key        string    `json:"key"`
	Paths      []string  `json:"paths"`
	Size       int64     `json:"size"`
	CreatedAt  time.Time `json:"created_at"`
	LastUsedAt time.Time `json:"last_used_at"`
	HitCount   int64     `json:"hit_count"`
}

// CacheStats provides aggregate statistics about the cache.
type CacheStats struct {
	TotalSize  int64   `json:"total_size"`
	EntryCount int     `json:"entry_count"`
	HitRate    float64 `json:"hit_rate"`
}

// Cache implements a file-system backed build cache with LRU eviction.
type Cache struct {
	basePath string
	maxSize  int64
	entries  map[string]*CacheEntry
	mu       sync.RWMutex
	hits     int64
	misses   int64
}

// NewCache creates a cache rooted at basePath with the given maximum size in
// bytes. It loads any previously persisted entries from disk.
func NewCache(basePath string, maxSize int64) (*Cache, error) {
	dataDir := filepath.Join(basePath, "data")
	metaDir := filepath.Join(basePath, "meta")
	for _, dir := range []string{dataDir, metaDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create cache directory %s: %w", dir, err)
		}
	}

	c := &Cache{
		basePath: basePath,
		maxSize:  maxSize,
		entries:  make(map[string]*CacheEntry),
	}

	// Load existing entries.
	metaEntries, _ := os.ReadDir(metaDir)
	for _, e := range metaEntries {
		if e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(metaDir, e.Name()))
		if err != nil {
			continue
		}
		var entry CacheEntry
		if err := json.Unmarshal(data, &entry); err != nil {
			continue
		}
		c.entries[entry.Key] = &entry
	}

	return c, nil
}

// Save stores the given file paths under the specified cache key as a
// compressed tar archive. If saving the entry would exceed maxSize, LRU
// eviction is performed first.
func (c *Cache) Save(key string, paths []string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	archivePath := c.archivePath(key)

	f, err := os.Create(archivePath)
	if err != nil {
		return fmt.Errorf("create cache archive: %w", err)
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	for _, p := range paths {
		if err := addToTar(tw, p); err != nil {
			// Clean up partial archive.
			_ = os.Remove(archivePath)
			return fmt.Errorf("add %s to tar: %w", p, err)
		}
	}

	// Close writers to flush before we stat the file.
	tw.Close()
	gw.Close()
	f.Close()

	info, err := os.Stat(archivePath)
	if err != nil {
		return fmt.Errorf("stat cache archive: %w", err)
	}

	entry := &CacheEntry{
		Key:        key,
		Paths:      paths,
		Size:       info.Size(),
		CreatedAt:  time.Now(),
		LastUsedAt: time.Now(),
		HitCount:   0,
	}

	// Evict if necessary.
	c.evictLocked(entry.Size)

	c.entries[key] = entry
	return c.saveMetaLocked(entry)
}

// Restore extracts a cached archive into destDir. Returns true if the cache
// contained the key, false on a miss.
func (c *Cache) Restore(key string, destDir string) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, exists := c.entries[key]
	if !exists {
		c.misses++
		return false, nil
	}

	archivePath := c.archivePath(key)
	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
		delete(c.entries, key)
		c.misses++
		return false, nil
	}

	f, err := os.Open(archivePath)
	if err != nil {
		c.misses++
		return false, fmt.Errorf("open cache archive: %w", err)
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		c.misses++
		return false, fmt.Errorf("open gzip reader: %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return false, fmt.Errorf("read tar header: %w", err)
		}

		target := filepath.Join(destDir, header.Name)

		// Guard against path traversal.
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(destDir)) {
			continue
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return false, fmt.Errorf("create directory %s: %w", target, err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return false, fmt.Errorf("create parent directory: %w", err)
			}
			outFile, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return false, fmt.Errorf("create file %s: %w", target, err)
			}
			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return false, fmt.Errorf("extract file %s: %w", target, err)
			}
			outFile.Close()
		}
	}

	entry.LastUsedAt = time.Now()
	entry.HitCount++
	c.hits++
	_ = c.saveMetaLocked(entry)

	return true, nil
}

// Invalidate removes a specific cache entry by key.
func (c *Cache) Invalidate(key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.entries[key]; !exists {
		return nil
	}

	_ = os.Remove(c.archivePath(key))
	_ = os.Remove(c.metaPath(key))
	delete(c.entries, key)
	return nil
}

// GenerateKey produces a deterministic SHA-256 cache key from arbitrary
// string inputs.
func GenerateKey(inputs ...string) string {
	h := sha256.New()
	for _, input := range inputs {
		h.Write([]byte(input))
		h.Write([]byte{0}) // separator
	}
	return hex.EncodeToString(h.Sum(nil))
}

// Prune removes cache entries that are older than maxAge since their last use.
func (c *Cache) Prune(maxAge time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	var errs []error

	for key, entry := range c.entries {
		if entry.LastUsedAt.Before(cutoff) {
			if err := os.Remove(c.archivePath(key)); err != nil && !os.IsNotExist(err) {
				errs = append(errs, err)
				continue
			}
			_ = os.Remove(c.metaPath(key))
			delete(c.entries, key)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("prune encountered %d errors; first: %w", len(errs), errs[0])
	}
	return nil
}

// GetStats returns aggregate cache statistics.
func (c *Cache) GetStats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var totalSize int64
	for _, entry := range c.entries {
		totalSize += entry.Size
	}

	var hitRate float64
	total := c.hits + c.misses
	if total > 0 {
		hitRate = float64(c.hits) / float64(total)
	}

	return CacheStats{
		TotalSize:  totalSize,
		EntryCount: len(c.entries),
		HitRate:    hitRate,
	}
}

// evictLocked removes the least-recently-used entries until there is enough
// room for sizeNeeded bytes. Must be called while holding c.mu.
func (c *Cache) evictLocked(sizeNeeded int64) {
	var totalSize int64
	for _, entry := range c.entries {
		totalSize += entry.Size
	}

	if totalSize+sizeNeeded <= c.maxSize {
		return
	}

	// Sort entries by LastUsedAt ascending (oldest first).
	type kv struct {
		key   string
		entry *CacheEntry
	}
	sorted := make([]kv, 0, len(c.entries))
	for k, v := range c.entries {
		sorted = append(sorted, kv{k, v})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].entry.LastUsedAt.Before(sorted[j].entry.LastUsedAt)
	})

	for _, item := range sorted {
		if totalSize+sizeNeeded <= c.maxSize {
			break
		}
		totalSize -= item.entry.Size
		_ = os.Remove(c.archivePath(item.key))
		_ = os.Remove(c.metaPath(item.key))
		delete(c.entries, item.key)
	}
}

// archivePath returns the filesystem path for a cache archive.
func (c *Cache) archivePath(key string) string {
	safeKey := hex.EncodeToString(sha256.New().Sum([]byte(key)))[:32]
	return filepath.Join(c.basePath, "data", safeKey+".tar.gz")
}

// metaPath returns the filesystem path for a cache entry's metadata file.
func (c *Cache) metaPath(key string) string {
	safeKey := hex.EncodeToString(sha256.New().Sum([]byte(key)))[:32]
	return filepath.Join(c.basePath, "meta", safeKey+".json")
}

// saveMetaLocked persists a cache entry's metadata to disk.
func (c *Cache) saveMetaLocked(entry *CacheEntry) error {
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal cache entry: %w", err)
	}
	return os.WriteFile(c.metaPath(entry.Key), data, 0o644)
}

// addToTar recursively adds a path (file or directory) to a tar writer.
func addToTar(tw *tar.Writer, path string) error {
	return filepath.Walk(path, func(file string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		header, err := tar.FileInfoHeader(fi, "")
		if err != nil {
			return fmt.Errorf("create tar header for %s: %w", file, err)
		}

		// Use relative path inside the archive.
		relPath, err := filepath.Rel(filepath.Dir(path), file)
		if err != nil {
			return fmt.Errorf("relative path: %w", err)
		}
		header.Name = relPath

		if err := tw.WriteHeader(header); err != nil {
			return fmt.Errorf("write tar header: %w", err)
		}

		if fi.IsDir() {
			return nil
		}

		f, err := os.Open(file)
		if err != nil {
			return fmt.Errorf("open %s: %w", file, err)
		}
		defer f.Close()

		if _, err := io.Copy(tw, f); err != nil {
			return fmt.Errorf("copy %s to tar: %w", file, err)
		}

		return nil
	})
}
