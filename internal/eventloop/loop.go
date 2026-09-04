package eventloop

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
)

// Handler receives readiness callbacks from the loop.
//
// Every method runs on the loop's own goroutine, one at a time. That is the
// contract the rest of the store is built on: the keyspace never locks because
// it is only ever touched from here. A non-nil error means that fd's work
// failed; the loop continues with the next event and does not exit.
//
// Design decision (see docs/DESIGN.md): for now all I/O is driven through
// OnReadable. Replies are written inside that path. OnWritable is kept on the
// interface so write-interest arming can land later without reshaping the loop;
// nothing registers Writable interest today, so OnWritable is not used in practice.
type Handler interface {
	// OnReadable is called when fd has data to read, or when a listening
	// socket has a connection ready to accept. Today this is also where
	// replies are written (accept path or read → process → write). Must not block.
	OnReadable(fd int) error

	// OnWritable is called when fd can accept more outbound bytes.
	//
	// Deferred: we do not arm Writable interest yet, so this is not invoked in
	// the current server. Kept for a later pass (out-buffer flush / slow peers).
	OnWritable(fd int) error
}

// Loop owns a Poller and turns readiness notifications into Handler calls.
//
// It is the single-threaded scheduler for the server: wait for ready fds,
// dispatch callbacks, and run Post tasks from other goroutines. Accept,
// framing, and commands live outside this type.
//
// FDs are registered Readable-only for now. The writable branch in Run exists
// for a future design; see docs/DESIGN.md.
type Loop struct {
	poller    Poller
	handler   Handler
	events    []Event
	timeoutMs int

	stopped atomic.Bool

	mu    sync.Mutex
	tasks []func()
}

// New builds a Loop that drives h.
//
// maxEvents caps how many readiness events one Wait may return (default 128).
// timeoutMs is how long Wait may block in the kernel. A value < 0 waits forever.
// A small positive timeout is how Stop and Post get noticed without a wakeup
// descriptor: the loop wakes, drains tasks, and checks the stopped flag.
func New(h Handler, maxEvents, timeoutMs int) (*Loop, error) {
	if h == nil {
		return nil, fmt.Errorf("evloop: nil handler")
	}
	if maxEvents <= 0 {
		maxEvents = 128
	}
	p, err := NewPoller(maxEvents)
	if err != nil {
		return nil, err
	}
	return &Loop{
		poller:    p,
		handler:   h,
		events:    make([]Event, maxEvents),
		timeoutMs: timeoutMs,
	}, nil
}

// Register puts fd under the loop's control.
//
// It first sets O_NONBLOCK (required: readiness can be stale, and a blocking
// read on a "ready" fd freezes every client), then adds fd to the poller with
// the given interest. interest is typically Readable for new clients.
func (l *Loop) Register(fd int, interest Kind) error {
	if err := syscall.SetNonblock(fd, true); err != nil {
		return fmt.Errorf("evloop: set nonblock fd=%d: %w", fd, err)
	}
	if err := l.poller.Add(fd, interest); err != nil {
		return fmt.Errorf("evloop: add fd=%d: %w", fd, err)
	}
	return nil
}

// Modify changes which readiness events the loop watches for fd.
//
// fd must already be registered. Passing Readable alone drops write interest;
// passing Readable|Writable watches both. Intended for a later pass that arms
// write interest only while an out buffer has pending bytes; unused today
// because replies are written inside OnReadable (see docs/DESIGN.md).
func (l *Loop) Modify(fd int, interest Kind) error {
	if err := l.poller.Modify(fd, interest); err != nil {
		return fmt.Errorf("evloop: modify fd=%d: %w", fd, err)
	}
	return nil
}

// Deregister stops watching fd.
//
// It is safe to call when fd was never registered or was already removed.
// Closing the underlying socket is the caller's responsibility.
func (l *Loop) Deregister(fd int) error {
	return l.poller.Remove(fd)
}

// Run drives the loop until Stop is called.
//
// It blocks the calling goroutine and pins it to one OS thread so the hot path
// stays on one core. Each iteration: drain Post tasks, Wait for ready fds,
// then dispatch handler callbacks. EINTR on Wait is retried. A handler error
// skips to the next event; it does not stop the loop. Returns nil on a clean Stop.
//
// Writable is checked before Readable so that, if write interest is armed later,
// outbound space is flushed before new reads generate more replies. Today fds
// are Readable-only, so only OnReadable runs in practice.
func (l *Loop) Run() error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	for !l.stopped.Load() {
		l.drainTasks()

		n, err := l.poller.Wait(l.events, l.timeoutMs)
		if err != nil {
			if err == syscall.EINTR {
				continue
			}
			if l.stopped.Load() {
				return nil
			}
			return fmt.Errorf("evloop: wait: %w", err)
		}
		if n == 0 {
			continue
		}

		for i := 0; i < n; i++ {
			ev := l.events[i]
			// OnWritable: unused while no fd has Writable interest (design decision).
			if ev.Kind.Has(Writable) {
				if err := l.handler.OnWritable(ev.FD); err != nil {
					continue
				}
			}
			if ev.Kind.Has(Readable) {
				if err := l.handler.OnReadable(ev.FD); err != nil {
					continue
				}
			}
		}
	}

	l.drainTasks()
	return nil
}

// Post queues fn to run on the loop goroutine before the next Wait.
//
// Use this for work started off the loop thread (signals, timers, background
// sampling) that must touch connection or store state. Only the task queue is
// synchronized; fn itself runs single-threaded on the loop. A nil fn is ignored.
func (l *Loop) Post(fn func()) {
	if fn == nil {
		return
	}
	l.mu.Lock()
	l.tasks = append(l.tasks, fn)
	l.mu.Unlock()
}

// drainTasks runs every function queued by Post, then clears the queue.
//
// It must only be called from the loop goroutine. The slice is swapped under
// the mutex so Post can enqueue while drainTasks is about to run the previous
// batch without holding the lock during callbacks.
func (l *Loop) drainTasks() {
	l.mu.Lock()
	pending := l.tasks
	l.tasks = nil
	l.mu.Unlock()

	for _, fn := range pending {
		fn()
	}
}

// Stop asks the loop to exit.
//
// It returns immediately. Run notices the flag within one poll timeout (or
// sooner if Wait is already returning events) and then returns. Safe to call
// from another goroutine, including from a Post callback.
func (l *Loop) Stop() {
	l.stopped.Store(true)
}

// Stopped reports whether Stop has been called.
func (l *Loop) Stopped() bool { return l.stopped.Load() }

// Close releases the poller.
//
// Call only after Run has returned. Closing while Wait is blocked is unsafe.
// Close is idempotent: a second call is a no-op.
func (l *Loop) Close() error {
	if l.poller == nil {
		return nil
	}
	err := l.poller.Close()
	l.poller = nil
	return err
}
