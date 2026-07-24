package AuthService

import (
	"crypto/rc4"
	"fmt"
	"gw1/server/db"
	GwPacket "gw1/server/gwpacket"
	"net"

	"github.com/rs/zerolog"
)

type State int

const (
	StateReadClientVersion State = iota
	StateReadClientSeed
	StateCharacterScreen
	StateCreateCharacter
	StateInInstance
)

type ASConn struct {
	socket *net.TCPConn
	state  State
	enc    *rc4.Cipher
	dec    *rc4.Cipher
	out    GwPacket.Out
	log    zerolog.Logger
	acc    db.Account
}

func NewASConn(socket *net.TCPConn, logCtx zerolog.Logger) *ASConn {
	conn := ASConn{
		socket: socket,
		state:  StateReadClientVersion,
		log:    logCtx.With().Str("srv", "auth").Logger(),
		out:    GwPacket.NewOutRaw(),
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

func (conn *ASConn) HandleBytes(data []byte) (int, error) {
	inPkt := GwPacket.NewIn(data)
	return conn.onRegularPacket(&inPkt)
}

func (conn *ASConn) onRegularPacket(in *GwPacket.In) (consumed int, err error) {
	op, _ := in.Uint16()

	handler, ok := packetHandlers[op]
	if !ok {
		return 0, fmt.Errorf("[%04x] UNEXPECTED; len=%d", op, in.Remaining())
	}
	consumed, err = handler(conn, in)

	if len(conn.out.GetBytes()) > 0 {
		conn.WritePacket(&conn.out)
		conn.out.Reset()
	}
	return
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
	conn.socket.Close()
}

func (conn *ASConn) EnqueuePacket(packet GwPacket.Out) {
	conn.out.Merge(packet)
}
