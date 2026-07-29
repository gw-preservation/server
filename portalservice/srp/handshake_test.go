package srp

import (
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Handshake read/write ---

func TestHandshake_ReadWrite_RoundTrip(t *testing.T) {
	hs := &Handshake{
		Type: handshakeClientHello,
		Body: []byte{0x01, 0x02, 0x03},
	}

	rec, err := WriteHandshake(hs)
	require.NoError(t, err)

	parsed, err := ReadHandshake(rec)
	require.NoError(t, err)
	assert.Equal(t, hs.Type, parsed.Type)
	assert.Equal(t, hs.Body, parsed.Body)
}

func TestHandshake_ReadWrite_EmptyBody(t *testing.T) {
	hs := &Handshake{
		Type: handshakeServerHelloDone,
		Body: nil,
	}

	rec, err := WriteHandshake(hs)
	require.NoError(t, err)

	parsed, err := ReadHandshake(rec)
	require.NoError(t, err)
	assert.Equal(t, hs.Type, parsed.Type)
	assert.Empty(t, parsed.Body)
}

func TestReadHandshake_WrongRecordType(t *testing.T) {
	rec := &Record{
		Type: recordAlert,
		Data: []byte{0},
	}

	_, err := ReadHandshake(rec)
	assert.Error(t, err)
}

func TestReadHandshake_LengthMismatch(t *testing.T) {
	// Craft a record with a length that doesn't match
	data := []byte{
		handshakeClientHello,
		0, 0, 10, // length = 10
		0x01, 0x02, // only 2 bytes of body
	}
	rec := &Record{
		Type:    recordHandshake,
		Version: tls12,
		Data:    data,
	}

	_, err := ReadHandshake(rec)
	assert.Error(t, err)
}

func TestHandshake_Bytes(t *testing.T) {
	hs := &Handshake{
		Type: handshakeServerHello,
		Body: []byte{0xAA, 0xBB},
	}

	b := hs.Bytes()
	assert.Equal(t, byte(handshakeServerHello), b[0])
	// Length encoded as 3 bytes
	assert.Equal(t, byte(0), b[1])
	assert.Equal(t, byte(0), b[2])
	assert.Equal(t, byte(2), b[3])
	assert.Equal(t, []byte{0xAA, 0xBB}, b[4:])
}

// --- ClientHello ---

func TestParseClientHello_Valid(t *testing.T) {
	var e encoder
	e.Uint16(tls12)                    // version
	e.Bytes(make([]byte, 32))          // random
	e.Vector8([]byte{})                // session ID
	e.Vector16([]byte{0xc0, 0x20})     // cipher suites (TLS_SRP_SHA_WITH_AES_256_CBC_SHA)
	e.Vector8([]byte{compressionNull}) // compression

	// SRP extension
	ext := encoder{}
	ext.Vector8([]byte("testuser"))
	srpExt := ext.BytesSlice()

	var extBuf encoder
	extBuf.Uint16(extensionSRP)
	extBuf.Vector16(srpExt)
	e.Vector16(extBuf.BytesSlice())

	hs := &Handshake{
		Type: handshakeClientHello,
		Body: e.BytesSlice(),
	}

	ch, err := ParseClientHello(hs)
	require.NoError(t, err)
	assert.Equal(t, tls12, ch.Version)
	assert.Equal(t, 32, len(ch.Random))
	assert.Equal(t, "testuser", ch.SRPUsername)
	assert.Contains(t, ch.CipherSuites, uint16(TLS_SRP_SHA_WITH_AES_256_CBC_SHA))
	assert.Equal(t, []uint8{compressionNull}, ch.CompressionMethods)
}

func TestParseClientHello_WrongType(t *testing.T) {
	hs := &Handshake{Type: handshakeServerHello, Body: []byte{}}
	_, err := ParseClientHello(hs)
	assert.Error(t, err)
}

// --- ValidateClientHello ---

func TestValidateClientHello_Valid(t *testing.T) {
	ch := &ClientHello{
		Version:            tls12,
		Random:             make([]byte, 32),
		CipherSuites:       []uint16{TLS_SRP_SHA_WITH_AES_256_CBC_SHA},
		CompressionMethods: []uint8{compressionNull},
		SRPUsername:        "user",
		SessionID:          []byte{},
	}
	assert.NoError(t, ValidateClientHello(ch))
}

func TestValidateClientHello_WrongVersion(t *testing.T) {
	ch := &ClientHello{
		Version:            0x0301,
		Random:             make([]byte, 32),
		CipherSuites:       []uint16{TLS_SRP_SHA_WITH_AES_256_CBC_SHA},
		CompressionMethods: []uint8{compressionNull},
		SRPUsername:        "user",
		SessionID:          []byte{},
	}
	assert.Error(t, ValidateClientHello(ch))
}

func TestValidateClientHello_NoSupportedCipher(t *testing.T) {
	ch := &ClientHello{
		Version:            tls12,
		Random:             make([]byte, 32),
		CipherSuites:       []uint16{0x00FF},
		CompressionMethods: []uint8{compressionNull},
		SRPUsername:        "user",
		SessionID:          []byte{},
	}
	assert.Error(t, ValidateClientHello(ch))
}

func TestValidateClientHello_WrongCompression(t *testing.T) {
	ch := &ClientHello{
		Version:            tls12,
		Random:             make([]byte, 32),
		CipherSuites:       []uint16{TLS_SRP_SHA_WITH_AES_256_CBC_SHA},
		CompressionMethods: []uint8{1},
		SRPUsername:        "user",
		SessionID:          []byte{},
	}
	assert.Error(t, ValidateClientHello(ch))
}

func TestValidateClientHello_MissingUsername(t *testing.T) {
	ch := &ClientHello{
		Version:            tls12,
		Random:             make([]byte, 32),
		CipherSuites:       []uint16{TLS_SRP_SHA_WITH_AES_256_CBC_SHA},
		CompressionMethods: []uint8{compressionNull},
		SRPUsername:        "",
		SessionID:          []byte{},
	}
	assert.Error(t, ValidateClientHello(ch))
}

func TestValidateClientHello_SessionResumption(t *testing.T) {
	ch := &ClientHello{
		Version:            tls12,
		Random:             make([]byte, 32),
		CipherSuites:       []uint16{TLS_SRP_SHA_WITH_AES_256_CBC_SHA},
		CompressionMethods: []uint8{compressionNull},
		SRPUsername:        "user",
		SessionID:          []byte{1, 2, 3},
	}
	assert.Error(t, ValidateClientHello(ch))
}

// --- ServerHello ---

func TestServerHello_Encode(t *testing.T) {
	sh := &ServerHello{
		Version:           tls12,
		Random:            make([]byte, 32),
		SessionID:         []byte{1, 2, 3},
		CipherSuite:       TLS_SRP_SHA_WITH_AES_256_CBC_SHA,
		CompressionMethod: compressionNull,
	}

	hs := sh.Encode()
	assert.Equal(t, handshakeServerHello, hs.Type)

	// Parse it back
	p := newParser(hs.Body)
	version, _ := p.Uint16()
	assert.Equal(t, tls12, version)

	random, _ := p.Bytes(32)
	assert.Equal(t, make([]byte, 32), random)

	sid, _ := p.Vector8()
	assert.Equal(t, []byte{1, 2, 3}, sid)

	cs, _ := p.Uint16()
	assert.Equal(t, uint16(TLS_SRP_SHA_WITH_AES_256_CBC_SHA), cs)

	comp, _ := p.Uint8()
	assert.Equal(t, uint8(compressionNull), comp)
}

func TestNewServerHello(t *testing.T) {
	sh := NewServerHello()
	assert.Equal(t, tls12, sh.Version)
	assert.Equal(t, uint16(TLS_SRP_SHA_WITH_AES_256_CBC_SHA), sh.CipherSuite)
	assert.Equal(t, uint8(compressionNull), sh.CompressionMethod)
	assert.Equal(t, 32, len(sh.Random))
}

func TestServerHello_WithExtensions(t *testing.T) {
	sh := &ServerHello{
		Version:           tls12,
		Random:            make([]byte, 32),
		CipherSuite:       TLS_SRP_SHA_WITH_AES_256_CBC_SHA,
		CompressionMethod: compressionNull,
		Extensions:        []byte{0x00, 0x01},
	}

	hs := sh.Encode()
	p := newParser(hs.Body)
	p.Uint16()  // version
	p.Bytes(32) // random
	p.Vector8() // session ID
	p.Uint16()  // cipher suite
	p.Uint8()   // compression
	ext, err := p.Vector16()
	assert.NoError(t, err)
	assert.Equal(t, []byte{0x00, 0x01}, ext)
}

// --- ClientKeyExchange ---

func TestParseClientKeyExchange_Valid(t *testing.T) {
	A := big.NewInt(12345)
	var e encoder
	e.Vector16(A.Bytes())

	hs := &Handshake{
		Type: handshakeClientKeyExchange,
		Body: e.BytesSlice(),
	}

	cke, err := ParseClientKeyExchange(hs)
	require.NoError(t, err)
	assert.Equal(t, A, cke.A)
}

func TestParseClientKeyExchange_WrongType(t *testing.T) {
	hs := &Handshake{Type: handshakeServerHello, Body: []byte{}}
	_, err := ParseClientKeyExchange(hs)
	assert.Error(t, err)
}

func TestParseClientKeyExchange_ZeroA(t *testing.T) {
	var e encoder
	e.Vector16([]byte{0})

	hs := &Handshake{
		Type: handshakeClientKeyExchange,
		Body: e.BytesSlice(),
	}

	_, err := ParseClientKeyExchange(hs)
	assert.Error(t, err)
}

func TestParseClientKeyExchange_TrailingData(t *testing.T) {
	var e encoder
	e.Vector16([]byte{1, 2})
	e.Bytes([]byte{3, 4})

	hs := &Handshake{
		Type: handshakeClientKeyExchange,
		Body: e.BytesSlice(),
	}

	_, err := ParseClientKeyExchange(hs)
	assert.Error(t, err)
}

// --- ServerKeyExchange ---

func TestServerKeyExchange_Encode(t *testing.T) {
	ske := &ServerKeyExchange{
		N:    big.NewInt(100),
		G:    big.NewInt(2),
		Salt: []byte{0x01, 0x02},
		B:    big.NewInt(200),
	}

	hs := ske.Encode()
	assert.Equal(t, handshakeServerKeyExchange, hs.Type)

	p := newParser(hs.Body)
	n, _ := p.Vector16()
	assert.Equal(t, big.NewInt(100).Bytes(), n)

	g, _ := p.Vector16()
	assert.Equal(t, big.NewInt(2).Bytes(), g)

	salt, _ := p.Vector8()
	assert.Equal(t, []byte{0x01, 0x02}, salt)

	b, _ := p.Vector16()
	assert.Equal(t, big.NewInt(200).Bytes(), b)
}

// --- ServerHelloDone ---

func TestNewServerHelloDone(t *testing.T) {
	hs := NewServerHelloDone()
	assert.Equal(t, handshakeServerHelloDone, hs.Type)
	assert.Empty(t, hs.Body)
}

// --- ChangeCipherSpec ---

func TestNewChangeCipherSpec(t *testing.T) {
	rec := NewChangeCipherSpec()
	assert.Equal(t, recordChangeCipherSpec, rec.Type)
	assert.Equal(t, tls12, rec.Version)
	assert.Equal(t, []byte{1}, rec.Data)
}

func TestParseChangeCipherSpec_Valid(t *testing.T) {
	rec := NewChangeCipherSpec()
	assert.NoError(t, ParseChangeCipherSpec(rec))
}

func TestParseChangeCipherSpec_WrongType(t *testing.T) {
	rec := &Record{
		Type: recordHandshake,
		Data: []byte{1},
	}
	assert.Error(t, ParseChangeCipherSpec(rec))
}

func TestParseChangeCipherSpec_InvalidData(t *testing.T) {
	rec := &Record{
		Type: recordChangeCipherSpec,
		Data: []byte{2},
	}
	assert.Error(t, ParseChangeCipherSpec(rec))
}

func TestParseChangeCipherSpec_WrongLength(t *testing.T) {
	rec := &Record{
		Type: recordChangeCipherSpec,
		Data: []byte{1, 2},
	}
	assert.Error(t, ParseChangeCipherSpec(rec))
}

// --- parseSRPExtension ---

func TestParseSRPExtension_Valid(t *testing.T) {
	var e encoder
	e.Vector8([]byte("testuser"))
	username, err := parseSRPExtension(e.BytesSlice())
	assert.NoError(t, err)
	assert.Equal(t, []byte("testuser"), username)
}

func TestParseSRPExtension_Empty(t *testing.T) {
	var e encoder
	e.Vector8([]byte{})
	_, err := parseSRPExtension(e.BytesSlice())
	assert.Error(t, err)
}

func TestParseSRPExtension_Trailing(t *testing.T) {
	var e encoder
	e.Vector8([]byte("user"))
	e.Bytes([]byte{0x01})
	_, err := parseSRPExtension(e.BytesSlice())
	assert.Error(t, err)
}

// --- ParseHandshakeBytes ---

func TestParseHandshakeBytes_Valid(t *testing.T) {
	data := []byte{
		handshakeFinished,
		0, 0, 4, // length = 4
		0x01, 0x02, 0x03, 0x04,
	}

	hs, err := ParseHandshakeBytes(data)
	require.NoError(t, err)
	assert.Equal(t, handshakeFinished, hs.Type)
	assert.Equal(t, []byte{0x01, 0x02, 0x03, 0x04}, hs.Body)
}

func TestParseHandshakeBytes_TooShort(t *testing.T) {
	_, err := ParseHandshakeBytes([]byte{1, 2, 3})
	assert.Error(t, err)
}

func TestParseHandshakeBytes_LengthMismatch(t *testing.T) {
	data := []byte{
		handshakeFinished,
		0, 0, 10, // claims 10 bytes
		0x01, 0x02, // only 2 bytes
	}
	_, err := ParseHandshakeBytes(data)
	assert.Error(t, err)
}

// --- EncryptHandshake ---

func TestWriteHandshake_BodyTooLarge(t *testing.T) {
	hs := &Handshake{
		Type: handshakeClientHello,
		Body: make([]byte, 0xffffff+1),
	}
	_, err := WriteHandshake(hs)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "too large")
}

func TestWriteHandshakeRecord_DelegatesToWriteHandshake(t *testing.T) {
	hs := &Handshake{
		Type: handshakeServerHelloDone,
		Body: nil,
	}
	rec, err := WriteHandshakeRecord(hs)
	require.NoError(t, err)
	assert.Equal(t, recordHandshake, rec.Type)
	assert.Equal(t, tls12, rec.Version)
}

func TestParseClientHello_TrailingData(t *testing.T) {
	var e encoder
	e.Uint16(tls12)
	e.Bytes(make([]byte, 32))
	e.Vector8([]byte{})
	e.Vector16([]byte{0xc0, 0x20})
	e.Vector8([]byte{compressionNull})
	e.Vector16([]byte{}) // empty extensions

	// Add trailing data
	e.Bytes([]byte{0x01, 0x02})

	hs := &Handshake{
		Type: handshakeClientHello,
		Body: e.BytesSlice(),
	}

	_, err := ParseClientHello(hs)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "trailing")
}

func TestParseClientHello_NoExtensions(t *testing.T) {
	var e encoder
	e.Uint16(tls12)
	e.Bytes(make([]byte, 32))
	e.Vector8([]byte{})
	e.Vector16([]byte{0xc0, 0x20})
	e.Vector8([]byte{compressionNull})
	e.Vector16([]byte{}) // empty extensions

	hs := &Handshake{
		Type: handshakeClientHello,
		Body: e.BytesSlice(),
	}

	ch, err := ParseClientHello(hs)
	require.NoError(t, err)
	assert.Empty(t, ch.SRPUsername) // no SRP extension, so empty
}

func TestParseClientHello_UnknownExtension(t *testing.T) {
	var e encoder
	e.Uint16(tls12)
	e.Bytes(make([]byte, 32))
	e.Vector8([]byte{})
	e.Vector16([]byte{0xc0, 0x20})
	e.Vector8([]byte{compressionNull})

	// Unknown extension (not SRP)
	extBuf := encoder{}
	extBuf.Uint16(9999) // unknown type
	extBuf.Vector16([]byte{1, 2, 3})
	e.Vector16(extBuf.BytesSlice())

	hs := &Handshake{
		Type: handshakeClientHello,
		Body: e.BytesSlice(),
	}

	ch, err := ParseClientHello(hs)
	require.NoError(t, err)
	assert.Empty(t, ch.SRPUsername) // unknown extension ignored
}

func TestParseClientHello_SRPExtensionError(t *testing.T) {
	var e encoder
	e.Uint16(tls12)
	e.Bytes(make([]byte, 32))
	e.Vector8([]byte{})
	e.Vector16([]byte{0xc0, 0x20})
	e.Vector8([]byte{compressionNull})

	// SRP extension with empty username (error)
	extBuf := encoder{}
	extBuf.Uint16(extensionSRP)
	inner := encoder{}
	inner.Vector8([]byte{}) // empty username
	extBuf.Vector16(inner.BytesSlice())
	e.Vector16(extBuf.BytesSlice())

	hs := &Handshake{
		Type: handshakeClientHello,
		Body: e.BytesSlice(),
	}

	_, err := ParseClientHello(hs)
	assert.Error(t, err)
}

func TestReadHandshake_TrailingData(t *testing.T) {
	// Craft a record with a length that doesn't match actual data
	data := []byte{
		handshakeClientHello,
		0, 0, 10, // length = 10
		0x01, 0x02, // only 2 bytes of body
	}
	rec := &Record{
		Type:    recordHandshake,
		Version: tls12,
		Data:    data,
	}

	_, err := ReadHandshake(rec)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "length mismatch")
}

func TestHandshakeIt_InvalidState(t *testing.T) {
	pr, pw := net.Pipe()
	defer pr.Close()
	defer pw.Close()

	sc := &ServerConnection{
		Reader: pr,
		Writer: pw,
		Handshake: ServerHandshake{
			State: 99, // invalid state
		},
		handshakeTimeout: 1 * time.Second,
	}

	// Send a valid record so ReadRecord doesn't block
	go func() {
		WriteRecord(pw, &Record{
			Type:    recordHandshake,
			Version: tls12,
			Data:    []byte{handshakeClientHello, 0, 0, 0},
		})
	}()

	err := sc.HandshakeIt()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not implemented")
}

func TestHandshakeIt_BuildFlightWriteError(t *testing.T) {
	g := SRP1024()
	user, _ := CreateSRPUser(g, "testuser", "testpass", randSalt())
	lookup := func(username string) (*SRPUser, error) {
		return user, nil
	}

	pr, pw := net.Pipe()
	defer pr.Close()

	sc := &ServerConnection{
		Reader: pr,
		Writer: pw,
		Lookup: lookup,
		Handshake: ServerHandshake{
			State: stateInitial,
		},
		handshakeTimeout: 1 * time.Second,
	}

	// Send valid ClientHello, then close pw so WriteRecord on server flight fails
	go func() {
		// Read the ClientHello record that the server will try to read
		// First send the ClientHello
		chBody := func() []byte {
			var e encoder
			e.Uint16(tls12)
			e.Bytes(make([]byte, 32))
			e.Vector8([]byte{})
			e.Vector16([]byte{0xc0, 0x20})
			e.Vector8([]byte{compressionNull})
			ext := encoder{}
			ext.Vector8([]byte("testuser"))
			srpExt := ext.BytesSlice()
			var extBuf encoder
			extBuf.Uint16(extensionSRP)
			extBuf.Vector16(srpExt)
			e.Vector16(extBuf.BytesSlice())
			return e.BytesSlice()
		}()
		chRec, _ := WriteHandshake(&Handshake{
			Type: handshakeClientHello,
			Body: chBody,
		})
		WriteRecord(pw, chRec)
		// Wait a bit for server to process then close write end
		time.Sleep(50 * time.Millisecond)
		pw.Close()
	}()

	err := sc.HandshakeIt()
	// Should fail because pw is closed when server tries to write its flight
	assert.Error(t, err)
}

func TestHandshakeIt_CCSWriteError(t *testing.T) {
	g := SRP1024()
	user, _ := CreateSRPUser(g, "testuser", "testpass", randSalt())
	lookup := func(username string) (*SRPUser, error) {
		return user, nil
	}

	chBody := func() []byte {
		var e encoder
		e.Uint16(tls12)
		e.Bytes(make([]byte, 32))
		e.Vector8([]byte{})
		e.Vector16([]byte{0xc0, 0x20})
		e.Vector8([]byte{compressionNull})
		ext := encoder{}
		ext.Vector8([]byte("testuser"))
		srpExt := ext.BytesSlice()
		var extBuf encoder
		extBuf.Uint16(extensionSRP)
		extBuf.Vector16(srpExt)
		e.Vector16(extBuf.BytesSlice())
		return e.BytesSlice()
	}()

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

	// Add ClientHello to transcript (as HandshakeIt does from stateInitial)
	h.Transcript.Add(&Handshake{
		Type: handshakeClientHello,
		Body: chBody,
	})

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

	// Pre-compute client cipher keys (same derivation as ActivateReadCipher)
	block := deriveKeyBlock(h.MasterSecret, h.ClientHello.Random, h.ServerHello.Random)
	keys, _ := splitKeyBlock(block)
	clientCipher := &CipherState{
		MACKey: keys.ClientMACKey,
		Key:    keys.ClientWriteKey,
	}

	pr, pw := net.Pipe()
	defer pr.Close()

	sc := &ServerConnection{
		Reader:           pr,
		Writer:           pw,
		Lookup:           lookup,
		Handshake:        *h,
		handshakeTimeout: 1 * time.Second,
	}

	go func() {
		WriteRecord(pw, NewChangeCipherSpec())

		clientFinished := GenerateFinished(
			h.MasterSecret,
			"client finished",
			h.Transcript.Hash(),
		)
		encRecord, _ := EncryptHandshake(clientCipher, clientFinished.Encode())
		WriteRecord(pw, encRecord)

		pw.Close()
	}()

	err := sc.HandshakeIt()
	assert.Error(t, err)
}

func TestHandshakeIt_ClientFinishedWriteError(t *testing.T) {
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
	_ = h.ActivateReadCipher()

	// State is stateWaitingForFinished
	pr, pw := net.Pipe()
	defer pr.Close()

	sc := &ServerConnection{
		Reader:           pr,
		Writer:           pw,
		Lookup:           lookup,
		Handshake:        *h,
		handshakeTimeout: 1 * time.Second,
	}

	go func() {
		clientCipher := &CipherState{
			MACKey:   h.ReadCipher.MACKey,
			Key:      h.ReadCipher.Key,
			Sequence: 0,
		}
		clientFinished := GenerateFinished(h.MasterSecret, "client finished", h.Transcript.Hash())
		encRecord, _ := EncryptHandshake(clientCipher, clientFinished.Encode())
		WriteRecord(pw, encRecord)
		// Close immediately after sending Finished
		pw.Close()
	}()

	err := sc.HandshakeIt()
	assert.Error(t, err)
}

func TestHandshakeIt_ClientFinishedBadMAC(t *testing.T) {
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
	_ = h.ActivateReadCipher()

	// Use TCP for proper bidirectional I/O
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	serverDone := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			serverDone <- err
			return
		}
		defer conn.Close()
		sc := &ServerConnection{
			Reader:           conn,
			Writer:           conn,
			Lookup:           lookup,
			Handshake:        *h,
			handshakeTimeout: 2 * time.Second,
		}
		serverDone <- sc.HandshakeIt()
	}()

	clientConn, err := net.Dial("tcp", listener.Addr().String())
	require.NoError(t, err)
	defer clientConn.Close()

	clientCipher := &CipherState{
		MACKey:   h.ReadCipher.MACKey,
		Key:      h.ReadCipher.Key,
		Sequence: 0,
	}
	badFinished := &Finished{VerifyData: make([]byte, finishedLength)}
	encRecord, _ := EncryptHandshake(clientCipher, badFinished.Encode())
	WriteRecord(clientConn, encRecord)

	select {
	case err := <-serverDone:
		assert.Error(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out")
	}
}

func TestHandshakeIt_ClientFinishedDecryptionError(t *testing.T) {
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
	_ = h.ActivateReadCipher()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	serverDone := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			serverDone <- err
			return
		}
		defer conn.Close()
		sc := &ServerConnection{
			Reader:           conn,
			Writer:           conn,
			Lookup:           lookup,
			Handshake:        *h,
			handshakeTimeout: 2 * time.Second,
		}
		serverDone <- sc.HandshakeIt()
	}()

	clientConn, err := net.Dial("tcp", listener.Addr().String())
	require.NoError(t, err)
	defer clientConn.Close()

	encRecord := &Record{
		Type:    recordHandshake,
		Version: tls12,
		Data:    make([]byte, 64),
	}
	WriteRecord(clientConn, encRecord)

	select {
	case err := <-serverDone:
		assert.Error(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out")
	}
}

func TestHandshakeIt_CCSInvalid(t *testing.T) {
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

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	serverDone := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			serverDone <- err
			return
		}
		defer conn.Close()
		sc := &ServerConnection{
			Reader:           conn,
			Writer:           conn,
			Lookup:           lookup,
			Handshake:        *h,
			handshakeTimeout: 2 * time.Second,
		}
		serverDone <- sc.HandshakeIt()
	}()

	clientConn, err := net.Dial("tcp", listener.Addr().String())
	require.NoError(t, err)
	defer clientConn.Close()

	WriteRecord(clientConn, &Record{
		Type:    recordChangeCipherSpec,
		Version: tls12,
		Data:    []byte{2},
	})

	select {
	case err := <-serverDone:
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid ChangeCipherSpec")
	case <-time.After(5 * time.Second):
		t.Fatal("timed out")
	}
}

func TestHandshakeIt_ExpectedHandshakeRecord(t *testing.T) {
	pr, pw := net.Pipe()
	defer pr.Close()
	defer pw.Close()

	sc := &ServerConnection{
		Reader: pr,
		Writer: pw,
		Handshake: ServerHandshake{
			State: stateInitial,
		},
		handshakeTimeout: 100 * time.Millisecond,
	}

	// Send a non-handshake record
	go func() {
		WriteRecord(pw, &Record{
			Type:    recordAlert,
			Version: tls12,
			Data:    []byte{0, 0},
		})
	}()

	err := sc.HandshakeIt()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expected handshake")
}

func TestHandshakeIt_ExpectedClientHello(t *testing.T) {
	pr, pw := net.Pipe()
	defer pr.Close()
	defer pw.Close()

	sc := &ServerConnection{
		Reader: pr,
		Writer: pw,
		Handshake: ServerHandshake{
			State: stateInitial,
		},
		handshakeTimeout: 100 * time.Millisecond,
	}

	// Send a handshake record but not ClientHello
	go func() {
		hs := &Handshake{
			Type: handshakeServerHello,
			Body: []byte{},
		}
		rec, _ := WriteHandshake(hs)
		WriteRecord(pw, rec)
	}()

	err := sc.HandshakeIt()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expected ClientHello")
}

func TestHandshakeIt_ExpectedClientKeyExchange(t *testing.T) {
	g := SRP1024()
	user, _ := CreateSRPUser(g, "testuser", "testpass", randSalt())
	lookup := func(username string) (*SRPUser, error) {
		return user, nil
	}

	pr, pw := net.Pipe()
	defer pr.Close()
	defer pw.Close()

	sc := &ServerConnection{
		Reader: pr,
		Writer: pw,
		Lookup: lookup,
		Handshake: ServerHandshake{
			State: stateServerFlightSent,
		},
		handshakeTimeout: 100 * time.Millisecond,
	}

	// Send a non-handshake record
	go func() {
		WriteRecord(pw, &Record{
			Type:    recordChangeCipherSpec,
			Version: tls12,
			Data:    []byte{1},
		})
	}()

	err := sc.HandshakeIt()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expected ClientKeyExchange")
}

func TestHandshakeIt_ChangeCipherSpec(t *testing.T) {
	g := SRP1024()
	user, _ := CreateSRPUser(g, "testuser", "testpass", randSalt())
	lookup := func(username string) (*SRPUser, error) {
		return user, nil
	}

	// Create a complete handshake state
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

	pr, pw := net.Pipe()
	defer pr.Close()
	defer pw.Close()

	sc := &ServerConnection{
		Reader:           pr,
		Writer:           pw,
		Lookup:           lookup,
		Handshake:        *h,
		handshakeTimeout: 100 * time.Millisecond,
	}

	// Send invalid ChangeCipherSpec
	go func() {
		WriteRecord(pw, &Record{
			Type:    recordChangeCipherSpec,
			Version: tls12,
			Data:    []byte{2}, // invalid (should be 1)
		})
	}()

	err := sc.HandshakeIt()
	assert.Error(t, err)
}

func TestHandshakeIt_ExpectedFinished(t *testing.T) {
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
	_ = h.ActivateReadCipher()

	pr, pw := net.Pipe()
	defer pr.Close()
	defer pw.Close()

	sc := &ServerConnection{
		Reader:           pr,
		Writer:           pw,
		Lookup:           lookup,
		Handshake:        *h,
		handshakeTimeout: 100 * time.Millisecond,
	}

	// Send a non-handshake record (should be Finished)
	go func() {
		WriteRecord(pw, &Record{
			Type:    recordChangeCipherSpec,
			Version: tls12,
			Data:    []byte{1},
		})
	}()

	err := sc.HandshakeIt()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expected Finished")
}

func TestParseChangeCipherSpec_EmptyData(t *testing.T) {
	rec := &Record{
		Type: recordChangeCipherSpec,
		Data: []byte{},
	}
	err := ParseChangeCipherSpec(rec)
	assert.Error(t, err)
}

func TestParseChangeCipherSpec_WrongRecordType(t *testing.T) {
	rec := &Record{
		Type: recordHandshake,
		Data: []byte{1},
	}
	err := ParseChangeCipherSpec(rec)
	assert.Error(t, err)
}

func TestHandshake_Bytes_RoundTrip(t *testing.T) {
	hs := &Handshake{
		Type: handshakeClientKeyExchange,
		Body: []byte{0x01, 0x02, 0x03, 0x04, 0x05},
	}

	b := hs.Bytes()
	// Verify structure: type(1) + length(3) + body
	assert.Equal(t, byte(handshakeClientKeyExchange), b[0])
	assert.Equal(t, byte(0), b[1])
	assert.Equal(t, byte(0), b[2])
	assert.Equal(t, byte(5), b[3])
	assert.Equal(t, hs.Body, b[4:])
}

func TestServerHello_Encode_NoExtensions(t *testing.T) {
	sh := &ServerHello{
		Version:           tls12,
		Random:            make([]byte, 32),
		CipherSuite:       TLS_SRP_SHA_WITH_AES_256_CBC_SHA,
		CompressionMethod: compressionNull,
		Extensions:        nil,
	}

	hs := sh.Encode()
	p := newParser(hs.Body)
	p.Uint16()  // version
	p.Bytes(32) // random
	p.Vector8() // session ID
	p.Uint16()  // cipher suite
	p.Uint8()   // compression
	// Should be empty now (no extensions encoded)
	assert.True(t, p.Empty())
}

func TestParseSRPExtension(t *testing.T) {
	var e encoder
	e.Vector8([]byte("alice"))
	username, err := parseSRPExtension(e.BytesSlice())
	assert.NoError(t, err)
	assert.Equal(t, []byte("alice"), username)
}

func TestParseClientHello_TruncatedRandom(t *testing.T) {
	var e encoder
	e.Uint16(tls12)
	e.Bytes(make([]byte, 10)) // only 10 bytes instead of 32

	hs := &Handshake{
		Type: handshakeClientHello,
		Body: e.BytesSlice(),
	}

	_, err := ParseClientHello(hs)
	assert.Error(t, err)
}

func TestParseClientHello_TruncatedSessionID(t *testing.T) {
	var e encoder
	e.Uint16(tls12)
	e.Bytes(make([]byte, 32))
	// Vector8 with length 5 but only 2 bytes follow
	e.Bytes([]byte{5, 0x01, 0x02})

	hs := &Handshake{
		Type: handshakeClientHello,
		Body: e.BytesSlice(),
	}

	_, err := ParseClientHello(hs)
	assert.Error(t, err)
}

func TestParseClientHello_TruncatedCipherSuites(t *testing.T) {
	var e encoder
	e.Uint16(tls12)
	e.Bytes(make([]byte, 32))
	e.Vector8([]byte{})
	// Vector16 with length 4 but only 1 byte follows
	e.Bytes([]byte{0, 4, 0x01})

	hs := &Handshake{
		Type: handshakeClientHello,
		Body: e.BytesSlice(),
	}

	_, err := ParseClientHello(hs)
	assert.Error(t, err)
}

func TestParseClientHello_TruncatedCompression(t *testing.T) {
	var e encoder
	e.Uint16(tls12)
	e.Bytes(make([]byte, 32))
	e.Vector8([]byte{})
	e.Vector16([]byte{0xc0, 0x20})
	// Vector8 with length 3 but no data
	e.Bytes([]byte{3})

	hs := &Handshake{
		Type: handshakeClientHello,
		Body: e.BytesSlice(),
	}

	_, err := ParseClientHello(hs)
	assert.Error(t, err)
}

func TestParseClientHello_TruncatedExtensions(t *testing.T) {
	var e encoder
	e.Uint16(tls12)
	e.Bytes(make([]byte, 32))
	e.Vector8([]byte{})
	e.Vector16([]byte{0xc0, 0x20})
	e.Vector8([]byte{compressionNull})
	// Vector16 with length 10 but no data
	e.Bytes([]byte{0, 10})

	hs := &Handshake{
		Type: handshakeClientHello,
		Body: e.BytesSlice(),
	}

	_, err := ParseClientHello(hs)
	assert.Error(t, err)
}

func TestReadHandshake_EmptyBody(t *testing.T) {
	data := []byte{
		handshakeServerHelloDone,
		0, 0, 0, // length = 0
	}
	rec := &Record{
		Type:    recordHandshake,
		Version: tls12,
		Data:    data,
	}

	parsed, err := ReadHandshake(rec)
	require.NoError(t, err)
	assert.Equal(t, handshakeServerHelloDone, parsed.Type)
	assert.Empty(t, parsed.Body)
}

func TestValidateCipherSuites(t *testing.T) {
	ch := &ClientHello{
		CipherSuites: []uint16{0x00FF, TLS_SRP_SHA_WITH_AES_256_CBC_SHA},
	}
	assert.True(t, validateCipherSuites(ch))

	ch2 := &ClientHello{
		CipherSuites: []uint16{0x00FF},
	}
	assert.False(t, validateCipherSuites(ch2))
}

func TestNewServerKeyExchange(t *testing.T) {
	g := SRP1024()
	user, _ := CreateSRPUser(g, "testuser", "testpass", randSalt())
	srp, _ := NewSRPServer(g, user)

	ske := NewServerKeyExchange(srp)
	assert.Equal(t, srp.Group.N, ske.N)
	assert.Equal(t, srp.Group.g, ske.G)
	assert.Equal(t, srp.User.Salt, ske.Salt)
	assert.Equal(t, srp.B, ske.B)
}

func TestEncryptHandshake(t *testing.T) {
	key := make([]byte, 32)
	macKey := make([]byte, 20)
	for i := range macKey {
		key[i] = byte(i)
		macKey[i] = byte(i)
	}

	cipher := &CipherState{MACKey: macKey, Key: key}
	hs := &Handshake{
		Type: handshakeFinished,
		Body: []byte{0x01, 0x02, 0x03, 0x04},
	}

	rec, err := EncryptHandshake(cipher, hs)
	require.NoError(t, err)
	assert.Equal(t, recordHandshake, rec.Type)
	assert.Equal(t, tls12, rec.Version)
	assert.NotEmpty(t, rec.Data)
}
