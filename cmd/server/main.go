package main

import (
	"os"
	"os/signal"
	"syscall"

	"memkv/internal/logger"
	"memkv/internal/server"
)

func main() {
	// Initialize logger
	logger.SetDefaultLevel(logger.INFO)

	// Print startup message
	logger.Info("========================================")
	logger.Info("  KV-Store Server v1.0")
	logger.Info("========================================")

	// Create server configuration
	cfg := server.Config{
		Port:    6178,
		WALPath: "/tmp/wal.log",
	}

	// Create server
	srv, err := server.New(cfg)
	if err != nil {
		logger.Fatal("Failed to create server: %v", err)
	}

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		logger.Info("\nShutting down gracefully...")
		logger.Info("Goodbye!")
		srv.Close()
		os.Exit(0)
	}()

	// Start server
	logger.Info("Server ready on port %d", cfg.Port)
	logger.Info("Press Ctrl+C to stop")

	if err := srv.Start(); err != nil {
		logger.Fatal("Server error: %v", err)
	}
}
