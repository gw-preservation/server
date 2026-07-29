package portalservice

import (
	"crypto/rand"
	"gw1/server/db"
	"sync"
	"time"
)

type ConnectionInfo struct {
	AccountID   uint64
	ClientIP    string
	AccountUUID [16]byte
}

type portalToken struct {
	info      ConnectionInfo
	createdAt time.Time
}

var (
	activeTokensMu sync.Mutex
	activeTokens   = make(map[string]portalToken)
)

func generateConnectionTokenWithRandomBytes(accountId uint64, tokenBytes []byte, clientIP string, accountUUID []byte) (token string) {
	token = db.UUIDStr(tokenBytes)
	var accUUID [16]byte
	copy(accUUID[:], accountUUID)
	activeTokensMu.Lock()
	activeTokens[token] = portalToken{
		info:      ConnectionInfo{AccountID: accountId, ClientIP: clientIP, AccountUUID: accUUID},
		createdAt: time.Now(),
	}
	activeTokensMu.Unlock()
	return token
}

func generateConnectionToken(accountId uint64, clientIP string, accountUUID []byte) (token string) {
	var tokenBytes = make([]byte, 16)
	rand.Read(tokenBytes)
	return generateConnectionTokenWithRandomBytes(accountId, tokenBytes, clientIP, accountUUID)
}

func ValidateConnectionToken(token string) (info ConnectionInfo, ok bool) {
	activeTokensMu.Lock()
	entry, ok := activeTokens[token]
	if ok {
		delete(activeTokens, token)
	}
	activeTokensMu.Unlock()
	return entry.info, ok
}

func init() {
	go func() {
		for {
			time.Sleep(1 * time.Minute)
			now := time.Now()
			activeTokensMu.Lock()
			for k, v := range activeTokens {
				if now.Sub(v.createdAt) > 5*time.Minute {
					delete(activeTokens, k)
				}
			}
			activeTokensMu.Unlock()
		}
	}()
}
