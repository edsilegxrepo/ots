// Package memcached implements the OTS storage.Storage interface for a distributed Memcached backend
package memcached

import (
	"encoding/json"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/gofrs/uuid"

	"github.com/Luzifer/ots/pkg/storage"
)

type memcachedSecretEntry struct {
	Content        string `json:"c"`
	ReadsRemaining int    `json:"r"`
}

// Storage implements the storage.Storage interface backed by Memcached
type Storage struct {
	client       *memcache.Client
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
		return "", err
	}

	ttlSeconds := int32(expireIn.Seconds())
	if ttlSeconds <= 0 {
		ttlSeconds = 86400 // Default 24h fallback
	}

	err = s.client.Set(&memcache.Item{
		Expiration: ttlSeconds,
		Key:        id,
		Value:      data,
	})
	if err != nil {
		return "", err
	}

	s.countTracker.Add(1)
	return id, nil
}

// ReadAndDestroy returns secret content, atomically decrements remaining reads via CAS, and deletes key when reads <= 0
func (s *Storage) ReadAndDestroy(id string) (string, int, error) {
	maxTries := 5
	for try := 0; try < maxTries; try++ {
		item, err := s.client.Get(id)
		if err != nil {
			if err == memcache.ErrCacheMiss {
				return "", 0, storage.ErrSecretNotFound
			}
			return "", 0, err
		}

		var entry memcachedSecretEntry
		if err := json.Unmarshal(item.Value, &entry); err != nil {
			return "", 0, err
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
			return "", 0, err
		}

		item.Value = updatedData
		err = s.client.CompareAndSwap(item)
		if err == nil {
			return entry.Content, entry.ReadsRemaining, nil
		}
		if err != memcache.ErrCASConflict {
			return "", 0, err
		}
		// If CAS conflict occurs, retry loop
	}

	// Fallback if CAS retries exhausted
	return "", 0, storage.ErrSecretNotFound
}
