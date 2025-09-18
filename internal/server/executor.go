package server

import (
	"KVStore/internal/logger"
	"KVStore/internal/parser"
	"KVStore/internal/storage"
	"KVStore/internal/wal"
)

// Executor handles command execution with storage and WAL
type Executor struct {
	Store  *storage.MemStore
	WAL    *wal.WAL
	logger *logger.Logger
}

// NewExecutor creates a new Executor instance
func NewExecutor(store *storage.MemStore, walInstance *wal.WAL) *Executor {
	return &Executor{
		Store:  store,
		WAL:    walInstance,
		logger: logger.NewLogger("executor"),
	}
}

// ProcessCmd processes a command and returns the result
func (e *Executor) ProcessCmd(cmd string) string {
	args, isValid := parser.ParseAndValidateCmd(cmd)
	if isValid {
		if args[0] == parser.CMD_GET {
			value, ok := e.Store.Get(args[1])
			if !ok {
				return "Key not found!"
			}
			return value
		}
		if args[0] == parser.CMD_PUT {
			// Insert into WAL before inserting into MemStore
			entry := &wal.WALEntry{
				Op:    "PUT",
				Key:   args[1],
				Value: args[2],
			}
			if !e.WAL.WriteToWAL(entry) {
				return "Failed to write to WAL!"
			}
			// Now insert into MemStore
			e.Store.Put(args[1], args[2])
			return "Successfully inserted! for key: " + args[1]
		}
	}

	return "Invalid input format!"
}