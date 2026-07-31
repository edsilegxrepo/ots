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

	// Create a secret with 1 hour expiration
	id, err := store.Create("top_secret_data", 1*time.Hour)
	require.NoError(t, err)
	assert.NotEmpty(t, id)

	count, err = store.Count()
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	// Read and destroy secret
	data, err := store.ReadAndDestroy(id)
	require.NoError(t, err)
	assert.Equal(t, "top_secret_data", data)

	// Count after destroy
	count, err = store.Count()
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)

	// Read again should fail with ErrSecretNotFound
	_, err = store.ReadAndDestroy(id)
	require.ErrorIs(t, err, storage.ErrSecretNotFound)
}

func TestMemoryStorageExpiration(t *testing.T) {
	store := New().(*storageMem)

	// Secret expired in the past
	id := "expired_id"
	store.store[id] = memStorageSecret{
		Expiry: time.Now().Add(-1 * time.Second),
		Secret: "expired_secret",
	}

	_, err := store.ReadAndDestroy(id)
	require.ErrorIs(t, err, storage.ErrSecretNotFound)

	// Test pruneStore explicitly
	store.store[id] = memStorageSecret{
		Expiry: time.Now().Add(-1 * time.Second),
		Secret: "old",
	}
	store.pruneStore()

	_, ok := store.store[id]
	assert.False(t, ok)
}
