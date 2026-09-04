package command

import (
	"testing"

	"memkv/internal/proto"
	"memkv/internal/storage"
)

func newTestExecutor(t *testing.T) *Executor {
	t.Helper()
	return NewWithStorage(storage.NewMemoryStorage())
}

func TestExecPing(t *testing.T) {
	e := newTestExecutor(t)
	got := e.Exec([]string{"PING"})
	want := proto.EncodeValue(proto.Simple("PONG"))
	if string(proto.EncodeValue(got)) != string(want) {
		t.Fatalf("got %+v", got)
	}
	got = e.Exec([]string{"ping", "hi"})
	if got.Kind != proto.KindBulk || got.Str != "hi" {
		t.Fatalf("got %+v", got)
	}
}

func TestExecSetGetDel(t *testing.T) {
	e := newTestExecutor(t)
	if v := e.Exec([]string{"SET", "k", "v"}); v.Kind != proto.KindSimple || v.Str != "OK" {
		t.Fatalf("SET %+v", v)
	}
	if v := e.Exec([]string{"GET", "k"}); v.Kind != proto.KindBulk || v.Str != "v" {
		t.Fatalf("GET %+v", v)
	}
	if v := e.Exec([]string{"GET", "missing"}); v.Kind != proto.KindNull {
		t.Fatalf("GET miss %+v", v)
	}
	if v := e.Exec([]string{"DEL", "k"}); v.Kind != proto.KindInteger || v.Int != 1 {
		t.Fatalf("DEL %+v", v)
	}
	if v := e.Exec([]string{"DELETE", "k"}); v.Kind != proto.KindInteger || v.Int != 0 {
		t.Fatalf("DELETE miss %+v", v)
	}
}

func TestExecUnknownCommand(t *testing.T) {
	e := newTestExecutor(t)
	v := e.Exec([]string{"NOPE"})
	if v.Kind != proto.KindError || v.Str != "ERR unknown command 'NOPE'" {
		t.Fatalf("got %+v", v)
	}
}

func TestExecWrongArity(t *testing.T) {
	e := newTestExecutor(t)
	cases := [][]string{
		{"GET"},
		{"GET", "a", "b"},
		{"ECHO"},
		{"PING", "a", "b"},
		{"SET", "onlykey"},
		{"KEYS", "pattern"},
	}
	for _, args := range cases {
		v := e.Exec(args)
		if v.Kind != proto.KindError {
			t.Fatalf("args %v: want error, got %+v", args, v)
		}
	}
}

func TestDispatchTableHasBuiltins(t *testing.T) {
	e := newTestExecutor(t)
	for _, name := range []string{"PING", "ECHO", "SET", "GET", "DEL", "DELETE", "EXISTS", "KEYS"} {
		if _, ok := e.commands[name]; !ok {
			t.Fatalf("missing command %s", name)
		}
	}
}
