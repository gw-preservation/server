package gw1

import (
	"errors"
	"gw1/server/authservice"
	"gw1/server/fileservice"
	"gw1/server/gameservice"
	"gw1/server/portalservice"
	"io"
	"net"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

var logger zerolog.Logger

type tcpsrv struct {
	laddr    *net.TCPAddr
	listener *net.TCPListener
}

type Transport interface {
	HandleBytes(data []byte) (int, error)
	Read(buf []byte) (int, error)
	Close()
}

func init() {
	// Set up the root logger (output to console)
	writer := zerolog.NewConsoleWriter()
	logger = zerolog.New(writer)
	logger = logger.With().Timestamp().Logger()
}

func NewTCPServer() tcpsrv {
	addr, err := net.ResolveTCPAddr("tcp", ":6112")
	if err != nil {
		panic(err)
	}
	listener, err := net.ListenTCP("tcp4", addr)
	if err != nil || listener == nil {
		logger.Fatal().Msg("unable to bind to port 6112 - is a server already running?")
	}
	return tcpsrv{
		listener: listener,
		laddr:    addr,
	}
}

func (srv tcpsrv) handleTCPConnection(conn *net.TCPConn) {
	logger.Info().Str("remoteAddr", conn.RemoteAddr().String()).Msg("connection established")
	// can buffer up to 512kb without a handler consuming any data before the server will close the connection
	const maxBufferSize = 512 * 1024
	var buffer []byte
	var transport Transport = nil
	var servicerName string
	var socketDeadline time.Time
	tempBuffer := make([]byte, 32*1024)
	for {
		// set idle timeout
		socketDeadline = time.Now().Add(5 * time.Minute)
		if servicerName == "game" {
			socketDeadline = time.Now().Add(2 * time.Minute)
		}
		conn.SetDeadline(socketDeadline)

		// read from socket
		var numBytesReadFromSocket int
		var err error
		if transport != nil {
			numBytesReadFromSocket, err = transport.Read(tempBuffer)
		} else {
			numBytesReadFromSocket, err = conn.Read(tempBuffer)
		}
		if err != nil {
			// error during read
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				logger.Info().Str("remoteAddr", conn.RemoteAddr().String()).Str("servicer", servicerName).Msg("read timed out")
			} else {
				logger.Info().Str("remoteAddr", conn.RemoteAddr().String()).Msg("connection closed")
			}
			// signal transport to close if open
			if transport != nil {
				transport.Close()
			} else {
				conn.Close()
			}
			return
		}
		if numBytesReadFromSocket == 0 {
			// empty read means disconnected
			logger.Error().Msg("0 bytes read from tcp socket")
			if transport != nil {
				transport.Close()
			} else {
				conn.Close()
			}
			return
		}
		// append (decrypted) bytes to working buffer:
		readData := tempBuffer[:numBytesReadFromSocket]
		if client, ok := transport.(*authservice.ASConn); ok {
			client.DecryptBytes(readData)
		}
		if client, ok := transport.(*gameservice.GSConn); ok {
			client.DecryptBytes(readData)
		}
		buffer = append(buffer, readData...)
		if len(buffer) > maxBufferSize {
			logger.Warn().Str("remoteAddr", conn.RemoteAddr().String()).Msg("buffer exceeded limit, disconnecting")
			if transport != nil {
				transport.Close()
			} else {
				conn.Close()
			}
			return
		}

		// figure out what transport type it is if we haven't already
		if transport == nil {
			if len(buffer) == 21 {
				transport = fileservice.NewFSConn(conn, logger)
				servicerName = "file"
			} else if len(buffer) == 16 {
				transport = authservice.NewASConn(conn, logger)
				servicerName = "auth"
			} else if len(buffer) == 64 {
				transport = gameservice.NewGSConn(conn, logger)
				servicerName = "game"
			} else if len(buffer) > 6 && string(buffer[:3]) == "P /" {
				transport = portalservice.NewPSConn(conn, logger)
				servicerName = "portal"
			} else if len(buffer) > 64 {
				logger.Warn().Int("len", len(buffer)).Str("remoteAddr", conn.RemoteAddr().String()).Msg("unrecognised connection type")
				conn.Close()
				return
			}
		}

		if transport == nil {
			continue
		}

		if client, ok := transport.(*gameservice.GSConn); ok {
			if len(buffer) > 0 {
				if err := client.AppendIn(buffer); err != nil {
					logger.Err(err).Str("servicer", servicerName).Msg("game input buffer overflow")
					transport.Close()
					return
				}
				buffer = nil
			}
			if err := client.DrainHandshake(); err != nil {
				logger.Err(err).Str("servicer", servicerName).Msg("game handshake error")
				transport.Close()
				return
			}
			continue
		}

		// consume as many bytes as we can
		for len(buffer) > 0 {
			numConsumedThisTime, err := transport.HandleBytes(buffer)
			if err != nil {
				if errors.Is(err, io.ErrUnexpectedEOF) {
				} else {
					logger.Err(err).Str("servicer", servicerName).Msg("servicer reported error")
					transport.Close()
					return
				}
			}
			if numConsumedThisTime == 0 {
				if len(buffer) >= 2 {
					logger.Warn().Msgf("Possible message fragmentation! Partially read %d / %d bytes [%02x%02x]", numConsumedThisTime, len(buffer), buffer[1], buffer[0])
				} else {
					logger.Warn().Msgf("Possible message fragmentation! Partially read %d / %d bytes", numConsumedThisTime, len(buffer))
				}
				break
			}
			if numConsumedThisTime > len(buffer) {
				logger.Warn().Str("servicer", servicerName).Msgf("servicer consumed more bytes than available (%d > %d), clamping", numConsumedThisTime, len(buffer))
				numConsumedThisTime = len(buffer)
			}
			buffer = buffer[numConsumedThisTime:]
		}
	}
}

const (
	expectedConnsPerClient = 4 // each real client uses 4 at once - File, Portal, Auth, Game
	maxConnsPerIP          = expectedConnsPerClient * 10
	maxGlobalConns         = expectedConnsPerClient * 100
)

type connLimiter struct {
	mu    sync.Mutex
	byIP  map[string]int
	total int
}

var activeConns = connLimiter{byIP: make(map[string]int)}

func connKey(addr net.Addr) string {
	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		return addr.String()
	}
	return host
}

func (l *connLimiter) tryAcquire(addr net.Addr) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.total >= maxGlobalConns {
		return false
	}
	ip := connKey(addr)
	if l.byIP[ip] >= maxConnsPerIP {
		return false
	}
	l.byIP[ip]++
	l.total++
	return true
}

func (l *connLimiter) release(addr net.Addr) {
	l.mu.Lock()
	defer l.mu.Unlock()
	ip := connKey(addr)
	if l.byIP[ip] > 0 {
		l.byIP[ip]--
	}
	if l.byIP[ip] == 0 {
		delete(l.byIP, ip)
	}
	if l.total > 0 {
		l.total--
	}
}

func (srv tcpsrv) Serve() {
	logger.Info().Int("port", srv.laddr.Port).Msg("waiting for connections")
	for {
		conn, err := srv.listener.AcceptTCP()
		if err != nil {
			logger.Fatal().Msgf("error accepting tcp connection: %s", err.Error())
		}
		if !activeConns.tryAcquire(conn.RemoteAddr()) {
			logger.Warn().Str("remoteAddr", conn.RemoteAddr().String()).Msg("connection limit exceeded, rejecting")
			conn.Close()
			continue
		}
		remoteAddr := conn.RemoteAddr()
		go func() {
			defer activeConns.release(remoteAddr)
			srv.handleTCPConnection(conn)
		}()
	}
}
