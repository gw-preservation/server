package PortalService

import (
	"errors"
	"fmt"
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

type Client struct {
	conn    *net.TCPConn
	tlsConn *tls.Conn
	state   State
	log     zerolog.Logger
}

func NewClient(conn *net.TCPConn, logCtx zerolog.Logger) *Client {

	client := Client{
		state:   StateInitial,
		conn:    conn,
		tlsConn: nil,
		log:     logCtx.With().Str("srv", "portal").Logger(),
	}
	client.log.Info().Msg("new client")
	return &client
}

func (client *Client) HandleBytes(data []byte) (int, error) {
	msg, err := Sts.UnmarshalReqMsg(data)
	if err != nil {
		if errors.Is(err, io.ErrUnexpectedEOF) {
			return 0, nil
		}
		return 0, err
	}
	client.log.Debug().Str("action", msg.Header.Action).Str("resource", msg.Header.Resource).Msg("")
	if msg.Header.Action == "P" && msg.Header.Resource == "/Sts/Connect" && client.state == StateInitial {
		client.state = StateStartTls
	} else if msg.Header.Action == "P" && msg.Header.Resource == "/Auth/StartTls" && client.state == StateStartTls {
		err = client.handleStartTls(msg)
	} else if msg.Header.Action == "P" && msg.Header.Resource == "/Auth/LoginFinish" && client.state == StateTlsUpgraded {
		err = client.handleLoginFinish(msg)
	} else if msg.Header.Action == "P" && msg.Header.Resource == "/Auth/ListMyGameAccounts" && client.state == StateSentAccInfo {
		err = client.handleListGameAccounts(msg)
	} else if msg.Header.Action == "P" && msg.Header.Resource == "/Auth/RequestGameToken" && client.state == StateSentAccCreationInfo {
		err = client.handleRequestGameToken(msg)
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
	salt := []byte("salt")
	v = tls.SRPVerifier(user, "p", salt, grp)
	return v, salt, grp, nil
}

var AA A

func (client *Client) handleStartTls(msg Sts.ReqMsg) error {
	m := Sts.NewErrorRespMsg(400, msg.Header.Seq, "1001", "2", "1146")
	err := client.Write([]byte(m))
	if err != nil {
		return err
	}

	client.tlsConn = tls.Server(client.conn, &tls.Config{
		SRPSaltKey:  "salt",
		SRPSaltSize: 4,
		SRPLookup:   AA,
	})
	err = client.tlsConn.Handshake()
	if err != nil {
		return err
	}
	// Handshake OK
	client.log.Info().Str("user", client.tlsConn.ConnectionState().SRPUser).Msg("SRP Verified!")
	client.state = StateTlsUpgraded
	return nil
}

func (client *Client) handleLoginFinish(msg Sts.ReqMsg) error {
	client.state = StateSentAccInfo
	// Send account info
	accInfo := Sts.NewAccountInfoMsg(200, msg.Header.Seq, "00010203-0405-0607-0809-0A0B0C0D0E0F", 4, ":Leo.1234", "00010203-0405-0607-0809-0A0B0C0D0E0F", 1)
	client.Write([]byte(accInfo))
	return nil
}

func (client *Client) handleListGameAccounts(msg Sts.ReqMsg) error {
	pl := msg.Payload.(*Sts.PayloadListGameAccounts)
	if pl.GameCode != "gw1" {
		return fmt.Errorf("unexpected GameCode %s", pl.GameCode)
	}
	client.state = StateSentAccCreationInfo
	creationInfo := Sts.NewAccountCreationInfoMsg(200, msg.Header.Seq, "gw1", "gw1", "2019-12-02T12:01:02Z")
	client.Write(creationInfo)

	return nil
}

func (client *Client) handleRequestGameToken(msg Sts.ReqMsg) error {
	pl := msg.Payload.(*Sts.PayloadRequestGameToken)
	if pl.GameCode != "gw1" {
		return fmt.Errorf("unexpected GameCode %s", pl.GameCode)
	}
	client.state = StateSentGameToken
	gameToken := Sts.NewGameTokenMsg(200, msg.Header.Seq, "00010203-0405-0607-0809-CAFEBABE8008")
	client.Write(gameToken)
	return nil
}

func (client *Client) Read(buf []byte) (int, error) {
	if client.tlsConn != nil {
		// If TLS layer is activated, use that instead
		return client.tlsConn.Read(buf)
	}
	return client.conn.Read(buf)
}

func (client *Client) Write(buf []byte) error {
	if client.tlsConn != nil {
		_, err := client.tlsConn.Write(buf)
		return err
	}
	_, err := client.conn.Write(buf)
	return err
}

func (client *Client) Close() {
	if client.tlsConn != nil {
		client.tlsConn.Close()
	}
	client.conn.Close()
}
