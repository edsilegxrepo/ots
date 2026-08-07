package factory

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateStorageEngineFactory(t *testing.T) {
	// Memory
	storeMem, err := CreateStorageEngine("memory://")
	require.NoError(t, err)
	assert.NotNil(t, storeMem)

	// SQLite
	storeSQLite, err := CreateStorageEngine("sqlite://:memory:")
	require.NoError(t, err)
	assert.NotNil(t, storeSQLite)

	// Badger
	storeBadger, err := CreateStorageEngine("badger://:memory:")
	require.NoError(t, err)
	assert.NotNil(t, storeBadger)

	// Memcached
	storeMC, err := CreateStorageEngine("memcached://127.0.0.1:11211")
	require.NoError(t, err)
	assert.NotNil(t, storeMC)

	// Unsupported scheme
	_, err = CreateStorageEngine("invalid_scheme://localhost")
	assert.Error(t, err)
}
