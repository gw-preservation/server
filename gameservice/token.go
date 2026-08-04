package gameservice

import (
	"crypto/rand"
	"encoding/binary"
	"time"

	"gw1/server/tokenstore"
)

type ConnectionInfo struct {
	InstanceTag   uint32   // tag of the instance this is will connect up to
	IsTransfer    bool     // whether it was a game transfer or a fresh login
	CharacterUUID [16]byte // who it is for
	AccountUUID   [16]byte // which account it belongs to
	ClientIP      string   // IP address that generated the token
	HasSpawnPoint bool
	SpawnX        float32
	SpawnY        float32
	SpawnPlane    int
}

const (
	tokenTTL             = 5 * time.Minute
	tokenCleanupInterval = 1 * time.Minute
)

var activeTokens = tokenstore.New[uint32, ConnectionInfo](tokenCleanupInterval, tokenTTL)

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
	activeTokens.Set(securityTag, ConnectionInfo{
		InstanceTag:   instanceTag,
		IsTransfer:    isTransfer,
		CharacterUUID: charID,
		AccountUUID:   accID,
		ClientIP:      clientIP,
	})
	return securityTag
}

func SetConnectionTokenSpawnPoint(tag uint32, x, y float32, plane int) {
	info, ok := activeTokens.Consume(tag)
	if !ok {
		return
	}
	info.HasSpawnPoint = true
	info.SpawnX = x
	info.SpawnY = y
	info.SpawnPlane = plane
	activeTokens.Set(tag, info)
}

func ValidateConnectionToken(securityTag uint32) (info ConnectionInfo, ok bool) {
	return activeTokens.Consume(securityTag)
}

func StopCleanup() {
	activeTokens.Stop()
}
