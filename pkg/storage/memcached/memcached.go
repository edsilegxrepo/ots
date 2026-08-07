// Package memcached implements the OTS storage.Storage interface backed by a distributed Memcached cluster.
//
// Objectives:
// - Provide high-speed distributed in-memory storage for non-persistent secret sharing clusters.
// - Leverage Memcached Compare-And-Set (CAS) atomic operations for concurrent read-and-destroy semantics.
// - Offload entry expiration directly to Memcached daemon key TTL handling.
//
// Core Components:
// - Storage: Wraps gomemcache.Client instance and atomic in-memory key counter tracker.
// - memcachedSecretEntry: Lightweight JSON container holding encrypted content payload and remaining view count.
// - New, Create, ReadAndDestroy, Delete: Performs atomic CAS operations against distributed Memcached daemons.
//
// Data Flow:
// 1. New() -> Parse connection string ("memcached://127.0.0.1:11211,10.0.0.1:11211") -> Initialize Client.
// 2. Create() -> Marshal entry -> mc.Set with TTL in seconds.
// 3. ReadAndDestroy() -> mc.Get (fetches item + CAS ID) -> Decrement reads -> mc.CompareAndSwap or mc.Delete.
package memcached

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/gofrs/uuid"

	"github.com/Luzifer/ots/pkg/storage"
)

const (
	defaultMemcachedTTLSeconds int32 = 86400 // Default 24h fallback TTL
	maxCASRetries              int32 = 5     // Maximum Compare-And-Swap retry attempts
)

type memcachedSecretEntry struct {
	Content        string `json:"c"`
	ReadsRemaining int    `json:"r"`
}

// MemcachedClient abstracts the gomemcache.Client methods for dependency injection and mocking.
type MemcachedClient interface {
	// Get retrieves a key item from Memcached.
	Get(key string) (*memcache.Item, error)
	// Set stores an item in Memcached.
	Set(item *memcache.Item) error
	// Delete removes a key from Memcached.
	Delete(key string) error
	// CompareAndSwap atomically updates an item using CAS ID.
	CompareAndSwap(item *memcache.Item) error
}

// Storage implements the storage.Storage interface backed by Memcached
type Storage struct {
	client       MemcachedClient
	countTracker atomic.Int64
}

// New initializes a Memcached storage instance from a connection string (e.g., "memcached://127.0.0.1:11211,127.0.0.1:11212")
func New(connStr string) (*Storage, error) {
	servers := []string{"127.0.0.1:11211"}
	if connStr != "" {
		trimmed := strings.TrimPrefix(connStr, "memcached://")
		if trimmed != "" {
			u, err := url.Parse("http://" + trimmed)
			if err == nil && u.Host != "" {
				servers = strings.Split(u.Host, ",")
			} else {
				servers = strings.Split(trimmed, ",")
			}
		}
	}

	mc := memcache.New(servers...)
	return &Storage{
		client: mc,
	}, nil
}

// NewWithClient creates a Storage instance using a provided MemcachedClient (useful for unit testing).
func NewWithClient(client MemcachedClient) *Storage {
	return &Storage{
		client: client,
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

// Create inserts a new secret with TTL expiration and total allowed reads
func (s *Storage) Create(secret string, expireIn time.Duration, reads int) (string, error) {
	id := uuid.Must(uuid.NewV4()).String()
	if reads < 1 {
		reads = 1
	}

	entry := memcachedSecretEntry{
		Content:        secret,
		ReadsRemaining: reads,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return "", fmt.Errorf("marshalling memcached entry: %w", err)
	}

	ttlSeconds := int32(expireIn.Seconds())
	if ttlSeconds <= 0 {
		ttlSeconds = defaultMemcachedTTLSeconds
	}

	err = s.client.Set(&memcache.Item{
		Expiration: ttlSeconds,
		Key:        id,
		Value:      data,
	})
	if err != nil {
		return "", fmt.Errorf("writing memcached key: %w", err)
	}

	s.countTracker.Add(1)
	return id, nil
}

// ReadAndDestroy returns secret content, atomically decrements remaining reads via CAS, and deletes key when reads <= 0
func (s *Storage) ReadAndDestroy(id string) (string, int, error) {
	for range maxCASRetries {
		item, err := s.client.Get(id)
		if err != nil {
			if err == memcache.ErrCacheMiss {
				return "", 0, storage.ErrSecretNotFound
			}
			return "", 0, fmt.Errorf("getting memcached item: %w", err)
		}

		var entry memcachedSecretEntry
		if err := json.Unmarshal(item.Value, &entry); err != nil {
			return "", 0, fmt.Errorf("unmarshalling memcached entry: %w", err)
		}

		entry.ReadsRemaining--

		if entry.ReadsRemaining <= 0 {
			// Delete key completely
			_ = s.client.Delete(id)
			s.countTracker.Add(-1)
			return entry.Content, 0, nil
		}

		// Update entry with CAS
		updatedData, err := json.Marshal(entry)
		if err != nil {
			return "", 0, fmt.Errorf("marshalling memcached CAS entry: %w", err)
		}

		item.Value = updatedData
		err = s.client.CompareAndSwap(item)
		if err == nil {
			return entry.Content, entry.ReadsRemaining, nil
		}
		if err != memcache.ErrCASConflict {
			return "", 0, fmt.Errorf("memcached CAS error: %w", err)
		}
		// If CAS conflict occurs, retry loop
	}

	// Fallback if CAS retries exhausted
	return "", 0, storage.ErrSecretNotFound
}
