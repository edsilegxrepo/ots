// Package factory provides a unified constructor for instantiating pluggable OTS secret storage backends based on connection URI schemes.
//
// Objectives:
// - Decouple storage engine creation from main server logic and prevent Go import cycles.
// - Support pluggable storage engine schemes (memory://, redis://, memcached://, sqlite://, badger://).
// - Fallback safely to in-memory storage when no URI scheme is specified.
//
// Core Components:
// - CreateStorageEngine: Parses connection URI strings and delegates instantiation to sub-packages.
//
// Data Flow:
// 1. CreateStorageEngine(storageURL) -> Inspect scheme prefix.
// 2. memory:// -> Return memory.New()
// 3. redis:// -> Return redis.New(storageURL)
// 4. memcached:// -> Return memcached.New(storageURL)
// 5. sqlite:// -> Return sqlite.New(storageURL)
// 6. badger:// -> Return badger.New(storageURL)
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
