package crypto

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateKey(t *testing.T) {
	key, err := GenerateKey()
	require.NoError(t, err)
	assert.Len(t, key, KeySize, "generated key must be exactly 32 bytes")

	// Two generated keys should be different (non-deterministic).
	key2, err := GenerateKey()
	require.NoError(t, err)
	assert.NotEqual(t, key, key2, "two generated keys should differ")
}

func TestGenerateKeyHex(t *testing.T) {
	hexKey, err := GenerateKeyHex()
	require.NoError(t, err)

	// Hex encoding of 32 bytes produces 64 hex characters.
	assert.Len(t, hexKey, KeySize*2, "hex key must be 64 characters")

	// Must be valid hex.
	decoded, err := hex.DecodeString(hexKey)
	require.NoError(t, err, "hex key must be valid hex")
	assert.Len(t, decoded, KeySize)
}

func TestEncryptDecrypt(t *testing.T) {
	key, err := GenerateKey()
	require.NoError(t, err)

	plaintext := []byte("the quick brown fox jumps over the lazy dog")

	ciphertext, err := Encrypt(plaintext, key)
	require.NoError(t, err)
	assert.NotEqual(t, plaintext, ciphertext, "ciphertext should differ from plaintext")

	decrypted, err := Decrypt(ciphertext, key)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted, "decrypted text must match original")
}

func TestEncryptDecryptString(t *testing.T) {
	key, err := GenerateKey()
	require.NoError(t, err)

	original := "super-secret-database-password-12345"

	encrypted, err := EncryptString(original, key)
	require.NoError(t, err)
	assert.NotEmpty(t, encrypted)
	assert.NotEqual(t, original, encrypted)

	decrypted, err := DecryptString(encrypted, key)
	require.NoError(t, err)
	assert.Equal(t, original, decrypted, "roundtrip must preserve the string")
}

func TestEncryptInvalidKey(t *testing.T) {
	shortKey := make([]byte, 16) // 16 bytes instead of 32
	_, err := Encrypt([]byte("data"), shortKey)
	assert.ErrorIs(t, err, ErrInvalidKeySize)

	longKey := make([]byte, 64) // 64 bytes instead of 32
	_, err = Encrypt([]byte("data"), longKey)
	assert.ErrorIs(t, err, ErrInvalidKeySize)
}

func TestDecryptInvalidKey(t *testing.T) {
	// First encrypt with a valid key.
	key, err := GenerateKey()
	require.NoError(t, err)

	ciphertext, err := Encrypt([]byte("secret"), key)
	require.NoError(t, err)

	// Attempt decrypt with a key of wrong size.
	shortKey := make([]byte, 10)
	_, err = Decrypt(ciphertext, shortKey)
	assert.ErrorIs(t, err, ErrInvalidKeySize)
}

func TestDecryptTooShort(t *testing.T) {
	key, err := GenerateKey()
	require.NoError(t, err)

	// A ciphertext shorter than the GCM nonce (12 bytes) must be rejected.
	shortCiphertext := []byte("short")
	_, err = Decrypt(shortCiphertext, key)
	assert.ErrorIs(t, err, ErrCiphertextTooShort)

	// Empty ciphertext.
	_, err = Decrypt([]byte{}, key)
	assert.ErrorIs(t, err, ErrCiphertextTooShort)
}

func TestDecryptWrongKey(t *testing.T) {
	key1, err := GenerateKey()
	require.NoError(t, err)
	key2, err := GenerateKey()
	require.NoError(t, err)

	ciphertext, err := Encrypt([]byte("classified information"), key1)
	require.NoError(t, err)

	// Decrypting with the wrong key (correct size) should fail with ErrDecryptionFailed.
	_, err = Decrypt(ciphertext, key2)
	assert.ErrorIs(t, err, ErrDecryptionFailed)
}

func TestEncryptEnvVar(t *testing.T) {
	hexKey, err := GenerateKeyHex()
	require.NoError(t, err)

	value := "my-env-secret-value"
	encrypted, err := EncryptEnvVar(value, hexKey)
	require.NoError(t, err)
	assert.NotEmpty(t, encrypted)

	decrypted, err := DecryptEnvVar(encrypted, hexKey)
	require.NoError(t, err)
	assert.Equal(t, value, decrypted, "EncryptEnvVar/DecryptEnvVar roundtrip must preserve value")
}

func TestHashPassword(t *testing.T) {
	hash, err := HashPassword("correcthorsebatterystaple")
	require.NoError(t, err)
	assert.NotEmpty(t, hash, "hash must not be empty")

	// bcrypt hashes always start with "$2a$" or "$2b$".
	assert.Contains(t, hash, "$2a$", "hash should be a bcrypt hash")

	// Empty password should return an error.
	_, err = HashPassword("")
	assert.Error(t, err)
}

func TestVerifyPassword(t *testing.T) {
	password := "P@ssw0rd!#2024"
	hash, err := HashPassword(password)
	require.NoError(t, err)

	assert.True(t, VerifyPassword(password, hash), "correct password must verify")
}

func TestVerifyPasswordWrong(t *testing.T) {
	password := "correct-password"
	hash, err := HashPassword(password)
	require.NoError(t, err)

	assert.False(t, VerifyPassword("wrong-password", hash), "wrong password must not verify")
	assert.False(t, VerifyPassword("", hash), "empty password must not verify")
}

func TestGenerateRandomToken(t *testing.T) {
	length := 16
	token, err := GenerateRandomToken(length)
	require.NoError(t, err)

	// Hex encoding doubles the byte length.
	assert.Len(t, token, length*2, "token hex length should be 2x the byte length")

	// Must be valid hex.
	_, err = hex.DecodeString(token)
	require.NoError(t, err)

	// Two tokens should be different.
	token2, err := GenerateRandomToken(length)
	require.NoError(t, err)
	assert.NotEqual(t, token, token2)

	// Zero or negative length should error.
	_, err = GenerateRandomToken(0)
	assert.Error(t, err)

	_, err = GenerateRandomToken(-5)
	assert.Error(t, err)
}

func TestHMACSignVerify(t *testing.T) {
	secret := []byte("webhook-secret-key")
	message := []byte(`{"event":"push","ref":"refs/heads/main"}`)

	signature := HMACSign(message, secret)
	assert.NotEmpty(t, signature)
	assert.Len(t, signature, 32, "HMAC-SHA256 produces 32 bytes")

	ok := HMACVerify(message, signature, secret)
	assert.True(t, ok, "HMACVerify must return true for a valid signature")
}

func TestHMACVerifyWrong(t *testing.T) {
	secret := []byte("my-secret")
	message := []byte("original message")

	signature := HMACSign(message, secret)

	// Tampered message should fail verification.
	tampered := []byte("tampered message")
	assert.False(t, HMACVerify(tampered, signature, secret), "tampered message must fail verification")

	// Wrong secret should fail verification.
	wrongSecret := []byte("wrong-secret")
	assert.False(t, HMACVerify(message, signature, wrongSecret), "wrong secret must fail verification")

	// Tampered signature should fail verification.
	badSig := make([]byte, len(signature))
	copy(badSig, signature)
	badSig[0] ^= 0xFF
	assert.False(t, HMACVerify(message, badSig, secret), "tampered signature must fail verification")
}

func TestDeriveKey(t *testing.T) {
	passphrase := "my-deterministic-passphrase"

	key1 := DeriveKey(passphrase)
	assert.Len(t, key1, KeySize, "derived key must be 32 bytes (SHA-256 output)")

	// Same passphrase must produce the same key.
	key2 := DeriveKey(passphrase)
	assert.Equal(t, key1, key2, "DeriveKey must be deterministic")

	// Different passphrase should produce a different key.
	key3 := DeriveKey("different-passphrase")
	assert.NotEqual(t, key1, key3, "different passphrases should produce different keys")

	// Derived key should be usable for encryption.
	ciphertext, err := Encrypt([]byte("test data"), key1)
	require.NoError(t, err)
	decrypted, err := Decrypt(ciphertext, key1)
	require.NoError(t, err)
	assert.Equal(t, []byte("test data"), decrypted)
}
