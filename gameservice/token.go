package gameservice

import (
	"crypto/rand"
	"encoding/binary"
	"sync"
	"time"
)

type ConnectionInfo struct {
	InstanceTag   uint32   // tag of the instance this is will connect up to
	IsTransfer    bool     // whether it was a game transfer or a fresh login
	CharacterUUID [16]byte // who it is for
	AccountUUID   [16]byte // which account it belongs to
	ClientIP      string   // IP address that generated the token
}

type gameToken struct {
	info      ConnectionInfo
	createdAt time.Time
}

var (
	activeTokensMu sync.Mutex
	activeTokens   = make(map[uint32]gameToken)
)

var CharCreationTag = randUint32()

func randUint32() uint32 {
	var b [4]byte
	_, err := rand.Read(b[:])
	if err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return binary.LittleEndian.Uint32(b[:])
}

func GenerateConnectionTokenForInstance(instanceTag uint32, isTransfer bool, characterUUID, accountUUID []byte, clientIP string) uint32 {
	securityTag := randUint32()
	var charID, accID [16]byte
	copy(charID[:], characterUUID)
	copy(accID[:], accountUUID)
	activeTokensMu.Lock()
	activeTokens[securityTag] = gameToken{
		info:      ConnectionInfo{InstanceTag: instanceTag, IsTransfer: isTransfer, CharacterUUID: charID, AccountUUID: accID, ClientIP: clientIP},
		createdAt: time.Now(),
	}
	activeTokensMu.Unlock()
	return securityTag
}

func ValidateConnectionToken(securityTag uint32) (info ConnectionInfo, ok bool) {
	activeTokensMu.Lock()
	entry, ok := activeTokens[securityTag]
	if ok {
		delete(activeTokens, securityTag)
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
