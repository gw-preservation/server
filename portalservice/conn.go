package PortalService

import (
	"errors"
	"fmt"
	"gw1/server/db"
	"gw1/server/portalservice/srp"
	Sts "gw1/server/portalservice/sts"
	"io"
	"net"

	"github.com/rs/zerolog"
)

type State int

const (
	StateInitial State = iota
	StateStartTls
	StateTlsUpgraded
	StateSentAccInfo
	StateSentAccCreationInfo
	StateSentGameToken
)

const (
	gameCodeGuildWars = "gw1"
)

const (
	pathConnect      = "/Sts/Connect"
	pathStartTls     = "/Auth/StartTls"
	pathLoginFinish  = "/Auth/LoginFinish"
	pathListAccounts = "/Auth/ListMyGameAccounts"
	pathRequestToken = "/Auth/RequestGameToken"
	pathPing         = "/Sts/Ping"
)

type PSConn struct {
	socket  *net.TCPConn
	tlsConn *srp.Conn
	state   State
	log     zerolog.Logger
	acc     db.Account
}

func NewPSConn(socket *net.TCPConn, logCtx zerolog.Logger) *PSConn {
	return &PSConn{
		state:   StateInitial,
		socket:  socket,
		tlsConn: nil,
		log:     logCtx.With().Str("srv", "portal").Logger(),
	}
}

func (conn *PSConn) HandleBytes(data []byte) (int, error) {
	msg, err := Sts.UnmarshalReqMsg(data)
	if err != nil {
		if errors.Is(err, io.ErrUnexpectedEOF) {
			return 0, nil
		}
		return 0, err
	}
	conn.log.Debug().Str("action", msg.Header.Action).Str("resource", msg.Header.Resource).Msg("")
	hdr := msg.Header
	if hdr.Action == "P" {
		switch conn.state {
		case StateInitial:
			if hdr.Resource == pathConnect {
				conn.state = StateStartTls
			}
		case StateStartTls:
			if hdr.Resource == pathStartTls {
				err = conn.handleStartTls(msg)
			}
		case StateTlsUpgraded:
			if hdr.Resource == pathLoginFinish {
				err = conn.handleLoginFinish(msg)
			}
		case StateSentAccInfo:
			if hdr.Resource == pathListAccounts {
				err = conn.handleListGameAccounts(msg)
			}
		case StateSentAccCreationInfo:
			if hdr.Resource == pathRequestToken {
				err = conn.handleRequestGameToken(msg)
			}
		}
	}
	return msg.Length(), err
}

func lookup(username string) (*srp.SRPUser, error) {
	acc, ok := db.GetAccountByEmail(username)
	if !ok {
		return nil, fmt.Errorf("unknown user")
	}

	user, err := srp.CreateSRPUser(srp.SRP1024(), username, acc.Password)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (conn *PSConn) handleStartTls(msg Sts.ReqMsg) error {
	m := Sts.NewErrorRespMsg(400, msg.Header.Seq, "1001", "2", "1146")
	err := conn.Write([]byte(m))
	if err != nil {
		return err
	}
	conn.tlsConn = srp.Server(conn.socket, lookup)
	if err := conn.tlsConn.Handshake(); err != nil {
		conn.log.Warn().Str("email", conn.tlsConn.Username()).Msg("failed login")
		conn.Close()
		return nil
	}

	// Handshake OK
	verifiedEmail := conn.tlsConn.Username()
	conn.log.Debug().Str("email", verifiedEmail).Msg("SRP Verified!")
	var ok bool
	if conn.acc, ok = db.GetAccountByEmail(verifiedEmail); !ok {
		// this should never be reached - we already verified their credentials
		conn.log.Error().Str("email", verifiedEmail).Msg("GetAccountByEmail !ok for a verified connection")
		return errors.New("database error")
	}
	conn.state = StateTlsUpgraded
	return nil
}

func (conn *PSConn) handleLoginFinish(msg Sts.ReqMsg) error {
	conn.state = StateSentAccInfo
	// Send account info
	accInfo := Sts.NewAccountInfoMsg(200, msg.Header.Seq, db.UUIDStr(conn.acc.UUID), 4, ":Unused.1234", "00010203-0405-0607-0809-0A0B0C0D0E0F", 1)
	conn.Write([]byte(accInfo))
	return nil
}

func (conn *PSConn) handleListGameAccounts(msg Sts.ReqMsg) error {
	pl := msg.Payload.(*Sts.PayloadListGameAccounts)
	if pl.GameCode != gameCodeGuildWars {
		return fmt.Errorf("unexpected GameCode %s", pl.GameCode)
	}
	conn.state = StateSentAccCreationInfo
	creationInfo := Sts.NewAccountCreationInfoMsg(200, msg.Header.Seq, gameCodeGuildWars, gameCodeGuildWars, "2019-12-02T12:01:02Z")
	conn.Write(creationInfo)

	return nil
}

func (conn *PSConn) handleRequestGameToken(msg Sts.ReqMsg) error {
	pl := msg.Payload.(*Sts.PayloadRequestGameToken)
	if pl.GameCode != gameCodeGuildWars {
		return fmt.Errorf("unexpected GameCode %s", pl.GameCode)
	}
	connectionToken := generateConnectionToken(conn.acc.ID)
	gameToken := Sts.NewGameTokenMsg(200, msg.Header.Seq, connectionToken)

	conn.state = StateSentGameToken
	conn.Write(gameToken)
	return nil
}

func (conn *PSConn) Read(buf []byte) (int, error) {
	if conn.tlsConn != nil {
		// If TLS layer is activated, use that instead
		return conn.tlsConn.Read(buf)
	}
	return conn.socket.Read(buf)
}

func (conn *PSConn) Write(buf []byte) error {
	if conn.tlsConn != nil {
		_, err := conn.tlsConn.Write(buf)
		return err
	}
	_, err := conn.socket.Write(buf)
	return err
}

func (conn *PSConn) Close() {
	if conn.tlsConn != nil {
		conn.tlsConn.Close()
	}
	conn.socket.Close()
}
