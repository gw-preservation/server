package srp

import (
	"errors"
	"fmt"
	"net"
)

type Conn struct {
	net.Conn
	username string
	server   *ServerConnection

	readCipher  *CipherState
	writeCipher *CipherState

	plainBuf []byte
}

func Server(conn net.Conn, lookup SRPLookup) *Conn {
	return &Conn{
		Conn: conn,

		server: &ServerConnection{
			Reader: conn,
			Writer: conn,
			Lookup: lookup,
		},
	}
}

func (c *Conn) Username() string {
	return c.username
}

func (c *Conn) Handshake() error {
	err := c.server.HandshakeIt()
	if c.server.Handshake.ClientHello != nil {
		// also set in case of error
		c.username = c.server.Handshake.ClientHello.SRPUsername
	}
	if err != nil {
		return err
	}
	hs := c.server.Handshake

	c.readCipher = hs.ReadCipher
	c.writeCipher = hs.WriteCipher
	return nil
}

func (c *Conn) Read(buf []byte) (int, error) {
	for len(c.plainBuf) == 0 {

		rec, err := ReadRecord(c.Conn)
		if err != nil {
			return 0, err
		}

		switch rec.Type {

		case recordApplicationData:
			plaintext, err := c.readCipher.Decrypt(
				rec.Type,
				rec.Version,
				rec.Data,
			)

			if err != nil {
				if errors.Is(err, ErrBadRecordMAC) {
					WriteRecord(c.Conn, NewAlert(alertFatal, alertBadRecordMAC))
				}
				return 0, err
			}

			c.plainBuf = append(
				c.plainBuf,
				plaintext...,
			)

		case recordAlert:
			alert, err := ParseAlert(rec)
			if err != nil {
				return 0, err
			}
			return 0, alert

		default:
			return 0, fmt.Errorf(
				"unexpected TLS record %d",
				rec.Type,
			)
		}
	}

	n := copy(buf, c.plainBuf)
	c.plainBuf = c.plainBuf[n:]

	return n, nil
}
func (c *Conn) Write(buf []byte) (int, error) {

	recData, err := c.writeCipher.Encrypt(
		recordApplicationData,
		tls12,
		buf,
	)

	if err != nil {
		return 0, err
	}

	rec := &Record{
		Type:    recordApplicationData,
		Version: tls12,
		Data:    recData,
	}

	err = WriteRecord(c.Conn, rec)

	if err != nil {
		return 0, err
	}

	return len(buf), nil
}
