package authservice

import (
	"crypto/rc4"
	"fmt"
	"gw1/server/db"
	GwPacket "gw1/server/gwpacket"
	"net"
	"sync"

	"github.com/rs/zerolog"
)

type State int

const (
	StateAwaitClientVersionInfo State = iota
	StateAwaitClientSeed
	StateAwaitComputerInfo
	StateAwaitClientHashInfo
	StateAwaitGetAccountInfo
	StateVerified
)

func (s State) String() string {
	switch s {
	case StateAwaitClientVersionInfo:
		return "AwaitClientVersionInfo"
	case StateAwaitClientSeed:
		return "AwaitClientSeed"
	case StateAwaitComputerInfo:
		return "AwaitComputerInfo"
	case StateAwaitClientHashInfo:
		return "AwaitClientHashInfo"
	case StateAwaitGetAccountInfo:
		return "AwaitGetAccountInfo"
	case StateVerified:
		return "Verified"
	}
	return fmt.Sprintf("State(%d)", int(s))
}

type ASConn struct {
	socket                 *net.TCPConn
	state                  State
	enc                    *rc4.Cipher
	dec                    *rc4.Cipher
	out                    GwPacket.Out
	log                    zerolog.Logger
	acc                    db.Account
	hasLoggedInThisSession bool
	activeCharacterName    string
	closeOnce              sync.Once
	accountID              uint64
}

func NewASConn(socket *net.TCPConn, logCtx zerolog.Logger) *ASConn {
	conn := ASConn{
		socket:                 socket,
		state:                  StateAwaitClientVersionInfo,
		log:                    logCtx.With().Str("srv", "auth").Logger(),
		out:                    GwPacket.NewOutRaw(),
		hasLoggedInThisSession: false,
	}
	return &conn
}

func (conn *ASConn) DecryptBytes(data []byte) {
	// Decrypt the data in place using the RC4 cipher if it has been initialized
	if conn.dec != nil {
		conn.dec.XORKeyStream(data, data)
	}
}

func (conn *ASConn) EncryptBytes(data []byte) {
	// Encrypt the data in place using the RC4 cipher if it has been initialized
	if conn.enc != nil {
		conn.enc.XORKeyStream(data, data)
	}
}

func (conn *ASConn) HandleBytes(data []byte) (consumed int, err error) {
	in := GwPacket.NewIn(data)
	op, err := in.Uint16()
	if err != nil {
		return 0, err
	}
	if !conn.allowedOp(op) {
		return 0, fmt.Errorf("[%04x] unexpected for %v connection; len=%d", op, conn.state, in.Remaining())
	}

	var handler packetHandler
	var ok bool
	if conn.state == StateVerified {
		handler, ok = accountHandlers[op]
	} else {
		handler, ok = handshakeHandlers[op]
	}
	if !ok {
		return 0, fmt.Errorf("[%04x] no handler for %v connection; len=%d", op, conn.state, in.Remaining())
	}
	consumed, err = handler(conn, &in)

	if len(conn.out.GetBytes()) > 0 {
		conn.WritePacket(&conn.out)
		conn.out.Reset()
	}
	return
}

func (conn *ASConn) allowedOp(op int) bool {
	if op == 0x8023 || op == 0x8000 {
		return true
	}
	switch conn.state {
	case StateAwaitClientVersionInfo:
		return op == 0x0400
	case StateAwaitClientSeed:
		return op == 0x4200
	case StateAwaitComputerInfo:
		return op == 0x8001
	case StateAwaitClientHashInfo:
		return op == 0x8002
	case StateAwaitGetAccountInfo:
		return op == 0x8038
	case StateVerified:
		_, ok := accountHandlers[op]
		return ok
	}
	return false
}

func (conn *ASConn) Read(buf []byte) (int, error) {
	return conn.socket.Read(buf)
}

func (conn *ASConn) WritePacket(packet *GwPacket.Out) error {
	bts := packet.GetBytes()
	conn.EncryptBytes(bts)
	_, err := conn.socket.Write(bts)
	return err
}

func (conn *ASConn) Close() {
	conn.closeOnce.Do(func() {
		if conn.accountID != 0 {
			UntrackAccount(conn.accountID)
		}
		conn.socket.Close()
	})
}

func (conn *ASConn) EnqueuePacket(packet GwPacket.Out) {
	conn.out.Merge(packet)
}

func (conn *ASConn) getLastOutpostId() (mapId int, uuid []byte, ok bool) {
	if conn.activeCharacterName == "" {
		return 0, nil, false
	}
	for _, char := range conn.acc.Characters {
		if char.Name == conn.activeCharacterName {
			return int(char.LastOutpostID), char.UUID, true
		}
	}
	return 0, nil, false
}

func (conn *ASConn) clientIP() string {
	host, _, err := net.SplitHostPort(conn.socket.RemoteAddr().String())
	if err != nil {
		return conn.socket.RemoteAddr().String()
	}
	return host
}

func (conn *ASConn) charBelongsToAccount(name string) bool {
	for _, char := range conn.acc.Characters {
		if char.Name == name {
			return true
		}
	}
	return false
}
