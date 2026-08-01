package gameservice

import (
	"fmt"
	"gw1/server/db"
	"net"
	"os"
	"testing"

	GwPacket "gw1/server/gwpacket"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testUUIDCounter byte = 0

func TestMain(m *testing.M) {
	if err := db.SetupTestDB(); err != nil {
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func setupTestConn(t *testing.T) (clientConn *net.TCPConn, conn *GSConn) {
	t.Helper()
	listener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	require.NoError(t, err)
	t.Cleanup(func() { listener.Close() })

	clientConn, err = net.DialTCP("tcp", nil, listener.Addr().(*net.TCPAddr))
	require.NoError(t, err)
	t.Cleanup(func() { clientConn.Close() })

	serverConn, err := listener.AcceptTCP()
	require.NoError(t, err)

	conn = NewGSConn(serverConn, zerolog.Nop())
	return
}

func createTestAccount(t *testing.T) db.Account {
	t.Helper()
	testUUIDCounter++
	uuid := make([]byte, 16)
	uuid[15] = testUUIDCounter
	acc := db.Account{
		Email:        fmt.Sprintf("gs_handshake_%d@localhost", testUUIDCounter),
		Password:     "p",
		PasswordSalt: []byte{0, 0, 0, 0, 0, 0, 0, 0},
		UUID:         uuid,
	}
	require.NoError(t, db.CreateAccountForTest(&acc))
	loaded, ok := db.GetFullAccountByUUID(uuid)
	require.True(t, ok)
	return loaded
}

func packet0500(securityTag, instanceTag int, accountUUID, characterUUID []byte) []byte {
	out := GwPacket.NewOut(0x0500)
	out.Uint32(37600) // clientVersion
	out.Uint16(0)     // unk3
	out.Uint32(0)     // unk4
	out.Uint32(instanceTag)
	out.Uint32(0) // mapId
	out.Uint32(securityTag)
	out.Bytes(accountUUID)
	out.Bytes(characterUUID)
	out.Uint32(0) // unk5
	out.Uint32(0) // unk6
	return out.GetBytes()
}

func packet4200() []byte {
	out := GwPacket.NewOut(0x4200)
	out.Bytes(make([]byte, 64))
	return out.GetBytes()
}

func TestAllowedOp_StateGating(t *testing.T) {
	conn := &GSConn{}
	cases := []struct {
		state State
		op    int
		want  bool
	}{
		{StateAwaitVerifyClientConnection, 0x0500, true},
		{StateAwaitVerifyClientConnection, 0x4200, false},
		{StateAwaitVerifyClientConnection, 0x8009, false},
		{StateAwaitClientSeed, 0x4200, true},
		{StateAwaitClientSeed, 0x0500, false},
		{StateAwaitClientSeed, 0x8009, false},
	}
	for _, c := range cases {
		conn.state = c.state
		assert.Equal(t, c.want, conn.allowedOp(c.op), "state=%v op=0x%04x", c.state, c.op)
	}

	// a disconnect can arrive at any time
	for _, st := range []State{StateAwaitVerifyClientConnection, StateAwaitClientSeed, StateVerified} {
		conn.state = st
		assert.True(t, conn.allowedOp(0x8008), "state=%v op=0x8008 should be allowed", st)
	}

	conn.state = StateVerified
	for _, op := range []int{0x8009, 0x800a, 0x800c, 0x8027, 0x802f, 0x8038, 0x803c, 0x803d, 0x803f, 0x8046, 0x805f, 0x8063, 0x8068, 0x8083, 0x8087, 0x8088, 0x8089, 0x808a, 0x808f, 0x8090, 0x8091, 0x80a0, 0x80b0, 0x80c0} {
		assert.True(t, conn.allowedOp(op), "state=Verified op=0x%04x should be allowed", op)
	}
	for _, op := range []int{0x0500, 0x4200} {
		assert.False(t, conn.allowedOp(op), "state=Verified op=0x%04x should be rejected", op)
	}
}

func TestHandleBytes_HandshakeTransitions(t *testing.T) {
	clearGameTracker()
	clearActiveTokens()
	acc := createTestAccount(t)

	clientConn, conn := setupTestConn(t)
	defer clientConn.Close()
	defer conn.Close()

	assert.Equal(t, StateAwaitVerifyClientConnection, conn.state)

	securityTag := int(GenerateConnectionTokenForInstance(CharCreationTag, false, nil, acc.UUID, "127.0.0.1"))
	characterUUID := make([]byte, 16)
	verify := packet0500(securityTag, int(CharCreationTag), acc.UUID, characterUUID)

	// VerifyClientConnection -> StateAwaitClientSeed
	consumed, err := conn.HandleBytes(verify)
	require.NoError(t, err)
	assert.Equal(t, len(verify), consumed)
	assert.Equal(t, StateAwaitClientSeed, conn.state)

	// ClientSeed -> StateVerified
	seed := packet4200()
	consumed, err = conn.HandleBytes(seed)
	require.NoError(t, err)
	assert.Equal(t, len(seed), consumed)
	assert.Equal(t, StateVerified, conn.state)

	// handshake opcodes are rejected once verified
	_, err = conn.HandleBytes(verify)
	require.Error(t, err)
	assert.Equal(t, StateVerified, conn.state)

	_, err = conn.HandleBytes(seed)
	require.Error(t, err)
	assert.Equal(t, StateVerified, conn.state)
}

func TestHandleBytes_RejectsUnexpectedOpcode(t *testing.T) {
	clientConn, conn := setupTestConn(t)
	defer clientConn.Close()
	defer conn.Close()

	// initial state only accepts 0x0500; ClientSeed is rejected
	_, err := conn.HandleBytes(packet4200())
	require.Error(t, err)
	assert.Equal(t, StateAwaitVerifyClientConnection, conn.state)

	// a verified-only opcode is also rejected mid-handshake
	_, err = conn.HandleBytes([]byte{0x87, 0x80, 0x01, 0x02, 0x03, 0x04})
	require.Error(t, err)
	assert.Equal(t, StateAwaitVerifyClientConnection, conn.state)
}

// A 0x8087 (InstanceLoadRequestSpawnPoint) packet sent before verification is
// rejected by the state machine, so it never reaches the instance handler,
// which used to dereference a nil connectedInstance and crash the whole server.
func TestHandleBytes_UnverifiedRejectsInstanceHandler(t *testing.T) {
	conn := &GSConn{log: zerolog.Nop()}
	packet := []byte{0x87, 0x80, 0x01, 0x02, 0x03, 0x04}
	_, err := conn.HandleBytes(packet)
	require.Error(t, err)
	assert.Equal(t, StateAwaitVerifyClientConnection, conn.state)
}

// The handshake requires a connection in StateAwaitClientSeed to be able to
// send ClientSeed (0x4200).
func TestHandleBytes_AcceptsClientSeed(t *testing.T) {
	listener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	require.NoError(t, err)
	t.Cleanup(func() { listener.Close() })

	clientConn, err := net.DialTCP("tcp", nil, listener.Addr().(*net.TCPAddr))
	require.NoError(t, err)
	defer clientConn.Close()

	serverConn, err := listener.AcceptTCP()
	require.NoError(t, err)

	conn := &GSConn{
		socket: serverConn,
		state:  StateAwaitClientSeed,
		out:    GwPacket.NewOutRaw(),
		log:    zerolog.Nop(),
		done:   make(chan struct{}),
	}
	conn.player = newPlayer(conn, zerolog.Nop())
	defer conn.Close()

	consumed, err := conn.HandleBytes(packet4200())
	require.NoError(t, err)
	assert.Equal(t, 66, consumed) // handler ran, consumed opcode + seed
	assert.Equal(t, StateVerified, conn.state)

	buf := make([]byte, 64)
	n, err := clientConn.Read(buf)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, n, 3) // seed response
}

// A verified connection in character creation has connectedInstance == nil;
// instance-load packets are dispatched to the char-creation table there and
// must not panic. The packet is unhandled in that context, so the whole thing
// is consumed with a warning.
func TestHandleBytes_VerifiedNilInstanceDoesNotPanic(t *testing.T) {
	conn := &GSConn{log: zerolog.Nop(), state: StateVerified}
	conn.player = newPlayer(conn, zerolog.Nop())
	packet := []byte{0x87, 0x80, 0x01, 0x02, 0x03, 0x04}
	consumed, err := conn.HandleBytes(packet)
	require.NoError(t, err)
	assert.Equal(t, len(packet), consumed) // unhandled packet consumed, no panic
}

// ClientSeed before verification previously called AddPlayer (now the
// fire-and-forget AcceptPlayer, only reached when connectedInstance is set) on
// a nil connectedInstance and crashed. The seed response must still be written.
func TestOnClientSeed_NoInstanceDoesNotPanic(t *testing.T) {
	listener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	require.NoError(t, err)
	t.Cleanup(func() { listener.Close() })

	clientConn, err := net.DialTCP("tcp", nil, listener.Addr().(*net.TCPAddr))
	require.NoError(t, err)
	defer clientConn.Close()

	serverConn, err := listener.AcceptTCP()
	require.NoError(t, err)

	conn := &GSConn{
		socket: serverConn,
		out:    GwPacket.NewOutRaw(),
		log:    zerolog.Nop(),
		done:   make(chan struct{}),
	}
	conn.player = newPlayer(conn, zerolog.Nop())
	defer conn.Close()

	payload := ClientSeed{seed: make([]byte, 64)}
	err = conn.onClientSeed(&payload)
	require.NoError(t, err)
	assert.Equal(t, StateVerified, conn.state)

	buf := make([]byte, 64)
	n, err := clientConn.Read(buf)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, n, 3) // 1 + 1 + 20-byte public key
}
