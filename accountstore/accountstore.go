package accountstore

import "sync"

type Tracker struct {
	mu    sync.Mutex
	items map[uint64]struct{}
}

func New() *Tracker {
	return &Tracker{items: make(map[uint64]struct{})}
}

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
