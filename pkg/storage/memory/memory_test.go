package memory

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Luzifer/ots/pkg/storage"
)

func TestMemoryStorageLifecycle(t *testing.T) {
	store := New()

	// Initial count
	count, err := store.Count()
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)

	// Create a secret with 1 hour expiration and 2 allowed reads
	id, err := store.Create("top_secret_data", 1*time.Hour, 2)
	require.NoError(t, err)
	assert.NotEmpty(t, id)

	count, err = store.Count()
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	// First read (should leave 1 read remaining)
	data, readsRem, err := store.ReadAndDestroy(id)
	require.NoError(t, err)
	assert.Equal(t, "top_secret_data", data)
	assert.Equal(t, 1, readsRem)

	// Second read (should reach 0 remaining and destroy entry)
	data, readsRem, err = store.ReadAndDestroy(id)
	require.NoError(t, err)
	assert.Equal(t, "top_secret_data", data)
	assert.Equal(t, 0, readsRem)

	// Count after destroy
	count, err = store.Count()
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)

	// Third read should fail with ErrSecretNotFound
	_, _, err = store.ReadAndDestroy(id)
	require.ErrorIs(t, err, storage.ErrSecretNotFound)
}

func TestMemoryStorageExpiration(t *testing.T) {
	store := New().(*storageMem)

	// Secret expired in the past
	id := "expired_id"
	store.store[id] = memStorageSecret{
		Expiry:         time.Now().Add(-1 * time.Second),
		ReadsRemaining: 1,
		Secret:         "expired_secret",
	}

	_, _, err := store.ReadAndDestroy(id)
	require.ErrorIs(t, err, storage.ErrSecretNotFound)

	// Test pruneStore explicitly
	store.store[id] = memStorageSecret{
		Expiry:         time.Now().Add(-1 * time.Second),
		ReadsRemaining: 1,
		Secret:         "old",
	}
	store.pruneStore()

	_, ok := store.store[id]
	assert.False(t, ok)
}
