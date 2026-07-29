package gameservice

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func clearGameTracker() {
	mu.Lock()
	defer mu.Unlock()
	loggedIn = map[uint64]struct{}{}
}

func TestTrackAccount_Success(t *testing.T) {
	clearGameTracker()
	assert.True(t, TrackAccount(1))
}

func TestTrackAccount_Duplicate(t *testing.T) {
	clearGameTracker()
	TrackAccount(1)
	assert.False(t, TrackAccount(1))
}

func TestTrackAccount_MultipleAccounts(t *testing.T) {
	clearGameTracker()
	assert.True(t, TrackAccount(1))
	assert.True(t, TrackAccount(2))
	assert.False(t, TrackAccount(1))
}

func TestUntrackAccount_FreesSlot(t *testing.T) {
	clearGameTracker()
	TrackAccount(1)
	UntrackAccount(1)
	assert.True(t, TrackAccount(1))
}

func TestUntrackAccount_NonExistent(t *testing.T) {
	clearGameTracker()
	UntrackAccount(999)
}

func TestUntrackAccount_OnlyRemovesSpecified(t *testing.T) {
	clearGameTracker()
	TrackAccount(1)
	TrackAccount(2)
	UntrackAccount(1)
	assert.False(t, TrackAccount(2))
	assert.True(t, TrackAccount(1))
}
