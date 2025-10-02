package wal

import "errors"

var (
	ErrInvalidEntry = errors.New("invalid WAL entry")
	ErrWALClosed    = errors.New("WAL is closed")
)

// Operation types
const (
	OpSet    = "SET"
	OpDelete = "DELETE"
)

// Entry represents a single WAL entry
type Entry struct {
	Op    string
	Key   string
	Value string
}

// WAL defines the interface for Write-Ahead Log operations
type WAL interface {
	// Write appends an entry to the WAL
	Write(entry *Entry) error

	// WriteSet writes a SET operation to the WAL
	WriteSet(key, value string) error

	// WriteDelete writes a DELETE operation to the WAL
	WriteDelete(key string) error

	// Replay replays all WAL entries using the provided callback
	Replay(callback func(*Entry) error) error

	// Close closes the WAL and flushes any pending writes
	Close() error

	// Truncate clears the WAL (use after snapshot/compaction)
	Truncate() error

	// Size returns the current size of the WAL in bytes
	Size() (int64, error)
}
