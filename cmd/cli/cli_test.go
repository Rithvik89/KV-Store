package main

import (
	"testing"

	"memkv/internal/proto"
)

func TestFormatReply(t *testing.T) {
	cases := []struct {
		v    proto.Value
		raw  bool
		want string
	}{
		{proto.Simple("PONG"), false, "PONG"},
		{proto.ErrorMsg("ERR nope"), false, "ERR nope"},
		{proto.Integer(1), false, "1"},
		{proto.Bulk("hi"), false, "hi"},
		{proto.Null(), false, "(nil)"},
		{proto.Array(proto.Bulk("a"), proto.Bulk("b")), false, "a\nb"},
		{proto.Simple("OK"), true, "+OK\r\n"},
	}
	for _, tc := range cases {
		got := formatReply(tc.v, tc.raw)
		if got != tc.want {
			t.Fatalf("got %q want %q", got, tc.want)
		}
	}
}
