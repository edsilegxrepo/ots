// Package storage describes the requirements a storage provider
// has to fulfill ot be usable in OTS
package storage

import (
	"errors"
	"time"
)

type (
	// Storage is the interface to implement in each storage provider
	Storage interface {
		// Count returns the number of stored secrets
		Count() (int64, error)
		// Create inserts a new secret with total allowed reads and returns its ID
		Create(secret string, expireIn time.Duration, reads int) (string, error)
		// ReadAndDestroy returns secret content, decrements remaining reads, and destroys entry when reads <= 0
		ReadAndDestroy(id string) (secret string, readsRemaining int, err error)
	}
)

// ErrSecretNotFound is a generic error to be returned when a secret
// does not exist in the backend. It will then be handled by API.
var ErrSecretNotFound = errors.New("secret not found")
