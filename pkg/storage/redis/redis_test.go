// Package redis - Redis Storage Engine Unit Test Suite
package redis

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRedisStorageNewValidation(t *testing.T) {
	// Backup original REDIS_URL
	origURL := os.Getenv("REDIS_URL")
	defer func() { _ = os.Setenv("REDIS_URL", origURL) }()

	// Test missing REDIS_URL
	_ = os.Unsetenv("REDIS_URL")
	store, err := New()
	assert.Error(t, err)
	assert.Nil(t, store)
	assert.Contains(t, err.Error(), "REDIS_URL environment variable not set")

	// Test invalid REDIS_URL
	_ = os.Setenv("REDIS_URL", "invalid_scheme://[::1]:namedport")
	store, err = New()
	assert.Error(t, err)
	assert.Nil(t, store)
	assert.Contains(t, err.Error(), "parsing REDIS_URL")
}

func TestRedisKeyFormatting(t *testing.T) {
	origPrefix := os.Getenv("REDIS_KEY")
	defer func() { _ = os.Setenv("REDIS_KEY", origPrefix) }()

	s := storageRedis{}

	// Default prefix
	_ = os.Unsetenv("REDIS_KEY")
	key := s.redisKey("test-id-123")
	assert.Equal(t, "io.luzifer.ots:test-id-123", key)

	// Custom prefix
	_ = os.Setenv("REDIS_KEY", "custom.prefix")
	key = s.redisKey("test-id-123")
	assert.Equal(t, "custom.prefix:test-id-123", key)
}
