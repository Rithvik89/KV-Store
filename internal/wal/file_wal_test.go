package wal

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"os"
	"path/filepath"
	"testing"
)

func TestEncodeDecodeRoundTrip(t *testing.T) {
	cases := []*Entry{
		{Op: OpSet, Key: "a", Value: "1"},
		{Op: OpSet, Key: "name", Value: "Ada Lovelace"},
		{Op: OpSet, Key: "bin", Value: "a\x00b\n"},
		{Op: OpDelete, Key: "gone", Value: ""},
	}
	for _, want := range cases {
		rec, err := encodeRecord(want)
		if err != nil {
			t.Fatal(err)
		}
		payloadLen := binary.BigEndian.Uint32(rec[0:4])
		payload := rec[4 : 4+payloadLen]
		sum := binary.BigEndian.Uint32(rec[4+payloadLen:])
		if crc32.ChecksumIEEE(payload) != sum {
			t.Fatal("crc")
		}
		got, err := decodePayload(payload)
		if err != nil {
			t.Fatal(err)
		}
		if got.Op != want.Op || got.Key != want.Key || got.Value != want.Value {
			t.Fatalf("got %+v want %+v", got, want)
		}
	}
}

func TestFileWALPersistAndReplay(t *testing.T) {
	path := filepath.Join(t.TempDir(), "w.wal")
	w, err := NewFileWAL(path, WithFsync(FsyncAlways))
	if err != nil {
		t.Fatal(err)
	}
	if err := w.WriteSet("name", "Ada Lovelace"); err != nil {
		t.Fatal(err)
	}
	if err := w.WriteSet("x", "1"); err != nil {
		t.Fatal(err)
	}
	if err := w.WriteDelete("x"); err != nil {
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

	var entries []*Entry
	if err := w2.Replay(func(e *Entry) error {
		cp := *e
		entries = append(entries, &cp)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("entries=%d", len(entries))
	}
	if entries[0].Key != "name" || entries[0].Value != "Ada Lovelace" {
		t.Fatalf("entry0 %+v", entries[0])
	}
	if entries[2].Op != OpDelete || entries[2].Key != "x" {
		t.Fatalf("entry2 %+v", entries[2])
	}
}

func TestFileWALTornTailStops(t *testing.T) {
	path := filepath.Join(t.TempDir(), "torn.wal")
	w, err := NewFileWAL(path, WithFsync(FsyncNo))
	if err != nil {
		t.Fatal(err)
	}
	if err := w.WriteSet("ok", "1"); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	// Append a partial record (length prefix claiming 100 bytes, no payload).
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

	w2, err := NewFileWAL(path)
	if err != nil {
		t.Fatal(err)
	}
	defer w2.Close()

	var n int
	if err := w2.Replay(func(*Entry) error {
		n++
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("want 1 complete record before tear, got %d", n)
	}
}

func TestFsyncNoStillDurableOnClose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nosync.wal")
	w, err := NewFileWAL(path, WithFsync(FsyncNo))
	if err != nil {
		t.Fatal(err)
	}
	if err := w.WriteSet("k", "v"); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(raw, []byte("k")) || !bytes.Contains(raw, []byte("v")) {
		t.Fatalf("expected key/value bytes in file after Close")
	}
}
