package storage

import "testing"

func TestSetGetDelete(t *testing.T) {
	ms := NewMemoryStorage()
	if err := ms.Set("a", "1"); err != nil {
		t.Fatal(err)
	}
	v, err := ms.Get("a")
	if err != nil || v != "1" {
		t.Fatalf("Get: %q %v", v, err)
	}
	if err := ms.Delete("a"); err != nil {
		t.Fatal(err)
	}
	if _, err := ms.Get("a"); err != ErrKeyNotFound {
		t.Fatalf("want ErrKeyNotFound, got %v", err)
	}
}

func TestLazyExpireOnGet(t *testing.T) {
	now := int64(1_000_000)
	ms := newMemoryStorageWithClock(func() int64 { return now })

	if err := ms.Set("k", "v"); err != nil {
		t.Fatal(err)
	}
	if n := ms.Expire("k", 5); n != 1 {
		t.Fatalf("Expire=%d", n)
	}
	// Still live.
	if v, err := ms.Get("k"); err != nil || v != "v" {
		t.Fatalf("Get live: %q %v", v, err)
	}
	// Past deadline.
	now = 1_000_000 + 5_000
	if _, err := ms.Get("k"); err != ErrKeyNotFound {
		t.Fatalf("want miss after expire, got %v", err)
	}
}

func TestExpiredKeyLingersUntilAccess(t *testing.T) {
	now := int64(1_000_000)
	ms := newMemoryStorageWithClock(func() int64 { return now })

	_ = ms.Set("linger", "x")
	_ = ms.Expire("linger", 1)
	now = 1_000_000 + 2_000 // past deadline; do not touch the key yet

	if ms.lenRaw() != 1 {
		t.Fatalf("expired key should linger until accessed, lenRaw=%d", ms.lenRaw())
	}
	if _, err := ms.Get("linger"); err != ErrKeyNotFound {
		t.Fatalf("Get after expire: %v", err)
	}
	if ms.lenRaw() != 0 {
		t.Fatalf("after Get, key should be gone, lenRaw=%d", ms.lenRaw())
	}
}

func TestTTLAndPersist(t *testing.T) {
	now := int64(1_000_000)
	ms := newMemoryStorageWithClock(func() int64 { return now })

	if ms.TTL("missing") != -2 {
		t.Fatal("TTL missing")
	}
	_ = ms.Set("k", "v")
	if ms.TTL("k") != -1 {
		t.Fatal("TTL no expiry")
	}
	if ms.Expire("k", 10) != 1 {
		t.Fatal("Expire")
	}
	if got := ms.TTL("k"); got != 10 {
		t.Fatalf("TTL=%d want 10", got)
	}
	now = 1_000_000 + 2_500 // 7.5s left → ceil 8
	if got := ms.TTL("k"); got != 8 {
		t.Fatalf("TTL ceil=%d want 8", got)
	}
	if ms.Persist("k") != 1 {
		t.Fatal("Persist")
	}
	if ms.TTL("k") != -1 {
		t.Fatal("TTL after Persist")
	}
	if ms.Persist("k") != 0 {
		t.Fatal("Persist again")
	}
}

func TestExpireMissingAndZero(t *testing.T) {
	ms := NewMemoryStorage()
	if ms.Expire("nope", 5) != 0 {
		t.Fatal("Expire missing")
	}
	_ = ms.Set("k", "v")
	if ms.Expire("k", 0) != 1 {
		t.Fatal("Expire 0")
	}
	if ms.Exists("k") {
		t.Fatal("Expire 0 should delete")
	}
}

func TestSetClearsExpiry(t *testing.T) {
	now := int64(1_000_000)
	ms := newMemoryStorageWithClock(func() int64 { return now })
	_ = ms.Set("k", "v")
	_ = ms.Expire("k", 5)
	_ = ms.Set("k", "v2")
	if ms.TTL("k") != -1 {
		t.Fatal("SET should clear expiry")
	}
}
