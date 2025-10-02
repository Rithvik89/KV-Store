package storage

// MemoryStorage is an in-memory implementation of Storage
type MemoryStorage struct {
	store map[string]string
}

// NewMemoryStorage creates a new in-memory storage
func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		store: make(map[string]string),
	}
}

// Get retrieves a value by key
func (ms *MemoryStorage) Get(key string) (string, error) {

	value, ok := ms.store[key]
	if !ok {
		return "", ErrKeyNotFound
	}
	return value, nil
}

// Set stores a key-value pair
func (ms *MemoryStorage) Set(key string, value string) error {

	ms.store[key] = value
	return nil
}

// Delete removes a key-value pair
func (ms *MemoryStorage) Delete(key string) error {

	if _, ok := ms.store[key]; !ok {
		return ErrKeyNotFound
	}

	delete(ms.store, key)
	return nil
}

// Exists checks if a key exists
func (ms *MemoryStorage) Exists(key string) bool {

	_, ok := ms.store[key]
	return ok
}

// Keys returns all keys in the store
func (ms *MemoryStorage) Keys() []string {

	keys := make([]string, 0, len(ms.store))
	for k := range ms.store {
		keys = append(keys, k)
	}
	return keys
}

// Clear removes all key-value pairs
func (ms *MemoryStorage) Clear() error {

	ms.store = make(map[string]string)
	return nil
}

// Size returns the number of key-value pairs
func (ms *MemoryStorage) Size() int {
	return len(ms.store)
}
