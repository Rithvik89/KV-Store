package server

import (
	"bufio"
	"fmt"
	"net"
	"strings"

	"KVStore/internal/config"
	"KVStore/internal/logger"
)

// Server represents the KV store server
type Server struct {
	executor *Executor
	logger   *logger.Logger
	port     int
}

// NewServer creates a new server instance
func NewServer(executor *Executor) *Server {
	return &Server{
		executor: executor,
		logger:   logger.NewLogger("server"),
		port:     config.PORT,
	}
}

// Start starts the server and handles incoming connections
func (s *Server) Start() error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		return fmt.Errorf("failed to start listener: %v", err)
	}
	defer listener.Close()
	
	s.logger.Info("Listening on port %d", s.port)

	for {
		conn, err := listener.Accept()
		if err != nil {
			s.logger.Error("Failed to accept connection: %v", err)
			continue
		}

		// Handle each connection in a goroutine
		go s.handleConnection(conn)
	}
}

// handleConnection handles a single client connection
func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()
	
	s.logger.Info("New connection established from %s", conn.RemoteAddr())
	
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		
		s.logger.Debug("Received command: %s", input)
		
		// Process the command
		output := s.executor.ProcessCmd(input)
		
		// Send response back to client
		_, err := conn.Write([]byte(output + "\n"))
		if err != nil {
			s.logger.Error("Failed to write response: %v", err)
			break
		}
	}
	
	if err := scanner.Err(); err != nil {
		s.logger.Error("Connection error: %v", err)
	}
	
	s.logger.Info("Connection closed for %s", conn.RemoteAddr())
}