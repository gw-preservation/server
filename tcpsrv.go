package gw1

import (
	"errors"
	AuthService "gw1/server/authservice"
	FileService "gw1/server/fileservice"
	GameService "gw1/server/gameservice"
	PortalService "gw1/server/portalservice"
	"io"
	"syscall"

	"net"

	"github.com/charmbracelet/log"
	"github.com/rs/zerolog"
)

var logger zerolog.Logger

type tcpsrv struct {
	laddr    *net.TCPAddr
	listener *net.TCPListener
}

type TCPServicer interface {
	HandleBytes(data []byte) (int, error)
	Read(buf []byte) (int, error)
	Close()
}

func init() {
	// Set up the root logger (output to console @ trace level)
	logger = zerolog.New(zerolog.NewConsoleWriter())
	logger = logger.Level(zerolog.InfoLevel)
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
	srv := tcpsrv{
		listener: listener,
		laddr:    addr,
	}
	return srv
}

func (srv tcpsrv) handleTCPConnection(conn *net.TCPConn) {
	log.Debugf("connection from %v established", conn.RemoteAddr())
	var buffer []byte // to store leftover data that wasn't consumed
	var servicer TCPServicer = nil
	var servicerName string
	var suffering = 0
	tempBuffer := make([]byte, 32*1024) // Buffer to read data into
	for {
		var numBytesReadFromSocket int
		var err error
		if servicer != nil {
			numBytesReadFromSocket, err = servicer.Read(tempBuffer)
		} else {
			numBytesReadFromSocket, err = conn.Read(tempBuffer)
		}
		if err != nil {
			if err != io.EOF && !errors.Is(err, net.ErrClosed) && !errors.Is(err, syscall.ECONNRESET) {
				log.Errorf("error reading from tcp socket: %s", err)
			}
			log.Infof("connection from %s closed", conn.RemoteAddr())
			conn.Close()
			return
		}
		if numBytesReadFromSocket == 0 {
			log.Errorf("0 bytes read from tcp socket!")
			conn.Close()
			return
		}
		// Add newly read data to the buffer
		buffer = append(buffer, tempBuffer[:numBytesReadFromSocket]...)
		if servicer == nil {
			//logger := RootLogger.With().Str("ip", conn.RemoteAddr().(*net.TCPAddr).IP.String()).Logger()
			// Determine what kind of connection this is for
			if len(buffer) == 21 {
				// FileClient
				servicer = FileService.NewClient(conn, logger)
				servicerName = "file"
			} else if len(buffer) == 16 {
				// AuthClient
				servicer = AuthService.NewClient(conn, logger)
				servicerName = "auth"
			} else if len(buffer) == 64 {
				// GameClient
				servicer = GameService.NewClient(conn, logger)
				servicerName = "game"
			} else if len(buffer) > 6 && string(buffer[:3]) == "P /" {
				// PortalClient
				servicer = PortalService.NewClient(conn, logger)
				servicerName = "portal"
			} else {
				logger.Error().Msg("unrecognised connection type")
				conn.Close()
				return
			}
		}
		if client, ok := servicer.(*AuthService.Client); ok {
			if suffering > 0 {
				client.DecryptBytes(buffer[suffering:])
			} else {
				client.DecryptBytes(buffer)
			}
		}
		if client, ok := servicer.(*GameService.Client); ok {
			if suffering > 0 {
				client.DecryptBytes(buffer[suffering:])
			} else {
				client.DecryptBytes(buffer)
			}
		}
		for len(buffer) > 0 {
			numConsumedThisTime, err := servicer.HandleBytes(buffer)
			if err != nil {
				if errors.Is(errors.Unwrap(err), io.ErrUnexpectedEOF) {
					// OK, just need more data
				} else {
					logger.Err(err).Str("servicer", servicerName).Msg("servicer reported error")
					servicer.Close()
					return
				}
			}
			if numConsumedThisTime == 0 {
				suffering = len(buffer)
				if len(buffer) >= 2 {
					logger.Warn().Msgf("Possible message fragmentation! Partially read %d / %d bytes [%02x%02x]", numConsumedThisTime, len(buffer), buffer[1], buffer[0])
				} else {
					logger.Warn().Msgf("Possible message fragmenation! Partially read %d / %d bytes", numConsumedThisTime, len(buffer))
				}
				break
			} else {
				suffering -= numConsumedThisTime
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
