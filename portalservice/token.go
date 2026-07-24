package PortalService

import (
	"crypto/rand"
	"gw1/server/db"
)

var activeTokens = make(map[string]uint64, 0)

func generateConnectionTokenWithRandomBytes(accountId uint64, tokenBytes []byte) (token string) {
	token = db.UUIDStr(tokenBytes)
	// bear in mind client requests in byte swapped order
	activeTokens[db.UUIDStrSwapped(tokenBytes)] = accountId
	return token
}

func generateConnectionToken(accountId uint64) (token string) {
	var tokenBytes = make([]byte, 16)
	rand.Read(tokenBytes)
	return generateConnectionTokenWithRandomBytes(accountId, tokenBytes)
}

func ValidateConnectionToken(token string) (accountId uint64, ok bool) {
	accountId, ok = activeTokens[token]
	if ok {
		delete(activeTokens, token)
	}
	return
}

// TODO: GC
