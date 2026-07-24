package GameService

import "math/rand"

var activeTokens = make(map[uint32]uint32, 0)

func GenerateConnectionTokenForInstance(instanceTag uint32) uint32 {
	securityTag := rand.Uint32()
	activeTokens[securityTag] = instanceTag
	return securityTag
}

func ValidateConnectionToken(securityTag uint32) (instanceTag uint32, ok bool) {
	instanceTag, ok = activeTokens[securityTag]
	if ok {
		delete(activeTokens, securityTag)
	}
	return
}

// TODO: GC
