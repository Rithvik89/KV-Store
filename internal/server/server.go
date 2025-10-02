package server

import (
	"fmt"
	"net"

	"memkv/internal/eventloop"
	"memkv/internal/executor"
	"memkv/internal/logger"
)

// Server represents the KV-Store server
type Server struct {
	port     int
	executor *executor.Executor
	loop     *eventloop.EventLoop
}

// Config holds server configuration
type Config struct {
	Port    int
	WALPath string
}

// New creates a new server instance
func New(cfg Config) (*Server, error) {
	// Create executor with storage and WAL
	exec, err := executor.New(cfg.WALPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create executor: %w", err)
	}

	return &Server{
		port:     cfg.Port,
		executor: exec,
	}, nil
}

// Start starts the server
func (s *Server) Start() error {
	// Start TCP listener
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		return fmt.Errorf("failed to start listener: %w", err)
	}
	defer listener.Close()

	logger.Info("Listening on port %d", s.port)

	// Create event loop
	loop, err := eventloop.New(listener, s.executor)
	if err != nil {
		listener.Close()
		return fmt.Errorf("failed to create event loop: %w", err)
	}
	s.loop = loop
	defer s.loop.Close()

	// Run the event loop (blocking)
	return s.loop.Run()
}

// Close gracefully shuts down the server
func (s *Server) Close() error {
	if s.loop != nil {
		s.loop.Close()
	}
	if s.executor != nil {
		return s.executor.Close()
	}
	return nil
}
