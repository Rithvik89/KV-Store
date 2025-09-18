package main

import (
	"KVStore/internal/config"
	"KVStore/internal/logger"
	"KVStore/internal/server"
	"KVStore/internal/storage"
	"KVStore/internal/wal"
)

func main() {
	logger := logger.NewLogger("main")

	// Initialize the storage
	store := storage.NewMemStore()

	// Initialize the WAL
	walInstance := wal.NewWAL(config.DefaultWALFile)

	// Recover from WAL
	if !walInstance.RecoverFromWAL(store) {
		logger.Fatal("Failed to recover from WAL")
	}

	// Initialize the executor
	executor := server.NewExecutor(store, walInstance)

	// Create and start the server
	srv := server.NewServer(executor)
	
	logger.Info("Starting KV Store server...")
	if err := srv.Start(); err != nil {
		logger.Fatal("Server failed to start: %v", err)
	}
}