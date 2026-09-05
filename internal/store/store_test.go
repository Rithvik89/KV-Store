package store

import (
	"path/filepath"
	"testing"

	"memkv/internal/wal"
)

func TestSetGetDelete(t *testing.T) {
	s := OpenMemory()
	if err := s.Set("a", "1"); err != nil {
		t.Fatal(err)
	}
	v, err := s.Get("a")
	if err != nil || v != "1" {
		t.Fatalf("Get: %q %v", v, err)
	}
	if err := s.Delete("a"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get("a"); err != ErrKeyNotFound {
		t.Fatalf("want ErrKeyNotFound, got %v", err)
	}
}

func TestLazyExpireOnGet(t *testing.T) {
	now := int64(1_000_000)
	s := openMemoryWithClock(func() int64 { return now })

	if err := s.Set("k", "v"); err != nil {
		t.Fatal(err)
	}
	if n := s.Expire("k", 5); n != 1 {
		t.Fatalf("Expire=%d", n)
	}
	if v, err := s.Get("k"); err != nil || v != "v" {
		t.Fatalf("Get live: %q %v", v, err)
	}
	now = 1_000_000 + 5_000
	if _, err := s.Get("k"); err != ErrKeyNotFound {
		t.Fatalf("want miss after expire, got %v", err)
	}
}

func TestExpiredKeyLingersUntilAccess(t *testing.T) {
	now := int64(1_000_000)
	s := openMemoryWithClock(func() int64 { return now })

	_ = s.Set("linger", "x")
	_ = s.Expire("linger", 1)
	now = 1_000_000 + 2_000

	if s.lenRaw() != 1 {
		t.Fatalf("expired key should linger until accessed, lenRaw=%d", s.lenRaw())
	}
	if _, err := s.Get("linger"); err != ErrKeyNotFound {
		t.Fatalf("Get after expire: %v", err)
	}
	if s.lenRaw() != 0 {
		t.Fatalf("after Get, key should be gone, lenRaw=%d", s.lenRaw())
	}
}

func TestTTLAndPersist(t *testing.T) {
	now := int64(1_000_000)
	s := openMemoryWithClock(func() int64 { return now })

	if s.TTL("missing") != -2 {
		t.Fatal("TTL missing")
	}
	_ = s.Set("k", "v")
	if s.TTL("k") != -1 {
		t.Fatal("TTL no expiry")
	}
	if s.Expire("k", 10) != 1 {
		t.Fatal("Expire")
	}
	if got := s.TTL("k"); got != 10 {
		t.Fatalf("TTL=%d want 10", got)
	}
	now = 1_000_000 + 2_500
	if got := s.TTL("k"); got != 8 {
		t.Fatalf("TTL ceil=%d want 8", got)
	}
	if s.Persist("k") != 1 {
		t.Fatal("Persist")
	}
	if s.TTL("k") != -1 {
		t.Fatal("TTL after Persist")
	}
	if s.Persist("k") != 0 {
		t.Fatal("Persist again")
	}
}

func TestExpireMissingAndZero(t *testing.T) {
	s := OpenMemory()
	if s.Expire("nope", 5) != 0 {
		t.Fatal("Expire missing")
	}
	_ = s.Set("k", "v")
	if s.Expire("k", 0) != 1 {
		t.Fatal("Expire 0")
	}
	if s.Exists("k") {
		t.Fatal("Expire 0 should delete")
	}
}

func TestSetClearsExpiry(t *testing.T) {
	now := int64(1_000_000)
	s := openMemoryWithClock(func() int64 { return now })
	_ = s.Set("k", "v")
	_ = s.Expire("k", 5)
	_ = s.Set("k", "v2")
	if s.TTL("k") != -1 {
		t.Fatal("SET should clear expiry")
	}
}

func TestOpenRecoverSpacesInValue(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.wal")

	s, err := Open(path, wal.WithFsync(wal.FsyncAlways))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Set("name", "Ada Lovelace"); err != nil {
		t.Fatal(err)
	}
	if err := s.Set("a", "1"); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete("a"); err != nil {
		t.Fatal(err)
	}
	if s.Expire("name", 60) != 1 {
		t.Fatal("Expire")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()

	v, err := s2.Get("name")
	if err != nil || v != "Ada Lovelace" {
		t.Fatalf("recover Get: %q %v", v, err)
	}
	if s2.Exists("a") {
		t.Fatal("a should be deleted after recover")
	}
	// TTL was memory-only — not restored from WAL.
	if s2.TTL("name") != -1 {
		t.Fatalf("TTL after recover want -1, got %d", s2.TTL("name"))
	}
}

func TestStoreCompactRewritesLiveKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "compact.wal")
	s, err := Open(path, wal.WithFsync(wal.FsyncAlways))
	if err != nil {
		t.Fatal(err)
	}
	_ = s.Set("a", "1")
	_ = s.Set("b", "2")
	_ = s.Set("a", "9")
	_ = s.Delete("b")
	_ = s.Set("c", "3")

	before, err := s.wal.Size()
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Compact(); err != nil {
		t.Fatal(err)
	}
	after, err := s.wal.Size()
	if err != nil {
		t.Fatal(err)
	}
	if after >= before {
		t.Fatalf("expected smaller WAL after compact: before=%d after=%d", before, after)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	if s2.Size() != 2 {
		t.Fatalf("size=%d", s2.Size())
	}
	if v, _ := s2.Get("a"); v != "9" {
		t.Fatalf("a=%q", v)
	}
	if v, _ := s2.Get("c"); v != "3" {
		t.Fatalf("c=%q", v)
	}
	if s2.Exists("b") {
		t.Fatal("b should be gone")
	}
}
