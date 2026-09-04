package eventloop

import (
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

type recHandler struct {
	onRead  func(fd int) error
	onWrite func(fd int) error
	reads   int
	writes  int
}

func (h *recHandler) OnReadable(fd int) error {
	h.reads++
	if h.onRead != nil {
		return h.onRead(fd)
	}
	return nil
}

func (h *recHandler) OnWritable(fd int) error {
	h.writes++
	if h.onWrite != nil {
		return h.onWrite(fd)
	}
	return nil
}

func socketpair(t *testing.T) (int, int) {
	t.Helper()
	fds, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM, 0)
	if err != nil {
		t.Fatalf("socketpair: %v", err)
	}
	return fds[0], fds[1]
}

func TestPostRunsOnLoopAndStop(t *testing.T) {
	h := &recHandler{}
	l, err := New(h, 8, 20)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	ran := false
	l.Post(func() {
		ran = true
		l.Stop()
	})
	if err := l.Run(); err != nil {
		t.Fatal(err)
	}
	if !ran {
		t.Fatal("Post callback did not run")
	}
}

func TestLoopReadableSocketpair(t *testing.T) {
	a, b := socketpair(t)
	defer unix.Close(a)
	defer unix.Close(b)

	var got []byte
	h := &recHandler{}
	l, err := New(h, 16, 50)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	h.onRead = func(fd int) error {
		buf := make([]byte, 64)
		n, err := unix.Read(fd, buf)
		if n > 0 {
			got = append(got, buf[:n]...)
		}
		if string(got) == "hello" {
			l.Stop()
		}
		if err == unix.EAGAIN || err == unix.EWOULDBLOCK {
			return nil
		}
		return err
	}

	if err := l.Register(a, Readable); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() { done <- l.Run() }()

	if _, err := unix.Write(b, []byte("hello")); err != nil {
		t.Fatal(err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		l.Stop()
		t.Fatal("timeout waiting for readable")
	}
	if string(got) != "hello" {
		t.Fatalf("got %q", got)
	}
}

func TestLoopWriteInterestThenDisarm(t *testing.T) {
	a, b := socketpair(t)
	defer unix.Close(a)
	defer unix.Close(b)

	h := &recHandler{}
	l, err := New(h, 16, 30)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	h.onWrite = func(fd int) error {
		if err := l.Modify(fd, 0); err != nil {
			return err
		}
		l.Stop()
		return nil
	}

	if err := l.Register(a, Writable); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() { done <- l.Run() }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		l.Stop()
		t.Fatal("timeout waiting for writable")
	}
	if h.writes < 1 {
		t.Fatal("OnWritable never fired")
	}

	// After disarm + Stop, we should not have spun a huge number of times.
	if h.writes > 5 {
		t.Fatalf("write interest looks stuck in a busy loop: %d wakes", h.writes)
	}
}

func TestRegisterSetsNonblock(t *testing.T) {
	a, b := socketpair(t)
	defer unix.Close(a)
	defer unix.Close(b)

	h := &recHandler{}
	l, err := New(h, 8, 20)
	if err != nil {
		t.Fatal(err)
	}
	l.Post(func() { l.Stop() })
	defer l.Close()

	if err := l.Register(a, Readable); err != nil {
		t.Fatal(err)
	}
	flags, err := unix.FcntlInt(uintptr(a), unix.F_GETFL, 0)
	if err != nil {
		t.Fatal(err)
	}
	if flags&unix.O_NONBLOCK == 0 {
		t.Fatal("Register did not set O_NONBLOCK")
	}

	if err := l.Run(); err != nil {
		t.Fatal(err)
	}
}

func TestDeregisterMissingFD(t *testing.T) {
	h := &recHandler{}
	l, err := New(h, 8, 20)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	if err := l.Deregister(1 << 20); err != nil {
		t.Fatalf("Deregister on missing fd: %v", err)
	}
}
