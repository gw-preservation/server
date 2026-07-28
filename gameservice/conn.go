package GameService

import (
	"crypto/rc4"
	"fmt"
	GwPacket "gw1/server/gwpacket"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
)

type GSConn struct {
	socket *net.TCPConn
	enc    *rc4.Cipher
	dec    *rc4.Cipher
	out    GwPacket.Out
	closed atomic.Bool
	log    zerolog.Logger
	player *Player
	done   chan struct{}
	closeOnce sync.Once
}

func NewGSConn(socket *net.TCPConn, logCtx zerolog.Logger) *GSConn {
	conn := GSConn{
		socket: socket,
		out:    GwPacket.NewOutRaw(),
		log:    logCtx.With().Str("srv", "game").Logger(),
		done:   make(chan struct{}),
	}
	conn.player = NewPlayer(&conn, logCtx)
	go func() {
		for {
			select {
			case <-conn.done:
				return
			case <-time.After(50 * time.Millisecond):
				if len(conn.out.GetBytes()) > 0 {
					conn.WritePacket(&conn.out)
					conn.out.Reset()
				}
			}
		}
	}()
	return &conn
}

func (conn *GSConn) DecryptBytes(data []byte) {
	if conn.dec != nil {
		conn.dec.XORKeyStream(data, data)
	}
}

func (conn *GSConn) HandleBytes(data []byte) (consumed int, err error) {
	if len(data) < 2 {
		return 0, nil
	}

	in := GwPacket.NewIn(data)
	op, _ := in.Uint16()
	//conn.log.Debug().Str("opcode", fmt.Sprintf("%04x", op)).Msg("recv")

	if handler, ok := packetHandlers[op]; ok {
		consumed, err = handler(conn, &in)
	} else {
		consumed = len(data)
		conn.log.Warn().Str("op", fmt.Sprintf("%04x", op)).Hex("data", data).Msg("unhandled packet")
	}

	/*if len(conn.out.GetBytes()) > 0 {
		conn.WritePacket(&conn.out)
		conn.out.Reset()
	}*/

	if err != nil {
		err = fmt.Errorf("HandleBytes(op=%04x): %w", op, err)
	}

	return consumed, err
}

func (conn *GSConn) Read(buf []byte) (int, error) {
	return conn.socket.Read(buf)
}

func (conn *GSConn) WritePacket(packet *GwPacket.Out) error {
	bts := packet.GetBytes()
	if conn.enc != nil {
		conn.enc.XORKeyStream(bts, bts)
	}
	_, err := conn.socket.Write(bts)
	return err
}

func (conn *GSConn) EnqueuePacket(packet GwPacket.Out) {
	conn.out.Merge(packet)
}
