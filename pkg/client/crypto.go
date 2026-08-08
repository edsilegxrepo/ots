package client

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/pbkdf2"
)

const (
	gcmHeaderMagic     = "OTSGCM1"
	openSSLHeaderMagic = "Salted__"

	gcmSaltSize     = 16
	gcmNonceSize    = 12
	gcmTagSize      = 16
	gcmKDFIterations = 300000

	pbkdf2Iterations = 300000
	aesKeySize       = 32
	aesIVSize        = 16
)

var (
	ErrInvalidCiphertext       = errors.New("invalid ciphertext format or magic header")
	ErrDecryptionFailed        = errors.New("decryption failed: invalid key or payload")
	ErrGCMAuthenticationFailed = errors.New("authentication tag mismatch: payload has been tampered with")
)

// Encrypt encrypts plaintext using AES-256-GCM with PBKDF2-HMAC-SHA256 (300000 iterations).
// It returns a Base64-encoded string representation of the self-contained OTSGCM1 payload.
func Encrypt(passphrase string, plaintext []byte) ([]byte, error) {
	return EncryptGCM(passphrase, plaintext)
}

// Decrypt decrypts encrypted payload by auto-detecting OTSGCM1 (GCM) vs Salted__ (legacy OpenSSL CBC).
func Decrypt(passphrase string, data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, nil
	}

	// Auto-decode Base64 if needed
	raw := data
	if bytes.HasPrefix(data, []byte("T1RTR0NN")) || bytes.HasPrefix(data, []byte("U2FsdGVkX1")) || !bytes.HasPrefix(data, []byte(gcmHeaderMagic)) && !bytes.HasPrefix(data, []byte(openSSLHeaderMagic)) {
		decoded, err := decodeBase64Flexible(string(data))
		if err == nil && len(decoded) > 0 {
			raw = decoded
		}
	}

	if bytes.HasPrefix(raw, []byte(gcmHeaderMagic)) {
		return DecryptGCM(passphrase, raw)
	}

	if bytes.HasPrefix(raw, []byte(openSSLHeaderMagic)) {
		return DecryptOpenSSL(passphrase, raw)
	}

	// Retry DecryptGCM then DecryptOpenSSL as fallbacks if header magic was embedded inside raw input
	if unpadded, err := DecryptGCM(passphrase, data); err == nil {
		return unpadded, nil
	} else if errors.Is(err, ErrGCMAuthenticationFailed) {
		return nil, ErrGCMAuthenticationFailed
	}
	if unpadded, err := DecryptOpenSSL(passphrase, data); err == nil {
		return unpadded, nil
	}

	return nil, ErrDecryptionFailed
}

// EncryptGCM encrypts plaintext using AES-256-GCM and PBKDF2-HMAC-SHA256.
// Output binary format: Header ("OTSGCM1", 7B) || Salt (16B) || Nonce (12B) || Ciphertext + Tag (16B)
func EncryptGCM(passphrase string, plaintext []byte) ([]byte, error) {
	if len(plaintext) == 0 {
		return nil, nil
	}

	salt := make([]byte, gcmSaltSize)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("generating salt: %w", err)
	}

	key := pbkdf2.Key([]byte(passphrase), salt, gcmKDFIterations, aesKeySize, sha256.New)
	defer zeroBytes(key)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("creating AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("creating GCM mode: %w", err)
	}

	nonce := make([]byte, gcmNonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generating nonce: %w", err)
	}

	// Seal appends ciphertext + 16-byte auth tag
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	// Build self-contained payload: Header (7B) || Salt (16B) || Nonce (12B) || Ciphertext+Tag
	raw := make([]byte, 0, len(gcmHeaderMagic)+gcmSaltSize+gcmNonceSize+len(ciphertext))
	raw = append(raw, []byte(gcmHeaderMagic)...)
	raw = append(raw, salt...)
	raw = append(raw, nonce...)
	raw = append(raw, ciphertext...)

	return []byte(base64.StdEncoding.EncodeToString(raw)), nil
}

// DecryptGCM decrypts payload using AES-256-GCM, verifying authentication tag.
func DecryptGCM(passphrase string, data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, nil
	}

	raw := data
	if bytes.HasPrefix(data, []byte("T1RTR0NN")) || !bytes.HasPrefix(data, []byte(gcmHeaderMagic)) {
		decoded, err := decodeBase64Flexible(string(data))
		if err == nil && bytes.HasPrefix(decoded, []byte(gcmHeaderMagic)) {
			raw = decoded
		}
	}

	minLen := len(gcmHeaderMagic) + gcmSaltSize + gcmNonceSize + gcmTagSize
	if len(raw) < minLen || !bytes.HasPrefix(raw, []byte(gcmHeaderMagic)) {
		return nil, ErrInvalidCiphertext
	}

	offset := len(gcmHeaderMagic)
	salt := raw[offset : offset+gcmSaltSize]
	offset += gcmSaltSize

	nonce := raw[offset : offset+gcmNonceSize]
	offset += gcmNonceSize

	ciphertext := raw[offset:]

	key := pbkdf2.Key([]byte(passphrase), salt, gcmKDFIterations, aesKeySize, sha256.New)
	defer zeroBytes(key)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("creating AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("creating GCM mode: %w", err)
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, ErrGCMAuthenticationFailed
	}

	return plaintext, nil
}

// EncryptOpenSSL AES-256-CBC encrypts plaintext using PBKDF2 (SHA-512, 300000 iterations) in standard OpenSSL format.
func EncryptOpenSSL(passphrase string, plaintext []byte) ([]byte, error) {
	if len(plaintext) == 0 {
		return nil, nil
	}

	salt := make([]byte, 8)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("generating salt: %w", err)
	}

	derived := pbkdf2.Key([]byte(passphrase), salt, pbkdf2Iterations, aesKeySize+aesIVSize, sha512.New)
	key := derived[:aesKeySize]
	iv := derived[aesKeySize:]

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("creating cipher: %w", err)
	}

	padded := pkcs7Padding(plaintext, aes.BlockSize)
	ciphertext := make([]byte, len(padded))

	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext, padded)

	header := append([]byte(openSSLHeaderMagic), salt...)
	raw := append(header, ciphertext...)
	return []byte(base64.StdEncoding.EncodeToString(raw)), nil
}

// DecryptOpenSSL AES-256-CBC decrypts OpenSSL formatted ciphertext ("Salted__" + 8-byte salt + data).
func DecryptOpenSSL(passphrase string, data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, nil
	}

	raw := data
	if bytes.HasPrefix(data, []byte("U2FsdGVkX1")) || !bytes.HasPrefix(data, []byte(openSSLHeaderMagic)) {
		decoded, err := decodeBase64Flexible(string(data))
		if err == nil && bytes.HasPrefix(decoded, []byte(openSSLHeaderMagic)) {
			raw = decoded
		}
	}

	if len(raw) < 16 || !bytes.HasPrefix(raw, []byte(openSSLHeaderMagic)) {
		return nil, ErrInvalidCiphertext
	}

	salt := raw[8:16]
	ciphertext := raw[16:]

	if len(ciphertext)%aes.BlockSize != 0 {
		return nil, ErrInvalidCiphertext
	}

	// Try PBKDF2 first
	derived := pbkdf2.Key([]byte(passphrase), salt, pbkdf2Iterations, aesKeySize+aesIVSize, sha512.New)
	key := derived[:aesKeySize]
	iv := derived[aesKeySize:]

	if unpadded, err := decryptAESCBC(key, iv, ciphertext); err == nil {
		return unpadded, nil
	}

	// Fallback to legacy OpenSSL EVP_BytesToKey (MD5 digest) for historical payloads
	keyMD5, ivMD5 := openSSLMD5KDF([]byte(passphrase), salt)
	if unpadded, err := decryptAESCBC(keyMD5, ivMD5, ciphertext); err == nil {
		return unpadded, nil
	}

	return nil, ErrDecryptionFailed
}

func decryptAESCBC(key, iv, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	plaintext := make([]byte, len(ciphertext))
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(plaintext, ciphertext)

	return pkcs7Unpadding(plaintext, aes.BlockSize)
}

func openSSLMD5KDF(passphrase, salt []byte) (key, iv []byte) {
	var prev []byte
	var derived []byte
	for len(derived) < aesKeySize+aesIVSize {
		h := md5.New()
		h.Write(prev)
		h.Write(passphrase)
		h.Write(salt)
		prev = h.Sum(nil)
		derived = append(derived, prev...)
	}
	return derived[:aesKeySize], derived[aesKeySize : aesKeySize+aesIVSize]
}

func pkcs7Padding(data []byte, blockSize int) []byte {
	padding := blockSize - (len(data) % blockSize)
	padText := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(data, padText...)
}

func pkcs7Unpadding(data []byte, blockSize int) ([]byte, error) {
	length := len(data)
	if length == 0 || length%blockSize != 0 {
		return nil, ErrInvalidCiphertext
	}
	padding := int(data[length-1])
	if padding == 0 || padding > blockSize || padding > length {
		return nil, ErrInvalidCiphertext
	}
	var match byte
	for i := length - padding; i < length; i++ {
		match |= data[i] ^ byte(padding)
	}
	if subtle.ConstantTimeByteEq(match, 0) != 1 {
		return nil, ErrInvalidCiphertext
	}
	return data[:length-padding], nil
}

func decodeBase64Flexible(s string) ([]byte, error) {
	if decoded, err := base64.StdEncoding.DecodeString(s); err == nil {
		return decoded, nil
	}
	if decoded, err := base64.RawURLEncoding.DecodeString(s); err == nil {
		return decoded, nil
	}
	return base64.URLEncoding.DecodeString(s)
}

func zeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

