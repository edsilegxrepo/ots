package main

import (
	"fmt"

	"github.com/edsilegxrepo/ots/pkg/storage"
	"github.com/edsilegxrepo/ots/pkg/storage/factory"
)

func getStorageByType(t string) (storage.Storage, error) {
	if t == "mem" {
		t = "memory://"
	}
	s, err := factory.CreateStorageEngine(t)
	if err != nil {
		return nil, fmt.Errorf("creating storage engine for %q: %w", t, err)
	}
	return s, nil
}
