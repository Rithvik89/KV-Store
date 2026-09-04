package wal

import "errors"

var (
	ErrInvalidEntry = errors.New("invalid WAL entry")
	ErrWALClosed    = errors.New("WAL is closed")
	ErrCorrupt      = errors.New("WAL corrupt or incomplete record")
	ErrBadMagic     = errors.New("WAL bad magic or version")
)

// Operation types (logical; on-disk uses a single byte).
const (
	OpSet    = "SET"
	OpDelete = "DELETE"
)

const (
	opSetByte    byte = 1
	opDeleteByte byte = 2

	magic   = "CNDR"
	version = byte(1)

	headerSize = 5 // magic(4) + version(1)
)

// FsyncPolicy controls when records are forced to durable storage.
type FsyncPolicy int

const (
	// FsyncAlways fsyncs after every record (default; safest).
	FsyncAlways FsyncPolicy = iota
	// FsyncEverySec fsyncs if at least one second has passed since the last sync.
	FsyncEverySec
	// FsyncNo only fsyncs on Close.
	FsyncNo
)

// Entry represents a single WAL entry.
type Entry struct {
	Op    string
	Key   string
	Value string
}

// WAL defines the interface for Write-Ahead Log operations.
type WAL interface {
	Write(entry *Entry) error
	WriteSet(key, value string) error
	WriteDelete(key string) error
	Replay(callback func(*Entry) error) error
	Close() error
	Truncate() error
	Size() (int64, error)
}
