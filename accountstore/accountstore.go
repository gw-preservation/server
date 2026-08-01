package accountstore

import "sync"

// Tracker prevents concurrent logins per account ID. Each service keeps its own
// Tracker so a login in one service doesn't block another.
type Tracker struct {
	mu    sync.Mutex
	items map[uint64]struct{}
}

func New() *Tracker {
	return &Tracker{items: make(map[uint64]struct{})}
}

// Track marks id as logged in. Returns false if it was already tracked.
func (t *Tracker) Track(id uint64) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, ok := t.items[id]; ok {
		return false
	}
	t.items[id] = struct{}{}
	return true
}

func (t *Tracker) Untrack(id uint64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.items, id)
}

func (t *Tracker) Reset() {
	t.mu.Lock()
	t.items = make(map[uint64]struct{})
	t.mu.Unlock()
}
