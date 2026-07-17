package PortalService

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	zeroTokenStr     = "00000000-0000-0000-0000-000000000000"
	anotherTokenStr  = "00000000-0000-0000-0000-0000000000aa"
	anotherTokenStr2 = "00000000-0000-0000-0000-0000000000bb"
)

func TestGenerateConnectionToken(t *testing.T) {
	var accountId = uint64(0x100000)
	rnd := make([]byte, 16)
	token := generateConnectionTokenWithRandomBytes(accountId, rnd)
	assert.Equal(t, zeroTokenStr, token)
	token = generateConnectionToken(accountId)
	assert.Len(t, token, 36)
}

func TestValidateConnectionToken(t *testing.T) {
	for k := range activeTokens {
		delete(activeTokens, k)
	}

	// non exist
	_, ok := ValidateConnectionToken(anotherTokenStr)
	assert.False(t, ok)

	// insert active entry
	activeTokens[anotherTokenStr2] = 0x1000

	accountId, ok := ValidateConnectionToken(anotherTokenStr2)
	assert.True(t, ok)
	assert.Equal(t, uint64(0x1000), accountId)

	// but now it should be deleted
	accountId, ok = ValidateConnectionToken(anotherTokenStr2)
	assert.False(t, ok)
}
