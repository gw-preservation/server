package gameservice

import (
	"net"
	"testing"

	GwPacket "gw1/server/gwpacket"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A 0x8087 (InstanceLoadRequestSpawnPoint) packet sent before verification must
// not reach the instance handler, which used to dereference a nil
// connectedInstance and crash the whole server.
func TestHandleBytes_UnverifiedRejectsInstanceHandler(t *testing.T) {
	conn := &GSConn{log: zerolog.Nop()}
	packet := []byte{0x87, 0x80, 0x01, 0x02, 0x03, 0x04}
	consumed, err := conn.HandleBytes(packet)
	require.NoError(t, err)
	assert.Equal(t, len(packet), consumed)
}

// The handshake requires an unverified connection to be able to send
// ClientSeed (0x4200) and VerifyClientConnection (0x0500).
func TestHandleBytes_UnverifiedAcceptsClientSeed(t *testing.T) {
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

	// opcode 0x4200 + 64-byte seed
	packet := make([]byte, 66)
	packet[0], packet[1] = 0x00, 0x42
	consumed, err := conn.HandleBytes(packet)
	require.NoError(t, err)
	assert.Equal(t, 66, consumed) // handler ran, consumed opcode + seed

	buf := make([]byte, 64)
	n, err := clientConn.Read(buf)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, n, 3) // seed response
}

// A verified connection in character creation has connectedInstance == nil;
// the instance-load handlers must guard instead of panicking.
func TestHandleBytes_VerifiedNilInstanceDoesNotPanic(t *testing.T) {
	conn := &GSConn{log: zerolog.Nop(), verified: true}
	conn.player = newPlayer(conn, zerolog.Nop())
	packet := []byte{0x87, 0x80, 0x01, 0x02, 0x03, 0x04}
	consumed, err := conn.HandleBytes(packet)
	require.NoError(t, err)
	assert.Equal(t, 2, consumed) // opcode consumed by the guarded handler
}

// ClientSeed before verification previously called AddPlayer on a nil
// connectedInstance and crashed. The seed response must still be written.
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

	buf := make([]byte, 64)
	n, err := clientConn.Read(buf)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, n, 3) // 1 + 1 + 20-byte public key
}
