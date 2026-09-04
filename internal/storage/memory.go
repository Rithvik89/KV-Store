package storage

import "time"

// entry is one keyspace value with an optional deadline.
type entry struct {
	value  string
	expire int64 // unix ms; 0 = no expiry
}

// MemoryStorage is the single in-memory dict. PersistentStorage wraps this
// plus a WAL — it does not own a second map.
type MemoryStorage struct {
	store map[string]entry
	now   func() int64 // unix ms; injectable for tests
}

// NewMemoryStorage creates a new in-memory storage.
func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		store: make(map[string]entry),
		now:   func() int64 { return time.Now().UnixMilli() },
	}
}

func newMemoryStorageWithClock(now func() int64) *MemoryStorage {
	return &MemoryStorage{
		store: make(map[string]entry),
		now:   now,
	}
}

// expireIfNeeded removes key if it is past its deadline.
//
// Lazy expiry only: a key past its deadline is removed when accessed.
// Expired keys that are never read/listed may linger in the dict until then.
// Active sampling (random expire sweeps) is intentionally not implemented yet.
func (ms *MemoryStorage) expireIfNeeded(key string) {
	e, ok := ms.store[key]
	if !ok || e.expire == 0 {
		return
	}
	if ms.now() >= e.expire {
		delete(ms.store, key)
	}
}

// Get retrieves a value by key (lazy-expires first).
func (ms *MemoryStorage) Get(key string) (string, error) {
	ms.expireIfNeeded(key)
	e, ok := ms.store[key]
	if !ok {
		return "", ErrKeyNotFound
	}
	return e.value, nil
}

// Set stores a key-value pair and clears any previous expiry.
func (ms *MemoryStorage) Set(key string, value string) error {
	ms.store[key] = entry{value: value, expire: 0}
	return nil
}

// Delete removes a key-value pair (lazy-expires first).
func (ms *MemoryStorage) Delete(key string) error {
	ms.expireIfNeeded(key)
	if _, ok := ms.store[key]; !ok {
		return ErrKeyNotFound
	}
	delete(ms.store, key)
	return nil
}

// Exists checks if a key exists (lazy-expires first).
func (ms *MemoryStorage) Exists(key string) bool {
	ms.expireIfNeeded(key)
	_, ok := ms.store[key]
	return ok
}

// Keys returns live keys; expired entries are purged as they are visited.
func (ms *MemoryStorage) Keys() []string {
	keys := make([]string, 0, len(ms.store))
	for k := range ms.store {
		ms.expireIfNeeded(k)
		if _, ok := ms.store[k]; ok {
			keys = append(keys, k)
		}
	}
	return keys
}

// Clear removes all key-value pairs.
func (ms *MemoryStorage) Clear() error {
	ms.store = make(map[string]entry)
	return nil
}

// Size returns the number of live keys; expired entries are purged on scan.
func (ms *MemoryStorage) Size() int {
	n := 0
	for k := range ms.store {
		ms.expireIfNeeded(k)
		if _, ok := ms.store[k]; ok {
			n++
		}
	}
	return n
}

// lenRaw is the map length without lazy expiry (documents linger behaviour).
func (ms *MemoryStorage) lenRaw() int {
	return len(ms.store)
}

// Close implements Storage. MemoryStorage has nothing to release.
func (ms *MemoryStorage) Close() error {
	return nil
}

// Expire sets a deadline of now+seconds. Returns 1 if the key exists, else 0.
func (ms *MemoryStorage) Expire(key string, seconds int64) int {
	ms.expireIfNeeded(key)
	e, ok := ms.store[key]
	if !ok {
		return 0
	}
	if seconds <= 0 {
		delete(ms.store, key)
		return 1
	}
	e.expire = ms.now() + seconds*1000
	ms.store[key] = e
	return 1
}

// TTL returns seconds remaining, -1 if no expiry, -2 if missing.
func (ms *MemoryStorage) TTL(key string) int64 {
	ms.expireIfNeeded(key)
	e, ok := ms.store[key]
	if !ok {
		return -2
	}
	if e.expire == 0 {
		return -1
	}
	remaining := e.expire - ms.now()
	if remaining <= 0 {
		// Race with the clock; treat as gone.
		delete(ms.store, key)
		return -2
	}
	// Ceil to whole seconds (Redis-shaped).
	return (remaining + 999) / 1000
}

// Persist clears expiry. Returns 1 if a timeout was removed, else 0.
func (ms *MemoryStorage) Persist(key string) int {
	ms.expireIfNeeded(key)
	e, ok := ms.store[key]
	if !ok || e.expire == 0 {
		return 0
	}
	e.expire = 0
	ms.store[key] = e
	return 1
}
