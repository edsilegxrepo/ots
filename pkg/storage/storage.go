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
		// Create inserts a raw binary secret blob with total allowed reads and returns its ID
		Create(payload []byte, expireIn time.Duration, reads int) (string, error)
		// ReadAndDestroy returns raw binary secret payload, decrements remaining reads, and destroys entry when reads <= 0
		ReadAndDestroy(id string) (payload []byte, readsRemaining int, err error)
	}
)

// ErrSecretNotFound is a generic error to be returned when a secret
// does not exist in the backend. It will then be handled by API.
var ErrSecretNotFound = errors.New("secret not found")
