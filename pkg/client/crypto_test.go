package client

import (
	"bytes"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAES256GCMEncryptionDecryption(t *testing.T) {
	passphrase := "SecurePassphrase123!@#"
	plaintext := []byte("Sensitive payload to be protected with AES-256-GCM AEAD.")

	// Test EncryptGCM
	encryptedBase64, err := EncryptGCM(passphrase, plaintext)
	require.NoError(t, err)
	assert.NotEmpty(t, encryptedBase64)

	// Decode raw payload and verify OTSGCM1 magic header
	raw, err := base64.StdEncoding.DecodeString(string(encryptedBase64))
	require.NoError(t, err)
	assert.True(t, bytes.HasPrefix(raw, []byte("OTSGCM1")))

	// Test DecryptGCM
	decrypted, err := DecryptGCM(passphrase, encryptedBase64)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)

	// Test unified Decrypt router
	decryptedRouter, err := Decrypt(passphrase, encryptedBase64)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decryptedRouter)
}

func TestAES256GCMTagVerificationTampering(t *testing.T) {
	passphrase := "SecretKey456"
	plaintext := []byte("Top secret payload requiring tamper resistance")

	encryptedBase64, err := EncryptGCM(passphrase, plaintext)
	require.NoError(t, err)

	raw, err := base64.StdEncoding.DecodeString(string(encryptedBase64))
	require.NoError(t, err)

	// Tamper with the last byte (auth tag)
	raw[len(raw)-1] ^= 0xFF
	tamperedBase64 := []byte(base64.StdEncoding.EncodeToString(raw))

	// Decrypt should fail with ErrGCMAuthenticationFailed
	_, err = DecryptGCM(passphrase, tamperedBase64)
	assert.ErrorIs(t, err, ErrGCMAuthenticationFailed)

	_, err = Decrypt(passphrase, tamperedBase64)
	assert.ErrorIs(t, err, ErrGCMAuthenticationFailed)
}

func TestAES256GCMWrongPassword(t *testing.T) {
	passphrase := "RightPassword"
	wrongPassphrase := "WrongPassword"
	plaintext := []byte("Payload for wrong password check")

	encryptedBase64, err := EncryptGCM(passphrase, plaintext)
	require.NoError(t, err)

	_, err = DecryptGCM(wrongPassphrase, encryptedBase64)
	assert.ErrorIs(t, err, ErrGCMAuthenticationFailed)

	_, err = Decrypt(wrongPassphrase, encryptedBase64)
	assert.ErrorIs(t, err, ErrGCMAuthenticationFailed)
}

func TestLegacyOpenSSLCBCBackwardCompatibility(t *testing.T) {
	passphrase := "LegacyPass123"
	plaintext := []byte("Legacy secret payload encrypted with OpenSSL AES-256-CBC")

	// Encrypt using legacy OpenSSL CBC
	encryptedBase64, err := EncryptOpenSSL(passphrase, plaintext)
	require.NoError(t, err)

	// Verify legacy Salted__ header
	raw, err := base64.StdEncoding.DecodeString(string(encryptedBase64))
	require.NoError(t, err)
	assert.True(t, bytes.HasPrefix(raw, []byte("Salted__")))

	// Decrypt using legacy DecryptOpenSSL
	decryptedLegacy, err := DecryptOpenSSL(passphrase, encryptedBase64)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decryptedLegacy)

	// Decrypt using unified Decrypt router (auto-detects Salted__ magic)
	decryptedRouter, err := Decrypt(passphrase, encryptedBase64)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decryptedRouter)
}

func TestPasswordLength32(t *testing.T) {
	assert.Equal(t, 32, PasswordLength)

	pass, err := genPass()
	require.NoError(t, err)
	assert.Len(t, pass, 32)
}

func TestAES256GCMNonceAndSaltUniqueness(t *testing.T) {
	passphrase := "UniqueNonceTestPass"
	plaintext := []byte("Constant Secret Message")

	seen := make(map[string]bool)
	for i := 0; i < 10; i++ {
		enc, err := EncryptGCM(passphrase, plaintext)
		require.NoError(t, err)

		encStr := string(enc)
		assert.False(t, seen[encStr], "Nonce or salt collision detected in GCM encryption")
		seen[encStr] = true
	}
}

func TestAES256GCMTruncatedPayloadHandling(t *testing.T) {
	passphrase := "TruncatedPayloadPass"

	// Test empty slice
	dec, err := DecryptGCM(passphrase, nil)
	require.NoError(t, err)
	assert.Nil(t, dec)

	// Test payloads smaller than min length (35 bytes)
	truncatedPayloads := [][]byte{
		[]byte("OTSGCM1"),
		[]byte("OTSGCM11234567890123456"),
		[]byte("OTSGCM1123456789012345678901234"),
	}

	for _, trunc := range truncatedPayloads {
		_, err := DecryptGCM(passphrase, trunc)
		assert.ErrorIs(t, err, ErrInvalidCiphertext)

		_, err = Decrypt(passphrase, trunc)
		assert.ErrorIs(t, err, ErrInvalidCiphertext)
	}
}

func TestAES256GCMSecretWithAttachmentsSerialization(t *testing.T) {
	passphrase := "AttachmentGCMPass"
	original := Secret{
		Secret: "Multi-attachment secret payload under GCM",
		Attachments: []SecretAttachment{
			{
				Name:    "document.pdf",
				Type:    "application/pdf",
				Content: []byte("%PDF-1.5 Confidential GCM Document"),
			},
			{
				Name:    "image.png",
				Type:    "image/png",
				Content: []byte("\x89PNG\r\n\x1a\nFakeGCMPNGData"),
			},
		},
	}

	serialized, err := original.serialize(passphrase)
	require.NoError(t, err)
	assert.NotEmpty(t, serialized)

	// Read back and decrypt
	var restored Secret
	err = restored.read(serialized, passphrase)
	require.NoError(t, err)

	assert.Equal(t, original.Secret, restored.Secret)
	require.Len(t, restored.Attachments, 2)
	assert.Equal(t, original.Attachments[0].Name, restored.Attachments[0].Name)
	assert.Equal(t, original.Attachments[0].Content, restored.Attachments[0].Content)
	assert.Equal(t, original.Attachments[1].Name, restored.Attachments[1].Name)
	assert.Equal(t, original.Attachments[1].Content, restored.Attachments[1].Content)
}
