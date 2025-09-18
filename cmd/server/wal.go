package main

import (
	"bufio"
	"fmt"
	"os"
)

type WALEntry struct {
	Op    string
	Key   string
	Value string
}

type WAL struct {
	file string
}

func initWAL(file string) *WAL {
	return &WAL{file: file}
}

func (w *WAL) writeToWAL(entry *WALEntry) bool {
	// Open the file in append mode, create if not exists
	file, err := os.OpenFile(w.file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		logger.Error("Failed to open WAL file: %v", err)
		return false
	}
	defer file.Close()

	// Write the entry to the file
	_, err = fmt.Fprintf(file, "%s %s %s\n", entry.Op, entry.Key, entry.Value)
	if err != nil {
		logger.Error("Failed to write to WAL file: %v", err)
		return false
	}
	return true
}

func (w *WAL) recoverFromWAL(store *MemStore) bool {

	file, err := os.Open(w.file)
	if err != nil {
		logger.Error("Failed to open WAL file: %v", err)
		return false
	}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		var entry WALEntry
		_, err := fmt.Sscanf(line, "%s %s %s", &entry.Op, &entry.Key, &entry.Value)
		if err != nil {
			logger.Error("Failed to parse WAL entry: %v", err)
			continue
		}
		if entry.Op == "PUT" {
			store.Put(entry.Key, entry.Value)
		}
	}
	if err := scanner.Err(); err != nil {
		logger.Error("Error reading WAL file: %v", err)
		return false
	}
	file.Close()
	return true
}
