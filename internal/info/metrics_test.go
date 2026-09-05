package info

import (
	"strings"
	"testing"
)

func TestFormatSections(t *testing.T) {
	m := New()
	m.OpsTotal.Store(3)
	m.Hits.Store(2)
	m.Misses.Store(1)
	m.Clients.Store(4)

	all := m.Format("all", Snapshot{Keys: 5, WALBytes: 100})
	for _, want := range []string{
		"# Stats",
		"ops_total:3",
		"keyspace_hits:2",
		"keyspace_misses:1",
		"# Clients",
		"connected_clients:4",
		"# Persistence",
		"wal_bytes:100",
		"# Keyspace",
		"keys:5",
	} {
		if !strings.Contains(all, want) {
			t.Fatalf("missing %q in:\n%s", want, all)
		}
	}

	clientsOnly := m.Format("clients", Snapshot{})
	if !strings.Contains(clientsOnly, "connected_clients:4") {
		t.Fatal(clientsOnly)
	}
	if strings.Contains(clientsOnly, "ops_total") {
		t.Fatal("clients section should omit stats")
	}
}

func TestNilMetricsSafe(t *testing.T) {
	var m *Metrics
	m.IncrOp("GET")
	m.IncrHit()
	m.ClientConnected()
	out := m.Format("all", Snapshot{})
	if !strings.Contains(out, "ops_total:0") {
		t.Fatal(out)
	}
}
