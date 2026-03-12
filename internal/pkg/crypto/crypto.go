package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/bcrypt"
)

const (
	// KeySize is the required key length for AES-256.
	KeySize = 32
	// NonceSize is the standard GCM nonce length (12 bytes).
	NonceSize = 12
	// BcryptCost is the default bcrypt work factor.
	BcryptCost = 12
)

var (
	ErrInvalidKeySize     = errors.New("crypto: key must be 32 bytes for AES-256")
	ErrCiphertextTooShort = errors.New("crypto: ciphertext too short")
	ErrDecryptionFailed   = errors.New("crypto: decryption failed")
)

// GenerateKey produces a cryptographically secure 32-byte key suitable for
// AES-256-GCM encryption.
func GenerateKey() ([]byte, error) {
	key := make([]byte, KeySize)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("crypto: generating key: %w", err)
	}
	return key, nil
}

// GenerateKeyHex returns a hex-encoded 32-byte key.
func GenerateKeyHex() (string, error) {
	key, err := GenerateKey()
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(key), nil
}

// Encrypt encrypts plaintext using AES-256-GCM with the provided key.
// The returned ciphertext has the nonce prepended: [nonce | encrypted | tag].
func Encrypt(plaintext, key []byte) ([]byte, error) {
	if len(key) != KeySize {
		return nil, ErrInvalidKeySize
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("crypto: creating cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: creating GCM: %w", err)
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("crypto: generating nonce: %w", err)
	}

	// Seal appends the encrypted and authenticated ciphertext to nonce.
	ciphertext := aesGCM.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt decrypts ciphertext that was produced by Encrypt. It expects the
// nonce to be prepended to the ciphertext.
func Decrypt(ciphertext, key []byte) ([]byte, error) {
	if len(key) != KeySize {
		return nil, ErrInvalidKeySize
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("crypto: creating cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: creating GCM: %w", err)
	}

	nonceSize := aesGCM.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, ErrCiphertextTooShort
	}

	nonce := ciphertext[:nonceSize]
	encryptedData := ciphertext[nonceSize:]

	plaintext, err := aesGCM.Open(nil, nonce, encryptedData, nil)
	if err != nil {
		return nil, ErrDecryptionFailed
	}
	return plaintext, nil
}

// EncryptString encrypts a string and returns the result as a base64-encoded
// string, suitable for storage in configuration files or databases.
func EncryptString(plaintext string, key []byte) (string, error) {
	ciphertext, err := Encrypt([]byte(plaintext), key)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptString decodes a base64-encoded ciphertext string and decrypts it.
func DecryptString(encoded string, key []byte) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("crypto: decoding base64: %w", err)
	}
	plaintext, err := Decrypt(ciphertext, key)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// EncryptEnvVar encrypts an environment variable value for secure storage.
// The key can be provided as a hex-encoded string.
func EncryptEnvVar(value, hexKey string) (string, error) {
	key, err := hex.DecodeString(hexKey)
	if err != nil {
		return "", fmt.Errorf("crypto: decoding hex key: %w", err)
	}
	return EncryptString(value, key)
}

// DecryptEnvVar decrypts an environment variable that was encrypted with
// EncryptEnvVar.
func DecryptEnvVar(encrypted, hexKey string) (string, error) {
	key, err := hex.DecodeString(hexKey)
	if err != nil {
		return "", fmt.Errorf("crypto: decoding hex key: %w", err)
	}
	return DecryptString(encrypted, key)
}

// HashPassword hashes a plaintext password using bcrypt with the default cost.
func HashPassword(password string) (string, error) {
	if len(password) == 0 {
		return "", errors.New("crypto: password cannot be empty")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
	if err != nil {
		return "", fmt.Errorf("crypto: hashing password: %w", err)
	}
	return string(hash), nil
}

// VerifyPassword checks whether a plaintext password matches a bcrypt hash.
func VerifyPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// GenerateRandomToken produces a cryptographically secure random token of the
// specified byte length, returned as a hex-encoded string. The resulting string
// will be twice the requested length.
func GenerateRandomToken(length int) (string, error) {
	if length <= 0 {
		return "", errors.New("crypto: token length must be positive")
	}
	b := make([]byte, length)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", fmt.Errorf("crypto: generating random bytes: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// GenerateRandomBytes returns a slice of cryptographically secure random bytes.
func GenerateRandomBytes(n int) ([]byte, error) {
	if n <= 0 {
		return nil, errors.New("crypto: byte count must be positive")
	}
	b := make([]byte, n)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return nil, fmt.Errorf("crypto: generating random bytes: %w", err)
	}
	return b, nil
}

// HMACSign produces an HMAC-SHA256 signature of the given message using the
// provided secret. This is useful for webhook signature verification.
func HMACSign(message, secret []byte) []byte {
	mac := hmac.New(sha256.New, secret)
	mac.Write(message)
	return mac.Sum(nil)
}

// HMACVerify checks that expectedMAC matches the HMAC-SHA256 of the message
// with the given secret, using a constant-time comparison.
func HMACVerify(message, expectedMAC, secret []byte) bool {
	mac := hmac.New(sha256.New, secret)
	mac.Write(message)
	computed := mac.Sum(nil)
	return hmac.Equal(computed, expectedMAC)
}

// DeriveKey derives a fixed-length key from a passphrase using SHA-256.
// This is a simple KDF; for production use consider Argon2 or scrypt.
func DeriveKey(passphrase string) []byte {
	h := sha256.Sum256([]byte(passphrase))
	return h[:]
}
