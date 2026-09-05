package wal

import (
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRepairTruncatesTornTailAndAllowsAppend(t *testing.T) {
	path := filepath.Join(t.TempDir(), "repair.wal")

	w, err := NewFileWAL(path, WithFsync(FsyncAlways))
	if err != nil {
		t.Fatal(err)
	}
	if err := w.WriteSet("ok", "1"); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	goodSize := before.Size()

	// Append a torn length prefix (claims 100-byte payload, no data).
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], 100)
	if _, err := f.Write(lenBuf[:]); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	torn, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if torn.Size() <= goodSize {
		t.Fatal("expected torn bytes to grow file")
	}

	// Reopen repairs (truncates tear), then a new write must be visible on replay.
	w2, err := NewFileWAL(path, WithFsync(FsyncAlways))
	if err != nil {
		t.Fatal(err)
	}
	repaired, err := w2.Size()
	if err != nil {
		t.Fatal(err)
	}
	if repaired != goodSize {
		t.Fatalf("after repair size=%d want %d", repaired, goodSize)
	}
	if err := w2.WriteSet("next", "2"); err != nil {
		t.Fatal(err)
	}
	if err := w2.Close(); err != nil {
		t.Fatal(err)
	}

	w3, err := NewFileWAL(path)
	if err != nil {
		t.Fatal(err)
	}
	defer w3.Close()

	var keys []string
	if err := w3.Replay(func(e *Entry) error {
		keys = append(keys, e.Key)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(keys) != 2 || keys[0] != "ok" || keys[1] != "next" {
		t.Fatalf("keys=%v want [ok next]", keys)
	}
}

func TestBadCRCAtEOFTreatedAsTorn(t *testing.T) {
	path := filepath.Join(t.TempDir(), "badcrc.wal")
	w, err := NewFileWAL(path, WithFsync(FsyncAlways))
	if err != nil {
		t.Fatal(err)
	}
	if err := w.WriteSet("a", "1"); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	good, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	rec, err := encodeRecord(&Entry{Op: OpSet, Key: "bogus", Value: "x"})
	if err != nil {
		t.Fatal(err)
	}
	rec[len(rec)-1] ^= 0xff
	if err := os.WriteFile(path, append(mustRead(t, path), rec...), 0644); err != nil {
		t.Fatal(err)
	}

	w2, err := NewFileWAL(path)
	if err != nil {
		t.Fatal(err)
	}
	defer w2.Close()
	sz, err := w2.Size()
	if err != nil {
		t.Fatal(err)
	}
	if sz != good.Size() {
		t.Fatalf("bad CRC at EOF should truncate to %d, got %d", good.Size(), sz)
	}
}

func TestMidFileCRCMismatchErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mid.wal")
	w, err := NewFileWAL(path, WithFsync(FsyncAlways))
	if err != nil {
		t.Fatal(err)
	}
	if err := w.WriteSet("a", "1"); err != nil {
		t.Fatal(err)
	}
	if err := w.WriteSet("b", "2"); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	raw := mustRead(t, path)
	off := headerSize
	payloadLen := int(binary.BigEndian.Uint32(raw[off : off+4]))
	crcOff := off + 4 + payloadLen
	raw[crcOff] ^= 0xff
	if err := os.WriteFile(path, raw, 0644); err != nil {
		t.Fatal(err)
	}

	_, err = NewFileWAL(path)
	if err == nil || !errors.Is(err, ErrCorrupt) {
		t.Fatalf("want ErrCorrupt, got %v", err)
	}
}

func TestAckSurvivesUncleanReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ack.wal")
	w, err := NewFileWAL(path, WithFsync(FsyncAlways))
	if err != nil {
		t.Fatal(err)
	}
	if err := w.WriteSet("k", "v"); err != nil {
		t.Fatal(err)
	}
	// Simulate hard kill after ACK: abandon without Close.
	// FsyncAlways already synced inside Write.
	w.file = nil
	w.closed = true

	w2, err := NewFileWAL(path)
	if err != nil {
		t.Fatal(err)
	}
	defer w2.Close()

	var got *Entry
	if err := w2.Replay(func(e *Entry) error {
		cp := *e
		got = &cp
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Key != "k" || got.Value != "v" {
		t.Fatalf("ACK'd write lost after unclean reopen: %+v", got)
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
