// Package badger implements the OTS storage.Storage interface backed by BadgerDB v4 LSM-tree key-value store.
//
// Objectives:
// - Provide high-performance embedded disk or in-memory persistence using log-structured merge trees.
// - Leverage BadgerDB's native entry-level TTL expiration (WithTTL) and ACID transactions.
// - Periodically trigger value log garbage collection (RunValueLogGC) to maintain optimal disk space utilization.
//
// Core Components:
// - Storage: Wraps *badgerdb.DB instance, close sync.Once, and background GC ticker.
// - badgerSecretEntry: Internal JSON struct encapsulating secret payload, expiry timestamp, and remaining reads.
// - New, Create, ReadAndDestroy, Delete, CleanupExpired: Implements atomic one-time secret operations.
//
// Data Flow:
// 1. New() -> Open badgerdb.DefaultOptions (in-memory or disk path) -> Start background GC ticker.
// 2. Create() -> Marshal badgerSecretEntry -> db.Update Tx -> SetEntry with WithTTL(expireIn).
// 3. ReadAndDestroy() -> db.Update Tx -> Get item -> Unmarshal entry -> Decrement remaining reads or Delete -> Commit.
package badger

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	badgerdb "github.com/dgraph-io/badger/v4"
	"github.com/gofrs/uuid"

	"github.com/edsilegxrepo/ots/pkg/storage"
)

type badgerSecretEntry struct {
	Content        string    `json:"c"`
	ExpiresAt      time.Time `json:"e"`
	ReadsRemaining int       `json:"r"`
}

// Storage implements storage.Storage backed by BadgerDB
type Storage struct {
	countTracker atomic.Int64
	db           *badgerdb.DB
	closeOnce    sync.Once
	stopChan     chan struct{}
}

// New initializes a BadgerDB storage instance (e.g. "badger:///path/to/data" or "badger://:memory:")
func New(connStr string) (*Storage, error) {
	dbPath := ""
	inMemory := false

	if connStr != "" {
		trimmed := strings.TrimPrefix(connStr, "badger://")
		trimmed = strings.TrimPrefix(trimmed, "badger:")
		if trimmed == ":memory:" || trimmed == "" {
			inMemory = true
		} else {
			u, err := url.Parse(trimmed)
			if err == nil && u.Path != "" {
				dbPath = u.Path
			} else {
				dbPath = trimmed
			}
		}
	} else {
		inMemory = true
	}

	var opts badgerdb.Options
	if inMemory || dbPath == "" {
		opts = badgerdb.DefaultOptions("").WithInMemory(true)
	} else {
		opts = badgerdb.DefaultOptions(filepath.Clean(dbPath))
	}
	opts = opts.WithLogger(nil)

	db, err := badgerdb.Open(opts)
	if err != nil {
		return nil, err
	}

	s := &Storage{
		db:       db,
		stopChan: make(chan struct{}),
	}

	// Start value log GC ticker
	go s.startGCTicker()

	return s, nil
}

// Close closes the BadgerDB database and stops background tickers
func (s *Storage) Close() error {
	s.closeOnce.Do(func() {
		close(s.stopChan)
	})
	return s.db.Close()
}

func (s *Storage) startGCTicker() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
			_ = s.db.RunValueLogGC(0.5)
		}
	}
}

// Count returns the estimated number of active secrets
func (s *Storage) Count() (int64, error) {
	c := s.countTracker.Load()
	if c < 0 {
		return 0, nil
	}
	return c, nil
}

// Create inserts a new secret with native TTL expiration and total allowed reads
func (s *Storage) Create(secret string, expireIn time.Duration, reads int) (string, error) {
	id := uuid.Must(uuid.NewV4()).String()
	if reads < 1 {
		reads = 1
	}

	var expiresAt time.Time
	if expireIn > 0 {
		expiresAt = time.Now().Add(expireIn)
	}

	entry := badgerSecretEntry{
		Content:        secret,
		ExpiresAt:      expiresAt,
		ReadsRemaining: reads,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return "", err
	}

	err = s.db.Update(func(txn *badgerdb.Txn) error {
		e := badgerdb.NewEntry([]byte(id), data)
		if expireIn > 0 {
			e = e.WithTTL(expireIn)
		}
		return txn.SetEntry(e)
	})
	if err != nil {
		return "", err
	}

	s.countTracker.Add(1)
	return id, nil
}

// ReadAndDestroy returns secret content, decrements remaining reads, and destroys entry when reads <= 0
func (s *Storage) ReadAndDestroy(id string) (string, int, error) {
	var (
		content        string
		readsRemaining int
		notFound       bool
	)

	err := s.db.Update(func(txn *badgerdb.Txn) error {
		item, err := txn.Get([]byte(id))
		if err != nil {
			if err == badgerdb.ErrKeyNotFound {
				notFound = true
				return storage.ErrSecretNotFound
			}
			return err
		}

		var entry badgerSecretEntry
		err = item.Value(func(val []byte) error {
			return json.Unmarshal(val, &entry)
		})
		if err != nil {
			return err
		}

		// Check expiration fallback
		if !entry.ExpiresAt.IsZero() && time.Now().After(entry.ExpiresAt) {
			_ = txn.Delete([]byte(id))
			s.countTracker.Add(-1)
			notFound = true
			return storage.ErrSecretNotFound
		}

		entry.ReadsRemaining--
		content = entry.Content
		readsRemaining = entry.ReadsRemaining

		if entry.ReadsRemaining <= 0 {
			s.countTracker.Add(-1)
			return txn.Delete([]byte(id))
		}

		// Re-save with updated remaining reads and remaining TTL
		updatedData, err := json.Marshal(entry)
		if err != nil {
			return err
		}

		e := badgerdb.NewEntry([]byte(id), updatedData)
		if !entry.ExpiresAt.IsZero() {
			remainingTTL := time.Until(entry.ExpiresAt)
			if remainingTTL > 0 {
				e = e.WithTTL(remainingTTL)
			}
		}

		return txn.SetEntry(e)
	})

	if notFound || err == storage.ErrSecretNotFound {
		return "", 0, storage.ErrSecretNotFound
	}
	if err != nil {
		return "", 0, fmt.Errorf("badger update transaction: %w", err)
	}

	return content, readsRemaining, nil
}
