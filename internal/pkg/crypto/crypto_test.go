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
	assert.Len(t, key, KeySize)

	// Two keys should differ
	key2, err := GenerateKey()
	require.NoError(t, err)
	assert.NotEqual(t, key, key2)
}

func TestGenerateKeyHex(t *testing.T) {
	hexKey, err := GenerateKeyHex()
	require.NoError(t, err)
	assert.Len(t, hexKey, KeySize*2) // hex doubles the length

	decoded, err := hex.DecodeString(hexKey)
	require.NoError(t, err)
	assert.Len(t, decoded, KeySize)
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key, err := GenerateKey()
	require.NoError(t, err)

	plaintext := []byte("hello, world!")
	ciphertext, err := Encrypt(plaintext, key)
	require.NoError(t, err)
	assert.NotEqual(t, plaintext, ciphertext)

	decrypted, err := Decrypt(ciphertext, key)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestEncryptInvalidKeySize(t *testing.T) {
	_, err := Encrypt([]byte("data"), []byte("short-key"))
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidKeySize)
}

func TestDecryptInvalidKeySize(t *testing.T) {
	_, err := Decrypt([]byte("data"), []byte("short-key"))
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidKeySize)
}

func TestDecryptCiphertextTooShort(t *testing.T) {
	key, _ := GenerateKey()
	_, err := Decrypt([]byte("short"), key)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCiphertextTooShort)
}

func TestDecryptTamperedCiphertext(t *testing.T) {
	key, _ := GenerateKey()
	ciphertext, _ := Encrypt([]byte("secret"), key)

	// Tamper with ciphertext
	ciphertext[len(ciphertext)-1] ^= 0xff

	_, err := Decrypt(ciphertext, key)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrDecryptionFailed)
}

func TestEncryptStringDecryptString(t *testing.T) {
	key, err := GenerateKey()
	require.NoError(t, err)

	original := "database_password=s3cr3t!"
	encoded, err := EncryptString(original, key)
	require.NoError(t, err)
	assert.NotEqual(t, original, encoded)

	decoded, err := DecryptString(encoded, key)
	require.NoError(t, err)
	assert.Equal(t, original, decoded)
}

func TestEncryptEnvVarDecryptEnvVar(t *testing.T) {
	hexKey, err := GenerateKeyHex()
	require.NoError(t, err)

	value := "MY_SECRET_VALUE"
	encrypted, err := EncryptEnvVar(value, hexKey)
	require.NoError(t, err)

	decrypted, err := DecryptEnvVar(encrypted, hexKey)
	require.NoError(t, err)
	assert.Equal(t, value, decrypted)
}

func TestHashPasswordAndVerify(t *testing.T) {
	hash, err := HashPassword("mypassword123")
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.NotEqual(t, "mypassword123", hash)

	assert.True(t, VerifyPassword("mypassword123", hash))
	assert.False(t, VerifyPassword("wrongpassword", hash))
}

func TestHashPasswordEmpty(t *testing.T) {
	_, err := HashPassword("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be empty")
}

func TestGenerateRandomToken(t *testing.T) {
	token, err := GenerateRandomToken(16)
	require.NoError(t, err)
	assert.Len(t, token, 32) // hex encoding doubles length

	token2, err := GenerateRandomToken(16)
	require.NoError(t, err)
	assert.NotEqual(t, token, token2)
}

func TestGenerateRandomTokenInvalidLength(t *testing.T) {
	_, err := GenerateRandomToken(0)
	require.Error(t, err)

	_, err = GenerateRandomToken(-1)
	require.Error(t, err)
}

func TestGenerateRandomBytes(t *testing.T) {
	b, err := GenerateRandomBytes(32)
	require.NoError(t, err)
	assert.Len(t, b, 32)

	_, err = GenerateRandomBytes(0)
	require.Error(t, err)
}

func TestHMACSignAndVerify(t *testing.T) {
	secret := []byte("webhook-secret")
	message := []byte("payload body")

	signature := HMACSign(message, secret)
	assert.NotEmpty(t, signature)

	assert.True(t, HMACVerify(message, signature, secret))
	assert.False(t, HMACVerify([]byte("tampered"), signature, secret))
	assert.False(t, HMACVerify(message, []byte("wrong-sig"), secret))
}

func TestDeriveKey(t *testing.T) {
	key := DeriveKey("my-passphrase")
	assert.Len(t, key, 32) // SHA-256 produces 32 bytes

	// Same passphrase should produce the same key
	key2 := DeriveKey("my-passphrase")
	assert.Equal(t, key, key2)

	// Different passphrase should produce different key
	key3 := DeriveKey("other-passphrase")
	assert.NotEqual(t, key, key3)
}
