package server

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"memkv/internal/wal"
)

const (
	defaultAddr    = ":9573"
	defaultWALPath = "/tmp/wal.log"
)

// Config holds server configuration.
type Config struct {
	// Addr is the listen address for net.Listen ("tcp"), e.g. ":9573" or "127.0.0.1:0".
	Addr    string
	WALPath string
	Fsync   wal.FsyncPolicy
}

// DefaultConfig returns built-in defaults (before env overlay).
func DefaultConfig() Config {
	return Config{
		Addr:    defaultAddr,
		WALPath: defaultWALPath,
		Fsync:   wal.FsyncAlways,
	}
}

// ConfigFromEnv loads Config from process environment.
//
//	CINDER_ADDR       listen address (default :9573)
//	CINDER_WAL_PATH   WAL file path (default /tmp/wal.log)
//	CINDER_FSYNC      always | everysec | no (default always)
func ConfigFromEnv() (Config, error) {
	return ConfigFromEnviron(os.Getenv)
}

// ConfigFromEnviron is like ConfigFromEnv but uses getenv for tests.
func ConfigFromEnviron(getenv func(string) string) (Config, error) {
	cfg := DefaultConfig()

	if v := strings.TrimSpace(getenv("CINDER_ADDR")); v != "" {
		cfg.Addr = v
	}
	if v := strings.TrimSpace(getenv("CINDER_WAL_PATH")); v != "" {
		cfg.WALPath = v
	}
	if v := strings.TrimSpace(getenv("CINDER_FSYNC")); v != "" {
		p, err := ParseFsync(v)
		if err != nil {
			return Config{}, err
		}
		cfg.Fsync = p
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Validate checks required fields.
func (c Config) Validate() error {
	if strings.TrimSpace(c.Addr) == "" {
		return fmt.Errorf("listen addr is empty")
	}
	if strings.TrimSpace(c.WALPath) == "" {
		return fmt.Errorf("WAL path is empty")
	}
	return nil
}

// ParseFsync maps always|everysec|no to a policy.
func ParseFsync(s string) (wal.FsyncPolicy, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "always":
		return wal.FsyncAlways, nil
	case "everysec":
		return wal.FsyncEverySec, nil
	case "no", "none":
		return wal.FsyncNo, nil
	default:
		return 0, fmt.Errorf("unknown CINDER_FSYNC %q (want always|everysec|no)", s)
	}
}

// FsyncName returns the canonical env string for a policy.
func FsyncName(p wal.FsyncPolicy) string {
	switch p {
	case wal.FsyncEverySec:
		return "everysec"
	case wal.FsyncNo:
		return "no"
	default:
		return "always"
	}
}

// PortFromAddr extracts the TCP port for logging; returns 0 if unknown.
func PortFromAddr(addr string) int {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return 0
	}
	n, err := strconv.Atoi(port)
	if err != nil {
		return 0
	}
	return n
}
