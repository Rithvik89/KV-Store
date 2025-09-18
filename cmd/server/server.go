package main

import (
	"fmt"
	"net"
	"strings"

	"golang.org/x/sys/unix"
)

const PORT = 6178

type Executor struct {
	Store *MemStore
	WAL   *WAL
}

var logger *Logger

func markListenerAsNonBlocking(l net.Listener) int {
	tcpListener := l.(*net.TCPListener)
	file, err := tcpListener.File()
	if err != nil {
		panic(err)
	}
	fd := file.Fd()
	logger.Info("File descriptor for listener is: %d", fd)

	err = unix.SetNonblock(int(fd), true)
	if err != nil {
		logger.Fatal("Failed to set listener as non-blocking: %v", err)
	}
	return int(fd)
}

func initializeEventLoop() int {
	kq, err := unix.Kqueue()
	if err != nil {
		logger.Fatal("Failed to create kqueue: %v", err)
	}
	return kq
}

func handleNewConnections(lfd int, kq int) int {
	for {
		nfd, _, err := unix.Accept(lfd)

		if err != nil {
			// Return if no more connections are present to accept.
			// When sockets are set to non-blocking,
			//  unix.EAGAIN and unix.EWOULDBLOCK tells us that there are no more connections to accept.
			if err == unix.EAGAIN || err == unix.EWOULDBLOCK {
				return 0
			}
			// Handle other errors.
			// For simplicity we just print and return.
			logger.Error("Error accepting connection: %v", err)
			return -1
		}

		err = unix.SetNonblock(nfd, true)
		if err != nil {
			// Handle Error
		}

		// Add this nfd to Kevent changelist.
		clientEvent := unix.Kevent_t{
			Ident:  uint64(nfd),
			Filter: unix.EVFILT_READ,
			Flags:  unix.EV_ADD,
		}

		_, err = unix.Kevent(kq, []unix.Kevent_t{clientEvent}, nil, nil)

		if err != nil {
			// Handle Error.
		}
		logger.Info("New connection has been established, using FD: %d", nfd)
	}
}

func main() {

	logger = NewLogger("main")

	// Initialize the Executor.
	executor := &Executor{
		Store: &MemStore{
			Store: make(map[string]string),
		},
		WAL: initWAL("/tmp/wal.log"),
	}

	// Recover from WAL
	if !executor.WAL.recoverFromWAL(executor.Store) {
		logger.Fatal("Failed to recover from WAL")
	}

	// Start the TCP listener.

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", PORT))

	if err != nil {
		logger.Fatal("Failed to start listener: %v", err)
	}
	defer listener.Close()
	logger.Info("Listening on port %d", PORT)

	// Once this Listener file descriptor is marked as NonBlocking, Now it will return immediatly.
	lfd := markListenerAsNonBlocking(listener)

	// Initialize the Event Loop.

	kq := initializeEventLoop()
	defer unix.Close(kq) // Close the Event Loop

	// Add Listener to the EventLoop

	listener_event := unix.Kevent_t{
		Ident:  uint64(lfd),
		Filter: unix.EVFILT_READ,
		Flags:  unix.EV_ADD,
	}

	// Note:  Think of Kevent() as doing two jobs depending on arguments:
	// 1. With a changelist, it modifies subscriptions.
	// 2. With an eventlist, it waits for notifications.

	// Here we are modifying subscriptions.

	_, err = unix.Kevent(kq, []unix.Kevent_t{listener_event}, nil, nil)

	if err != nil {
		logger.Fatal("Failed to add listener to kqueue: %v", err)
	}

	// These are the buffer capacity we use to get the ready desciptors.
	events := make([]unix.Kevent_t, 16)

	for {
		n, err := unix.Kevent(kq, nil, events, nil)
		if err != nil {
			if err == unix.EINTR {
				continue // retry
			}
			logger.Error("Signal Interupt occured")
		}

		for i := 0; i < n; i++ {
			ev := events[i]

			fd := int(ev.Ident)

			if fd == lfd {
				// handle connection
				handleNewConnections(fd, kq)
			} else {
				// handle client FDs
				if ev.Filter == unix.EVFILT_READ {
					buf := make([]byte, 4096)
					m, err := unix.Read(int(ev.Ident), buf)

					if m > 0 {
						// Trim the buffer to actual read size with all the trailing and leading spaces/newlines removed.
						input := strings.TrimSpace(string(buf[:m]))
						//TODO: pick input and process command.
						output := processCmd(input, executor) + "\n"
						// Write the console / Process out.
						unix.Write(fd, []byte(output))
					}
					if err != nil || m == 0 {
						// Client closed connection or error occurred.
						// Clean up: remove fd from kqueue and close fd.
						clientEvent := unix.Kevent_t{
							Ident:  uint64(fd),
							Filter: unix.EVFILT_READ,
							Flags:  unix.EV_DELETE,
						}
						unix.Kevent(kq, []unix.Kevent_t{clientEvent}, nil, nil)
						unix.Close(fd)
						logger.Info("Closed connection on fd %d", fd)
					}
				}
			}

		}

	}
}
