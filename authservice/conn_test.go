package authservice

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A fragmented heartbeat (opcode-only, 2 bytes) previously returned consumed=6,
// which overflowed the tcpsrv buffer slicing and crashed the server. It must
// wait for the full 6-byte packet instead.
func TestHandleBytes_HeartbeatPartialRead(t *testing.T) {
	clientConn, conn := setupTestConn(t)
	defer clientConn.Close()
	defer conn.Close()

	consumed, err := conn.HandleBytes([]byte{0x00, 0x80})
	require.NoError(t, err)
	assert.Equal(t, 0, consumed)

	consumed, err = conn.HandleBytes([]byte{0x00, 0x80, 0x01, 0x02, 0x03, 0x04})
	require.NoError(t, err)
	assert.Equal(t, 6, consumed)
	assert.NotEmpty(t, conn.out.GetBytes()) // heartbeat response queued

	// The heartbeat branch returns before the tail flush in HandleBytes, so
	// the response is only sent when a later packet flushes the out buffer.
	flushOut(conn)

	buf := make([]byte, 64)
	n, err := clientConn.Read(buf)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, n, 6)
}
