//go:build linux

package eventloop

import (
	"fmt"

	"golang.org/x/sys/unix"
)

type epollPoller struct {
	epfd    int
	scratch []unix.EpollEvent
}

func NewPoller(maxEvents int) (Poller, error) {
	if maxEvents <= 0 {
		maxEvents = 128
	}
	epfd, err := unix.EpollCreate1(0)
	if err != nil {
		return nil, fmt.Errorf("epoll_create1: %w", err)
	}
	return &epollPoller{
		epfd:    epfd,
		scratch: make([]unix.EpollEvent, maxEvents),
	}, nil
}

func (p *epollPoller) Add(fd int, interest Kind) error {
	ev := unix.EpollEvent{Events: kindToEpoll(interest), Fd: int32(fd)}
	if err := unix.EpollCtl(p.epfd, unix.EPOLL_CTL_ADD, fd, &ev); err != nil {
		return fmt.Errorf("epoll add fd=%d: %w", fd, err)
	}
	return nil
}

func (p *epollPoller) Modify(fd int, interest Kind) error {
	ev := unix.EpollEvent{Events: kindToEpoll(interest), Fd: int32(fd)}
	if err := unix.EpollCtl(p.epfd, unix.EPOLL_CTL_MOD, fd, &ev); err != nil {
		return fmt.Errorf("epoll modify fd=%d: %w", fd, err)
	}
	return nil
}

func (p *epollPoller) Remove(fd int) error {
	err := unix.EpollCtl(p.epfd, unix.EPOLL_CTL_DEL, fd, nil)
	if err != nil && err != unix.ENOENT && err != unix.EBADF {
		return fmt.Errorf("epoll remove fd=%d: %w", fd, err)
	}
	return nil
}

func (p *epollPoller) Wait(events []Event, timeoutMs int) (int, error) {
	if len(events) == 0 {
		return 0, nil
	}
	buf := p.scratch
	if len(buf) > len(events) {
		buf = buf[:len(events)]
	}
	timeout := timeoutMs
	if timeoutMs < 0 {
		timeout = -1
	}
	n, err := unix.EpollWait(p.epfd, buf, timeout)
	if err != nil {
		return 0, err
	}
	for i := 0; i < n; i++ {
		events[i] = Event{
			FD:   int(buf[i].Fd),
			Kind: epollToKind(buf[i].Events),
		}
	}
	return n, nil
}

func (p *epollPoller) Close() error {
	if p.epfd < 0 {
		return nil
	}
	err := unix.Close(p.epfd)
	p.epfd = -1
	return err
}

func kindToEpoll(k Kind) uint32 {
	var e uint32
	if k.Has(Readable) {
		e |= unix.EPOLLIN
	}
	if k.Has(Writable) {
		e |= unix.EPOLLOUT
	}
	return e
}

func epollToKind(events uint32) Kind {
	var k Kind
	if events&(unix.EPOLLIN|unix.EPOLLRDHUP|unix.EPOLLHUP|unix.EPOLLERR) != 0 {
		k |= Readable
	}
	if events&unix.EPOLLOUT != 0 {
		k |= Writable
	}
	return k
}
