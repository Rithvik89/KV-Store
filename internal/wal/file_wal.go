package wal

import (
	"fmt"
	"os"
	"time"
)

const compactTmpSuffix = ".compact.tmp"

func compactTmpPath(walPath string) string {
	return walPath + compactTmpSuffix
}

// FileWAL is a single-file WAL with checksummed framed records.
type FileWAL struct {
	filepath string
	file     *os.File
	closed   bool
	fsync    FsyncPolicy
	lastSync time.Time
}

// Option configures FileWAL construction.
type Option func(*FileWAL)

// WithFsync sets the fsync policy (default FsyncAlways).
func WithFsync(p FsyncPolicy) Option {
	return func(w *FileWAL) { w.fsync = p }
}

// NewFileWAL opens or creates a file-based WAL at path.
// Existing files are repaired: a torn trailing record is truncated before
// appends are accepted, so later writes are not hidden behind garbage.
func NewFileWAL(filepath string, opts ...Option) (*FileWAL, error) {
	if filepath == "" {
		filepath = "wal.log"
	}

	w := &FileWAL{
		filepath: filepath,
		fsync:    FsyncAlways,
	}
	for _, opt := range opts {
		opt(w)
	}

	// Leftover from a crash mid-compact (before rename) — safe to drop.
	_ = os.Remove(compactTmpPath(filepath))

	file, err := os.OpenFile(filepath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open WAL file: %w", err)
	}
	w.file = file

	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to stat WAL: %w", err)
	}

	if info.Size() == 0 {
		if _, err := file.Write(encodeHeader()); err != nil {
			file.Close()
			return nil, fmt.Errorf("failed to write WAL header: %w", err)
		}
		if err := w.maybeSync(true); err != nil {
			file.Close()
			return nil, err
		}
	} else {
		if err := repairTornTail(file); err != nil {
			file.Close()
			return nil, err
		}
	}

	w.lastSync = time.Now()
	return w, nil
}

// Write appends an entry to the WAL.
func (w *FileWAL) Write(entry *Entry) error {
	if w.closed {
		return ErrWALClosed
	}
	rec, err := encodeRecord(entry)
	if err != nil {
		return err
	}
	if _, err := w.file.Write(rec); err != nil {
		return fmt.Errorf("failed to write to WAL: %w", err)
	}
	return w.maybeSync(false)
}

func (w *FileWAL) maybeSync(force bool) error {
	switch {
	case force || w.fsync == FsyncAlways:
		// sync
	case w.fsync == FsyncNo:
		return nil
	case w.fsync == FsyncEverySec:
		if time.Since(w.lastSync) < time.Second {
			return nil
		}
	}
	if err := w.file.Sync(); err != nil {
		return fmt.Errorf("failed to sync WAL: %w", err)
	}
	w.lastSync = time.Now()
	return nil
}

// WriteSet writes a SET operation to the WAL.
func (w *FileWAL) WriteSet(key, value string) error {
	return w.Write(&Entry{Op: OpSet, Key: key, Value: value})
}

// WriteDelete writes a DELETE operation to the WAL.
func (w *FileWAL) WriteDelete(key string) error {
	return w.Write(&Entry{Op: OpDelete, Key: key})
}

// Replay walks well-formed records from the start of the file.
// An incomplete trailing record stops replay without error (torn tail).
// After NewFileWAL repair, the file should not contain a torn suffix.
func (w *FileWAL) Replay(callback func(*Entry) error) error {
	if w.closed {
		return ErrWALClosed
	}

	info, err := os.Stat(w.filepath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to stat WAL for replay: %w", err)
	}

	f, err := os.Open(w.filepath)
	if err != nil {
		return fmt.Errorf("failed to open WAL for replay: %w", err)
	}
	defer f.Close()

	_, err = walkRecords(f, info.Size(), callback)
	return err
}

// Rewrite replaces the WAL contents with live SET entries via temp file + rename.
func (w *FileWAL) Rewrite(entries []Entry) error {
	if w.closed {
		return ErrWALClosed
	}
	if w.file == nil {
		return ErrWALClosed
	}

	for i := range entries {
		if entries[i].Op != OpSet {
			return fmt.Errorf("%w: Rewrite only accepts SET entries, got %q", ErrInvalidEntry, entries[i].Op)
		}
	}

	tmp := compactTmpPath(w.filepath)
	_ = os.Remove(tmp)

	tf, err := os.OpenFile(tmp, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to create compact temp: %w", err)
	}

	writeTmp := func() error {
		if _, err := tf.Write(encodeHeader()); err != nil {
			return fmt.Errorf("failed to write compact header: %w", err)
		}
		for i := range entries {
			rec, err := encodeRecord(&entries[i])
			if err != nil {
				return err
			}
			if _, err := tf.Write(rec); err != nil {
				return fmt.Errorf("failed to write compact record: %w", err)
			}
		}
		if err := tf.Sync(); err != nil {
			return fmt.Errorf("failed to sync compact temp: %w", err)
		}
		return nil
	}

	if err := writeTmp(); err != nil {
		_ = tf.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := tf.Close(); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("failed to close compact temp: %w", err)
	}

	// Release the old inode so rename can replace the path.
	if err := w.file.Sync(); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("failed to sync WAL before compact rename: %w", err)
	}
	if err := w.file.Close(); err != nil {
		w.file = nil
		_ = os.Remove(tmp)
		return fmt.Errorf("failed to close WAL before compact rename: %w", err)
	}
	w.file = nil

	if err := os.Rename(tmp, w.filepath); err != nil {
		_ = os.Remove(tmp)
		if reopenErr := w.reopenAppend(); reopenErr != nil {
			return fmt.Errorf("compact rename failed: %v (reopen: %w)", err, reopenErr)
		}
		return fmt.Errorf("failed to rename compact temp into place: %w", err)
	}

	if err := w.reopenAppend(); err != nil {
		return fmt.Errorf("failed to reopen WAL after compact: %w", err)
	}
	return nil
}

func (w *FileWAL) reopenAppend() error {
	file, err := os.OpenFile(w.filepath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	if _, err := file.Seek(0, 2); err != nil { // SeekEnd
		_ = file.Close()
		return err
	}
	w.file = file
	w.closed = false
	w.lastSync = time.Now()
	return nil
}

// Close syncs (per policy / always on close) and closes the file.
func (w *FileWAL) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true
	if w.file != nil {
		if err := w.file.Sync(); err != nil {
			_ = w.file.Close()
			w.file = nil
			return fmt.Errorf("failed to sync before close: %w", err)
		}
		if err := w.file.Close(); err != nil {
			w.file = nil
			return fmt.Errorf("failed to close WAL: %w", err)
		}
		w.file = nil
	}
	return nil
}

// Size returns the current size of the WAL file in bytes.
func (w *FileWAL) Size() (int64, error) {
	if w.closed {
		return 0, ErrWALClosed
	}
	info, err := os.Stat(w.filepath)
	if err != nil {
		return 0, fmt.Errorf("failed to stat WAL: %w", err)
	}
	return info.Size(), nil
}
