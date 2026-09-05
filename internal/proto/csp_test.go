package proto

import (
	"bytes"
	"testing"
)

func TestEncodeDecodeRoundTrip(t *testing.T) {
	cases := []Value{
		Simple("PONG"),
		ErrorMsg("ERR unknown command"),
		Integer(42),
		Bulk("hello"),
		Bulk(""),
		Null(),
		Array(Bulk("GET"), Bulk("key")),
		Array(Bulk("SET"), Bulk("k"), Bulk("v with spaces")),
	}
	for _, want := range cases {
		raw := EncodeValue(want)
		got, rest, ok, err := Decode(raw)
		if err != nil {
			t.Fatalf("%+v: decode error: %v", want, err)
		}
		if !ok {
			t.Fatalf("%+v: incomplete", want)
		}
		if len(rest) != 0 {
			t.Fatalf("%+v: leftover %q", want, rest)
		}
		if !valuesEqual(got, want) {
			t.Fatalf("got %+v want %+v (raw=%q)", got, want, raw)
		}
	}
}

func TestDecodePartialBuffer(t *testing.T) {
	full := EncodeValue(Array(Bulk("PING")))
	for i := 1; i < len(full); i++ {
		_, _, ok, err := Decode(full[:i])
		if err != nil {
			t.Fatalf("split at %d: unexpected err %v", i, err)
		}
		if ok {
			t.Fatalf("split at %d: expected incomplete", i)
		}
	}
	v, rest, ok, err := Decode(full)
	if err != nil || !ok || len(rest) != 0 {
		t.Fatalf("full decode failed: v=%+v rest=%q ok=%v err=%v", v, rest, ok, err)
	}
	if v.Kind != KindArray || len(v.Arr) != 1 || v.Arr[0].Str != "PING" {
		t.Fatalf("unexpected value %+v", v)
	}
}

func TestDecodeTwoValuesBackToBack(t *testing.T) {
	var buf []byte
	buf = Encode(buf, Array(Bulk("PING")))
	buf = Encode(buf, Array(Bulk("GET"), Bulk("a")))

	v1, rest, ok, err := Decode(buf)
	if err != nil || !ok {
		t.Fatalf("first: ok=%v err=%v", ok, err)
	}
	args, err := CommandArgs(v1)
	if err != nil || len(args) != 1 || args[0] != "PING" {
		t.Fatalf("first args=%v err=%v", args, err)
	}

	v2, rest2, ok, err := Decode(rest)
	if err != nil || !ok || len(rest2) != 0 {
		t.Fatalf("second: ok=%v rest=%q err=%v", ok, rest2, err)
	}
	args, err = CommandArgs(v2)
	if err != nil || len(args) != 2 || args[0] != "GET" || args[1] != "a" {
		t.Fatalf("second args=%v err=%v", args, err)
	}
}

func TestDecodeIncrementalAppend(t *testing.T) {
	full := EncodeValue(Array(Bulk("ECHO"), Bulk("hi")))
	var buf []byte
	for i := 0; i < len(full); i++ {
		buf = append(buf, full[i])
		v, rest, ok, err := Decode(buf)
		if err != nil {
			t.Fatalf("at %d: %v", i, err)
		}
		if i < len(full)-1 {
			if ok {
				t.Fatalf("at %d: premature complete", i)
			}
			continue
		}
		if !ok || len(rest) != 0 {
			t.Fatalf("final: ok=%v rest=%q", ok, rest)
		}
		args, err := CommandArgs(v)
		if err != nil || len(args) != 2 || args[1] != "hi" {
			t.Fatalf("args=%v err=%v", args, err)
		}
	}
}

func TestNestedArray(t *testing.T) {
	want := Array(Array(Bulk("a"), Bulk("b")), Integer(1))
	raw := EncodeValue(want)
	got, rest, ok, err := Decode(raw)
	if err != nil || !ok || len(rest) != 0 {
		t.Fatalf("ok=%v rest=%q err=%v", ok, rest, err)
	}
	if !valuesEqual(got, want) {
		t.Fatalf("got %+v want %+v", got, want)
	}
}

func TestDecodeCorruptTypeByte(t *testing.T) {
	_, _, _, err := Decode([]byte("?\r\n"))
	if err == nil {
		t.Fatal("expected protocol error")
	}
}

func TestNullBulk(t *testing.T) {
	raw := EncodeValue(Null())
	if string(raw) != "$-1\r\n" {
		t.Fatalf("null encode %q", raw)
	}
	v, _, ok, err := Decode(raw)
	if err != nil || !ok || v.Kind != KindNull {
		t.Fatalf("null decode %+v ok=%v err=%v", v, ok, err)
	}
}

func TestCommandArgs(t *testing.T) {
	v := Array(Bulk("SET"), Bulk("k"), Bulk("v"))
	args, err := CommandArgs(v)
	if err != nil {
		t.Fatal(err)
	}
	if got := bytes.Join(stringSliceToBytes(args), []byte{','}); string(got) != "SET,k,v" {
		t.Fatalf("args=%v", args)
	}
	if _, err := CommandArgs(Simple("PING")); err == nil {
		t.Fatal("expected error for non-array")
	}
}

func stringSliceToBytes(ss []string) [][]byte {
	out := make([][]byte, len(ss))
	for i, s := range ss {
		out[i] = []byte(s)
	}
	return out
}

func valuesEqual(a, b Value) bool {
	if a.Kind != b.Kind || a.Str != b.Str || a.Int != b.Int {
		return false
	}
	if len(a.Arr) != len(b.Arr) {
		return false
	}
	for i := range a.Arr {
		if !valuesEqual(a.Arr[i], b.Arr[i]) {
			return false
		}
	}
	return true
}
