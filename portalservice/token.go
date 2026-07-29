package portalservice

import (
	"crypto/rand"
	"gw1/server/db"
	"time"
)

type portalToken struct {
	accountId uint64
	createdAt time.Time
}

var activeTokens = make(map[string]portalToken)

func generateConnectionTokenWithRandomBytes(accountId uint64, tokenBytes []byte) (token string) {
	token = db.UUIDStr(tokenBytes)
	activeTokens[db.UUIDStrSwapped(tokenBytes)] = portalToken{
		accountId: accountId,
		createdAt: time.Now(),
	}
	return token
}

func generateConnectionToken(accountId uint64) (token string) {
	var tokenBytes = make([]byte, 16)
	rand.Read(tokenBytes)
	return generateConnectionTokenWithRandomBytes(accountId, tokenBytes)
}

func ValidateConnectionToken(token string) (accountId uint64, ok bool) {
	entry, ok := activeTokens[token]
	if ok {
		delete(activeTokens, token)
		return entry.accountId, true
	}
	return 0, false
}

func init() {
	go func() {
		for {
			time.Sleep(1 * time.Minute)
			now := time.Now()
			for k, v := range activeTokens {
				if now.Sub(v.createdAt) > 5*time.Minute {
					delete(activeTokens, k)
				}
			}
		}
	}()
}
