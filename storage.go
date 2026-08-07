package main

import (
	"fmt"

	"github.com/Luzifer/ots/pkg/storage"
	"github.com/Luzifer/ots/pkg/storage/factory"
)

func getStorageByType(t string) (storage.Storage, error) {
	switch t {
	case "mem":
		return factory.CreateStorageEngine("memory://")

	case "redis":
		return factory.CreateStorageEngine("redis://")
	}

	s, err := factory.CreateStorageEngine(t)
	if err != nil {
		return nil, fmt.Errorf("creating storage engine for %q: %w", t, err)
	}
	return s, nil
}
