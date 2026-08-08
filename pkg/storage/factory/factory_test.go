// Package factory - Unified Storage Engine Factory Unit Test Suite
//
// Test Strategy Explanation:
// - Factory URI Scheme Resolution: Verifies correct engine selection across memory://, sqlite://, badger://, memcached://, and unsupported schemes.
package factory

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateStorageEngineFactory(t *testing.T) {
	// Empty & Memory defaults
	for _, u := range []string{"", "memory", "memory://"} {
		store, err := CreateStorageEngine(u)
		require.NoError(t, err)
		assert.NotNil(t, store)
	}

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

	// Redis scheme
	t.Setenv("REDIS_URL", "redis://127.0.0.1:6379")
	storeRedis, err := CreateStorageEngine("redis://127.0.0.1:6379")
	require.NoError(t, err)
	assert.NotNil(t, storeRedis)

	// Prefix error path handling for sqlite and badger
	storePrefSQLite, err := CreateStorageEngine("sqlite::memory:")
	require.NoError(t, err)
	assert.NotNil(t, storePrefSQLite)

	storePrefBadger, err := CreateStorageEngine("badger::memory:")
	require.NoError(t, err)
	assert.NotNil(t, storePrefBadger)

	// Invalid URL parse error
	_, err = CreateStorageEngine(":%invalid_url")
	require.Error(t, err)

	// Unsupported scheme
	_, err = CreateStorageEngine("ftp://localhost")
	require.Error(t, err)
}
