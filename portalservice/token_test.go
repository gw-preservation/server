package portalservice

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	zeroTokenStr     = "00000000-0000-0000-0000-000000000000"
	anotherTokenStr  = "00000000-0000-0000-0000-0000000000aa"
	anotherTokenStr2 = "00000000-0000-0000-0000-0000000000bb"
	testIP           = "192.0.2.1"
)

func clearActiveTokens() {
	activeTokens.Clear()
}

func testUUID(v byte) []byte {
	return []byte{v, v, v, v, v, v, v, v, v, v, v, v, v, v, v, v}
}

func TestGenerateConnectionToken(t *testing.T) {
	clearActiveTokens()

	var accountId = uint64(0x100000)
	rnd := make([]byte, 16)
	token := generateConnectionTokenWithRandomBytes(accountId, rnd, testIP, testUUID(0xAA))
	assert.Equal(t, zeroTokenStr, token)
	token = generateConnectionToken(accountId, testIP, testUUID(0xBB))
	assert.Len(t, token, 36)
}

func TestValidateConnectionToken(t *testing.T) {
	clearActiveTokens()

	// non exist
	_, ok := ValidateConnectionToken(anotherTokenStr)
	assert.False(t, ok)

	activeTokens.Set(anotherTokenStr2, ConnectionInfo{AccountID: 0x1000, ClientIP: testIP, AccountUUID: [16]byte{0xCC}})

	info, ok := ValidateConnectionToken(anotherTokenStr2)
	assert.True(t, ok)
	assert.Equal(t, uint64(0x1000), info.AccountID)
	assert.Equal(t, testIP, info.ClientIP)
	assert.Equal(t, [16]byte{0xCC}, info.AccountUUID)

	// but now it should be deleted
	_, ok = ValidateConnectionToken(anotherTokenStr2)
	assert.False(t, ok)
}

func TestValidateConnectionToken_ClientIP(t *testing.T) {
	clearActiveTokens()

	accountUUID := testUUID(0xDD)
	token := generateConnectionToken(42, testIP, accountUUID)

	info, ok := ValidateConnectionToken(token)
	assert.True(t, ok)
	assert.Equal(t, uint64(42), info.AccountID)
	assert.Equal(t, testIP, info.ClientIP)
	assert.Equal(t, accountUUID, info.AccountUUID[:])
}

func TestValidateConnectionToken_Consumed(t *testing.T) {
	clearActiveTokens()

	token := generateConnectionToken(99, testIP, testUUID(0xEE))

	_, ok := ValidateConnectionToken(token)
	assert.True(t, ok)

	_, ok = ValidateConnectionToken(token)
	assert.False(t, ok)
}
