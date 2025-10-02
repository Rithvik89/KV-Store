package storage

import (
	"fmt"

	"memkv/internal/wal"
)

// PersistentStorage is a memory storage with WAL for durability
type PersistentStorage struct {
	store map[string]string
	wal   wal.WAL // Now using interface
}

// NewPersistentStorage creates a new persistent storage with file-based WAL
func NewPersistentStorage(walPath string) (*PersistentStorage, error) {
	w, err := wal.NewFileWAL(walPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create WAL: %w", err)
	}

	return newPersistentStorageWithWAL(w)
}

// NewPersistentStorageWithWAL creates storage with a custom WAL implementation
func NewPersistentStorageWithWAL(w wal.WAL) (*PersistentStorage, error) {
	return newPersistentStorageWithWAL(w)
}

func newPersistentStorageWithWAL(w wal.WAL) (*PersistentStorage, error) {
	ps := &PersistentStorage{
		store: make(map[string]string),
		wal:   w,
	}

	// Recover from WAL
	if err := ps.recover(); err != nil {
		w.Close()
		return nil, fmt.Errorf("failed to recover from WAL: %w", err)
	}

	return ps, nil
}

// recover replays the WAL to restore state
func (ps *PersistentStorage) recover() error {
	return ps.wal.Replay(func(entry *wal.Entry) error {
		switch entry.Op {
		case wal.OpSet:
			ps.store[entry.Key] = entry.Value
		case wal.OpDelete:
			delete(ps.store, entry.Key)
		default:
			return fmt.Errorf("unknown operation: %s", entry.Op)
		}
		return nil
	})
}

func (ps *PersistentStorage) Get(key string) (string, error) {
	value, ok := ps.store[key]
	if !ok {
		return "", ErrKeyNotFound
	}
	return value, nil
}

func (ps *PersistentStorage) Set(key string, value string) error {
	// Write to WAL first (Write-Ahead)
	if err := ps.wal.WriteSet(key, value); err != nil {
		return fmt.Errorf("WAL write failed: %w", err)
	}

	// Then update memory
	ps.store[key] = value
	return nil
}

func (ps *PersistentStorage) Delete(key string) error {
	if _, ok := ps.store[key]; !ok {
		return ErrKeyNotFound
	}

	// Write to WAL first
	if err := ps.wal.WriteDelete(key); err != nil {
		return fmt.Errorf("WAL write failed: %w", err)
	}

	// Then delete from memory
	delete(ps.store, key)
	return nil
}

func (ps *PersistentStorage) Exists(key string) bool {
	_, ok := ps.store[key]
	return ok
}

func (ps *PersistentStorage) Keys() []string {
	keys := make([]string, 0, len(ps.store))
	for k := range ps.store {
		keys = append(keys, k)
	}
	return keys
}

func (ps *PersistentStorage) Size() int {
	return len(ps.store)
}

func (ps *PersistentStorage) Close() error {
	return ps.wal.Close()
}

// Compact truncates the WAL (call after creating snapshot)
func (ps *PersistentStorage) Compact() error {
	return ps.wal.Truncate()
}
