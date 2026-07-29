package gameservice

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func clearActiveTokens() {
	for k := range activeTokens {
		delete(activeTokens, k)
	}
}

func TestGenerateConnectionTokenForInstance(t *testing.T) {
	clearActiveTokens()

	token := GenerateConnectionTokenForInstance(100, false)
	assert.NotZero(t, token)

	entry, ok := activeTokens[token]
	assert.True(t, ok)
	assert.Equal(t, uint32(100), entry.info.InstanceTag)
	assert.False(t, entry.info.IsTransfer)
}

func TestGenerateConnectionTokenForInstance_Transfer(t *testing.T) {
	clearActiveTokens()

	token := GenerateConnectionTokenForInstance(200, true)

	entry, ok := activeTokens[token]
	assert.True(t, ok)
	assert.Equal(t, uint32(200), entry.info.InstanceTag)
	assert.True(t, entry.info.IsTransfer)
}

func TestGenerateConnectionTokenForInstance_UniqueTokens(t *testing.T) {
	clearActiveTokens()

	token1 := GenerateConnectionTokenForInstance(1, false)
	token2 := GenerateConnectionTokenForInstance(2, false)
	assert.NotEqual(t, token1, token2)
	assert.Len(t, activeTokens, 2)
}

func TestValidateConnectionToken_Success(t *testing.T) {
	clearActiveTokens()

	token := GenerateConnectionTokenForInstance(42, true)

	info, ok := ValidateConnectionToken(token)
	assert.True(t, ok)
	assert.Equal(t, uint32(42), info.InstanceTag)
	assert.True(t, info.IsTransfer)
}

func TestValidateConnectionToken_NotFound(t *testing.T) {
	clearActiveTokens()

	_, ok := ValidateConnectionToken(999999)
	assert.False(t, ok)
}

func TestValidateConnectionToken_Consumed(t *testing.T) {
	clearActiveTokens()

	token := GenerateConnectionTokenForInstance(10, false)

	_, ok := ValidateConnectionToken(token)
	assert.True(t, ok)

	_, ok = ValidateConnectionToken(token)
	assert.False(t, ok)
	assert.Len(t, activeTokens, 0)
}
