package env

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/kasidit-wansudon/nexusops/internal/pkg/crypto"
)

var (
	ErrInvalidKeyLength = errors.New("env: encryption key must be 32 bytes")
	ErrProjectNotFound  = errors.New("env: project not found")
	ErrVarNotFound      = errors.New("env: environment variable not found")
	ErrEmptyKey         = errors.New("env: variable key cannot be empty")
	ErrEmptyProjectID   = errors.New("env: project ID cannot be empty")
	ErrEmptyEnvironment = errors.New("env: environment cannot be empty")
	ErrDecryptionFailed = errors.New("env: failed to decrypt variable")
)

// EnvVar represents an environment variable with metadata.
type EnvVar struct {
	Key         string    `json:"key"`
	Value       string    `json:"value"`
	Environment string    `json:"environment"`
	Encrypted   bool      `json:"encrypted"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Manager handles encrypted environment variable storage and retrieval.
// Variables are organized by project ID and environment name. All values
// are encrypted at rest using AES-256-GCM.
type Manager struct {
	encryptionKey []byte
	mu            sync.RWMutex
	// store maps projectID -> compositeKey(key+environment) -> EnvVar
	store map[string]map[string]*EnvVar
}

// NewManager creates a new environment variable manager.
// The encryptionKey must be exactly 32 bytes for AES-256-GCM.
func NewManager(encryptionKey []byte) (*Manager, error) {
	if len(encryptionKey) != 32 {
		return nil, ErrInvalidKeyLength
	}

	return &Manager{
		encryptionKey: encryptionKey,
		store:         make(map[string]map[string]*EnvVar),
	}, nil
}

// compositeKey creates a unique key for the store from the variable key and environment.
func compositeKey(key, environment string) string {
	return key + "\x00" + environment
}

// Set creates or updates an environment variable for the given project.
// The value is encrypted using AES-256-GCM before being stored.
func (m *Manager) Set(projectID, key, value, environment string) error {
	if projectID == "" {
		return ErrEmptyProjectID
	}
	if key == "" {
		return ErrEmptyKey
	}
	if environment == "" {
		return ErrEmptyEnvironment
	}

	encryptedValue, err := crypto.EncryptString(value, m.encryptionKey)
	if err != nil {
		return fmt.Errorf("env: failed to encrypt value for key %q: %w", key, err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.store[projectID] == nil {
		m.store[projectID] = make(map[string]*EnvVar)
	}

	ck := compositeKey(key, environment)
	now := time.Now().UTC()

	existing, exists := m.store[projectID][ck]
	if exists {
		existing.Value = encryptedValue
		existing.UpdatedAt = now
	} else {
		m.store[projectID][ck] = &EnvVar{
			Key:         key,
			Value:       encryptedValue,
			Environment: environment,
			Encrypted:   true,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
	}

	return nil
}

// Get retrieves and decrypts an environment variable value.
func (m *Manager) Get(projectID, key, environment string) (string, error) {
	if projectID == "" {
		return "", ErrEmptyProjectID
	}
	if key == "" {
		return "", ErrEmptyKey
	}
	if environment == "" {
		return "", ErrEmptyEnvironment
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	projectVars, ok := m.store[projectID]
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrProjectNotFound, projectID)
	}

	ck := compositeKey(key, environment)
	envVar, ok := projectVars[ck]
	if !ok {
		return "", fmt.Errorf("%w: %s (environment: %s)", ErrVarNotFound, key, environment)
	}

	plaintext, err := crypto.DecryptString(envVar.Value, m.encryptionKey)
	if err != nil {
		return "", fmt.Errorf("%w: key %q: %v", ErrDecryptionFailed, key, err)
	}

	return plaintext, nil
}

// List returns all environment variables for a project and environment with
// values masked for security. Only the first 4 characters of the decrypted
// value are shown, followed by asterisks.
func (m *Manager) List(projectID, environment string) ([]*EnvVar, error) {
	if projectID == "" {
		return nil, ErrEmptyProjectID
	}
	if environment == "" {
		return nil, ErrEmptyEnvironment
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	projectVars, ok := m.store[projectID]
	if !ok {
		return []*EnvVar{}, nil
	}

	var result []*EnvVar
	for _, envVar := range projectVars {
		if envVar.Environment != environment {
			continue
		}

		masked := &EnvVar{
			Key:         envVar.Key,
			Value:       maskValue(envVar.Value, m.encryptionKey),
			Environment: envVar.Environment,
			Encrypted:   envVar.Encrypted,
			CreatedAt:   envVar.CreatedAt,
			UpdatedAt:   envVar.UpdatedAt,
		}
		result = append(result, masked)
	}

	return result, nil
}

// maskValue decrypts the value and returns a masked version showing
// only the first few characters.
func maskValue(encryptedValue string, key []byte) string {
	decrypted, err := crypto.DecryptString(encryptedValue, key)
	if err != nil {
		return "********"
	}

	if len(decrypted) <= 4 {
		return strings.Repeat("*", len(decrypted))
	}

	return decrypted[:4] + strings.Repeat("*", len(decrypted)-4)
}

// Delete removes an environment variable from the store.
func (m *Manager) Delete(projectID, key, environment string) error {
	if projectID == "" {
		return ErrEmptyProjectID
	}
	if key == "" {
		return ErrEmptyKey
	}
	if environment == "" {
		return ErrEmptyEnvironment
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	projectVars, ok := m.store[projectID]
	if !ok {
		return fmt.Errorf("%w: %s", ErrProjectNotFound, projectID)
	}

	ck := compositeKey(key, environment)
	if _, ok := projectVars[ck]; !ok {
		return fmt.Errorf("%w: %s (environment: %s)", ErrVarNotFound, key, environment)
	}

	delete(projectVars, ck)

	// Clean up empty project maps
	if len(projectVars) == 0 {
		delete(m.store, projectID)
	}

	return nil
}

// Export returns all decrypted environment variables for a project and
// environment as a plain key-value map. This is useful for injecting
// variables into build or deploy processes.
func (m *Manager) Export(projectID, environment string) (map[string]string, error) {
	if projectID == "" {
		return nil, ErrEmptyProjectID
	}
	if environment == "" {
		return nil, ErrEmptyEnvironment
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	projectVars, ok := m.store[projectID]
	if !ok {
		return make(map[string]string), nil
	}

	result := make(map[string]string)
	for _, envVar := range projectVars {
		if envVar.Environment != environment {
			continue
		}

		plaintext, err := crypto.DecryptString(envVar.Value, m.encryptionKey)
		if err != nil {
			return nil, fmt.Errorf("%w: key %q: %v", ErrDecryptionFailed, envVar.Key, err)
		}

		result[envVar.Key] = plaintext
	}

	return result, nil
}

// Import bulk-imports environment variables from a plain key-value map.
// Each value is encrypted before storage. Existing variables with the
// same key and environment are overwritten.
func (m *Manager) Import(projectID, environment string, vars map[string]string) error {
	if projectID == "" {
		return ErrEmptyProjectID
	}
	if environment == "" {
		return ErrEmptyEnvironment
	}

	for key, value := range vars {
		if key == "" {
			continue
		}
		if err := m.Set(projectID, key, value, environment); err != nil {
			return fmt.Errorf("env: failed to import key %q: %w", key, err)
		}
	}

	return nil
}

// Count returns the number of environment variables stored for a project
// in a specific environment.
func (m *Manager) Count(projectID, environment string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	projectVars, ok := m.store[projectID]
	if !ok {
		return 0
	}

	count := 0
	for _, envVar := range projectVars {
		if envVar.Environment == environment {
			count++
		}
	}

	return count
}

// Environments returns a list of distinct environments that have variables
// stored for the given project.
func (m *Manager) Environments(projectID string) ([]string, error) {
	if projectID == "" {
		return nil, ErrEmptyProjectID
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	projectVars, ok := m.store[projectID]
	if !ok {
		return []string{}, nil
	}

	envSet := make(map[string]struct{})
	for _, envVar := range projectVars {
		envSet[envVar.Environment] = struct{}{}
	}

	envs := make([]string, 0, len(envSet))
	for env := range envSet {
		envs = append(envs, env)
	}

	return envs, nil
}
