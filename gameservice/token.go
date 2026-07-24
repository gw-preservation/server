package GameService

import "math/rand"

type ConnectionInfo struct {
	InstanceTag uint32
	IsTransfer  bool
}

var activeTokens = make(map[uint32]ConnectionInfo, 0)

func GenerateConnectionTokenForInstance(instanceTag uint32, isTransfer bool) uint32 {
	securityTag := rand.Uint32()
	activeTokens[securityTag] = ConnectionInfo{InstanceTag: instanceTag, IsTransfer: isTransfer}
	return securityTag
}

func ValidateConnectionToken(securityTag uint32) (info ConnectionInfo, ok bool) {
	info, ok = activeTokens[securityTag]
	if ok {
		delete(activeTokens, securityTag)
	}
	return
}

// TODO: GC
