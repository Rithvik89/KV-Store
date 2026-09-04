package storage

import (
	"fmt"

	"memkv/internal/wal"
)

// PersistentStorage is the one dict plus a WAL for durability.
// It does not maintain a second map — all keyspace state lives in dict.
type PersistentStorage struct {
	dict *MemoryStorage
	wal  wal.WAL
}

// NewPersistentStorage creates a new persistent storage with file-based WAL.
func NewPersistentStorage(walPath string) (*PersistentStorage, error) {
	w, err := wal.NewFileWAL(walPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create WAL: %w", err)
	}

	return newPersistentStorageWithWAL(w)
}

// NewPersistentStorageWithWAL creates storage with a custom WAL implementation.
func NewPersistentStorageWithWAL(w wal.WAL) (*PersistentStorage, error) {
	return newPersistentStorageWithWAL(w)
}

func newPersistentStorageWithWAL(w wal.WAL) (*PersistentStorage, error) {
	ps := &PersistentStorage{
		dict: NewMemoryStorage(),
		wal:  w,
	}

	if err := ps.recover(); err != nil {
		w.Close()
		return nil, fmt.Errorf("failed to recover from WAL: %w", err)
	}

	return ps, nil
}

// recover replays the WAL into the shared dict.
func (ps *PersistentStorage) recover() error {
	return ps.wal.Replay(func(entry *wal.Entry) error {
		switch entry.Op {
		case wal.OpSet:
			return ps.dict.Set(entry.Key, entry.Value)
		case wal.OpDelete:
			// Ignore missing keys during replay (idempotent).
			_ = ps.dict.Delete(entry.Key)
			return nil
		default:
			return fmt.Errorf("unknown operation: %s", entry.Op)
		}
	})
}

func (ps *PersistentStorage) Get(key string) (string, error) {
	return ps.dict.Get(key)
}

func (ps *PersistentStorage) Set(key string, value string) error {
	// Write to WAL first (Write-Ahead), then update the dict.
	if err := ps.wal.WriteSet(key, value); err != nil {
		return fmt.Errorf("WAL write failed: %w", err)
	}
	return ps.dict.Set(key, value)
}

func (ps *PersistentStorage) Delete(key string) error {
	if !ps.dict.Exists(key) {
		return ErrKeyNotFound
	}

	if err := ps.wal.WriteDelete(key); err != nil {
		return fmt.Errorf("WAL write failed: %w", err)
	}
	return ps.dict.Delete(key)
}

func (ps *PersistentStorage) Exists(key string) bool {
	return ps.dict.Exists(key)
}

func (ps *PersistentStorage) Keys() []string {
	return ps.dict.Keys()
}

func (ps *PersistentStorage) Size() int {
	return ps.dict.Size()
}

func (ps *PersistentStorage) Close() error {
	return ps.wal.Close()
}

// Expire is memory-only (not WAL'd in this sitting).
func (ps *PersistentStorage) Expire(key string, seconds int64) int {
	return ps.dict.Expire(key, seconds)
}

// TTL is memory-only.
func (ps *PersistentStorage) TTL(key string) int64 {
	return ps.dict.TTL(key)
}

// Persist is memory-only.
func (ps *PersistentStorage) Persist(key string) int {
	return ps.dict.Persist(key)
}

// Compact truncates the WAL (call after creating snapshot).
func (ps *PersistentStorage) Compact() error {
	return ps.wal.Truncate()
}
