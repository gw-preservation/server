package authservice

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func clearTracker() {
	mu.Lock()
	defer mu.Unlock()
	loggedIn = map[uint64]struct{}{}
}

func TestTrackAccount_Success(t *testing.T) {
	clearTracker()
	assert.True(t, TrackAccount(1))
}

func TestTrackAccount_Duplicate(t *testing.T) {
	clearTracker()
	TrackAccount(1)
	assert.False(t, TrackAccount(1))
}

func TestTrackAccount_MultipleAccounts(t *testing.T) {
	clearTracker()
	assert.True(t, TrackAccount(1))
	assert.True(t, TrackAccount(2))
	assert.False(t, TrackAccount(1))
}

func TestUntrackAccount_FreesSlot(t *testing.T) {
	clearTracker()
	TrackAccount(1)
	UntrackAccount(1)
	assert.True(t, TrackAccount(1))
}

func TestUntrackAccount_NonExistent(t *testing.T) {
	clearTracker()
	UntrackAccount(999)
}

func TestUntrackAccount_OnlyRemovesSpecified(t *testing.T) {
	clearTracker()
	TrackAccount(1)
	TrackAccount(2)
	UntrackAccount(1)
	assert.False(t, TrackAccount(2))
	assert.True(t, TrackAccount(1))
}
