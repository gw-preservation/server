package PortalService

import (
	"errors"
	"fmt"
	"gw1/server/db"
	Sts "gw1/server/portalservice/sts"
	"io"
	"net"

	"github.com/mjl-/go-tls-srp"
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

type PSConn struct {
	socket  *net.TCPConn
	tlsConn *tls.Conn
	state   State
	log     zerolog.Logger
	acc     db.Account
}

func NewPSConn(socket *net.TCPConn, logCtx zerolog.Logger) *PSConn {

	conn := PSConn{
		state:   StateInitial,
		socket:  socket,
		tlsConn: nil,
		log:     logCtx.With().Str("srv", "portal").Logger(),
	}
	conn.log.Info().Msg("new client")
	return &conn
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
	if msg.Header.Action == "P" && msg.Header.Resource == "/Sts/Connect" && conn.state == StateInitial {
		conn.state = StateStartTls
	} else if msg.Header.Action == "P" && msg.Header.Resource == "/Auth/StartTls" && conn.state == StateStartTls {
		err = conn.handleStartTls(msg)
	} else if msg.Header.Action == "P" && msg.Header.Resource == "/Auth/LoginFinish" && conn.state == StateTlsUpgraded {
		err = conn.handleLoginFinish(msg)
	} else if msg.Header.Action == "P" && msg.Header.Resource == "/Auth/ListMyGameAccounts" && conn.state == StateSentAccInfo {
		err = conn.handleListGameAccounts(msg)
	} else if msg.Header.Action == "P" && msg.Header.Resource == "/Auth/RequestGameToken" && conn.state == StateSentAccCreationInfo {
		err = conn.handleRequestGameToken(msg)
	} else if msg.Header.Resource == "/Sts/Ping" {
	} else {
		return msg.Length(), fmt.Errorf("unexpected Sts message '%s %s'", msg.Header.Action, msg.Header.Resource)
	}
	return msg.Length(), err
}

type A struct {
}

func (a A) Lookup(user string) (v, s []byte, grp tls.SRPGroup, err error) {
	grp = tls.SRPGroup1024
	acc, ok := db.GetAccountByEmail(user)
	if !ok {
		return nil, nil, grp, nil
	}
	salt := []byte("salt") // this should come from the database i think?
	v = tls.SRPVerifier(user, acc.Password, salt, grp)
	return v, salt, grp, nil
}

var AA A

func (conn *PSConn) handleStartTls(msg Sts.ReqMsg) error {
	m := Sts.NewErrorRespMsg(400, msg.Header.Seq, "1001", "2", "1146")
	err := conn.Write([]byte(m))
	if err != nil {
		return err
	}

	conn.tlsConn = tls.Server(conn.socket, &tls.Config{
		SRPSaltKey:  "salt",
		SRPSaltSize: 4,
		SRPLookup:   AA,
	})
	err = conn.tlsConn.Handshake()
	if err != nil {
		return err
	}
	// Handshake OK
	verifiedEmail := conn.tlsConn.ConnectionState().SRPUser
	conn.log.Info().Str("email", verifiedEmail).Msg("SRP Verified!")
	var ok bool
	if conn.acc, ok = db.GetAccountByEmail(verifiedEmail); !ok {
		// this should never be reached - we already verified their credentials
		panic("suddenly !ok")
	}
	conn.state = StateTlsUpgraded
	return nil
}

func (conn *PSConn) handleLoginFinish(msg Sts.ReqMsg) error {
	conn.state = StateSentAccInfo
	// Send account info
	fmt.Printf("((Portal)) AccUUID=%s\n", db.UUIDStr(conn.acc.UUID))
	accInfo := Sts.NewAccountInfoMsg(200, msg.Header.Seq, db.UUIDStr(conn.acc.UUID), 4, ":Unused.1234", "00010203-0405-0607-0809-0A0B0C0D0E0F", 1)
	conn.Write([]byte(accInfo))
	return nil
}

func (conn *PSConn) handleListGameAccounts(msg Sts.ReqMsg) error {
	pl := msg.Payload.(*Sts.PayloadListGameAccounts)
	if pl.GameCode != "gw1" {
		return fmt.Errorf("unexpected GameCode %s", pl.GameCode)
	}
	conn.state = StateSentAccCreationInfo
	creationInfo := Sts.NewAccountCreationInfoMsg(200, msg.Header.Seq, "gw1", "gw1", "2019-12-02T12:01:02Z")
	conn.Write(creationInfo)

	return nil
}

func (conn *PSConn) handleRequestGameToken(msg Sts.ReqMsg) error {
	pl := msg.Payload.(*Sts.PayloadRequestGameToken)
	if pl.GameCode != "gw1" {
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
