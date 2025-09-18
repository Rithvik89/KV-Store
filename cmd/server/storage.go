package main

type MemStore struct {
	Store map[string]string
}

func (ms *MemStore) Put(Key string, Value string) {
	ms.Store[Key] = Value
}

func (ms *MemStore) Get(Key string) (string, bool) {
	value, ok := ms.Store[Key]
	return value, ok
}

func (ms *MemStore) Delete(Key string) bool {
	_, ok := ms.Store[Key]
	if ok {
		delete(ms.Store, Key)
		return true
	}
	return false
}
