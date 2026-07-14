package srp

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/big"
	"slices"
	"time"
)

const (
	handshakeClientHello       uint8 = 1
	handshakeServerHello       uint8 = 2
	handshakeServerKeyExchange uint8 = 12
	handshakeServerHelloDone   uint8 = 14
	handshakeClientKeyExchange uint8 = 16
	handshakeFinished          uint8 = 20
)

type Handshake struct {
	Type uint8
	Body []byte
}

func ReadHandshake(rec *Record) (*Handshake, error) {
	if rec.Type != recordHandshake {
		return nil, fmt.Errorf("expected handshake record, got %d", rec.Type)
	}

	p := newParser(rec.Data)

	msgType, err := p.Uint8()
	if err != nil {
		return nil, err
	}

	length, err := p.Uint24()
	if err != nil {
		return nil, err
	}

	if int(length) != p.Remaining() {
		return nil, fmt.Errorf(
			"handshake length mismatch: header says %d bytes, have %d",
			length,
			p.Remaining(),
		)
	}

	body, err := p.Bytes(int(length))
	if err != nil {
		return nil, err
	}

	if !p.Empty() {
		return nil, fmt.Errorf("unexpected trailing handshake data")
	}

	return &Handshake{
		Type: msgType,
		Body: body,
	}, nil

}

func WriteHandshake(hs *Handshake) (*Record, error) {
	if len(hs.Body) > 0xffffff {
		return nil, fmt.Errorf("handshake message too large")
	}

	return &Record{
		Type:    recordHandshake,
		Version: tls12,
		Data:    hs.Bytes(),
	}, nil

}

func WriteHandshakeRecord(hs *Handshake) (*Record, error) {
	return WriteHandshake(hs)
}

func (hs *Handshake) Bytes() []byte {
	e := encoder{}

	e.Uint8(hs.Type)
	e.Uint24(uint32(len(hs.Body)))
	e.Bytes(hs.Body)

	return e.BytesSlice()

}

type ClientHello struct {
	Version            uint16
	Random             []byte
	SessionID          []byte
	CipherSuites       []uint16
	CompressionMethods []uint8
	SRPUsername        string
}

func ParseClientHello(hs *Handshake) (*ClientHello, error) {
	if hs.Type != handshakeClientHello {
		return nil, fmt.Errorf("expected ClientHello, got %d", hs.Type)
	}

	p := newParser(hs.Body)

	version, err := p.Uint16()
	if err != nil {
		return nil, err
	}

	random, err := p.Bytes(32)
	if err != nil {
		return nil, err
	}

	sessionID, err := p.Vector8()
	if err != nil {
		return nil, err
	}

	cipherSuites, err := p.Vector16Uint()
	if err != nil {
		return nil, err
	}

	compressionMethods, err := p.Vector8Uint()
	if err != nil {
		return nil, err
	}

	extensions, err := p.Vector16()
	if err != nil {
		return nil, err
	}

	if !p.Empty() {
		return nil, fmt.Errorf("trailing ClientHello data")
	}

	ch := &ClientHello{
		Version:            version,
		Random:             random,
		SessionID:          sessionID,
		CipherSuites:       cipherSuites,
		CompressionMethods: compressionMethods,
	}

	if err := parseClientHelloExtensions(ch, extensions); err != nil {
		return nil, err
	}

	return ch, nil

}

func parseClientHelloExtensions(ch *ClientHello, data []byte) error {
	p := newParser(data)

	for !p.Empty() {
		extensionType, err := p.Uint16()
		if err != nil {
			return err
		}

		extensionData, err := p.Vector16()
		if err != nil {
			return err
		}

		switch extensionType {
		case extensionSRP:
			username, err := parseSRPExtension(extensionData)
			if err != nil {
				return err
			}

			ch.SRPUsername = string(username)
		}
	}

	return nil

}

func parseSRPExtension(data []byte) ([]byte, error) {
	p := newParser(data)

	username, err := p.Vector8()
	if err != nil {
		return nil, err
	}

	if !p.Empty() {
		return nil, fmt.Errorf("trailing SRP extension data")
	}

	if len(username) == 0 {
		return nil, fmt.Errorf("empty SRP username")
	}

	return username, nil

}

func validateCipherSuites(ch *ClientHello) bool {
	return slices.Contains(ch.CipherSuites, TLS_SRP_SHA_WITH_AES_256_CBC_SHA)
}

func ValidateClientHello(ch *ClientHello) error {
	if ch.Version != tls12 {
		return fmt.Errorf("unsupported TLS version")
	}

	if !validateCipherSuites(ch) {
		return fmt.Errorf("no supported cipher suites: %v", ch.CipherSuites)
	}

	if len(ch.CompressionMethods) != 1 ||
		ch.CompressionMethods[0] != compressionNull {
		return fmt.Errorf("unsupported compression")
	}

	if len(ch.SRPUsername) == 0 {
		return fmt.Errorf("missing SRP username")
	}

	if len(ch.SessionID) != 0 {
		return fmt.Errorf("session resumption unsupported")
	}

	return nil

}

type ServerHello struct {
	Version           uint16
	Random            []byte
	SessionID         []byte
	CipherSuite       uint16
	CompressionMethod uint8
	Extensions        []byte
}

func (sh *ServerHello) Encode() *Handshake {
	e := encoder{}

	e.Uint16(sh.Version)
	e.Bytes(sh.Random)
	e.Vector8(sh.SessionID)
	e.Uint16(sh.CipherSuite)
	e.Uint8(sh.CompressionMethod)

	if len(sh.Extensions) > 0 {
		e.Vector16(sh.Extensions)
	}

	return &Handshake{
		Type: handshakeServerHello,
		Body: e.BytesSlice(),
	}

}

func NewServerHello() *ServerHello {
	sh := &ServerHello{
		Version:           tls12,
		Random:            make([]byte, 32),
		CipherSuite:       TLS_SRP_SHA_WITH_AES_256_CBC_SHA,
		CompressionMethod: compressionNull,
	}

	rand.Read(sh.Random)
	binary.BigEndian.PutUint32(sh.Random[:4], uint32(time.Now().Unix()))

	return sh

}

type ClientKeyExchange struct {
	A *big.Int
}

func ParseClientKeyExchange(hs *Handshake) (*ClientKeyExchange, error) {
	if hs.Type != handshakeClientKeyExchange {
		return nil, fmt.Errorf("expected ClientKeyExchange")
	}

	p := newParser(hs.Body)

	aBytes, err := p.Vector16()
	if err != nil {
		return nil, err
	}

	if !p.Empty() {
		return nil, fmt.Errorf("trailing ClientKeyExchange data")
	}

	A := new(big.Int).SetBytes(aBytes)

	if A.Sign() == 0 {
		return nil, fmt.Errorf("invalid SRP public value A")
	}

	return &ClientKeyExchange{A: A}, nil

}

type ServerKeyExchange struct {
	N    *big.Int
	G    *big.Int
	Salt []byte
	B    *big.Int
}

func encodeBigInt(v *big.Int) []byte {
	return v.Bytes()
}

func (ske *ServerKeyExchange) Encode() *Handshake {
	e := encoder{}

	e.Vector16(encodeBigInt(ske.N))
	e.Vector16(encodeBigInt(ske.G))
	e.Vector8(ske.Salt)
	e.Vector16(encodeBigInt(ske.B))

	return &Handshake{
		Type: handshakeServerKeyExchange,
		Body: e.BytesSlice(),
	}

}

func NewServerKeyExchange(server *SRPServer) *ServerKeyExchange {
	return &ServerKeyExchange{
		N:    server.Group.N,
		G:    server.Group.g,
		Salt: server.User.Salt,
		B:    server.B,
	}
}

func NewServerHelloDone() *Handshake {
	return &Handshake{
		Type: handshakeServerHelloDone,
		Body: nil,
	}
}

func ParseChangeCipherSpec(rec *Record) error {
	if rec.Type != recordChangeCipherSpec {
		return fmt.Errorf("expected ChangeCipherSpec record")
	}

	if len(rec.Data) != 1 || rec.Data[0] != 1 {
		return fmt.Errorf("invalid ChangeCipherSpec")
	}

	return nil

}

func NewChangeCipherSpec() *Record {
	return &Record{
		Type:    recordChangeCipherSpec,
		Version: tls12,
		Data:    []byte{1},
	}
}

type ServerConnection struct {
	Reader io.Reader
	Writer io.Writer

	Lookup SRPLookup

	Handshake ServerHandshake
}

func (c *ServerConnection) HandshakeIt() error {
	c.Handshake.Lookup = c.Lookup

	for {
		rec, err := ReadRecord(c.Reader)
		if err != nil {
			return err
		}

		switch c.Handshake.State {

		case stateInitial:
			if rec.Type != recordHandshake {
				return fmt.Errorf("expected handshake")
			}

			hs, err := ReadHandshake(rec)
			if err != nil {
				return err
			}

			if hs.Type != handshakeClientHello {
				return fmt.Errorf("expected ClientHello")
			}

			ch, err := ParseClientHello(hs)
			if err != nil {
				return err
			}

			c.Handshake.ClientHello = ch
			c.Handshake.Transcript.Add(hs)

			flight, err := c.Handshake.BuildServerFlight()
			if err != nil {
				return err
			}

			for _, msg := range flight {
				rec, err := WriteHandshakeRecord(msg)
				if err != nil {
					return err
				}

				if err := WriteRecord(c.Writer, rec); err != nil {
					return err
				}
			}

		case stateServerFlightSent:
			if rec.Type != recordHandshake {
				return fmt.Errorf("expected ClientKeyExchange")
			}

			hs, err := ReadHandshake(rec)
			if err != nil {
				return err
			}

			if err := c.Handshake.HandleClientKeyExchange(hs); err != nil {
				return err
			}

		case stateClientKeyExchangeReceived:
			if err := ParseChangeCipherSpec(rec); err != nil {
				return err
			}

			if err := c.Handshake.ActivateReadCipher(); err != nil {
				return err
			}

		case stateWaitingForFinished:
			if rec.Type != recordHandshake {
				return fmt.Errorf("expected Finished")
			}

			response, err := c.Handshake.HandleClientFinished(rec)
			if err != nil {
				if errors.Is(err, ErrBadRecordMAC) {
					_ = WriteRecord(
						c.Writer,
						NewAlert(alertFatal, alertBadRecordMAC),
					)
				}

				return err
			}

			if err := WriteRecord(c.Writer, NewChangeCipherSpec()); err != nil {
				return err
			}

			if err := WriteRecord(c.Writer, response); err != nil {
				return err
			}

			return nil

		default:
			return fmt.Errorf(
				"state %d not implemented",
				c.Handshake.State,
			)
		}
	}
}
