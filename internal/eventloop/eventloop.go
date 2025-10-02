package eventloop

import (
	"fmt"
	"net"
	"strings"

	"memkv/internal/executor"
	"memkv/internal/logger"

	"golang.org/x/sys/unix"
)

// EventLoop handles the kqueue-based event loop
type EventLoop struct {
	kq       int
	lfd      int
	listener net.Listener
	executor *executor.Executor
	events   []unix.Kevent_t
}

// New creates a new event loop
func New(listener net.Listener, exec *executor.Executor) (*EventLoop, error) {
	// Mark listener as non-blocking
	lfd, err := markListenerAsNonBlocking(listener)
	if err != nil {
		return nil, fmt.Errorf("failed to set non-blocking: %w", err)
	}

	// Create kqueue
	kq, err := unix.Kqueue()
	if err != nil {
		return nil, fmt.Errorf("failed to create kqueue: %w", err)
	}

	el := &EventLoop{
		kq:       kq,
		lfd:      lfd,
		listener: listener,
		executor: exec,
		events:   make([]unix.Kevent_t, 16),
	}

	// Add listener to kqueue
	if err := el.addListenerToKqueue(); err != nil {
		unix.Close(kq)
		return nil, fmt.Errorf("failed to add listener to kqueue: %w", err)
	}

	return el, nil
}

// markListenerAsNonBlocking marks the listener socket as non-blocking
func markListenerAsNonBlocking(l net.Listener) (int, error) {
	tcpListener := l.(*net.TCPListener)
	file, err := tcpListener.File()
	if err != nil {
		return 0, fmt.Errorf("failed to get listener file: %w", err)
	}

	fd := int(file.Fd())
	logger.Info("File descriptor for listener is: %d", fd)

	if err := unix.SetNonblock(fd, true); err != nil {
		return 0, fmt.Errorf("failed to set non-blocking: %w", err)
	}

	return fd, nil
}

// addListenerToKqueue adds the listener to the kqueue
func (el *EventLoop) addListenerToKqueue() error {
	listenerEvent := unix.Kevent_t{
		Ident:  uint64(el.lfd),
		Filter: unix.EVFILT_READ,
		Flags:  unix.EV_ADD,
	}

	_, err := unix.Kevent(el.kq, []unix.Kevent_t{listenerEvent}, nil, nil)
	if err != nil {
		return fmt.Errorf("kevent add failed: %w", err)
	}

	return nil
}

// Run starts the event loop (blocking)
func (el *EventLoop) Run() error {
	logger.Info("Event loop started")

	for {
		n, err := unix.Kevent(el.kq, nil, el.events, nil)
		if err != nil {
			if err == unix.EINTR {
				continue // Retry on interrupt
			}
			return fmt.Errorf("kevent wait failed: %w", err)
		}

		for i := 0; i < n; i++ {
			ev := el.events[i]
			fd := int(ev.Ident)

			if fd == el.lfd {
				// Handle new connections
				if err := el.handleNewConnections(); err != nil {
					logger.Error("Error handling new connections: %v", err)
				}
			} else {
				// Handle client data
				if ev.Filter == unix.EVFILT_READ {
					el.handleClientData(fd)
				}
			}
		}
	}
}

// handleNewConnections accepts new client connections
func (el *EventLoop) handleNewConnections() error {
	for {
		nfd, _, err := unix.Accept(el.lfd)
		if err != nil {
			// No more connections to accept
			if err == unix.EAGAIN || err == unix.EWOULDBLOCK {
				return nil
			}
			return fmt.Errorf("accept failed: %w", err)
		}

		// Set new connection as non-blocking
		if err := unix.SetNonblock(nfd, true); err != nil {
			unix.Close(nfd)
			logger.Error("Failed to set client fd as non-blocking: %v", err)
			continue
		}

		// Add client to kqueue
		clientEvent := unix.Kevent_t{
			Ident:  uint64(nfd),
			Filter: unix.EVFILT_READ,
			Flags:  unix.EV_ADD,
		}

		if _, err := unix.Kevent(el.kq, []unix.Kevent_t{clientEvent}, nil, nil); err != nil {
			unix.Close(nfd)
			logger.Error("Failed to add client to kqueue: %v", err)
			continue
		}

		logger.Info("New connection established on fd %d", nfd)
	}
}

// handleClientData reads and processes data from a client
func (el *EventLoop) handleClientData(fd int) {
	buf := make([]byte, 4096)
	n, err := unix.Read(fd, buf)

	if n > 0 {
		// Process the command
		input := strings.TrimSpace(string(buf[:n]))
		output := el.executor.ProcessCommand(input) + "\n"

		// Write response
		unix.Write(fd, []byte(output))
	}

	// Handle connection close or error
	if err != nil || n == 0 {
		el.closeConnection(fd)
	}
}

// closeConnection closes a client connection
func (el *EventLoop) closeConnection(fd int) {
	// Remove from kqueue
	clientEvent := unix.Kevent_t{
		Ident:  uint64(fd),
		Filter: unix.EVFILT_READ,
		Flags:  unix.EV_DELETE,
	}
	unix.Kevent(el.kq, []unix.Kevent_t{clientEvent}, nil, nil)

	// Close the file descriptor
	unix.Close(fd)
	logger.Info("Closed connection on fd %d", fd)
}

// Close closes the event loop
func (el *EventLoop) Close() error {
	if el.kq > 0 {
		return unix.Close(el.kq)
	}
	return nil
}
