package storage

import (
	"path/filepath"
	"testing"
)

func TestPersistentWrapsOneDict(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.wal")

	ps, err := NewPersistentStorage(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := ps.Set("a", "1"); err != nil {
		t.Fatal(err)
	}
	if ps.Expire("a", 60) != 1 {
		t.Fatal("Expire")
	}
	if ps.TTL("a") <= 0 {
		t.Fatal("TTL")
	}
	if err := ps.Close(); err != nil {
		t.Fatal(err)
	}

	ps2, err := NewPersistentStorage(path)
	if err != nil {
		t.Fatal(err)
	}
	defer ps2.Close()
	v, err := ps2.Get("a")
	if err != nil || v != "1" {
		t.Fatalf("recover Get: %q %v", v, err)
	}
	// TTL was memory-only — not restored from WAL.
	if ps2.TTL("a") != -1 {
		t.Fatalf("TTL after recover want -1, got %d", ps2.TTL("a"))
	}
}
