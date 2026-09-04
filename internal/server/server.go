package server

import (
	"fmt"
	"net"
	"strings"

	"memkv/internal/eventloop"
	"memkv/internal/executor"
	"memkv/internal/logger"

	"golang.org/x/sys/unix"
)

// Server represents the KV-Store server.
//
// The event loop only reports readiness. Accept, read, and write live here.
// Design decision (docs/DESIGN.md): client fds are registered Readable-only;
// replies are written inside OnReadable. OnWritable is a no-op until we need
// out-buffer / write-interest arming.
type Server struct {
	port     int
	executor *executor.Executor
	loop     *eventloop.Loop
	lnFD     int
	listener net.Listener
}

// Config holds server configuration
type Config struct {
	Port    int
	WALPath string
}

// New creates a new server instance
func New(cfg Config) (*Server, error) {
	exec, err := executor.New(cfg.WALPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create executor: %w", err)
	}

	return &Server{
		port:     cfg.Port,
		executor: exec,
		lnFD:     -1,
	}, nil
}

// Start starts the server
func (s *Server) Start() error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		return fmt.Errorf("failed to start listener: %w", err)
	}
	s.listener = listener

	lnFD, err := listenerFD(listener)
	if err != nil {
		listener.Close()
		return err
	}
	s.lnFD = lnFD

	loop, err := eventloop.New(s, 256, 100)
	if err != nil {
		s.closeListener()
		return fmt.Errorf("failed to create event loop: %w", err)
	}
	s.loop = loop
	defer s.loop.Close()
	defer s.closeListener()

	if err := s.loop.Register(s.lnFD, eventloop.Readable); err != nil {
		s.closeListener()
		return fmt.Errorf("failed to register listener: %w", err)
	}

	logger.Info("Listening on port %d", s.port)
	return s.loop.Run()
}

// Close asks the loop to stop. Run returns within one poll timeout.
func (s *Server) Close() error {
	if s.loop != nil {
		s.loop.Stop()
	}
	if s.executor != nil {
		return s.executor.Close()
	}
	return nil
}

func (s *Server) closeListener() {
	if s.lnFD >= 0 {
		if s.loop != nil {
			_ = s.loop.Deregister(s.lnFD)
		}
		_ = unix.Close(s.lnFD)
		s.lnFD = -1
	}
	if s.listener != nil {
		_ = s.listener.Close()
		s.listener = nil
	}
}

// OnReadable is the loop callback. The listener and a client take different paths.
func (s *Server) OnReadable(fd int) error {
	if fd == s.lnFD {
		return s.accept()
	}
	s.readClient(fd)
	return nil
}

// OnWritable is intentionally unused for now.
//
// Replies go out from OnReadable / readClient. We will revisit write-interest
// arming if partial writes or slow subscribers become a real problem
// (see docs/DESIGN.md).
func (s *Server) OnWritable(fd int) error {
	return nil
}

func (s *Server) accept() error {
	for {
		nfd, _, err := unix.Accept(s.lnFD)
		if err != nil {
			if err == unix.EINTR {
				continue
			}
			if err == unix.EAGAIN || err == unix.EWOULDBLOCK {
				return nil
			}
			return fmt.Errorf("accept: %w", err)
		}

		if err := s.loop.Register(nfd, eventloop.Readable); err != nil {
			unix.Close(nfd)
			logger.Error("Failed to register client: %v", err)
			continue
		}
		logger.Info("New connection established on fd %d", nfd)
	}
}

func (s *Server) readClient(fd int) {
	buf := make([]byte, 4096)
	n, err := unix.Read(fd, buf)

	if n > 0 {
		input := strings.TrimSpace(string(buf[:n]))
		output := s.executor.ProcessCommand(input) + "\n"
		_, _ = unix.Write(fd, []byte(output))
	}

	if err != nil || n == 0 {
		s.closeConn(fd)
	}
}

func (s *Server) closeConn(fd int) {
	_ = s.loop.Deregister(fd)
	_ = unix.Close(fd)
	logger.Info("Closed connection on fd %d", fd)
}

func listenerFD(l net.Listener) (int, error) {
	tcp, ok := l.(*net.TCPListener)
	if !ok {
		return -1, fmt.Errorf("listener is not TCP")
	}
	f, err := tcp.File()
	if err != nil {
		return -1, fmt.Errorf("listener file: %w", err)
	}
	defer f.Close()

	fd, err := unix.Dup(int(f.Fd()))
	if err != nil {
		return -1, fmt.Errorf("dup listener fd: %w", err)
	}
	return fd, nil
}
