//go:build darwin

package eventloop

import (
	"fmt"

	"golang.org/x/sys/unix"
)

type kqueuePoller struct {
	kq       int
	interest map[int]Kind
	scratch  []unix.Kevent_t
}

func NewPoller(maxEvents int) (Poller, error) {
	if maxEvents <= 0 {
		maxEvents = 128
	}
	kq, err := unix.Kqueue()
	if err != nil {
		return nil, fmt.Errorf("kqueue: %w", err)
	}
	return &kqueuePoller{
		kq:       kq,
		interest: make(map[int]Kind),
		scratch:  make([]unix.Kevent_t, maxEvents),
	}, nil
}

func (p *kqueuePoller) Add(fd int, interest Kind) error {
	if err := p.apply(fd, interest, 0); err != nil {
		return fmt.Errorf("kqueue add fd=%d: %w", fd, err)
	}
	p.interest[fd] = interest
	return nil
}

func (p *kqueuePoller) Modify(fd int, interest Kind) error {
	old := p.interest[fd]
	if err := p.apply(fd, interest, old); err != nil {
		return fmt.Errorf("kqueue modify fd=%d: %w", fd, err)
	}
	p.interest[fd] = interest
	return nil
}

func (p *kqueuePoller) Remove(fd int) error {
	old := p.interest[fd]
	delete(p.interest, fd)
	if old == 0 {
		return nil
	}
	err := p.apply(fd, 0, old)
	if err != nil && err != unix.ENOENT && err != unix.EBADF {
		return fmt.Errorf("kqueue remove fd=%d: %w", fd, err)
	}
	return nil
}

func (p *kqueuePoller) Wait(events []Event, timeoutMs int) (int, error) {
	if len(events) == 0 {
		return 0, nil
	}
	buf := p.scratch
	if len(buf) > len(events) {
		buf = buf[:len(events)]
	}
	var ts *unix.Timespec
	if timeoutMs >= 0 {
		t := unix.NsecToTimespec(int64(timeoutMs) * 1e6)
		ts = &t
	}
	n, err := unix.Kevent(p.kq, nil, buf, ts)
	if err != nil {
		return 0, err
	}
	for i := 0; i < n; i++ {
		events[i] = Event{
			FD:   int(buf[i].Ident),
			Kind: kqueueToKind(buf[i]),
		}
	}
	return n, nil
}

func (p *kqueuePoller) Close() error {
	if p.kq < 0 {
		return nil
	}
	err := unix.Close(p.kq)
	p.kq = -1
	return err
}

func (p *kqueuePoller) apply(fd int, want, have Kind) error {
	var ch []unix.Kevent_t
	ch = append(ch, filterChange(fd, unix.EVFILT_READ, want.Has(Readable), have.Has(Readable))...)
	ch = append(ch, filterChange(fd, unix.EVFILT_WRITE, want.Has(Writable), have.Has(Writable))...)
	if len(ch) == 0 {
		return nil
	}
	_, err := unix.Kevent(p.kq, ch, nil, nil)
	return err
}

func filterChange(fd int, filter int16, want, have bool) []unix.Kevent_t {
	if want == have {
		return nil
	}
	flags := uint16(unix.EV_ADD)
	if !want {
		flags = unix.EV_DELETE
	}
	return []unix.Kevent_t{{
		Ident:  uint64(fd),
		Filter: filter,
		Flags:  flags,
	}}
}

func kqueueToKind(ev unix.Kevent_t) Kind {
	switch ev.Filter {
	case unix.EVFILT_READ:
		return Readable
	case unix.EVFILT_WRITE:
		return Writable
	default:
		return 0
	}
}
