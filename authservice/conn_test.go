package authservice

import (
	"net"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestClose_UntracksAccount(t *testing.T) {
	clearTracker()

	listener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	assert.NoError(t, err)
	defer listener.Close()

	clientConn, err := net.DialTCP("tcp", nil, listener.Addr().(*net.TCPAddr))
	assert.NoError(t, err)
	defer clientConn.Close()

	serverConn, err := listener.AcceptTCP()
	assert.NoError(t, err)

	conn := NewASConn(serverConn, zerolog.Nop())
	conn.accountID = 42
	TrackAccount(42)

	assert.False(t, TrackAccount(42), "account should be tracked")

	conn.Close()

	assert.True(t, TrackAccount(42), "account should be freed after Close")
}

func TestClose_ZeroAccountIDDoesNotUntrack(t *testing.T) {
	clearTracker()

	listener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	assert.NoError(t, err)
	defer listener.Close()

	clientConn, err := net.DialTCP("tcp", nil, listener.Addr().(*net.TCPAddr))
	assert.NoError(t, err)
	defer clientConn.Close()

	serverConn, err := listener.AcceptTCP()
	assert.NoError(t, err)

	conn := NewASConn(serverConn, zerolog.Nop())
	TrackAccount(42)

	conn.Close()

	assert.False(t, TrackAccount(42), "should still be tracked since accountID was 0")
}

func TestClose_Idempotent(t *testing.T) {
	clearTracker()

	listener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	assert.NoError(t, err)
	defer listener.Close()

	clientConn, err := net.DialTCP("tcp", nil, listener.Addr().(*net.TCPAddr))
	assert.NoError(t, err)
	defer clientConn.Close()

	serverConn, err := listener.AcceptTCP()
	assert.NoError(t, err)

	conn := NewASConn(serverConn, zerolog.Nop())
	conn.accountID = 42
	TrackAccount(42)

	conn.Close()
	conn.Close()

	assert.True(t, TrackAccount(42), "account should be freed (no panic on double close)")
}
