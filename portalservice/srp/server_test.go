package srp

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func randSalt() []byte {
	res := make([]byte, 8)
	rand.Read(res)
	return res
}

// --- CreateSRPUser ---

func TestCreateSRPUser_Fields(t *testing.T) {
	g := SRP1024()
	user, err := CreateSRPUser(g, "alice", "secret123", randSalt())
	require.NoError(t, err)

	assert.Equal(t, "alice", user.Username)
	assert.NotEmpty(t, user.Salt)
	assert.Equal(t, 8, len(user.Salt))
	assert.NotNil(t, user.Verifier)
	assert.True(t, user.Verifier.Sign() > 0)
}

func TestCreateSRPUser_DifferentUsers(t *testing.T) {
	g := SRP1024()
	u1, _ := CreateSRPUser(g, "alice", "pass1", randSalt())
	u2, _ := CreateSRPUser(g, "bob", "pass2", randSalt())

	assert.NotEqual(t, u1.Verifier, u2.Verifier)
}

// --- SRPVerifier ---

// --- BuildServerFlight ---

func TestBuildServerFlight_Valid(t *testing.T) {
	g := SRP1024()
	user, _ := CreateSRPUser(g, "testuser", "testpass", randSalt())

	lookup := func(username string) (*SRPUser, error) {
		return user, nil
	}

	h := &ServerHandshake{
		Lookup: lookup,
		ClientHello: &ClientHello{
			Version:            tls12,
			Random:             make([]byte, 32),
			CipherSuites:       []uint16{TLS_SRP_SHA_WITH_AES_256_CBC_SHA},
			CompressionMethods: []uint8{compressionNull},
			SRPUsername:        "testuser",
			SessionID:          []byte{},
		},
	}

	flight, err := h.BuildServerFlight()
	require.NoError(t, err)
	assert.Len(t, flight, 3) // ServerHello, ServerKeyExchange, ServerHelloDone

	assert.Equal(t, handshakeServerHello, flight[0].Type)
	assert.Equal(t, handshakeServerKeyExchange, flight[1].Type)
	assert.Equal(t, handshakeServerHelloDone, flight[2].Type)

	assert.NotNil(t, h.SRP)
	assert.Equal(t, stateServerFlightSent, h.State)
}

func TestBuildServerFlight_MissingClientHello(t *testing.T) {
	h := &ServerHandshake{}
	_, err := h.BuildServerFlight()
	assert.Error(t, err)
}

func TestBuildServerFlight_MissingLookup(t *testing.T) {
	h := &ServerHandshake{
		ClientHello: &ClientHello{
			Version:            tls12,
			Random:             make([]byte, 32),
			CipherSuites:       []uint16{TLS_SRP_SHA_WITH_AES_256_CBC_SHA},
			CompressionMethods: []uint8{compressionNull},
			SRPUsername:        "user",
			SessionID:          []byte{},
		},
	}
	_, err := h.BuildServerFlight()
	assert.Error(t, err)
}

func TestBuildServerFlight_InvalidClientHello(t *testing.T) {
	h := &ServerHandshake{
		ClientHello: &ClientHello{
			Version:     0x0301,
			SRPUsername: "user",
		},
	}
	_, err := h.BuildServerFlight()
	assert.Error(t, err)
}

// --- HandleClientKeyExchange ---

func TestHandleClientKeyExchange_Valid(t *testing.T) {
	g := SRP1024()
	user, _ := CreateSRPUser(g, "testuser", "testpass", randSalt())

	lookup := func(username string) (*SRPUser, error) {
		return user, nil
	}

	h := &ServerHandshake{
		Lookup: lookup,
		ClientHello: &ClientHello{
			Version:            tls12,
			Random:             make([]byte, 32),
			CipherSuites:       []uint16{TLS_SRP_SHA_WITH_AES_256_CBC_SHA},
			CompressionMethods: []uint8{compressionNull},
			SRPUsername:        "testuser",
			SessionID:          []byte{},
		},
	}

	_, _ = h.BuildServerFlight()

	// Client generates A
	a := big.NewInt(123456)
	A := new(big.Int).Exp(g.g, a, g.N)

	var ckeEncoder encoder
	ckeEncoder.Vector16(A.Bytes())
	cke := &Handshake{
		Type: handshakeClientKeyExchange,
		Body: ckeEncoder.BytesSlice(),
	}

	err := h.HandleClientKeyExchange(cke)
	require.NoError(t, err)
	assert.NotNil(t, h.MasterSecret)
	assert.Equal(t, stateClientKeyExchangeReceived, h.State)
}

func TestHandleClientKeyExchange_WrongState(t *testing.T) {
	h := &ServerHandshake{State: stateInitial}
	cke := &Handshake{Type: handshakeClientKeyExchange, Body: []byte{}}
	err := h.HandleClientKeyExchange(cke)
	assert.Error(t, err)
}

// --- ActivateReadCipher ---

func TestActivateReadCipher_Valid(t *testing.T) {
	g := SRP1024()
	user, _ := CreateSRPUser(g, "testuser", "testpass", randSalt())

	lookup := func(username string) (*SRPUser, error) {
		return user, nil
	}

	h := &ServerHandshake{
		Lookup: lookup,
		ClientHello: &ClientHello{
			Version:            tls12,
			Random:             make([]byte, 32),
			CipherSuites:       []uint16{TLS_SRP_SHA_WITH_AES_256_CBC_SHA},
			CompressionMethods: []uint8{compressionNull},
			SRPUsername:        "testuser",
			SessionID:          []byte{},
		},
	}

	_, _ = h.BuildServerFlight()

	a := big.NewInt(123456)
	A := new(big.Int).Exp(g.g, a, g.N)

	var ckeEncoder encoder
	ckeEncoder.Vector16(A.Bytes())
	cke := &Handshake{
		Type: handshakeClientKeyExchange,
		Body: ckeEncoder.BytesSlice(),
	}

	_ = h.HandleClientKeyExchange(cke)
	err := h.ActivateReadCipher()
	require.NoError(t, err)

	assert.NotNil(t, h.ReadCipher)
	assert.NotNil(t, h.WriteCipher)
	assert.Equal(t, stateWaitingForFinished, h.State)
}

func TestActivateReadCipher_NoMasterSecret(t *testing.T) {
	h := &ServerHandshake{}
	err := h.ActivateReadCipher()
	assert.Error(t, err)
}

// --- EncryptHandshake ---

func TestHandleClientFinished_Valid(t *testing.T) {
	g := SRP1024()
	user, _ := CreateSRPUser(g, "testuser", "testpass", randSalt())

	lookup := func(username string) (*SRPUser, error) {
		return user, nil
	}

	h := &ServerHandshake{
		Lookup: lookup,
		ClientHello: &ClientHello{
			Version:            tls12,
			Random:             make([]byte, 32),
			CipherSuites:       []uint16{TLS_SRP_SHA_WITH_AES_256_CBC_SHA},
			CompressionMethods: []uint8{compressionNull},
			SRPUsername:        "testuser",
			SessionID:          []byte{},
		},
	}

	_, _ = h.BuildServerFlight()

	a := big.NewInt(99999)
	A := new(big.Int).Exp(g.g, a, g.N)

	var ckeEncoder encoder
	ckeEncoder.Vector16(A.Bytes())
	cke := &Handshake{
		Type: handshakeClientKeyExchange,
		Body: ckeEncoder.BytesSlice(),
	}

	_ = h.HandleClientKeyExchange(cke)
	_ = h.ActivateReadCipher()

	// Simulate the client: compute Finished with the same master secret and transcript,
	// then encrypt it with a cipher using the server's read keys (client's write = server's read).
	clientFinished := GenerateFinished(h.MasterSecret, "client finished", h.Transcript.Hash())

	// Create a "client-side" cipher with the same keys as the server's read cipher (sequence 0)
	clientCipher := &CipherState{
		MACKey:   h.ReadCipher.MACKey,
		Key:      h.ReadCipher.Key,
		Sequence: 0,
	}

	encRecord, err := EncryptHandshake(clientCipher, clientFinished.Encode())
	require.NoError(t, err)

	// The server should successfully decrypt and verify the client Finished
	resp, err := h.HandleClientFinished(encRecord)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, recordHandshake, resp.Type)
	assert.Equal(t, stateClientFinishedReceived, h.State)
}

func TestHandleClientFinished_WrongState(t *testing.T) {
	h := &ServerHandshake{State: stateInitial}
	_, err := h.HandleClientFinished(&Record{})
	assert.Error(t, err)
}

func TestHandleClientFinished_NoReadCipher(t *testing.T) {
	h := &ServerHandshake{State: stateWaitingForFinished}
	_, err := h.HandleClientFinished(&Record{})
	assert.Error(t, err)
}

func TestHandleClientFinished_InvalidFinished(t *testing.T) {
	g := SRP1024()
	user, _ := CreateSRPUser(g, "testuser", "testpass", randSalt())

	lookup := func(username string) (*SRPUser, error) {
		return user, nil
	}

	h := &ServerHandshake{
		Lookup: lookup,
		ClientHello: &ClientHello{
			Version:            tls12,
			Random:             make([]byte, 32),
			CipherSuites:       []uint16{TLS_SRP_SHA_WITH_AES_256_CBC_SHA},
			CompressionMethods: []uint8{compressionNull},
			SRPUsername:        "testuser",
			SessionID:          []byte{},
		},
	}

	_, _ = h.BuildServerFlight()

	a := big.NewInt(99999)
	A := new(big.Int).Exp(g.g, a, g.N)

	var ckeEncoder encoder
	ckeEncoder.Vector16(A.Bytes())
	cke := &Handshake{
		Type: handshakeClientKeyExchange,
		Body: ckeEncoder.BytesSlice(),
	}

	_ = h.HandleClientKeyExchange(cke)
	_ = h.ActivateReadCipher()

	// Create a Finished with wrong verify data
	badFinished := &Finished{VerifyData: make([]byte, finishedLength)}

	clientCipher := &CipherState{
		MACKey:   h.ReadCipher.MACKey,
		Key:      h.ReadCipher.Key,
		Sequence: 0,
	}

	encRecord, err := EncryptHandshake(clientCipher, badFinished.Encode())
	require.NoError(t, err)

	_, err = h.HandleClientFinished(encRecord)
	assert.Error(t, err)
}

// --- Full server handshake flow ---

func TestBuildServerFinishedFlight_Valid(t *testing.T) {
	g := SRP1024()
	user, _ := CreateSRPUser(g, "testuser", "testpass", randSalt())

	lookup := func(username string) (*SRPUser, error) {
		return user, nil
	}

	h := &ServerHandshake{
		Lookup: lookup,
		ClientHello: &ClientHello{
			Version:            tls12,
			Random:             make([]byte, 32),
			CipherSuites:       []uint16{TLS_SRP_SHA_WITH_AES_256_CBC_SHA},
			CompressionMethods: []uint8{compressionNull},
			SRPUsername:        "testuser",
			SessionID:          []byte{},
		},
	}

	_, _ = h.BuildServerFlight()

	a := big.NewInt(99999)
	A := new(big.Int).Exp(g.g, a, g.N)

	var ckeEncoder encoder
	ckeEncoder.Vector16(A.Bytes())
	cke := &Handshake{
		Type: handshakeClientKeyExchange,
		Body: ckeEncoder.BytesSlice(),
	}

	_ = h.HandleClientKeyExchange(cke)
	_ = h.ActivateReadCipher()

	// Manually set state to stateClientFinishedReceived
	h.State = stateClientFinishedReceived

	records, err := h.BuildServerFinishedFlight()
	require.NoError(t, err)
	assert.Len(t, records, 2) // ChangeCipherSpec + encrypted Finished
	assert.Equal(t, recordChangeCipherSpec, records[0].Type)
	assert.Equal(t, recordHandshake, records[1].Type)
	assert.Equal(t, stateEstablished, h.State)
}

func TestBuildServerFinishedFlight_WrongState(t *testing.T) {
	h := &ServerHandshake{State: stateInitial}
	_, err := h.BuildServerFinishedFlight()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected state")
}

func TestBuildServerFinishedFlight_NoWriteCipher(t *testing.T) {
	h := &ServerHandshake{
		State: stateClientFinishedReceived,
	}
	_, err := h.BuildServerFinishedFlight()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "write cipher not active")
}

func TestBuildServerFlight_LookupError(t *testing.T) {
	g := SRP1024()
	user, _ := CreateSRPUser(g, "testuser", "testpass", randSalt())
	_ = user

	lookup := func(username string) (*SRPUser, error) {
		return nil, fmt.Errorf("database error")
	}

	h := &ServerHandshake{
		Lookup: lookup,
		ClientHello: &ClientHello{
			Version:            tls12,
			Random:             make([]byte, 32),
			CipherSuites:       []uint16{TLS_SRP_SHA_WITH_AES_256_CBC_SHA},
			CompressionMethods: []uint8{compressionNull},
			SRPUsername:        "testuser",
			SessionID:          []byte{},
		},
	}

	_, err := h.BuildServerFlight()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database error")
}

func TestBuildServerFlight_UserNotFound(t *testing.T) {
	lookup := func(username string) (*SRPUser, error) {
		return nil, nil
	}

	h := &ServerHandshake{
		Lookup: lookup,
		ClientHello: &ClientHello{
			Version:            tls12,
			Random:             make([]byte, 32),
			CipherSuites:       []uint16{TLS_SRP_SHA_WITH_AES_256_CBC_SHA},
			CompressionMethods: []uint8{compressionNull},
			SRPUsername:        "missinguser",
			SessionID:          []byte{},
		},
	}

	flight, err := h.BuildServerFlight()
	require.NoError(t, err)
	assert.Len(t, flight, 3)
	assert.Equal(t, handshakeServerHello, flight[0].Type)
	assert.Equal(t, handshakeServerKeyExchange, flight[1].Type)
	assert.Equal(t, handshakeServerHelloDone, flight[2].Type)
	assert.NotNil(t, h.SRP)
	assert.Equal(t, "missinguser", h.SRP.User.Username)
	assert.Equal(t, stateServerFlightSent, h.State)
}

func TestHandleClientKeyExchange_BadParse(t *testing.T) {
	g := SRP1024()
	user, _ := CreateSRPUser(g, "testuser", "testpass", randSalt())

	lookup := func(username string) (*SRPUser, error) {
		return user, nil
	}

	h := &ServerHandshake{
		Lookup: lookup,
		ClientHello: &ClientHello{
			Version:            tls12,
			Random:             make([]byte, 32),
			CipherSuites:       []uint16{TLS_SRP_SHA_WITH_AES_256_CBC_SHA},
			CompressionMethods: []uint8{compressionNull},
			SRPUsername:        "testuser",
			SessionID:          []byte{},
		},
	}

	_, _ = h.BuildServerFlight()

	// Send a ClientKeyExchange with wrong type
	cke := &Handshake{
		Type: handshakeServerHello, // wrong type
		Body: []byte{},
	}
	err := h.HandleClientKeyExchange(cke)
	assert.Error(t, err)
}

func TestActivateReadCipher_SplitKeyBlockError(t *testing.T) {
	// splitKeyBlock error via ActivateReadCipher is not reachable because
	// deriveKeyBlock always produces keyBlockLen bytes via TLS PRF.
	// Just verify splitKeyBlock error directly.
	_, err := splitKeyBlock(make([]byte, 10))
	assert.Error(t, err)
}

func TestHandleClientFinished_DecryptError(t *testing.T) {
	g := SRP1024()
	user, _ := CreateSRPUser(g, "testuser", "testpass", randSalt())

	lookup := func(username string) (*SRPUser, error) {
		return user, nil
	}

	h := &ServerHandshake{
		Lookup: lookup,
		ClientHello: &ClientHello{
			Version:            tls12,
			Random:             make([]byte, 32),
			CipherSuites:       []uint16{TLS_SRP_SHA_WITH_AES_256_CBC_SHA},
			CompressionMethods: []uint8{compressionNull},
			SRPUsername:        "testuser",
			SessionID:          []byte{},
		},
	}

	_, _ = h.BuildServerFlight()

	a := big.NewInt(99999)
	A := new(big.Int).Exp(g.g, a, g.N)

	var ckeEncoder encoder
	ckeEncoder.Vector16(A.Bytes())
	cke := &Handshake{
		Type: handshakeClientKeyExchange,
		Body: ckeEncoder.BytesSlice(),
	}

	_ = h.HandleClientKeyExchange(cke)
	_ = h.ActivateReadCipher()

	// Send a record that will fail to decrypt (too short)
	shortRecord := &Record{
		Type:    recordHandshake,
		Version: tls12,
		Data:    make([]byte, 5), // too short for IV
	}

	_, err := h.HandleClientFinished(shortRecord)
	assert.Error(t, err)
}

func TestHandleClientFinished_BadFinishedVerifyData(t *testing.T) {
	g := SRP1024()
	user, _ := CreateSRPUser(g, "testuser", "testpass", randSalt())

	lookup := func(username string) (*SRPUser, error) {
		return user, nil
	}

	h := &ServerHandshake{
		Lookup: lookup,
		ClientHello: &ClientHello{
			Version:            tls12,
			Random:             make([]byte, 32),
			CipherSuites:       []uint16{TLS_SRP_SHA_WITH_AES_256_CBC_SHA},
			CompressionMethods: []uint8{compressionNull},
			SRPUsername:        "testuser",
			SessionID:          []byte{},
		},
	}

	_, _ = h.BuildServerFlight()

	a := big.NewInt(99999)
	A := new(big.Int).Exp(g.g, a, g.N)

	var ckeEncoder encoder
	ckeEncoder.Vector16(A.Bytes())
	cke := &Handshake{
		Type: handshakeClientKeyExchange,
		Body: ckeEncoder.BytesSlice(),
	}

	_ = h.HandleClientKeyExchange(cke)
	_ = h.ActivateReadCipher()

	// Create a properly encrypted Finished with wrong verify data
	badFinished := &Finished{VerifyData: make([]byte, finishedLength)}
	clientCipher := &CipherState{
		MACKey:   h.ReadCipher.MACKey,
		Key:      h.ReadCipher.Key,
		Sequence: 0,
	}
	encRecord, err := EncryptHandshake(clientCipher, badFinished.Encode())
	require.NoError(t, err)

	_, err = h.HandleClientFinished(encRecord)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid client Finished")
}

func TestServerHandshake_FullFlow(t *testing.T) {
	g := SRP1024()
	user, _ := CreateSRPUser(g, "testuser", "testpass", randSalt())

	lookup := func(username string) (*SRPUser, error) {
		return user, nil
	}

	// 1. Server handshake setup
	h := &ServerHandshake{
		Lookup: lookup,
		ClientHello: &ClientHello{
			Version:            tls12,
			Random:             make([]byte, 32),
			CipherSuites:       []uint16{TLS_SRP_SHA_WITH_AES_256_CBC_SHA},
			CompressionMethods: []uint8{compressionNull},
			SRPUsername:        "testuser",
			SessionID:          []byte{},
		},
	}

	// 2. Build server flight
	flight, err := h.BuildServerFlight()
	require.NoError(t, err)
	assert.Len(t, flight, 3)

	// 3. Client processes server flight, generates A and client Finished
	a := big.NewInt(99999)
	A := new(big.Int).Exp(g.g, a, g.N)

	// Client computes shared secret (same as server will)
	// We need to verify the server can handle this A
	var ckeEncoder encoder
	ckeEncoder.Vector16(A.Bytes())
	cke := &Handshake{
		Type: handshakeClientKeyExchange,
		Body: ckeEncoder.BytesSlice(),
	}

	// 4. Handle client key exchange
	err = h.HandleClientKeyExchange(cke)
	require.NoError(t, err)

	// 5. Activate ciphers
	err = h.ActivateReadCipher()
	require.NoError(t, err)
	assert.NotNil(t, h.ReadCipher)
	assert.NotNil(t, h.WriteCipher)

	// 6. Verify master secret is derived
	assert.Equal(t, 48, len(h.MasterSecret))
}
