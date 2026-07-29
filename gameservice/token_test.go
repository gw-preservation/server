package gameservice

import (
	"net"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func testUUID(v byte) []byte {
	return []byte{v, v, v, v, v, v, v, v, v, v, v, v, v, v, v, v}
}

const testIP = "192.0.2.1"

func clearActiveTokens() {
	activeTokensMu.Lock()
	defer activeTokensMu.Unlock()
	for k := range activeTokens {
		delete(activeTokens, k)
	}
}

func getActiveToken(key uint32) (gameToken, bool) {
	activeTokensMu.Lock()
	defer activeTokensMu.Unlock()
	v, ok := activeTokens[key]
	return v, ok
}

func activeTokenCount() int {
	activeTokensMu.Lock()
	defer activeTokensMu.Unlock()
	return len(activeTokens)
}

func TestGenerateConnectionTokenForInstance(t *testing.T) {
	clearActiveTokens()

	charUUID := testUUID(0xAA)
	accUUID := testUUID(0xBB)
	token := GenerateConnectionTokenForInstance(100, false, charUUID, accUUID, testIP)
	assert.NotZero(t, token)

	entry, ok := getActiveToken(token)
	assert.True(t, ok)
	assert.Equal(t, uint32(100), entry.info.InstanceTag)
	assert.False(t, entry.info.IsTransfer)
	assert.Equal(t, charUUID, entry.info.CharacterUUID[:])
	assert.Equal(t, accUUID, entry.info.AccountUUID[:])
	assert.Equal(t, testIP, entry.info.ClientIP)
}

func TestGenerateConnectionTokenForInstance_Transfer(t *testing.T) {
	clearActiveTokens()

	charUUID := testUUID(0xBB)
	accUUID := testUUID(0xCC)
	token := GenerateConnectionTokenForInstance(200, true, charUUID, accUUID, testIP)

	entry, ok := getActiveToken(token)
	assert.True(t, ok)
	assert.Equal(t, uint32(200), entry.info.InstanceTag)
	assert.True(t, entry.info.IsTransfer)
	assert.Equal(t, charUUID, entry.info.CharacterUUID[:])
	assert.Equal(t, accUUID, entry.info.AccountUUID[:])
	assert.Equal(t, testIP, entry.info.ClientIP)
}

func TestGenerateConnectionTokenForInstance_NilUUIDs(t *testing.T) {
	clearActiveTokens()

	token := GenerateConnectionTokenForInstance(300, false, nil, nil, "")
	assert.NotZero(t, token)

	entry, ok := getActiveToken(token)
	assert.True(t, ok)
	assert.Equal(t, [16]byte{}, entry.info.CharacterUUID)
	assert.Equal(t, [16]byte{}, entry.info.AccountUUID)
	assert.Equal(t, "", entry.info.ClientIP)
}

func TestGenerateConnectionTokenForInstance_UniqueTokens(t *testing.T) {
	clearActiveTokens()

	token1 := GenerateConnectionTokenForInstance(1, false, testUUID(0x01), testUUID(0x02), testIP)
	token2 := GenerateConnectionTokenForInstance(2, false, testUUID(0x03), testUUID(0x04), testIP)
	assert.NotEqual(t, token1, token2)
	assert.Equal(t, 2, activeTokenCount())
}

func TestValidateConnectionToken_Success(t *testing.T) {
	clearActiveTokens()

	charUUID := testUUID(0x42)
	accUUID := testUUID(0x99)
	token := GenerateConnectionTokenForInstance(42, true, charUUID, accUUID, testIP)

	info, ok := ValidateConnectionToken(token)
	assert.True(t, ok)
	assert.Equal(t, uint32(42), info.InstanceTag)
	assert.True(t, info.IsTransfer)
	assert.Equal(t, charUUID, info.CharacterUUID[:])
	assert.Equal(t, accUUID, info.AccountUUID[:])
	assert.Equal(t, testIP, info.ClientIP)
}

func TestValidateConnectionToken_NotFound(t *testing.T) {
	clearActiveTokens()

	_, ok := ValidateConnectionToken(999999)
	assert.False(t, ok)
}

func TestValidateConnectionToken_Consumed(t *testing.T) {
	clearActiveTokens()

	token := GenerateConnectionTokenForInstance(10, false, testUUID(0x10), testUUID(0x20), testIP)

	_, ok := ValidateConnectionToken(token)
	assert.True(t, ok)

	_, ok = ValidateConnectionToken(token)
	assert.False(t, ok)
	assert.Equal(t, 0, activeTokenCount())
}

func TestRejectsWrongClientIP(t *testing.T) {
	clearActiveTokens()

	listener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	assert.NoError(t, err)
	defer listener.Close()

	clientConn, err := net.DialTCP("tcp", nil, listener.Addr().(*net.TCPAddr))
	assert.NoError(t, err)
	defer clientConn.Close()

	serverConn, err := listener.AcceptTCP()
	assert.NoError(t, err)

	charUUID := testUUID(0xAA)
	accUUID := testUUID(0xBB)

	// Token bound to a different IP than the test connection's actual source IP
	attackerIP := "10.0.0.99"
	securityTag := GenerateConnectionTokenForInstance(42, false, charUUID, accUUID, attackerIP)

	gsConn := NewGSConn(serverConn, zerolog.Nop(), 0)
	defer gsConn.Close()

	payload := &VerifyClientConnection{
		clientVersion: 37600,
		instanceTag:   42,
		securityTag:   int(securityTag),
		accountUUID:   accUUID,
		characterUUID: charUUID,
	}

	err = gsConn.onVerifyClientConnection(payload)
	assert.NoError(t, err)

	// IP check should have fired before any DB access, closing the connection
	assert.True(t, gsConn.closed.Load(), "expected connection closed after IP mismatch")
}
