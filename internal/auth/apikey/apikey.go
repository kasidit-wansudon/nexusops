// Package apikey provides API key generation, validation, and management
// for the NexusOps platform. Keys are stored hashed; the plaintext key
// is returned only once at creation time.
package apikey

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	// keyPrefix is prepended to every generated API key for easy identification.
	keyPrefix = "nxo_"
	// keyByteLength is the number of random bytes used to generate a key.
	keyByteLength = 32
	// defaultTTL is the default time-to-live for API keys (1 year).
	defaultTTL = 365 * 24 * time.Hour
)

// APIKey represents an API key with its associated metadata.
type APIKey struct {
	ID          string    `json:"id"`
	Key         string    `json:"key,omitempty"` // Only populated on creation; never stored.
	KeyHash     string    `json:"-"`             // HMAC-SHA256 hash of the key for storage.
	Name        string    `json:"name"`
	ProjectID   string    `json:"project_id"`
	UserID      string    `json:"user_id"`
	Permissions []string  `json:"permissions"`
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   time.Time `json:"expires_at"`
	LastUsedAt  time.Time `json:"last_used_at,omitempty"`
	Revoked     bool      `json:"revoked"`
}

// Manager handles the lifecycle of API keys. Keys are stored in an
// in-memory map protected by a mutex. The encryption key is used as an
// HMAC secret so that key hashes are deterministic but not reversible.
type Manager struct {
	mu            sync.RWMutex
	keys          map[string]*APIKey // keyed by APIKey.ID
	hashIndex     map[string]string  // hash -> APIKey.ID for fast lookup
	encryptionKey []byte
}

// NewManager creates a new API key manager. The encryptionKey is used as
// an HMAC secret for hashing keys before storage.
func NewManager(encryptionKey []byte) *Manager {
	if len(encryptionKey) == 0 {
		panic("apikey: encryption key must not be empty")
	}
	return &Manager{
		keys:          make(map[string]*APIKey),
		hashIndex:     make(map[string]string),
		encryptionKey: encryptionKey,
	}
}

// Generate creates a new API key with the given metadata. The returned
// APIKey has the plaintext key populated in the Key field. This is the
// only time the plaintext is available; subsequent lookups will not
// include it.
func (m *Manager) Generate(name, projectID, userID string, perms []string) (*APIKey, error) {
	if name == "" {
		return nil, fmt.Errorf("apikey: name must not be empty")
	}
	if userID == "" {
		return nil, fmt.Errorf("apikey: user ID must not be empty")
	}

	rawBytes := make([]byte, keyByteLength)
	if _, err := rand.Read(rawBytes); err != nil {
		return nil, fmt.Errorf("apikey: failed to generate random key: %w", err)
	}

	plaintext := keyPrefix + hex.EncodeToString(rawBytes)
	hash := m.hashKey(plaintext)

	now := time.Now().UTC()
	key := &APIKey{
		ID:          uuid.New().String(),
		Key:         plaintext,
		KeyHash:     hash,
		Name:        name,
		ProjectID:   projectID,
		UserID:      userID,
		Permissions: perms,
		CreatedAt:   now,
		ExpiresAt:   now.Add(defaultTTL),
		Revoked:     false,
	}

	m.mu.Lock()
	m.keys[key.ID] = key
	m.hashIndex[hash] = key.ID
	m.mu.Unlock()

	// Return a copy with the plaintext key so the caller can show it once.
	result := *key
	return &result, nil
}

// Validate looks up an API key by its plaintext value. It verifies the
// key exists, has not been revoked, and has not expired. On success it
// updates the LastUsedAt timestamp and returns the key metadata (without
// the plaintext).
func (m *Manager) Validate(key string) (*APIKey, error) {
	if !strings.HasPrefix(key, keyPrefix) {
		return nil, fmt.Errorf("apikey: invalid key format")
	}

	hash := m.hashKey(key)

	m.mu.Lock()
	defer m.mu.Unlock()

	id, ok := m.hashIndex[hash]
	if !ok {
		return nil, fmt.Errorf("apikey: key not found")
	}

	stored := m.keys[id]
	if stored == nil {
		return nil, fmt.Errorf("apikey: key not found")
	}

	if stored.Revoked {
		return nil, fmt.Errorf("apikey: key has been revoked")
	}

	if time.Now().UTC().After(stored.ExpiresAt) {
		return nil, fmt.Errorf("apikey: key has expired")
	}

	stored.LastUsedAt = time.Now().UTC()

	// Return a copy without the plaintext key.
	result := *stored
	result.Key = ""
	return &result, nil
}

// Revoke marks an API key as revoked so it can no longer be used for
// authentication. The key metadata is retained for audit purposes.
func (m *Manager) Revoke(keyID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key, ok := m.keys[keyID]
	if !ok {
		return fmt.Errorf("apikey: key ID %q not found", keyID)
	}

	key.Revoked = true
	return nil
}

// List returns all non-revoked API keys belonging to the given user.
// Plaintext keys are never included in the results.
func (m *Manager) List(userID string) []*APIKey {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*APIKey
	for _, key := range m.keys {
		if key.UserID == userID && !key.Revoked {
			copy := *key
			copy.Key = ""
			result = append(result, &copy)
		}
	}
	return result
}

// HasPermission checks whether the given API key grants a specific
// permission. A wildcard "*" in the permissions list grants all access.
func (m *Manager) HasPermission(keyID, permission string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key, ok := m.keys[keyID]
	if !ok || key.Revoked {
		return false
	}

	for _, p := range key.Permissions {
		if p == "*" || p == permission {
			return true
		}
	}
	return false
}

// Count returns the total number of active (non-revoked) keys for a user.
func (m *Manager) Count(userID string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, key := range m.keys {
		if key.UserID == userID && !key.Revoked {
			count++
		}
	}
	return count
}

// hashKey computes an HMAC-SHA256 hash of the key using the manager's
// encryption key. This produces a deterministic, non-reversible hash
// suitable for storage and lookup.
func (m *Manager) hashKey(key string) string {
	mac := hmac.New(sha256.New, m.encryptionKey)
	mac.Write([]byte(key))
	return hex.EncodeToString(mac.Sum(nil))
}
