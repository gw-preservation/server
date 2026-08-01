package tokenstore

import (
	"sync"
	"time"
)

type timedEntry[V any] struct {
	value     V
	createdAt time.Time
}

// Store is a thread-safe map of single-use, expiring values (e.g. connection
// tokens). A background goroutine purges entries older than ttl every
// cleanupInterval.
type Store[K comparable, V any] struct {
	mu       sync.Mutex
	entries  map[K]timedEntry[V]
	ttl      time.Duration
	stop     chan struct{}
	stopOnce sync.Once
}

func New[K comparable, V any](cleanupInterval, ttl time.Duration) *Store[K, V] {
	s := &Store[K, V]{
		entries: make(map[K]timedEntry[V]),
		ttl:     ttl,
		stop:    make(chan struct{}),
	}
	go s.cleanupLoop(cleanupInterval)
	return s
}

func (s *Store[K, V]) Set(key K, value V) {
	s.mu.Lock()
	s.entries[key] = timedEntry[V]{value: value, createdAt: time.Now()}
	s.mu.Unlock()
}

// Consume atomically reads and removes the entry for key.
func (s *Store[K, V]) Consume(key K) (V, bool) {
	s.mu.Lock()
	e, ok := s.entries[key]
	if ok {
		delete(s.entries, key)
	}
	s.mu.Unlock()
	return e.value, ok
}

func (s *Store[K, V]) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.entries)
}

func (s *Store[K, V]) Clear() {
	s.mu.Lock()
	s.entries = make(map[K]timedEntry[V])
	s.mu.Unlock()
}

// Stop stops the background cleanup goroutine. Safe to call multiple times.
func (s *Store[K, V]) Stop() {
	s.stopOnce.Do(func() {
		close(s.stop)
	})
}

func (s *Store[K, V]) cleanupLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.purgeExpired()
		case <-s.stop:
			return
		}
	}
}

func (s *Store[K, V]) purgeExpired() {
	now := time.Now()
	s.mu.Lock()
	for k, e := range s.entries {
		if now.Sub(e.createdAt) > s.ttl {
			delete(s.entries, k)
		}
	}
	s.mu.Unlock()
}
