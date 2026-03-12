package artifact

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Artifact represents a stored build artifact with metadata.
type Artifact struct {
	ID         string    `json:"id"`
	PipelineID string    `json:"pipeline_id"`
	StepName   string    `json:"step_name"`
	Path       string    `json:"path"`
	Size       int64     `json:"size"`
	Checksum   string    `json:"checksum"`
	CreatedAt  time.Time `json:"created_at"`
	ExpiresAt  time.Time `json:"expires_at"`
}

// Store manages build artifact storage on the local filesystem.
type Store struct {
	basePath string
	mu       sync.RWMutex
}

// NewStore creates a new artifact store rooted at basePath. The directory
// structure is created automatically if it does not exist.
func NewStore(basePath string) (*Store, error) {
	if err := os.MkdirAll(basePath, 0o755); err != nil {
		return nil, fmt.Errorf("create artifact store directory: %w", err)
	}
	metaDir := filepath.Join(basePath, "meta")
	if err := os.MkdirAll(metaDir, 0o755); err != nil {
		return nil, fmt.Errorf("create meta directory: %w", err)
	}
	return &Store{basePath: basePath}, nil
}

// Save copies a source file into the artifact store. It computes a SHA-256
// checksum, records metadata, and returns the resulting Artifact.
func (s *Store) Save(pipelineID, stepName, sourcePath string) (*Artifact, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	srcFile, err := os.Open(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("open source file %s: %w", sourcePath, err)
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat source file: %w", err)
	}

	id := uuid.New().String()

	// Create the artifact directory tree: basePath/data/<pipelineID>/<id>
	artifactDir := filepath.Join(s.basePath, "data", pipelineID, id)
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		return nil, fmt.Errorf("create artifact directory: %w", err)
	}

	destPath := filepath.Join(artifactDir, filepath.Base(sourcePath))
	destFile, err := os.Create(destPath)
	if err != nil {
		return nil, fmt.Errorf("create destination file: %w", err)
	}
	defer destFile.Close()

	hasher := sha256.New()
	writer := io.MultiWriter(destFile, hasher)

	written, err := io.Copy(writer, srcFile)
	if err != nil {
		// Clean up on failure.
		_ = os.RemoveAll(artifactDir)
		return nil, fmt.Errorf("copy artifact data: %w", err)
	}

	checksum := hex.EncodeToString(hasher.Sum(nil))

	art := &Artifact{
		ID:         id,
		PipelineID: pipelineID,
		StepName:   stepName,
		Path:       destPath,
		Size:       written,
		Checksum:   checksum,
		CreatedAt:  time.Now(),
		ExpiresAt:  time.Now().Add(7 * 24 * time.Hour), // default 7-day retention
	}

	if srcInfo.Size() != written {
		_ = os.RemoveAll(artifactDir)
		return nil, fmt.Errorf("size mismatch: expected %d, wrote %d", srcInfo.Size(), written)
	}

	if err := s.saveMeta(art); err != nil {
		_ = os.RemoveAll(artifactDir)
		return nil, err
	}

	return art, nil
}

// Get retrieves artifact metadata by ID.
func (s *Store) Get(artifactID string) (*Artifact, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.loadMeta(artifactID)
}

// Download opens the artifact file for reading. The caller is responsible for
// closing the returned ReadCloser.
func (s *Store) Download(artifactID string) (io.ReadCloser, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	art, err := s.loadMeta(artifactID)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(art.Path)
	if err != nil {
		return nil, fmt.Errorf("open artifact file: %w", err)
	}
	return f, nil
}

// List returns all artifacts belonging to a given pipeline.
func (s *Store) List(pipelineID string) ([]*Artifact, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	metaDir := filepath.Join(s.basePath, "meta")
	entries, err := os.ReadDir(metaDir)
	if err != nil {
		return nil, fmt.Errorf("read meta directory: %w", err)
	}

	var artifacts []*Artifact
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		art, err := s.loadMetaFromFile(filepath.Join(metaDir, entry.Name()))
		if err != nil {
			continue
		}
		if art.PipelineID == pipelineID {
			artifacts = append(artifacts, art)
		}
	}
	return artifacts, nil
}

// Delete removes an artifact's data and metadata from the store.
func (s *Store) Delete(artifactID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	art, err := s.loadMeta(artifactID)
	if err != nil {
		return err
	}

	// Remove data directory (two levels up from the file: data/<pipeline>/<id>).
	artifactDir := filepath.Dir(art.Path)
	if err := os.RemoveAll(artifactDir); err != nil {
		return fmt.Errorf("remove artifact data: %w", err)
	}

	// Remove metadata file.
	metaPath := filepath.Join(s.basePath, "meta", artifactID+".json")
	if err := os.Remove(metaPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove artifact metadata: %w", err)
	}

	return nil
}

// Cleanup removes all artifacts whose ExpiresAt has passed.
func (s *Store) Cleanup() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	metaDir := filepath.Join(s.basePath, "meta")
	entries, err := os.ReadDir(metaDir)
	if err != nil {
		return fmt.Errorf("read meta directory: %w", err)
	}

	now := time.Now()
	var errs []error

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		art, err := s.loadMetaFromFile(filepath.Join(metaDir, entry.Name()))
		if err != nil {
			errs = append(errs, err)
			continue
		}

		if now.After(art.ExpiresAt) {
			artifactDir := filepath.Dir(art.Path)
			if removeErr := os.RemoveAll(artifactDir); removeErr != nil {
				errs = append(errs, fmt.Errorf("remove expired artifact %s: %w", art.ID, removeErr))
				continue
			}
			metaPath := filepath.Join(metaDir, entry.Name())
			if removeErr := os.Remove(metaPath); removeErr != nil && !os.IsNotExist(removeErr) {
				errs = append(errs, fmt.Errorf("remove expired meta %s: %w", art.ID, removeErr))
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("cleanup encountered %d errors; first: %w", len(errs), errs[0])
	}
	return nil
}

// calculateChecksum computes a SHA-256 hex digest for the given file path.
func calculateChecksum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open for checksum: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("compute checksum: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// saveMeta writes artifact metadata as a JSON file.
func (s *Store) saveMeta(art *Artifact) error {
	metaPath := filepath.Join(s.basePath, "meta", art.ID+".json")
	data, err := json.MarshalIndent(art, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal artifact metadata: %w", err)
	}
	if err := os.WriteFile(metaPath, data, 0o644); err != nil {
		return fmt.Errorf("write artifact metadata: %w", err)
	}
	return nil
}

// loadMeta reads artifact metadata by ID.
func (s *Store) loadMeta(artifactID string) (*Artifact, error) {
	metaPath := filepath.Join(s.basePath, "meta", artifactID+".json")
	return s.loadMetaFromFile(metaPath)
}

// loadMetaFromFile reads artifact metadata from a specific file path.
func (s *Store) loadMetaFromFile(path string) (*Artifact, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read metadata %s: %w", path, err)
	}
	var art Artifact
	if err := json.Unmarshal(data, &art); err != nil {
		return nil, fmt.Errorf("unmarshal metadata: %w", err)
	}
	return &art, nil
}
