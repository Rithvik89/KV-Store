package storage

// MemStore represents an in-memory key-value store
type MemStore struct {
	Store map[string]string
}

// NewMemStore creates a new instance of MemStore
func NewMemStore() *MemStore {
	return &MemStore{
		Store: make(map[string]string),
	}
}

// Put stores a key-value pair in the memory store
func (ms *MemStore) Put(Key string, Value string) {
	ms.Store[Key] = Value
}

// Get retrieves a value by key from the memory store
func (ms *MemStore) Get(Key string) (string, bool) {
	value, ok := ms.Store[Key]
	return value, ok
}