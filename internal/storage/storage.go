package storage

import "errors"

var (
	ErrKeyNotFound = errors.New("key not found")
	ErrKeyExists   = errors.New("key already exists")
)

// Storage is the keyspace API implemented by Store.
//
// Expire / TTL / Persist are in-memory only (not WAL'd yet).
type Storage interface {
	Get(key string) (string, error)
	Set(key string, value string) error
	Delete(key string) error
	Exists(key string) bool
	Keys() []string
	Size() int
	Close() error

	// Expire sets a TTL in seconds. Returns 1 if the key exists, else 0.
	// seconds <= 0 deletes the key immediately (Redis-compatible).
	Expire(key string, seconds int64) int
	// TTL returns seconds left, -1 if no expiry, -2 if missing.
	TTL(key string) int64
	// Persist clears expiry. Returns 1 if a timeout was removed, else 0.
	Persist(key string) int
}
