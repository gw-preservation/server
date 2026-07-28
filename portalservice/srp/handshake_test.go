package srp

import (
	"math/big"
	"testing"

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
	e.Uint16(tls12)                           // version
	e.Bytes(make([]byte, 32))                  // random
	e.Vector8([]byte{})                        // session ID
	e.Vector16([]byte{0xc0, 0x20})            // cipher suites (TLS_SRP_SHA_WITH_AES_256_CBC_SHA)
	e.Vector8([]byte{compressionNull})         // compression

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
	p.Uint16()   // version
	p.Bytes(32)  // random
	p.Vector8()  // session ID
	p.Uint16()   // cipher suite
	p.Uint8()    // compression
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
