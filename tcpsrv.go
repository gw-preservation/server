package gw1

import (
	"errors"
	"gw1/server/authservice"
	"gw1/server/fileservice"
	"gw1/server/gameservice"
	"gw1/server/portalservice"
	"io"
	"net"
	"time"

	"github.com/charmbracelet/log"
	"github.com/rs/zerolog"
)

var logger zerolog.Logger

type tcpsrv struct {
	laddr        *net.TCPAddr
	listener     *net.TCPListener
	readTimeout  time.Duration
	writeTimeout time.Duration
}

type Option func(*tcpsrv)

func WithReadTimeout(d time.Duration) Option {
	return func(s *tcpsrv) { s.readTimeout = d }
}

func WithWriteTimeout(d time.Duration) Option {
	return func(s *tcpsrv) { s.writeTimeout = d }
}

type Transport interface {
	HandleBytes(data []byte) (int, error)
	Read(buf []byte) (int, error)
	Close()
}

func init() {
	// Set up the root logger (output to console @ trace level)
	writer := zerolog.NewConsoleWriter()
	logger = zerolog.New(writer)
	logger = logger.Level(zerolog.DebugLevel)
	logger = logger.With().Timestamp().Logger()
}

func NewTCPServer(opts ...Option) tcpsrv {
	addr, err := net.ResolveTCPAddr("tcp", ":6112")
	if err != nil {
		panic(err)
	}
	listener, err := net.ListenTCP("tcp4", addr)
	if err != nil || listener == nil {
		logger.Fatal().Msg("unable to bind to port 6112 - is a server already running?")
	}
	srv := tcpsrv{
		listener:     listener,
		laddr:        addr,
		readTimeout:  30 * time.Second,
		writeTimeout: 30 * time.Second,
	}
	for _, opt := range opts {
		opt(&srv)
	}
	return srv
}

func (srv tcpsrv) handleTCPConnection(conn *net.TCPConn) {
	logger.Info().Str("remoteAddr", conn.RemoteAddr().String()).Msg("connection established")
	var buffer []byte
	var transport Transport = nil
	var servicerName string
	tempBuffer := make([]byte, 32*1024)
	for {
		conn.SetReadDeadline(time.Now().Add(srv.readTimeout))
		var numBytesReadFromSocket int
		var err error
		if transport != nil {
			numBytesReadFromSocket, err = transport.Read(tempBuffer)
		} else {
			numBytesReadFromSocket, err = conn.Read(tempBuffer)
		}
		if err != nil {
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				logger.Info().Str("remoteAddr", conn.RemoteAddr().String()).Msg("read timed out")
			} else {
				logger.Info().Str("remoteAddr", conn.RemoteAddr().String()).Msg("connection closed")
			}
			if transport != nil {
				transport.Close()
			} else {
				conn.Close()
			}
			return
		}
		if numBytesReadFromSocket == 0 {
			logger.Error().Msg("0 bytes read from tcp socket")
			if transport != nil {
				transport.Close()
			} else {
				conn.Close()
			}
			return
		}
		readData := tempBuffer[:numBytesReadFromSocket]
		if client, ok := transport.(*authservice.ASConn); ok {
			client.DecryptBytes(readData)
		}
		if client, ok := transport.(*gameservice.GSConn); ok {
			client.DecryptBytes(readData)
		}
		buffer = append(buffer, readData...)
		if transport == nil {
			if len(buffer) == 21 {
				transport = fileservice.NewFSConn(conn, logger)
				servicerName = "file"
			} else if len(buffer) == 16 {
				transport = authservice.NewASConn(conn, logger)
				servicerName = "auth"
			} else if len(buffer) == 64 {
				transport = gameservice.NewGSConn(conn, logger, srv.writeTimeout)
				servicerName = "game"
			} else if len(buffer) > 6 && string(buffer[:3]) == "P /" {
				transport = portalservice.NewPSConn(conn, logger)
				servicerName = "portal"
			} else {
				logger.Error().Int("len", len(buffer)).Msg("unrecognised connection type")
				conn.Close()
				return
			}
		}
		if transport != nil {
			conn.SetWriteDeadline(time.Now().Add(srv.writeTimeout))
		}
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
			buffer = buffer[numConsumedThisTime:]
		}
	}
}

func (srv tcpsrv) Serve() {
	logger.Info().Int("port", srv.laddr.Port).Msg("waiting for connections")
	for {
		conn, err := srv.listener.AcceptTCP()
		if err != nil {
			log.Fatalf("error accepting tcp connection: %s", err.Error())
		}
		go srv.handleTCPConnection(conn)
	}
}
