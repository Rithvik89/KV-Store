package storage

import "errors"

var (
	ErrKeyNotFound = errors.New("key not found")
	ErrKeyExists   = errors.New("key already exists")
)

// Storage defines the interface for key-value storage operations
type Storage interface {
	Get(key string) (string, error)
	Set(key string, value string) error
	Delete(key string) error
	Exists(key string) bool
	Keys() []string
	Size() int
	Close() error
}
