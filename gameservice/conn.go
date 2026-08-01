package gameservice

import (
	"crypto/rc4"
	"errors"
	"fmt"
	GwPacket "gw1/server/gwpacket"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
)

const (
	// maxInBufferSize bounds the decrypted bytes buffered per game connection
	// before the connection is dropped. This replaces the tcpsrv-level buffer
	// cap for game connections, whose input is now drained by the instance
	// actor on its game tick instead of immediately.
	maxInBufferSize = 512 * 1024

	// flushWriteTimeout bounds a single outbound socket write so a slow or
	// stuck client cannot stall the instance actor's per-tick flush.
	flushWriteTimeout = 100 * time.Millisecond
)

type State int

const (
	StateAwaitVerifyClientConnection State = iota
	StateAwaitClientSeed
	StateVerified
)

func (s State) String() string {
	switch s {
	case StateAwaitVerifyClientConnection:
		return "AwaitVerifyClientConnection"
	case StateAwaitClientSeed:
		return "AwaitClientSeed"
	case StateVerified:
		return "Verified"
	}
	return fmt.Sprintf("State(%d)", int(s))
}

type GSConn struct {
	socket    *net.TCPConn
	state     State
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

	// inBuf holds decrypted bytes that have not yet been parsed. The tcpsrv
	// read loop appends to it; packets are drained from it on the game tick by
	// the instance actor (in-instance) or immediately by the connection
	// goroutine (handshake / character creation).
	inBuf []byte
	inMu  sync.Mutex

	// handedOver is true once the player has joined an instance. From that
	// point the connection goroutine stops draining and the instance actor
	// owns the buffer. It is the ownership handoff, never set for
	// character-creation connections.
	handedOver atomic.Bool
}

func NewGSConn(socket *net.TCPConn, logCtx zerolog.Logger) *GSConn {
	conn := GSConn{
		socket: socket,
		state:  StateAwaitVerifyClientConnection,
		out:    GwPacket.NewOutRaw(),
		log:    logCtx.With().Str("srv", "game").Logger(),
		done:   make(chan struct{}),
	}
	conn.player = NewPlayer(&conn, logCtx)
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

// HandleBytes appends decrypted data to the connection's input buffer and
// drains as many complete packets as are available for this connection's
// current context (handshake, character creation, or in-instance). It exists
// to satisfy the tcpsrv Transport interface and for tests; the live read loop
// uses AppendIn + DrainHandshake, and the instance actor drains in-instance
// buffers via DrainInInstance on its game tick.
func (conn *GSConn) HandleBytes(data []byte) (consumed int, err error) {
	if err := conn.AppendIn(data); err != nil {
		return 0, err
	}
	return conn.drain()
}

// AppendIn buffers decrypted bytes for later draining. It is called by the
// tcpsrv read loop asynchronously from the actor's per-tick drain.
func (conn *GSConn) AppendIn(data []byte) error {
	conn.inMu.Lock()
	defer conn.inMu.Unlock()
	if len(conn.inBuf)+len(data) > maxInBufferSize {
		return fmt.Errorf("game input buffer exceeded limit (%d bytes)", maxInBufferSize)
	}
	conn.inBuf = append(conn.inBuf, data...)
	return nil
}

// DrainHandshake processes packets while the connection is not yet owned by an
// instance actor: the login handshake, then character-creation packets. It is
// a no-op once the player has joined an instance. Buffered output is flushed
// at the end so handshake/char-creation responses go out immediately; after
// the handoff the instance actor owns flushing.
func (conn *GSConn) DrainHandshake() error {
	for !conn.handedOver.Load() && !conn.closed.Load() {
		n, err := conn.drain()
		if err != nil {
			return err
		}
		if n == 0 {
			break
		}
	}
	if conn.handedOver.Load() {
		return nil
	}
	return conn.Flush()
}

// DrainInInstance parses and processes the connection's buffered packets. It
// must only be called from the instance actor's game tick (phase 1), after
// the handshake handoff, so the packets it dispatches only record requests on
// the player and never touch world state.
func (conn *GSConn) DrainInInstance() error {
	_, err := conn.drain()
	return err
}

func (conn *GSConn) HandedOver() bool {
	return conn.handedOver.Load()
}

// drain consumes complete packets from inBuf, dispatching each to the handler
// table that matches the connection's current context. It returns the number
// of bytes consumed. A partial packet (io.ErrUnexpectedEOF) leaves its bytes
// buffered for a later drain, mirroring the old fragmentation handling.
func (conn *GSConn) drain() (consumed int, err error) {
	if conn.closed.Load() {
		return 0, nil
	}
	for {
		conn.inMu.Lock()
		if len(conn.inBuf) == 0 {
			conn.inMu.Unlock()
			return consumed, nil
		}
		buf := conn.inBuf
		conn.inMu.Unlock()

		var handlers map[int]packetHandler
		switch {
		case conn.state < StateVerified:
			handlers = handshakeHandlers
		case conn.handedOver.Load():
			handlers = inInstanceHandlers
		default:
			handlers = charCreationHandlers
		}

		n, perr := conn.processOne(buf, handlers)
		if perr != nil {
			if errors.Is(perr, io.ErrUnexpectedEOF) {
				return consumed, nil
			}
			return consumed, perr
		}
		if n == 0 {
			return consumed, nil
		}
		conn.inMu.Lock()
		n = min(n, len(conn.inBuf))
		conn.inBuf = conn.inBuf[n:]
		conn.inMu.Unlock()
		consumed += n
	}
}

// processOne parses and dispatches a single packet from the head of data using
// the given handler table. It returns the number of bytes that packet spans.
func (conn *GSConn) processOne(data []byte, handlers map[int]packetHandler) (consumed int, err error) {
	if len(data) < 2 {
		return 0, nil
	}

	in := GwPacket.NewIn(data)
	op, _ := in.Uint16()

	if !conn.allowedOp(op) {
		return 0, fmt.Errorf("[%04x] unexpected for %v connection; len=%d", op, conn.state, in.Remaining())
	}

	handler, ok := handlers[op]
	if !ok {
		if conn.state < StateVerified {
			return 0, fmt.Errorf("[%04x] no handler for %v connection; len=%d", op, conn.state, in.Remaining())
		}
		conn.log.Warn().Str("op", fmt.Sprintf("%04x", op)).Hex("data", data).Msg("unhandled packet")
		return len(data), nil
	}

	consumed, err = handler(conn, &in)
	if err != nil {
		err = fmt.Errorf("HandleBytes(op=%04x): %w", op, err)
	}
	return consumed, err
}

func (conn *GSConn) allowedOp(op int) bool {
	if op == 0x8008 {
		return true
	}
	switch conn.state {
	case StateAwaitVerifyClientConnection:
		return op == 0x0500
	case StateAwaitClientSeed:
		return op == 0x4200
	case StateVerified:
		return op != 0x0500 && op != 0x4200
	}
	return false
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

// Flush writes any buffered output to the socket under a bounded write
// deadline so a slow client cannot stall the instance actor's game tick. The
// caller (conn goroutine during handshake, instance actor during the tick) is
// responsible for disconnecting on error.
func (conn *GSConn) Flush() error {
	conn.outMu.Lock()
	defer conn.outMu.Unlock()
	if len(conn.out.GetBytes()) == 0 {
		return nil
	}
	conn.socket.SetWriteDeadline(time.Now().Add(flushWriteTimeout))
	err := conn.writeLocked(&conn.out)
	conn.socket.SetWriteDeadline(time.Time{})
	if err != nil {
		conn.log.Error().Err(err).Msg("flush write failed")
		return err
	}
	conn.out.Reset()
	return nil
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

// Close tears down the connection. It removes the player from any connected
// instance via the (blocking) instance mailbox. It must not be called from the
// instance actor unless connectedInstance has already been cleared (e.g. after
// removePlayer), otherwise it would deliver to its own actor and deadlock.
func (conn *GSConn) Close() {
	conn.closeOnce.Do(func() {
		conn.closed.Store(true)
		close(conn.done)
		if inst := conn.player.connectedInstance.Load(); inst != nil {
			inst.RemovePlayer(conn.player)
		}
		if conn.accountID != 0 {
			UntrackAccount(conn.accountID)
		}
		conn.socket.Close()
	})
}
