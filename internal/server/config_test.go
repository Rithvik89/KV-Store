package server

import (
	"testing"

	"memkv/internal/wal"
)

func TestConfigFromEnvironDefaults(t *testing.T) {
	cfg, err := ConfigFromEnviron(func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != defaultAddr || cfg.WALPath != defaultWALPath || cfg.Fsync != wal.FsyncAlways {
		t.Fatalf("%+v", cfg)
	}
}

func TestConfigFromEnvironOverrides(t *testing.T) {
	env := map[string]string{
		"CINDER_ADDR":     "127.0.0.1:19000",
		"CINDER_WAL_PATH": "/tmp/x.wal",
		"CINDER_FSYNC":    "everysec",
	}
	cfg, err := ConfigFromEnviron(func(k string) string { return env[k] })
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != "127.0.0.1:19000" || cfg.WALPath != "/tmp/x.wal" || cfg.Fsync != wal.FsyncEverySec {
		t.Fatalf("%+v", cfg)
	}
}

func TestParseFsync(t *testing.T) {
	cases := []struct {
		in   string
		want wal.FsyncPolicy
		ok   bool
	}{
		{"always", wal.FsyncAlways, true},
		{"everysec", wal.FsyncEverySec, true},
		{"no", wal.FsyncNo, true},
		{"none", wal.FsyncNo, true},
		{"bogus", 0, false},
	}
	for _, tc := range cases {
		got, err := ParseFsync(tc.in)
		if tc.ok && (err != nil || got != tc.want) {
			t.Fatalf("%s: got %v %v", tc.in, got, err)
		}
		if !tc.ok && err == nil {
			t.Fatalf("%s: want error", tc.in)
		}
	}
}

func TestPortFromAddr(t *testing.T) {
	if PortFromAddr(":9573") != 9573 {
		t.Fatal(PortFromAddr(":9573"))
	}
	if PortFromAddr("127.0.0.1:19000") != 19000 {
		t.Fatal()
	}
}
