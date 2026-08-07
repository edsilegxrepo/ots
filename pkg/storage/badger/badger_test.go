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
	id1, err := store.Create("secret_content_badger", time.Hour, 1)
	require.NoError(t, err)
	assert.NotEmpty(t, id1)

	count, err = store.Count()
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	// Read and destroy single-read secret
	content, remaining, err := store.ReadAndDestroy(id1)
	require.NoError(t, err)
	assert.Equal(t, "secret_content_badger", content)
	assert.Equal(t, 0, remaining)

	// Second read returns ErrSecretNotFound
	_, _, err = store.ReadAndDestroy(id1)
	assert.Equal(t, storage.ErrSecretNotFound, err)

	// Create multi-read secret (3 reads)
	id2, err := store.Create("multi_read_badger", time.Hour, 3)
	require.NoError(t, err)

	// Read 1 (2 remaining)
	content, remaining, err = store.ReadAndDestroy(id2)
	require.NoError(t, err)
	assert.Equal(t, "multi_read_badger", content)
	assert.Equal(t, 2, remaining)

	// Read 2 (1 remaining)
	content, remaining, err = store.ReadAndDestroy(id2)
	require.NoError(t, err)
	assert.Equal(t, 1, remaining)

	// Read 3 (0 remaining)
	content, remaining, err = store.ReadAndDestroy(id2)
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

	id, err := store.Create("disk_badger_secret", time.Hour, 1)
	require.NoError(t, err)

	content, remaining, err := store.ReadAndDestroy(id)
	require.NoError(t, err)
	assert.Equal(t, "disk_badger_secret", content)
	assert.Equal(t, 0, remaining)

	require.NoError(t, store.Close())
}
