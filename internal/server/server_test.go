package server

import (
	"io"
	"net"
	"path/filepath"
	"testing"
	"time"

	"memkv/internal/proto"
)

func TestServerCSPPing(t *testing.T) {
	srv, addr := startTestServer(t)
	defer srv.Close()

	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	req := proto.EncodeValue(proto.Array(proto.Bulk("PING")))
	if _, err := conn.Write(req); err != nil {
		t.Fatal(err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 64)
	n, err := conn.Read(buf)
	if err != nil && err != io.EOF {
		t.Fatal(err)
	}
	got := string(buf[:n])
	want := string(proto.EncodeValue(proto.Simple("PONG")))
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestServerCSPSetGet(t *testing.T) {
	srv, addr := startTestServer(t)
	defer srv.Close()

	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))

	writeValue(t, conn, proto.Array(proto.Bulk("SET"), proto.Bulk("k"), proto.Bulk("v")))
	assertValue(t, conn, proto.Simple("OK"))

	writeValue(t, conn, proto.Array(proto.Bulk("GET"), proto.Bulk("k")))
	assertValue(t, conn, proto.Bulk("v"))

	writeValue(t, conn, proto.Array(proto.Bulk("GET"), proto.Bulk("missing")))
	assertValue(t, conn, proto.Null())
}

func startTestServer(t *testing.T) (*Server, string) {
	t.Helper()
	dir := t.TempDir()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	srv, err := New(Config{
		Addr:    addr,
		WALPath: filepath.Join(dir, "wal.log"),
	})
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() { done <- srv.Start() }()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("tcp", addr, 50*time.Millisecond)
		if err == nil {
			_ = c.Close()
			t.Cleanup(func() {
				_ = srv.Close()
				select {
				case <-done:
				case <-time.After(2 * time.Second):
				}
			})
			return srv, addr
		}
		time.Sleep(10 * time.Millisecond)
	}
	_ = srv.Close()
	t.Fatal("server did not start")
	return nil, ""
}

func TestShutdownStopsStart(t *testing.T) {
	dir := t.TempDir()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	srv, err := New(Config{Addr: addr, WALPath: filepath.Join(dir, "wal.log")})
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() { done <- srv.Start() }()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("tcp", addr, 50*time.Millisecond)
		if err == nil {
			_ = c.Close()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	srv.Shutdown()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return after Shutdown")
	}
	if err := srv.Close(); err != nil {
		t.Fatal(err)
	}
}

func writeValue(t *testing.T, conn net.Conn, v proto.Value) {
	t.Helper()
	if _, err := conn.Write(proto.EncodeValue(v)); err != nil {
		t.Fatal(err)
	}
}

func assertValue(t *testing.T, conn net.Conn, want proto.Value) {
	t.Helper()
	raw := make([]byte, 0, 256)
	tmp := make([]byte, 256)
	deadline := time.Now().Add(2 * time.Second)
	for {
		_ = conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		n, err := conn.Read(tmp)
		if n > 0 {
			raw = append(raw, tmp[:n]...)
			v, rest, ok, derr := proto.Decode(raw)
			if derr != nil {
				t.Fatal(derr)
			}
			if ok {
				if len(rest) != 0 {
					t.Fatalf("leftover %q", rest)
				}
				got := proto.EncodeValue(v)
				wantRaw := proto.EncodeValue(want)
				if string(got) != string(wantRaw) {
					t.Fatalf("got %q want %q", got, wantRaw)
				}
				return
			}
		}
		if err != nil {
			if time.Now().After(deadline) {
				t.Fatalf("timeout reading reply, buf=%q err=%v", raw, err)
			}
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			t.Fatal(err)
		}
	}
}
