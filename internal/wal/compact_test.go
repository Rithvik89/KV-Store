package wal

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRewriteDropsHistory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "c.wal")
	w, err := NewFileWAL(path, WithFsync(FsyncAlways))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range []struct{ k, v string }{
		{"a", "1"},
		{"b", "2"},
		{"a", "9"},
	} {
		if err := w.WriteSet(e.k, e.v); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.WriteDelete("b"); err != nil {
		t.Fatal(err)
	}
	if err := w.WriteSet("c", "3"); err != nil {
		t.Fatal(err)
	}

	before, err := w.Size()
	if err != nil {
		t.Fatal(err)
	}

	live := []Entry{
		{Op: OpSet, Key: "a", Value: "9"},
		{Op: OpSet, Key: "c", Value: "3"},
	}
	if err := w.Rewrite(live); err != nil {
		t.Fatal(err)
	}
	after, err := w.Size()
	if err != nil {
		t.Fatal(err)
	}
	if after >= before {
		t.Fatalf("compact should shrink log: before=%d after=%d", before, after)
	}

	// Further appends still work on the reopened fd.
	if err := w.WriteSet("d", "4"); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	w2, err := NewFileWAL(path)
	if err != nil {
		t.Fatal(err)
	}
	defer w2.Close()

	got := map[string]string{}
	if err := w2.Replay(func(e *Entry) error {
		switch e.Op {
		case OpSet:
			got[e.Key] = e.Value
		case OpDelete:
			delete(got, e.Key)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got["a"] != "9" || got["c"] != "3" || got["d"] != "4" {
		t.Fatalf("replay after compact: %v", got)
	}
}

func TestLeftoverCompactTempRemovedOnOpen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "w.wal")
	w, err := NewFileWAL(path, WithFsync(FsyncAlways))
	if err != nil {
		t.Fatal(err)
	}
	if err := w.WriteSet("k", "v"); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	tmp := compactTmpPath(path)
	if err := os.WriteFile(tmp, []byte("garbage"), 0644); err != nil {
		t.Fatal(err)
	}

	w2, err := NewFileWAL(path)
	if err != nil {
		t.Fatal(err)
	}
	defer w2.Close()
	if _, err := os.Stat(tmp); !os.IsNotExist(err) {
		t.Fatalf("expected temp removed, stat err=%v", err)
	}
}

func TestRewriteRejectsDeleteEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.wal")
	w, err := NewFileWAL(path)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	err = w.Rewrite([]Entry{{Op: OpDelete, Key: "x"}})
	if err == nil {
		t.Fatal("expected error")
	}
}
