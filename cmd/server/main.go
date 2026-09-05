package main

import (
	"os"
	"os/signal"
	"syscall"

	"memkv/internal/logger"
	"memkv/internal/server"
)

func main() {
	level, err := logger.ParseLevel(envOr("CINDER_LOG_LEVEL", "info"))
	if err != nil {
		logger.Fatal("%v", err)
	}
	logger.SetDefaultLevel(level)

	cfg, err := server.ConfigFromEnv()
	if err != nil {
		logger.Fatal("config: %v", err)
	}

	logger.Info("========================================")
	logger.Info("  Cinder / MemKV server")
	logger.Info("========================================")
	logger.Info("addr=%s wal=%s fsync=%s log=%s",
		cfg.Addr, cfg.WALPath, server.FsyncName(cfg.Fsync), envOr("CINDER_LOG_LEVEL", "info"))

	srv, err := server.New(cfg)
	if err != nil {
		logger.Fatal("Failed to create server: %v", err)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		logger.Info("received %v — shutting down (no os.Exit)", sig)
		srv.Shutdown()
	}()

	logger.Info("Server ready on %s — Ctrl+C to stop", cfg.Addr)
	if err := srv.Start(); err != nil {
		logger.Error("Server error: %v", err)
	}

	if err := srv.Close(); err != nil {
		logger.Error("Close: %v", err)
		os.Exit(1)
	}
	logger.Info("Goodbye!")
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
