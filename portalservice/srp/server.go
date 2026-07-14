package srp

import (
	"crypto/hmac"
	"crypto/sha1"
	"fmt"
	"math/big"
)

type ServerState uint8

const (
	stateInitial ServerState = iota
	stateServerFlightSent
	stateClientKeyExchangeReceived
	stateWaitingForFinished
	stateClientFinishedReceived
	stateEstablished
)

type ServerHandshake struct {
	State ServerState

	Lookup SRPLookup

	Transcript HandshakeTranscript

	ClientHello *ClientHello
	ServerHello *ServerHello

	SRP *SRPServer

	MasterSecret []byte

	ReadCipher  *CipherState
	WriteCipher *CipherState
}

func (h *ServerHandshake) addHandshake(hs *Handshake) {
	h.Transcript.Add(hs)
}
func CreateSRPUser(group *SRPGroup, username, password string) (*SRPUser, error) {
	salt := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}

	inner := sha1.Sum([]byte(username + ":" + password))

	h := sha1.New()
	h.Write(salt)
	h.Write(inner[:])

	xBytes := h.Sum(nil)
	x := new(big.Int).SetBytes(xBytes)

	v := new(big.Int).Exp(group.g, x, group.N)

	return &SRPUser{
		Username: username,
		Salt:     salt,
		Verifier: v,
	}, nil
}

func SRPVerifier(username, password string, salt []byte, group *SRPGroup) *big.Int {

	inner := sha1.Sum([]byte(username + ":" + password))

	h := sha1.New()
	h.Write(salt)
	h.Write(inner[:])

	xBytes := h.Sum(nil)
	x := new(big.Int).SetBytes(xBytes)

	return new(big.Int).Exp(group.g, x, group.N)
}

func (h *ServerHandshake) BuildServerFlight() ([]*Handshake, error) {
	if h.ClientHello == nil {
		return nil, fmt.Errorf("missing ClientHello")
	}

	if err := ValidateClientHello(h.ClientHello); err != nil {
		return nil, err
	}

	if h.Lookup == nil {
		return nil, fmt.Errorf("missing SRP lookup")
	}

	user, err := h.Lookup(h.ClientHello.SRPUsername)
	if err != nil {
		return nil, err
	}

	srp, err := NewSRPServer(
		SRP1024(),
		user,
	)

	if err != nil {
		return nil, err
	}

	h.SRP = srp

	serverHello := NewServerHello()

	h.ServerHello = serverHello

	messages := []*Handshake{
		serverHello.Encode(),
		NewServerKeyExchange(srp).Encode(),
		NewServerHelloDone(),
	}

	for _, msg := range messages {
		h.addHandshake(msg)
	}

	h.State = stateServerFlightSent

	return messages, nil
}

func (h *ServerHandshake) HandleClientKeyExchange(
	hs *Handshake,
) error {
	if h.State != stateServerFlightSent {
		return fmt.Errorf("unexpected ClientKeyExchange")
	}

	cke, err := ParseClientKeyExchange(hs)
	if err != nil {
		return err
	}

	premaster, err := h.SRP.PremasterSecret(cke.A)
	if err != nil {
		return err
	}

	h.MasterSecret = deriveMasterSecret(
		premaster,
		h.ClientHello.Random,
		h.ServerHello.Random,
	)

	h.Transcript.Add(hs)

	h.State = stateClientKeyExchangeReceived

	return nil
}

func (h *ServerHandshake) ActivateReadCipher() error {
	if h.MasterSecret == nil {
		return fmt.Errorf("missing master secret")
	}

	block := deriveKeyBlock(
		h.MasterSecret,
		h.ClientHello.Random,
		h.ServerHello.Random,
	)

	keys, err := splitKeyBlock(block)
	if err != nil {
		return err
	}

	h.ReadCipher = &CipherState{
		MACKey: keys.ClientMACKey,
		Key:    keys.ClientWriteKey,
	}

	h.WriteCipher = &CipherState{
		MACKey: keys.ServerMACKey,
		Key:    keys.ServerWriteKey,
	}
	h.State = stateWaitingForFinished

	return nil
}

func (h *ServerHandshake) HandleClientFinished(
	rec *Record,
) (*Record, error) {
	if h.State != stateWaitingForFinished {
		return nil, fmt.Errorf("unexpected Finished (my state was %v)", h.State)
	}

	if h.ReadCipher == nil {
		return nil, fmt.Errorf("read cipher not active")
	}

	plaintext, err := h.ReadCipher.Decrypt(
		rec.Type,
		rec.Version,
		rec.Data,
	)

	if err != nil {
		return nil, err
	}

	hs, err := ParseHandshakeBytes(plaintext)
	if err != nil {
		return nil, err
	}

	finished, err := ParseFinished(hs)
	if err != nil {
		return nil, err
	}

	expected := GenerateFinished(
		h.MasterSecret,
		"client finished",
		h.Transcript.Hash(),
	)

	if !hmac.Equal(
		finished.VerifyData,
		expected.VerifyData,
	) {
		return nil, fmt.Errorf("invalid client Finished")
	}

	// Finished is included after verification.
	h.Transcript.Add(hs)

	h.State = stateClientFinishedReceived

	// Generate server Finished using the updated transcript.
	serverFinished := GenerateFinished(
		h.MasterSecret,
		"server finished",
		h.Transcript.Hash(),
	)

	h.Transcript.Add(serverFinished.Encode())

	return EncryptHandshake(
		h.WriteCipher,
		serverFinished.Encode(),
	)
}

func EncryptHandshake(
	cipher *CipherState,
	hs *Handshake,
) (*Record, error) {

	data, err := cipher.Encrypt(
		recordHandshake,
		tls12,
		hs.Bytes(),
	)

	if err != nil {
		return nil, err
	}

	return &Record{
		Type:    recordHandshake,
		Version: tls12,
		Data:    data,
	}, nil
}

func ParseHandshakeBytes(data []byte) (*Handshake, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("handshake too short")
	}

	hs := &Handshake{
		Type: data[0],
		Body: make([]byte, len(data)-4),
	}

	length := int(data[1])<<16 |
		int(data[2])<<8 |
		int(data[3])

	if length != len(data)-4 {
		return nil, fmt.Errorf("invalid handshake length")
	}

	copy(hs.Body, data[4:])

	return hs, nil
}

func (h *ServerHandshake) BuildServerFinishedFlight() ([]*Record, error) {
	if h.State != stateClientFinishedReceived {
		return nil, fmt.Errorf("unexpected state")
	}

	if h.WriteCipher == nil {
		return nil, fmt.Errorf("write cipher not active")
	}

	records := []*Record{
		NewChangeCipherSpec(),
	}

	finished := GenerateFinished(
		h.MasterSecret,
		"server finished",
		h.Transcript.Hash(),
	)

	hs := finished.Encode()

	h.Transcript.Add(hs)

	finishedRecord, err := EncryptHandshake(
		h.WriteCipher,
		hs,
	)

	if err != nil {
		return nil, err
	}

	records = append(
		records,
		finishedRecord,
	)

	h.State = stateEstablished

	return records, nil
}
