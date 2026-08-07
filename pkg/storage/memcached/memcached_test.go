// Package memcached - Distributed Memcached Storage Engine Unit Test Suite
//
// Test Strategy Explanation:
// - Connection URI Parsing: Verifies instantiation of gomemcache Client across comma-separated daemon addresses.
// - Resilience & Offline Safety: Exercises graceful handling when target Memcached daemons are unavailable or keys missing.
package memcached

import (
	"sync"
	"testing"
	"time"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Luzifer/ots/pkg/storage"
)

type mockMemcachedClient struct {
	mu          sync.Mutex
	items       map[string]*memcache.Item
	casCounter  uint64
	casFailOnce bool
}

func newMockMemcachedClient() *mockMemcachedClient {
	return &mockMemcachedClient{
		items: make(map[string]*memcache.Item),
	}
}

func (m *mockMemcachedClient) Get(key string) (*memcache.Item, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	item, ok := m.items[key]
	if !ok {
		return nil, memcache.ErrCacheMiss
	}
	// Return a copy with current CAS id
	cp := *item
	return &cp, nil
}

func (m *mockMemcachedClient) Set(item *memcache.Item) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.casCounter++
	item.CasID = m.casCounter
	cp := *item
	m.items[item.Key] = &cp
	return nil
}

func (m *mockMemcachedClient) Delete(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.items, key)
	return nil
}

func (m *mockMemcachedClient) CompareAndSwap(item *memcache.Item) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.casFailOnce {
		m.casFailOnce = false
		return memcache.ErrCASConflict
	}

	existing, ok := m.items[item.Key]
	if !ok {
		return memcache.ErrCacheMiss
	}

	if existing.CasID != item.CasID {
		return memcache.ErrCASConflict
	}

	m.casCounter++
	item.CasID = m.casCounter
	cp := *item
	m.items[item.Key] = &cp
	return nil
}

func TestMemcachedStorageInterfaceContract(t *testing.T) {
	store, err := New("memcached://127.0.0.1:11211,127.0.0.1:11212")
	require.NoError(t, err)
	assert.NotNil(t, store)

	// Read non-existent secret (returns error if missing or offline daemon)
	_, _, err = store.ReadAndDestroy("non_existent_secret_id")
	assert.Error(t, err)
}

func TestMockMemcachedStorageLifecycle(t *testing.T) {
	mock := newMockMemcachedClient()
	store := NewWithClient(mock)

	// Count initial
	count, err := store.Count()
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)

	// Create single-read secret
	id1, err := store.Create("mc_secret_1", time.Hour, 1)
	require.NoError(t, err)
	assert.NotEmpty(t, id1)

	count, err = store.Count()
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	// Read & destroy single read secret
	content, remaining, err := store.ReadAndDestroy(id1)
	require.NoError(t, err)
	assert.Equal(t, "mc_secret_1", content)
	assert.Equal(t, 0, remaining)

	// Second read returns ErrSecretNotFound
	_, _, err = store.ReadAndDestroy(id1)
	assert.Equal(t, storage.ErrSecretNotFound, err)

	// Create multi-read secret with CAS retry simulation
	mock.casFailOnce = true
	id2, err := store.Create("mc_secret_multi", time.Hour, 2)
	require.NoError(t, err)

	// Read 1 (1 remaining) - exercises CAS retry on 1st attempt
	content, remaining, err = store.ReadAndDestroy(id2)
	require.NoError(t, err)
	assert.Equal(t, "mc_secret_multi", content)
	assert.Equal(t, 1, remaining)

	// Read 2 (0 remaining)
	content, remaining, err = store.ReadAndDestroy(id2)
	require.NoError(t, err)
	assert.Equal(t, 0, remaining)
}
