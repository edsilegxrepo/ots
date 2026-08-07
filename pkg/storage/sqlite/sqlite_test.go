package sqlite

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Luzifer/ots/pkg/storage"
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
	id1, err := store.Create("secret_content_1", time.Hour, 1)
	require.NoError(t, err)
	assert.NotEmpty(t, id1)

	count, err = store.Count()
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	// Read and destroy single-read secret
	content, remaining, err := store.ReadAndDestroy(id1)
	require.NoError(t, err)
	assert.Equal(t, "secret_content_1", content)
	assert.Equal(t, 0, remaining)

	// Second read returns ErrSecretNotFound
	_, _, err = store.ReadAndDestroy(id1)
	assert.Equal(t, storage.ErrSecretNotFound, err)

	// Create multi-read secret (3 reads)
	id2, err := store.Create("multi_read_secret", time.Hour, 3)
	require.NoError(t, err)

	// Read 1 (2 remaining)
	content, remaining, err = store.ReadAndDestroy(id2)
	require.NoError(t, err)
	assert.Equal(t, "multi_read_secret", content)
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
