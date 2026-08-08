// Package memory implements a pure in-memory store for secrets which
// is suitable for testing and should not be used for productive use
package memory

import (
	"sync"
	"time"

	"github.com/edsilegxrepo/ots/pkg/storage"
)

type (
	memStorageSecret struct {
		Expiry         time.Time
		ReadsRemaining int
		Payload        []byte //#nosec:G117 // This application works with secrets
	}

	storageMem struct {
		sync.RWMutex
		store           map[string]memStorageSecret
		storePruneTimer *time.Ticker
	}
)

// New creates a new In-Mem storage
func New() storage.Storage {
	store := &storageMem{
		store:           make(map[string]memStorageSecret),
		storePruneTimer: time.NewTicker(time.Minute),
	}

	go store.storePruner()

	return store
}

func (s *storageMem) Count() (int64, error) {
	s.RLock()
	defer s.RUnlock()

	return int64(len(s.store)), nil
}

func (s *storageMem) Create(payload []byte, expireIn time.Duration, reads int) (string, error) {
	s.Lock()
	defer s.Unlock()

	id := storage.GenerateUUID()
	expire := storage.CalculateExpiry(expireIn)
	reads = storage.NormalizeReads(reads)

	s.store[id] = memStorageSecret{
		Expiry:         expire,
		ReadsRemaining: reads,
		Payload:        payload,
	}

	return id, nil
}

// Purge immediately destroys a stored secret entry in memory
func (s *storageMem) Purge(id string) ([]byte, error) {
	s.Lock()
	defer s.Unlock()

	entry, ok := s.store[id]
	if !ok {
		return nil, storage.ErrSecretNotFound
	}

	delete(s.store, id)
	return entry.Payload, nil
}

func (s *storageMem) ReadAndDestroy(id string) ([]byte, int, error) {
	s.Lock()
	defer s.Unlock()

	secret, ok := s.store[id]
	if !ok {
		return nil, 0, storage.ErrSecretNotFound
	}

	// Still check to see if the secret has expired in order to prevent a
	// race condition where a secret has expired but the store pruner has
	// not yet been invoked.
	if secret.hasExpired() {
		delete(s.store, id)
		return nil, 0, storage.ErrSecretNotFound
	}

	secret.ReadsRemaining--
	if secret.ReadsRemaining <= 0 {
		delete(s.store, id)
	} else {
		s.store[id] = secret
	}

	return secret.Payload, secret.ReadsRemaining, nil
}

func (s *storageMem) pruneStore() {
	s.Lock()
	defer s.Unlock()

	for k, v := range s.store {
		if v.hasExpired() {
			delete(s.store, k)
		}
	}
}

func (s *storageMem) storePruner() {
	for range s.storePruneTimer.C {
		s.pruneStore()
	}
}

func (m *memStorageSecret) hasExpired() bool {
	return !m.Expiry.IsZero() && m.Expiry.Before(time.Now())
}
