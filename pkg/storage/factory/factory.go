// Package factory provides a unified constructor for initializing OTS storage providers
package factory

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/Luzifer/ots/pkg/storage"
	"github.com/Luzifer/ots/pkg/storage/badger"
	"github.com/Luzifer/ots/pkg/storage/memcached"
	"github.com/Luzifer/ots/pkg/storage/memory"
	"github.com/Luzifer/ots/pkg/storage/redis"
	"github.com/Luzifer/ots/pkg/storage/sqlite"
)

// CreateStorageEngine instantiates a storage.Storage provider based on a connection URI scheme
func CreateStorageEngine(storageURL string) (storage.Storage, error) {
	if storageURL == "" || storageURL == "memory" || storageURL == "memory://" {
		return memory.New(), nil
	}

	u, err := url.Parse(storageURL)
	if err != nil {
		if strings.HasPrefix(storageURL, "sqlite:") {
			return sqlite.New(storageURL)
		}
		if strings.HasPrefix(storageURL, "badger:") {
			return badger.New(storageURL)
		}
		return nil, fmt.Errorf("invalid storage URL: %w", err)
	}

	switch u.Scheme {
	case "memory":
		return memory.New(), nil
	case "redis", "rediss":
		return redis.New()
	case "memcached":
		return memcached.New(storageURL)
	case "sqlite":
		return sqlite.New(storageURL)
	case "badger":
		return badger.New(storageURL)
	default:
		return nil, fmt.Errorf("unsupported storage scheme: %s", u.Scheme)
	}
}
