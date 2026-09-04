package eventloop

// Poller is the platform readiness API (epoll on Linux, kqueue on Darwin).
//
// Callers never talk to the OS directly. Loop uses a Poller to learn which
// registered file descriptors are ready without blocking on a single socket.
//
// Interest is level-triggered: a fd stays reported as ready until the
// condition clears (for example, until all pending data is read).
//
// NewPoller lives in poller_linux.go or poller_darwin.go; build tags pick one.
type Poller interface {
	// Add starts watching fd for the given interest (Readable, Writable, or both).
	// fd must already be non-blocking; Loop.Register sets that before calling Add.
	Add(fd int, interest Kind) error

	// Modify changes the interest set for an fd that is already watched.
	// Intended for arming/disarming Writable while an out buffer has pending
	// bytes; the server does not use this yet (replies go through OnReadable).
	Modify(fd int, interest Kind) error

	// Remove stops watching fd. It is safe to call when fd was never added or
	// was already removed; those cases return nil.
	Remove(fd int) error

	// Wait blocks until at least one watched fd is ready, or until timeoutMs
	// elapses. timeoutMs < 0 means wait forever. Events are written into the
	// prefix of events; n is how many were filled. A timeout with nothing ready
	// returns n == 0 and a nil error.
	Wait(events []Event, timeoutMs int) (n int, err error)

	// Close releases the underlying OS resource (epoll fd or kqueue). Call only
	// after Wait is no longer in progress.
	Close() error
}
