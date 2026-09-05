package server

import (
	"fmt"
	"net"

	"memkv/internal/command"
	"memkv/internal/eventloop"
	"memkv/internal/logger"
	"memkv/internal/proto"
	"memkv/internal/wal"

	"golang.org/x/sys/unix"
)

// Server represents the KV-Store server.
//
// The event loop only reports readiness. Accept, read, and write live here.
// Design decision (docs/DESIGN.md): client fds are registered Readable-only;
// replies are written inside OnReadable. OnWritable is a no-op until we need
// out-buffer / write-interest arming.
//
// Wire format is CSP (internal/proto). A small pending buffer per fd holds
// incomplete values across reads — enough for incremental Decode, not the
// fuller conn model parked in #22.
type Server struct {
	addr     string
	commands *command.Executor
	loop     *eventloop.Loop
	lnFD     int
	listener net.Listener
	pending  map[int][]byte // incomplete CSP bytes per client fd
}

// New creates a new server instance
func New(cfg Config) (*Server, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	cmds, err := command.New(cfg.WALPath, wal.WithFsync(cfg.Fsync))
	if err != nil {
		return nil, fmt.Errorf("failed to create command registry: %w", err)
	}

	return &Server{
		addr:     cfg.Addr,
		commands: cmds,
		lnFD:     -1,
		pending:  make(map[int][]byte),
	}, nil
}

// Start starts the server
func (s *Server) Start() error {
	listener, err := net.Listen("tcp", s.addr)
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

	logger.Info("Listening on %s", s.addr)
	return s.loop.Run()
}

// Shutdown asks the loop to stop. Safe to call from another goroutine.
// Start returns after the current poll timeout once Stop is noticed.
func (s *Server) Shutdown() {
	if s.loop != nil {
		s.loop.Stop()
	}
}

// Close stops the loop (if still running) and closes the store/WAL.
// Prefer: signal → Shutdown(); wait for Start to return; then Close().
func (s *Server) Close() error {
	s.Shutdown()
	if s.commands != nil {
		err := s.commands.Close()
		s.commands = nil
		return err
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
		s.pending[nfd] = nil
		if m := s.commands.Metrics(); m != nil {
			m.ClientConnected()
		}
		logger.Info("New connection established on fd %d", nfd)
	}
}

func (s *Server) readClient(fd int) {
	buf := make([]byte, 4096)
	n, err := unix.Read(fd, buf)

	if n > 0 {
		s.pending[fd] = append(s.pending[fd], buf[:n]...)
		s.drainCSP(fd)
	}

	if err != nil || n == 0 {
		s.closeConn(fd)
	}
}

// drainCSP decodes and executes every complete CSP value in the pending buffer.
func (s *Server) drainCSP(fd int) {
	for {
		cur := s.pending[fd]
		v, rest, ok, err := proto.Decode(cur)
		if err != nil {
			reply := proto.EncodeValue(proto.ErrorMsg("ERR protocol error"))
			_, _ = unix.Write(fd, reply)
			s.closeConn(fd)
			return
		}
		if !ok {
			return
		}
		s.pending[fd] = rest

		var reply proto.Value
		args, argErr := proto.CommandArgs(v)
		if argErr != nil {
			reply = proto.ErrorMsg("ERR protocol error")
		} else {
			reply = s.commands.Exec(args)
		}
		_, _ = unix.Write(fd, proto.EncodeValue(reply))
	}
}

func (s *Server) closeConn(fd int) {
	if _, ok := s.pending[fd]; ok {
		if m := s.commands.Metrics(); m != nil {
			m.ClientDisconnected()
		}
	}
	delete(s.pending, fd)
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
