package authservice

import (
	"gw1/server/portalservice"
	"testing"

	"gw1/server/gwpacket"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func handshakePacket(opcode int, build func(*gwpacket.Out)) []byte {
	out := gwpacket.NewOut(opcode)
	build(&out)
	return out.GetBytes()
}

func packet0400() []byte {
	return handshakePacket(0x0400, func(out *gwpacket.Out) {
		out.Uint16(0)
		out.Uint32(37600)
		out.Uint32(0)
		out.Uint32(0)
	})
}

func packet4200() []byte {
	return handshakePacket(0x4200, func(out *gwpacket.Out) {
		out.Bytes(make([]byte, 64))
	})
}

func packet8001() []byte {
	return handshakePacket(0x8001, func(out *gwpacket.Out) {
		out.UTF16WithLengthPrefix("testuser")
		out.UTF16WithLengthPrefix("testpc")
	})
}

func packet8002() []byte {
	return handshakePacket(0x8002, func(out *gwpacket.Out) {
		out.Uint32(37600)
		out.Bytes(make([]byte, 16))
	})
}

func packet8023() []byte {
	return handshakePacket(0x8023, func(out *gwpacket.Out) {
		out.Uint32(0)
	})
}

func packet8038(transactionId int, uuid1, token []byte) []byte {
	return handshakePacket(0x8038, func(out *gwpacket.Out) {
		out.Uint32(transactionId)
		out.Bytes(uuid1)
		out.Bytes(token)
		out.UTF16WithLengthPrefix("")
	})
}

func packet8009() []byte {
	return handshakePacket(0x8009, func(out *gwpacket.Out) {
		out.Uint32(1)
		out.UTF16WithLengthPrefix("Hero")
		out.Uint16(0)
	})
}

func TestAllowedOp_StateGating(t *testing.T) {
	conn := &ASConn{}
	cases := []struct {
		state State
		op    int
		want  bool
	}{
		{StateAwaitClientVersionInfo, 0x0400, true},
		{StateAwaitClientVersionInfo, 0x4200, false},
		{StateAwaitClientSeed, 0x4200, true},
		{StateAwaitClientSeed, 0x0400, false},
		{StateAwaitComputerInfo, 0x8001, true},
		{StateAwaitComputerInfo, 0x8002, false},
		{StateAwaitClientHashInfo, 0x8002, true},
		{StateAwaitClientHashInfo, 0x8038, false},
		{StateAwaitGetAccountInfo, 0x8038, true},
		{StateAwaitGetAccountInfo, 0x0400, false},
		{StateAwaitGetAccountInfo, 0x8009, false},
		{StateAwaitGetAccountInfo, 0x800a, false},
		{StateAwaitGetAccountInfo, 0x8029, false},
	}
	for _, c := range cases {
		conn.state = c.state
		assert.Equal(t, c.want, conn.allowedOp(c.op), "state=%v op=0x%04x", c.state, c.op)
	}

	conn.state = StateVerified
	for _, op := range []int{0x8007, 0x8009, 0x800a, 0x800d, 0x800e, 0x8016, 0x801c, 0x8020, 0x8021, 0x8029, 0x8035, 0x8037} {
		assert.True(t, conn.allowedOp(op), "state=Verified op=0x%04x should be allowed", op)
	}
	for _, op := range []int{0x0400, 0x4200, 0x8001, 0x8002, 0x8038} {
		assert.False(t, conn.allowedOp(op), "state=Verified op=0x%04x should be rejected", op)
	}
}

func TestHandleBytes_HandshakeTransitions(t *testing.T) {
	clientConn, conn := setupTestConn(t)
	defer clientConn.Close()
	defer conn.Close()

	assert.Equal(t, StateAwaitClientVersionInfo, conn.state)

	steps := []struct {
		packet []byte
		want   State
	}{
		{packet0400(), StateAwaitClientSeed},
		{packet4200(), StateAwaitComputerInfo},
		{packet8001(), StateAwaitClientHashInfo},
		{packet8002(), StateAwaitGetAccountInfo},
	}
	for _, s := range steps {
		consumed, err := conn.HandleBytes(s.packet)
		require.NoError(t, err, "state=%v", conn.state)
		assert.Equal(t, len(s.packet), consumed)
		assert.Equal(t, s.want, conn.state)
	}
}

func TestHandleBytes_RejectsUnexpectedOpcode(t *testing.T) {
	clientConn, conn := setupTestConn(t)
	defer clientConn.Close()
	defer conn.Close()

	// initial state only accepts 0x0400
	_, err := conn.HandleBytes(packet4200())
	require.Error(t, err)
	assert.Equal(t, StateAwaitClientVersionInfo, conn.state)

	// advance through the handshake
	for _, p := range [][]byte{packet0400(), packet4200(), packet8001(), packet8002()} {
		_, err = conn.HandleBytes(p)
		require.NoError(t, err)
	}
	assert.Equal(t, StateAwaitGetAccountInfo, conn.state)

	// a verified-only opcode is still rejected mid-handshake
	_, err = conn.HandleBytes(packet8009())
	require.Error(t, err)
	assert.Equal(t, StateAwaitGetAccountInfo, conn.state)
}

func TestHandleBytes_FullHandshakeToVerified(t *testing.T) {
	clearTracker()
	acc := createTestAccount(t, "handshake@localhost", "Hero")

	clientConn, conn := setupTestConn(t)
	defer clientConn.Close()
	defer conn.Close()

	steps := []struct {
		packet []byte
		want   State
	}{
		{packet0400(), StateAwaitClientSeed},
		{packet4200(), StateAwaitComputerInfo},
		{packet8001(), StateAwaitClientHashInfo},
		{packet8002(), StateAwaitGetAccountInfo},
	}
	for _, s := range steps {
		_, err := conn.HandleBytes(s.packet)
		require.NoError(t, err)
		assert.Equal(t, s.want, conn.state)
	}

	tokenBytes := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10}
	portalservice.GenerateConnectionTokenForTest(acc.ID, tokenBytes, "127.0.0.1", acc.UUID)

	_, err := conn.HandleBytes(packet8038(1, acc.UUID, tokenBytes))
	require.NoError(t, err)
	assert.Equal(t, StateVerified, conn.state)

	// handshake opcodes are rejected once verified
	_, err = conn.HandleBytes(packet4200())
	require.Error(t, err)
	assert.Equal(t, StateVerified, conn.state)

	// but 0x8023 is still tolerated
	_, err = conn.HandleBytes(packet8023())
	require.NoError(t, err)
	assert.Equal(t, StateVerified, conn.state)
}

func TestHandleBytes_8023IsNoOp(t *testing.T) {
	clientConn, conn := setupTestConn(t)
	defer clientConn.Close()
	defer conn.Close()

	// accepted before the handshake begins, without advancing state
	consumed, err := conn.HandleBytes(packet8023())
	require.NoError(t, err)
	assert.Equal(t, 6, consumed)
	assert.Equal(t, StateAwaitClientVersionInfo, conn.state)

	// and interleaved with handshake packets, again without advancing
	steps := []struct {
		packet []byte
		want   State
	}{
		{packet0400(), StateAwaitClientSeed},
		{packet8023(), StateAwaitClientSeed},
		{packet4200(), StateAwaitComputerInfo},
		{packet8001(), StateAwaitClientHashInfo},
		{packet8023(), StateAwaitClientHashInfo},
		{packet8002(), StateAwaitGetAccountInfo},
	}
	for _, s := range steps {
		_, err := conn.HandleBytes(s.packet)
		require.NoError(t, err)
		assert.Equal(t, s.want, conn.state)
	}
}
