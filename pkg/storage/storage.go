// Package storage defines the pluggable storage interface and UUID generation.
package storage

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"time"
)

type (
	// Storage is the interface to implement in each storage provider
	Storage interface {
		// Count returns the number of stored secrets
		Count() (int64, error)
		// Create inserts a raw binary secret blob with total allowed reads and returns its ID
		Create(payload []byte, expireIn time.Duration, reads int) (string, error)
		// ReadAndDestroy returns raw binary secret payload, decrements remaining reads, and destroys entry when reads <= 0
		ReadAndDestroy(id string) (payload []byte, readsRemaining int, err error)
		// Purge immediately destroys a secret entry in a single atomic operation without looping decrements
		Purge(id string) (payload []byte, err error)
	}
)

// ErrSecretNotFound is a generic error to be returned when a secret
// does not exist in the backend. It will then be handled by API.
var ErrSecretNotFound = errors.New("secret not found")

// GenerateUUID generates a cryptographically random RFC 4122 version 4 UUID string using Go standard library crypto/rand
func GenerateUUID() string {
	var b [16]byte
	if _, err := io.ReadFull(rand.Reader, b[:]); err != nil {
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	b[6] = (b[6] & 0x0f) | 0x40 // Version 4
	b[8] = (b[8] & 0x3f) | 0x80 // Variant RFC 4122
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// NormalizeReads ensures total allowed reads is at least 1
func NormalizeReads(reads int) int {
	if reads < 1 {
		return 1
	}
	return reads
}

// CalculateExpiry returns the future expiration timestamp for a duration, or zero Time if duration <= 0
func CalculateExpiry(expireIn time.Duration) time.Time {
	if expireIn > 0 {
		return time.Now().Add(expireIn)
	}
	return time.Time{}
}

// CalculateExpiryUnix returns the Unix timestamp for a duration, or 0 if duration <= 0
func CalculateExpiryUnix(expireIn time.Duration) int64 {
	if expireIn > 0 {
		return time.Now().Add(expireIn).Unix()
	}
	return 0
}
