package wal

import (
	"bufio"
	"fmt"
	"os"

	"KVStore/internal/logger"
	"KVStore/internal/storage"
)

// WALEntry represents a single entry in the Write-Ahead Log
type WALEntry struct {
	Op    string
	Key   string
	Value string
}

// WAL represents the Write-Ahead Log
type WAL struct {
	file   string
	logger *logger.Logger
}

// NewWAL creates a new WAL instance
func NewWAL(file string) *WAL {
	return &WAL{
		file:   file,
		logger: logger.NewLogger("wal"),
	}
}

// WriteToWAL writes an entry to the WAL file
func (w *WAL) WriteToWAL(entry *WALEntry) bool {
	// Open the file in append mode, create if not exists
	file, err := os.OpenFile(w.file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		w.logger.Error("Failed to open WAL file: %v", err)
		return false
	}
	defer file.Close()

	// Write the entry to the file
	_, err = fmt.Fprintf(file, "%s %s %s\n", entry.Op, entry.Key, entry.Value)
	if err != nil {
		w.logger.Error("Failed to write to WAL file: %v", err)
		return false
	}
	return true
}

// RecoverFromWAL reads the WAL file and replays entries to restore the store
func (w *WAL) RecoverFromWAL(store *storage.MemStore) bool {
	file, err := os.Open(w.file)
	if err != nil {
		if os.IsNotExist(err) {
			w.logger.Info("WAL file does not exist, starting with empty store")
			return true
		}
		w.logger.Error("Failed to open WAL file: %v", err)
		return false
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		var entry WALEntry
		_, err := fmt.Sscanf(line, "%s %s %s", &entry.Op, &entry.Key, &entry.Value)
		if err != nil {
			w.logger.Error("Failed to parse WAL entry: %v", err)
			continue
		}
		if entry.Op == "PUT" {
			store.Put(entry.Key, entry.Value)
		}
	}
	if err := scanner.Err(); err != nil {
		w.logger.Error("Error reading WAL file: %v", err)
		return false
	}
	return true
}