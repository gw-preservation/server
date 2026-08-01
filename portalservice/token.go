package portalservice

import (
	"crypto/rand"
	"gw1/server/db"
	"gw1/server/tokenstore"
	"time"
)

type ConnectionInfo struct {
	AccountID   uint64
	ClientIP    string
	AccountUUID [16]byte
}

const (
	tokenTTL             = 5 * time.Minute
	tokenCleanupInterval = 1 * time.Minute
)

var activeTokens = tokenstore.New[string, ConnectionInfo](tokenCleanupInterval, tokenTTL)

func generateConnectionTokenWithRandomBytes(accountId uint64, tokenBytes []byte, clientIP string, accountUUID []byte) (token string) {
	token = db.UUIDStr(tokenBytes)
	var accUUID [16]byte
	copy(accUUID[:], accountUUID)
	activeTokens.Set(token, ConnectionInfo{AccountID: accountId, ClientIP: clientIP, AccountUUID: accUUID})
	return token
}

func generateConnectionToken(accountId uint64, clientIP string, accountUUID []byte) (token string) {
	var tokenBytes = make([]byte, 16)
	rand.Read(tokenBytes)
	return generateConnectionTokenWithRandomBytes(accountId, tokenBytes, clientIP, accountUUID)
}

// GenerateConnectionTokenForTest mints a connection token from caller-supplied
// token bytes, so a test can reproduce the same bytes in a GetAccountInfo
// packet. Used by authservice tests to drive the full login handshake.
func GenerateConnectionTokenForTest(accountId uint64, tokenBytes []byte, clientIP string, accountUUID []byte) string {
	return generateConnectionTokenWithRandomBytes(accountId, tokenBytes, clientIP, accountUUID)
}

func ValidateConnectionToken(token string) (info ConnectionInfo, ok bool) {
	return activeTokens.Consume(token)
}

func StopCleanup() {
	activeTokens.Stop()
}
