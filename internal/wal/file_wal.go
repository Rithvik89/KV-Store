package wal

import (
	"bufio"
	"fmt"
	"os"
)

// FileWAL is a file-based implementation of WAL
type FileWAL struct {
	filepath string
	file     *os.File
	closed   bool
}

// NewFileWAL creates a new file-based WAL
func NewFileWAL(filepath string) (*FileWAL, error) {
	if filepath == "" {
		filepath = "wal.log"
	}

	file, err := os.OpenFile(filepath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open WAL file: %w", err)
	}

	return &FileWAL{
		filepath: filepath,
		file:     file,
		closed:   false,
	}, nil
}

// Write appends an entry to the WAL
func (w *FileWAL) Write(entry *Entry) error {
	if w.closed {
		return ErrWALClosed
	}

	if entry == nil {
		return ErrInvalidEntry
	}

	// Write in simple space-separated format
	line := fmt.Sprintf("%s %s %s\n", entry.Op, entry.Key, entry.Value)
	if _, err := w.file.WriteString(line); err != nil {
		return fmt.Errorf("failed to write to WAL: %w", err)
	}

	// Sync to ensure durability
	if err := w.file.Sync(); err != nil {
		return fmt.Errorf("failed to sync WAL: %w", err)
	}

	return nil
}

// WriteSet writes a SET operation to the WAL
func (w *FileWAL) WriteSet(key, value string) error {
	return w.Write(&Entry{
		Op:    OpSet,
		Key:   key,
		Value: value,
	})
}

// WriteDelete writes a DELETE operation to the WAL
func (w *FileWAL) WriteDelete(key string) error {
	return w.Write(&Entry{
		Op:  OpDelete,
		Key: key,
	})
}

// Replay replays the WAL entries using the provided callback
func (w *FileWAL) Replay(callback func(*Entry) error) error {
	file, err := os.Open(w.filepath)
	if err != nil {
		if os.IsNotExist(err) {
			// No WAL file exists yet, nothing to replay
			return nil
		}
		return fmt.Errorf("failed to open WAL for replay: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		var entry Entry
		n, err := fmt.Sscanf(line, "%s %s %s", &entry.Op, &entry.Key, &entry.Value)
		if err != nil || n < 2 {
			// Log warning but continue - don't let one bad entry stop recovery
			fmt.Printf("Warning: failed to parse WAL entry at line %d: %v\n", lineNum, err)
			continue
		}

		if err := callback(&entry); err != nil {
			return fmt.Errorf("replay callback failed at line %d: %w", lineNum, err)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading WAL: %w", err)
	}

	return nil
}

// Close closes the WAL file
func (w *FileWAL) Close() error {
	if w.closed {
		return nil
	}

	w.closed = true

	if w.file != nil {
		if err := w.file.Sync(); err != nil {
			return fmt.Errorf("failed to sync before close: %w", err)
		}
		if err := w.file.Close(); err != nil {
			return fmt.Errorf("failed to close WAL: %w", err)
		}
		w.file = nil
	}
	return nil
}

// Truncate clears the WAL file (use after successful snapshot/compaction)
func (w *FileWAL) Truncate() error {
	if w.closed {
		return ErrWALClosed
	}

	if err := w.file.Truncate(0); err != nil {
		return fmt.Errorf("failed to truncate WAL: %w", err)
	}

	if _, err := w.file.Seek(0, 0); err != nil {
		return fmt.Errorf("failed to seek WAL: %w", err)
	}

	return nil
}

// Size returns the current size of the WAL file in bytes
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
