package client

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/pbkdf2"
)

const (
	openSSLHeaderMagic = "Salted__"
	pbkdf2Iterations   = 300000
	aesKeySize         = 32
	aesIVSize          = 16
)

var (
	ErrInvalidCiphertext = errors.New("invalid OpenSSL ciphertext format")
	ErrDecryptionFailed = errors.New("decryption failed: invalid key or payload")
)

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

	// Support base64 encoded input strings (e.g. "U2FsdGVkX1...")
	if bytes.HasPrefix(data, []byte("U2FsdGVkX1")) {
		decoded, err := base64.StdEncoding.DecodeString(string(data))
		if err == nil {
			data = decoded
		}
	}

	if len(data) < 16 || !bytes.HasPrefix(data, []byte(openSSLHeaderMagic)) {
		return nil, ErrInvalidCiphertext
	}

	salt := data[8:16]
	ciphertext := data[16:]

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
