package logger

import "testing"

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
