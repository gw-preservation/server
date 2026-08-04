package gameservice

import (
	"crypto/rc4"
	"errors"
	"fmt"
	"gw1/server/packet"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
)

const (
	maxInBufferSize   = 512 * 1024
	flushWriteTimeout = 100 * time.Millisecond
)

type flushRequest struct {
	done chan error // nil for async flush
}

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
	out       packet.Out
	outMu     sync.Mutex
	closed    atomic.Bool
	log       zerolog.Logger
	player    *Player
	done      chan struct{}
	closeOnce sync.Once
	accountID uint64

	flushCh    chan flushRequest
	writerDone chan struct{}

	inBuf      []byte
	inMu       sync.Mutex
	handedOver atomic.Bool
}

func NewGSConn(socket *net.TCPConn, logCtx zerolog.Logger) *GSConn {
	conn := GSConn{
		socket:     socket,
		state:      StateAwaitVerifyClientConnection,
		out:        packet.NewOutRaw(),
		log:        logCtx.With().Str("srv", "game").Logger(),
		done:       make(chan struct{}),
		flushCh:    make(chan flushRequest, 1),
		writerDone: make(chan struct{}),
	}
	conn.player = NewPlayer(&conn, logCtx)
	go conn.writeLoop()
	return &conn
}

func (conn *GSConn) DecryptBytes(data []byte) {
	if conn.dec != nil {
		conn.dec.XORKeyStream(data, data)
	}
}

// responsible for writing to the underlying socket in a dedicated goroutine.
func (conn *GSConn) writeLoop() {
	// when this ends, send writerDone signal to allow Close() to return
	defer close(conn.writerDone)
	for {
		select {
		case <-conn.done:
			return
		case req := <-conn.flushCh:
			for {
				if err := conn.doFlush(); err != nil {
					if req.done != nil {
						req.done <- err
					}
					conn.Close()
					return
				}
				conn.outMu.Lock()
				hasMore := len(conn.out.GetBytes()) > 0
				conn.outMu.Unlock()
				if !hasMore {
					break
				}
			}
			if req.done != nil {
				req.done <- nil
			}
		}
	}
}

// internal flush method - should be called from a dedicated goroutine as it might block
func (conn *GSConn) doFlush() error {
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

func (conn *GSConn) IsClosed() bool {
	return conn.closed.Load()
}

func (conn *GSConn) HandedOver() bool {
	return conn.handedOver.Load()
}

// Take a slice of bytes from a socket and append to the input buffer.
func (conn *GSConn) AppendIn(data []byte) error {
	conn.inMu.Lock()
	defer conn.inMu.Unlock()
	if len(conn.inBuf)+len(data) > maxInBufferSize {
		return fmt.Errorf("game input buffer exceeded limit (%d bytes)", maxInBufferSize)
	}
	conn.inBuf = append(conn.inBuf, data...)
	return nil
}

// To be used during the handshake phase; repeatedly processes all
func (conn *GSConn) DrainHandshake() error {
	for !conn.handedOver.Load() && !conn.closed.Load() {
		n, err := conn.processAll()
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

func (conn *GSConn) DrainInInstance() error {
	_, err := conn.processAll()
	return err
}

func (conn *GSConn) HandleBytes(data []byte) (consumed int, err error) {
	if err := conn.AppendIn(data); err != nil {
		return 0, err
	}
	return conn.processAll()
}

// Flush flushes the output buffer to the socket, blocking until complete or an error occurs.
func (conn *GSConn) Flush() error {
	req := flushRequest{done: make(chan error, 1)}
	select {
	case conn.flushCh <- req:
	default:
		return nil
	}
	return <-req.done
}

// FlushAsync requests a background flush of the output buffer. It does not block and does not report errors.
func (conn *GSConn) FlushAsync() {
	select {
	case conn.flushCh <- flushRequest{}:
	default:
	}
}

// processAll processes all complete packets in the input buffer, returning the number of bytes consumed and any error encountered.
func (conn *GSConn) processAll() (consumed int, err error) {
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

		n, perr := conn.processOne(buf)
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

// processOne takes up to one complete packet from the input buffer, returning the number of bytes consumed and any error encountered.
func (conn *GSConn) processOne(data []byte) (consumed int, err error) {
	if len(data) < 2 {
		return 0, nil
	}

	in := packet.NewIn(data)
	op, _ := in.Uint16()

	//conn.log.Info().Str("op", fmt.Sprintf("%04x", op)).Hex("data", data).Msg("received packet")
	if !conn.allowedOp(op) {
		return 0, fmt.Errorf("[%04x] unexpected for %v connection; len=%d", op, conn.state, in.Remaining())
	}

	var handlers map[int]packetHandler
	if conn.handedOver.Load() {
		handlers = inGameHandlers
	} else {
		handlers = preGameHandlers
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

func (conn *GSConn) writeLocked(packet *packet.Out) error {
	bts := packet.GetBytes()
	if conn.enc != nil {
		conn.enc.XORKeyStream(bts, bts)
	}
	_, err := conn.socket.Write(bts)
	return err
}

func (conn *GSConn) WritePacket(packet *packet.Out) error {
	conn.outMu.Lock()
	defer conn.outMu.Unlock()
	return conn.writeLocked(packet)
}

func (conn *GSConn) EnqueuePacket(packet packet.Out) {
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

func (conn *GSConn) Close() {
	conn.closeOnce.Do(func() {
		conn.closed.Store(true)
		close(conn.done)
		if conn.accountID != 0 {
			UntrackAccount(conn.accountID)
		}
		conn.socket.Close()
		<-conn.writerDone
	})
}
