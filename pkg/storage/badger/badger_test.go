// Package badger - Pure Go BadgerDB LSM Storage Engine Unit Test Suite
//
// Test Strategy Explanation:
// - Interface Contract Verification: Ensures BadgerDB engine complies with storage.Storage interface.
// - In-Memory & Disk Initialization: Tests diskless memory mode ("badger://:memory:") and directory-backed persistence.
// - Multi-Read & Native Expiration: Tests atomic multi-read decrements and native BadgerDB entry TTL expiration.
package badger

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/edsilegxrepo/ots/pkg/storage"
)

func TestBadgerStorageInterfaceContract(t *testing.T) {
	store, err := New("badger://:memory:")
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	// Count initial
	count, err := store.Count()
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)

	// Create single-read secret
	id1, err := store.Create([]byte("secret_content_badger"), time.Hour, 1)
	require.NoError(t, err)
	assert.NotEmpty(t, id1)

	count, err = store.Count()
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	// Read and destroy single-read secret
	content, remaining, err := store.ReadAndDestroy(id1)
	require.NoError(t, err)
	assert.Equal(t, []byte("secret_content_badger"), content)
	assert.Equal(t, 0, remaining)

	// Second read returns ErrSecretNotFound
	_, _, err = store.ReadAndDestroy(id1)
	assert.Equal(t, storage.ErrSecretNotFound, err)

	// Create multi-read secret (3 reads)
	id2, err := store.Create([]byte("multi_read_badger"), time.Hour, 3)
	require.NoError(t, err)

	// Read 1 (2 remaining)
	content, remaining, err = store.ReadAndDestroy(id2)
	require.NoError(t, err)
	assert.Equal(t, []byte("multi_read_badger"), content)
	assert.Equal(t, 2, remaining)

	// Read 2 (1 remaining)
	_, remaining, err = store.ReadAndDestroy(id2)
	require.NoError(t, err)
	assert.Equal(t, 1, remaining)

	// Read 3 (0 remaining)
	_, remaining, err = store.ReadAndDestroy(id2)
	require.NoError(t, err)
	assert.Equal(t, 0, remaining)

	// Read 4 returns ErrSecretNotFound
	_, _, err = store.ReadAndDestroy(id2)
	assert.Equal(t, storage.ErrSecretNotFound, err)
}

func TestBadgerDiskStorage(t *testing.T) {
	dir := t.TempDir()
	store, err := New("badger://" + dir)
	require.NoError(t, err)

	id, err := store.Create([]byte("disk_badger_secret"), time.Hour, 1)
	require.NoError(t, err)

	content, remaining, err := store.ReadAndDestroy(id)
	require.NoError(t, err)
	assert.Equal(t, []byte("disk_badger_secret"), content)
	assert.Equal(t, 0, remaining)

	require.NoError(t, store.Close())
}

func TestBadgerPurgeAndExpiredFallback(t *testing.T) {
	store, err := New("")
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	// Test Purge
	id, err := store.Create([]byte("purge_badger_payload"), time.Hour, 1)
	require.NoError(t, err)

	purged, err := store.Purge(id)
	require.NoError(t, err)
	assert.Equal(t, []byte("purge_badger_payload"), purged)

	_, err = store.Purge(id)
	require.ErrorIs(t, err, storage.ErrSecretNotFound)

	// Purge missing secret
	_, err = store.Purge("non-existent-badger-id")
	require.ErrorIs(t, err, storage.ErrSecretNotFound)

	// Test Expired Fallback in ReadAndDestroy
	expID, err := store.Create([]byte("expired_payload"), 1*time.Millisecond, 1)
	require.NoError(t, err)
	time.Sleep(5 * time.Millisecond)

	_, _, err = store.ReadAndDestroy(expID)
	assert.ErrorIs(t, err, storage.ErrSecretNotFound)
}
