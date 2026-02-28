package store

import (
	"sync"
	"sync/atomic"
	"time"
)

// TypedStore is a generic, concurrency-safe, in-memory key-value store.
// Each TypedStore has its own RWMutex, giving per-type locking granularity.
// It tracks when data was last modified for staleness detection.
type TypedStore[T any] struct {
	mu          sync.RWMutex
	items       map[string]T
	lastUpdated atomic.Int64 // UnixMilli timestamp of last Set/Delete
}

// NewTypedStore creates a new, empty TypedStore.
func NewTypedStore[T any]() *TypedStore[T] {
	s := &TypedStore[T]{
		items: make(map[string]T),
	}
	s.lastUpdated.Store(time.Now().UnixMilli())
	return s
}

// Set inserts or updates a value for the given key.
func (s *TypedStore[T]) Set(key string, value T) {
	s.mu.Lock()
	s.items[key] = value
	s.mu.Unlock()
	s.lastUpdated.Store(time.Now().UnixMilli())
}

// Delete removes a key from the store. No-op if the key doesn't exist.
func (s *TypedStore[T]) Delete(key string) {
	s.mu.Lock()
	delete(s.items, key)
	s.mu.Unlock()
	s.lastUpdated.Store(time.Now().UnixMilli())
}

// LastUpdated returns the UnixMilli timestamp of the last modification.
func (s *TypedStore[T]) LastUpdated() int64 {
	return s.lastUpdated.Load()
}

// Get retrieves a value by key. Returns the value and true if found,
// or the zero value and false if not.
func (s *TypedStore[T]) Get(key string) (T, bool) {
	s.mu.RLock()
	v, ok := s.items[key]
	s.mu.RUnlock()
	return v, ok
}

// Len returns the number of items in the store.
func (s *TypedStore[T]) Len() int {
	s.mu.RLock()
	n := len(s.items)
	s.mu.RUnlock()
	return n
}

// Snapshot returns a shallow copy of all items. Mutations to the returned
// map do not affect the store.
func (s *TypedStore[T]) Snapshot() map[string]T {
	s.mu.RLock()
	cp := make(map[string]T, len(s.items))
	for k, v := range s.items {
		cp[k] = v
	}
	s.mu.RUnlock()
	return cp
}

// Values returns all values as a slice. Order is not guaranteed.
func (s *TypedStore[T]) Values() []T {
	s.mu.RLock()
	vals := make([]T, 0, len(s.items))
	for _, v := range s.items {
		vals = append(vals, v)
	}
	s.mu.RUnlock()
	return vals
}

// Clear removes all items from the store.
func (s *TypedStore[T]) Clear() {
	s.mu.Lock()
	s.items = make(map[string]T)
	s.mu.Unlock()
}
