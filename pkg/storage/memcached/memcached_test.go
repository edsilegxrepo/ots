package memcached

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemcachedStorageInterfaceContract(t *testing.T) {
	store, err := New("memcached://127.0.0.1:11211")
	require.NoError(t, err)
	assert.NotNil(t, store)

	// Read non-existent secret (returns error if missing or offline daemon)
	_, _, err = store.ReadAndDestroy("non_existent_secret_id")
	assert.Error(t, err)
}
