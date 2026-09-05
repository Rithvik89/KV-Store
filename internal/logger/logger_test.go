package logger

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseLevel(t *testing.T) {
	cases := []struct {
		in   string
		want LogLevel
		ok   bool
	}{
		{"debug", DEBUG, true},
		{"INFO", INFO, true},
		{"", INFO, true},
		{"warn", WARN, true},
		{"error", ERROR, true},
		{"nope", INFO, false},
	}
	for _, tc := range cases {
		got, err := ParseLevel(tc.in)
		if tc.ok && (err != nil || got != tc.want) {
			t.Fatalf("%q: got %v %v", tc.in, got, err)
		}
		if !tc.ok && err == nil {
			t.Fatalf("%q: want error", tc.in)
		}
	}
}

func TestDiscardSilentAndNoExit(t *testing.T) {
	l := Discard()
	l.Info("should not appear")
	l.Fatal("must not exit process")
}

func TestSetDefaultDiscard(t *testing.T) {
	prev := Default()
	t.Cleanup(func() { SetDefault(prev) })

	var buf bytes.Buffer
	SetDefault(newLogger("test", INFO, &buf, func(int) {}))
	Info("hello")
	if !strings.Contains(buf.String(), "hello") {
		t.Fatalf("%q", buf.String())
	}

	SetDefault(Discard())
	buf.Reset()
	Info("silent")
	if buf.Len() != 0 {
		t.Fatalf("expected silence, got %q", buf.String())
	}
}
