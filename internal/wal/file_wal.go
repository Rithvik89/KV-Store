package wal

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"time"
)

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
		hdr := make([]byte, headerSize)
		if _, err := io.ReadFull(file, hdr); err != nil {
			file.Close()
			return nil, fmt.Errorf("failed to read WAL header: %w", err)
		}
		if err := checkHeader(hdr); err != nil {
			file.Close()
			return nil, err
		}
		if _, err := file.Seek(0, io.SeekEnd); err != nil {
			file.Close()
			return nil, fmt.Errorf("failed to seek WAL: %w", err)
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
func (w *FileWAL) Replay(callback func(*Entry) error) error {
	if w.closed {
		return ErrWALClosed
	}

	f, err := os.Open(w.filepath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to open WAL for replay: %w", err)
	}
	defer f.Close()

	hdr := make([]byte, headerSize)
	if _, err := io.ReadFull(f, hdr); err != nil {
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return nil // empty / header-only incomplete — nothing to replay
		}
		return fmt.Errorf("failed to read WAL header: %w", err)
	}
	if err := checkHeader(hdr); err != nil {
		return err
	}

	lenBuf := make([]byte, 4)
	for {
		if _, err := io.ReadFull(f, lenBuf); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				return nil // clean EOF or torn length prefix
			}
			return fmt.Errorf("failed to read record length: %w", err)
		}
		payloadLen := binary.BigEndian.Uint32(lenBuf)
		// Cap absurd lengths to avoid huge allocs from corruption mid-file.
		if payloadLen > 64<<20 {
			return fmt.Errorf("%w: payload too large", ErrCorrupt)
		}
		payload := make([]byte, payloadLen)
		if _, err := io.ReadFull(f, payload); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				return nil // torn payload
			}
			return fmt.Errorf("failed to read payload: %w", err)
		}
		crcBuf := make([]byte, 4)
		if _, err := io.ReadFull(f, crcBuf); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				return nil // torn crc
			}
			return fmt.Errorf("failed to read crc: %w", err)
		}
		want := binary.BigEndian.Uint32(crcBuf)
		got := crc32.ChecksumIEEE(payload)
		if want != got {
			return fmt.Errorf("%w: crc mismatch", ErrCorrupt)
		}
		entry, err := decodePayload(payload)
		if err != nil {
			return err
		}
		if err := callback(entry); err != nil {
			return fmt.Errorf("replay callback failed: %w", err)
		}
	}
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

// Truncate clears the WAL (compaction stub until #31).
func (w *FileWAL) Truncate() error {
	if w.closed {
		return ErrWALClosed
	}
	if err := w.file.Truncate(0); err != nil {
		return fmt.Errorf("failed to truncate WAL: %w", err)
	}
	if _, err := w.file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek WAL: %w", err)
	}
	if _, err := w.file.Write(encodeHeader()); err != nil {
		return fmt.Errorf("failed to rewrite WAL header: %w", err)
	}
	return w.maybeSync(true)
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
