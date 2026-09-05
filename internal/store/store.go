package store

import (
	"fmt"
	"time"

	"memkv/internal/wal"
)

// entry is one keyspace value with an optional deadline.
type entry struct {
	value  string
	expire int64 // unix ms; 0 = no expiry
}

// Store is the keyspace: one in-memory dict, optional WAL for durability.
// When wal is nil (OpenMemory), mutations stay in RAM only.
type Store struct {
	data map[string]entry
	wal  wal.WAL
	now  func() int64
}

// OpenMemory returns a Store with no WAL (tests, ephemeral use).
func OpenMemory() *Store {
	return &Store{
		data: make(map[string]entry),
		now:  func() int64 { return time.Now().UnixMilli() },
	}
}

func openMemoryWithClock(now func() int64) *Store {
	return &Store{
		data: make(map[string]entry),
		now:  now,
	}
}

// Open opens a durable Store at walPath (default fsync: always).
func Open(walPath string, opts ...wal.Option) (*Store, error) {
	w, err := wal.NewFileWAL(walPath, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create WAL: %w", err)
	}
	return OpenWithWAL(w)
}

// OpenWithWAL builds a Store around an existing WAL and replays it.
func OpenWithWAL(w wal.WAL) (*Store, error) {
	s := &Store{
		data: make(map[string]entry),
		wal:  w,
		now:  func() int64 { return time.Now().UnixMilli() },
	}
	if err := s.recover(); err != nil {
		_ = w.Close()
		return nil, fmt.Errorf("failed to recover from WAL: %w", err)
	}
	return s, nil
}

func (s *Store) recover() error {
	return s.wal.Replay(func(e *wal.Entry) error {
		switch e.Op {
		case wal.OpSet:
			s.put(e.Key, e.Value)
			return nil
		case wal.OpDelete:
			delete(s.data, e.Key)
			return nil
		default:
			return fmt.Errorf("unknown operation: %s", e.Op)
		}
	})
}

func (s *Store) put(key, value string) {
	s.data[key] = entry{value: value, expire: 0}
}

// expireIfNeeded removes key if it is past its deadline.
//
// Lazy expiry only: a key past its deadline is removed when accessed.
// Expired keys that are never read/listed may linger in the dict until then.
// Active sampling (random expire sweeps) is intentionally not implemented yet.
func (s *Store) expireIfNeeded(key string) {
	e, ok := s.data[key]
	if !ok || e.expire == 0 {
		return
	}
	if s.now() >= e.expire {
		delete(s.data, key)
	}
}

// Get retrieves a value by key (lazy-expires first).
func (s *Store) Get(key string) (string, error) {
	s.expireIfNeeded(key)
	e, ok := s.data[key]
	if !ok {
		return "", ErrKeyNotFound
	}
	return e.value, nil
}

// Set stores a key-value pair (WAL first when durable) and clears expiry.
func (s *Store) Set(key string, value string) error {
	if s.wal != nil {
		if err := s.wal.WriteSet(key, value); err != nil {
			return fmt.Errorf("WAL write failed: %w", err)
		}
	}
	s.put(key, value)
	return nil
}

// Delete removes a key (WAL first when durable).
func (s *Store) Delete(key string) error {
	s.expireIfNeeded(key)
	if _, ok := s.data[key]; !ok {
		return ErrKeyNotFound
	}
	if s.wal != nil {
		if err := s.wal.WriteDelete(key); err != nil {
			return fmt.Errorf("WAL write failed: %w", err)
		}
	}
	delete(s.data, key)
	return nil
}

// Exists checks if a key exists (lazy-expires first).
func (s *Store) Exists(key string) bool {
	s.expireIfNeeded(key)
	_, ok := s.data[key]
	return ok
}

// Keys returns live keys; expired entries are purged as they are visited.
func (s *Store) Keys() []string {
	keys := make([]string, 0, len(s.data))
	for k := range s.data {
		s.expireIfNeeded(k)
		if _, ok := s.data[k]; ok {
			keys = append(keys, k)
		}
	}
	return keys
}

// Size returns the number of live keys; expired entries are purged on scan.
func (s *Store) Size() int {
	n := 0
	for k := range s.data {
		s.expireIfNeeded(k)
		if _, ok := s.data[k]; ok {
			n++
		}
	}
	return n
}

func (s *Store) lenRaw() int {
	return len(s.data)
}

// Close closes the WAL if present.
func (s *Store) Close() error {
	if s.wal == nil {
		return nil
	}
	return s.wal.Close()
}

// Expire sets a deadline of now+seconds. Returns 1 if the key exists, else 0.
// Memory-only (not WAL'd in this sitting).
func (s *Store) Expire(key string, seconds int64) int {
	s.expireIfNeeded(key)
	e, ok := s.data[key]
	if !ok {
		return 0
	}
	if seconds <= 0 {
		delete(s.data, key)
		return 1
	}
	e.expire = s.now() + seconds*1000
	s.data[key] = e
	return 1
}

// TTL returns seconds remaining, -1 if no expiry, -2 if missing.
func (s *Store) TTL(key string) int64 {
	s.expireIfNeeded(key)
	e, ok := s.data[key]
	if !ok {
		return -2
	}
	if e.expire == 0 {
		return -1
	}
	remaining := e.expire - s.now()
	if remaining <= 0 {
		delete(s.data, key)
		return -2
	}
	return (remaining + 999) / 1000
}

// Persist clears expiry. Returns 1 if a timeout was removed, else 0.
func (s *Store) Persist(key string) int {
	s.expireIfNeeded(key)
	e, ok := s.data[key]
	if !ok || e.expire == 0 {
		return 0
	}
	e.expire = 0
	s.data[key] = e
	return 1
}

// Compact rewrites the WAL to contain only live keys (maintenance path).
// Skips expired entries. Not wired to a CSP command — can stall if called
// on the request path with a large keyspace.
func (s *Store) Compact() error {
	if s.wal == nil {
		return nil
	}
	entries := make([]wal.Entry, 0, len(s.data))
	for k := range s.data {
		s.expireIfNeeded(k)
		e, ok := s.data[k]
		if !ok {
			continue
		}
		entries = append(entries, wal.Entry{Op: wal.OpSet, Key: k, Value: e.value})
	}
	if err := s.wal.Rewrite(entries); err != nil {
		return fmt.Errorf("WAL compact failed: %w", err)
	}
	return nil
}
