package gameservice

import "gw1/server/accountstore"

var accountTracker = accountstore.New()

func TrackAccount(accountID uint64) bool {
	return accountTracker.Track(accountID)
}

func UntrackAccount(accountID uint64) {
	accountTracker.Untrack(accountID)
}
