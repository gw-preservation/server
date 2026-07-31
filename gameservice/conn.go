package gameservice

import (
	"crypto/rc4"
	"fmt"
	GwPacket "gw1/server/gwpacket"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
)

type GSConn struct {
	socket    *net.TCPConn
	enc       *rc4.Cipher
	dec       *rc4.Cipher
	out       GwPacket.Out
	outMu     sync.Mutex
	closed    atomic.Bool
	log       zerolog.Logger
	player    *Player
	done      chan struct{}
	closeOnce sync.Once
	accountID uint64
	verified  bool
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
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-conn.done:
				return
			case <-ticker.C:
				conn.outMu.Lock()
				if len(conn.out.GetBytes()) > 0 {
					if err := conn.writeLocked(&conn.out); err != nil {
						conn.log.Error().Err(err).Msg("flush write failed, closing")
						conn.outMu.Unlock()
						conn.Close()
						return
					}
					conn.out.Reset()
				}
				conn.outMu.Unlock()
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

func (conn *GSConn) IsClosed() bool {
	return conn.closed.Load()
}

func (conn *GSConn) HandleBytes(data []byte) (consumed int, err error) {
	if len(data) < 2 {
		return 0, nil
	}

	in := GwPacket.NewIn(data)
	op, _ := in.Uint16()
	//conn.log.Debug().Str("opcode", fmt.Sprintf("%04x", op)).Msg("recv")

	var handler packetHandler
	var ok bool
	if conn.verified {
		handler, ok = verifiedHandlers[op]
	} else {
		handler, ok = unverifiedHandlers[op]
	}
	if ok {
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

// writeLocked encrypts and writes a packet. The caller must hold conn.outMu.
func (conn *GSConn) writeLocked(packet *GwPacket.Out) error {
	bts := packet.GetBytes()
	if conn.enc != nil {
		conn.enc.XORKeyStream(bts, bts)
	}
	_, err := conn.socket.Write(bts)
	return err
}

func (conn *GSConn) WritePacket(packet *GwPacket.Out) error {
	conn.outMu.Lock()
	defer conn.outMu.Unlock()
	return conn.writeLocked(packet)
}

func prettyBytesString(in []byte) string {
	var sb strings.Builder

	for i, b := range in {
		// If we are starting a new line (and it's not the very first byte)
		if i > 0 && i%16 == 0 {
			sb.WriteString("\n")
		}
		// Append the hex representation followed by a space
		sb.WriteString(fmt.Sprintf("%02x ", b))
	}
	return sb.String()
}

func (conn *GSConn) EnqueuePacket(packet GwPacket.Out) {
	//bts := packet.GetBytes()
	//x := prettyBytesString(bts)
	//fmt.Printf("%s\n", x)
	conn.outMu.Lock()
	defer conn.outMu.Unlock()
	conn.out.Merge(packet)
}

func (conn *GSConn) clientIP() string {
	host, _, err := net.SplitHostPort(conn.socket.RemoteAddr().String())
	if err != nil {
		return conn.socket.RemoteAddr().String()
	}
	return host
}
