package info

import (
	"fmt"
	"strings"
	"sync/atomic"
)

// Metrics holds process-level counters for INFO and benches.
// All fields are safe for concurrent use.
type Metrics struct {
	OpsTotal atomic.Uint64
	OpsGet   atomic.Uint64
	OpsSet   atomic.Uint64
	OpsDel   atomic.Uint64

	Hits    atomic.Uint64
	Misses  atomic.Uint64
	Expired atomic.Uint64

	Clients atomic.Int64
}

// New returns zeroed metrics.
func New() *Metrics {
	return &Metrics{}
}

// Snapshot is live state read at INFO time (not accumulated counters).
type Snapshot struct {
	Keys     int
	WALBytes int64
}

// IncrOp records one executed command by verb name (upper-case).
func (m *Metrics) IncrOp(verb string) {
	if m == nil {
		return
	}
	m.OpsTotal.Add(1)
	switch verb {
	case "GET":
		m.OpsGet.Add(1)
	case "SET":
		m.OpsSet.Add(1)
	case "DEL", "DELETE":
		m.OpsDel.Add(1)
	}
}

func (m *Metrics) IncrHit() {
	if m != nil {
		m.Hits.Add(1)
	}
}

func (m *Metrics) IncrMiss() {
	if m != nil {
		m.Misses.Add(1)
	}
}

func (m *Metrics) IncrExpired() {
	if m != nil {
		m.Expired.Add(1)
	}
}

func (m *Metrics) ClientConnected() {
	if m != nil {
		m.Clients.Add(1)
	}
}

func (m *Metrics) ClientDisconnected() {
	if m != nil {
		m.Clients.Add(-1)
	}
}

// Format builds a Redis-ish INFO bulk string.
// section is empty/"default"/"all" for everything; otherwise a section name.
func (m *Metrics) Format(section string, snap Snapshot) string {
	if m == nil {
		m = &Metrics{}
	}
	section = strings.ToLower(strings.TrimSpace(section))
	if section == "" || section == "default" {
		section = "all"
	}

	var b strings.Builder
	write := func(name, title string, body func()) {
		if section != "all" && section != name {
			return
		}
		b.WriteString("# ")
		b.WriteString(title)
		b.WriteString("\r\n")
		body()
		b.WriteString("\r\n")
	}

	write("stats", "Stats", func() {
		fmt.Fprintf(&b, "ops_total:%d\r\n", m.OpsTotal.Load())
		fmt.Fprintf(&b, "ops_get:%d\r\n", m.OpsGet.Load())
		fmt.Fprintf(&b, "ops_set:%d\r\n", m.OpsSet.Load())
		fmt.Fprintf(&b, "ops_del:%d\r\n", m.OpsDel.Load())
		fmt.Fprintf(&b, "keyspace_hits:%d\r\n", m.Hits.Load())
		fmt.Fprintf(&b, "keyspace_misses:%d\r\n", m.Misses.Load())
		fmt.Fprintf(&b, "expired_keys:%d\r\n", m.Expired.Load())
	})
	write("clients", "Clients", func() {
		c := m.Clients.Load()
		if c < 0 {
			c = 0
		}
		fmt.Fprintf(&b, "connected_clients:%d\r\n", c)
	})
	write("persistence", "Persistence", func() {
		fmt.Fprintf(&b, "wal_bytes:%d\r\n", snap.WALBytes)
	})
	write("keyspace", "Keyspace", func() {
		fmt.Fprintf(&b, "keys:%d\r\n", snap.Keys)
	})

	return b.String()
}
