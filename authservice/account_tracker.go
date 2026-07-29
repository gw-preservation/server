package authservice

import "sync"

var (
	mu       sync.RWMutex
	loggedIn = map[uint64]struct{}{}
)

func TrackAccount(accountID uint64) bool {
	mu.Lock()
	defer mu.Unlock()
	if _, ok := loggedIn[accountID]; ok {
		return false
	}
	loggedIn[accountID] = struct{}{}
	return true
}

func UntrackAccount(accountID uint64) {
	mu.Lock()
	defer mu.Unlock()
	delete(loggedIn, accountID)
}
