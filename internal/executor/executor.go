package executor

import (
	"fmt"

	"memkv/internal/logger"
	"memkv/internal/storage"
)

// Executor handles command execution
type Executor struct {
	storage storage.Storage
}

// New creates a new executor with Persistant In-Memory Storage.
func New(walPath string) (*Executor, error) {
	store, err := storage.NewPersistentStorage(walPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage: %w", err)
	}

	logger.Info("Recovered %d keys from WAL", store.Size())

	return &Executor{
		storage: store,
	}, nil
}

// Close closes the executor and its resources
func (e *Executor) Close() error {
	return e.storage.Close()
}
