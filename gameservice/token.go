package GameService

import (
	"math/rand"
	"time"
)

type ConnectionInfo struct {
	InstanceTag uint32
	IsTransfer  bool
}

type gameToken struct {
	info      ConnectionInfo
	createdAt time.Time
}

var activeTokens = make(map[uint32]gameToken)

func GenerateConnectionTokenForInstance(instanceTag uint32, isTransfer bool) uint32 {
	securityTag := rand.Uint32()
	activeTokens[securityTag] = gameToken{
		info:      ConnectionInfo{InstanceTag: instanceTag, IsTransfer: isTransfer},
		createdAt: time.Now(),
	}
	return securityTag
}

func ValidateConnectionToken(securityTag uint32) (info ConnectionInfo, ok bool) {
	entry, ok := activeTokens[securityTag]
	if ok {
		delete(activeTokens, securityTag)
		return entry.info, true
	}
	return ConnectionInfo{}, false
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
