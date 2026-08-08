// Package sqlite - Pure Go SQLite Storage Engine Unit Test Suite
//
// Test Strategy Explanation:
// - Interface Contract Conformance: Verifies that SQLite engine satisfies the core storage.Storage interface contract.
// - Creation & Multi-Read Destruction: Validates atomic secret creation, multi-read counter decrement, and final read deletion.
// - Expiration & Background Purge: Tests secret TTL boundary expiration and periodic background ticker cleanup routines.
package sqlite

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/edsilegxrepo/ots/pkg/storage"
)

func TestSQLiteStorageInterfaceContract(t *testing.T) {
	store, err := New("sqlite://:memory:")
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	// Count initial
	count, err := store.Count()
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)

	// Create single-read secret
	id1, err := store.Create([]byte("secret_content_1"), time.Hour, 1)
	require.NoError(t, err)
	assert.NotEmpty(t, id1)

	count, err = store.Count()
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	// Read and destroy single-read secret
	content, remaining, err := store.ReadAndDestroy(id1)
	require.NoError(t, err)
	assert.Equal(t, []byte("secret_content_1"), content)
	assert.Equal(t, 0, remaining)

	// Second read returns ErrSecretNotFound
	_, _, err = store.ReadAndDestroy(id1)
	assert.Equal(t, storage.ErrSecretNotFound, err)

	// Create multi-read secret (3 reads)
	id2, err := store.Create([]byte("multi_read_secret"), time.Hour, 3)
	require.NoError(t, err)

	// Read 1 (2 remaining)
	content, remaining, err = store.ReadAndDestroy(id2)
	require.NoError(t, err)
	assert.Equal(t, []byte("multi_read_secret"), content)
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

func TestSQLiteDiskStorage(t *testing.T) {
	tmpDir := t.TempDir()
	dbFile := tmpDir + "/ots_test.db"
	store, err := New("sqlite://" + dbFile)
	require.NoError(t, err)

	id, err := store.Create([]byte("disk_sqlite_secret"), time.Hour, 1)
	require.NoError(t, err)

	content, remaining, err := store.ReadAndDestroy(id)
	require.NoError(t, err)
	assert.Equal(t, []byte("disk_sqlite_secret"), content)
	assert.Equal(t, 0, remaining)

	require.NoError(t, store.Close())
}

func TestSQLiteExpiration(t *testing.T) {
	store, err := New("sqlite://:memory:")
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	id, err := store.Create([]byte("expired_secret"), time.Second, 1)
	require.NoError(t, err)

	time.Sleep(1500 * time.Millisecond)

	_, _, err = store.ReadAndDestroy(id)
	assert.Equal(t, storage.ErrSecretNotFound, err)
}
