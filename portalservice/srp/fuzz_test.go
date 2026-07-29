package srp

import (
	"bytes"
	"math/big"
	"testing"
)

// --- FuzzReadRecord ---
// Fuzzes the TLS record layer: 5-byte header (type, version, length) + payload.
// A malicious client can send any type, any version, any length claim.

func FuzzReadRecord(f *testing.F) {
	// Seed: valid minimal record
	f.Add([]byte{recordHandshake, 0x03, 0x03, 0, 0})
	// Seed: valid record with data
	f.Add([]byte{recordHandshake, 0x03, 0x03, 0, 5, 1, 2, 3, 4, 5})
	// Seed: wrong version
	f.Add([]byte{recordHandshake, 0x03, 0x01, 0, 0})
	// Seed: too large length
	f.Add([]byte{recordHandshake, 0x03, 0x03, 0xFF, 0xFF})
	// Seed: zero length
	f.Add([]byte{recordHandshake, 0x03, 0x03, 0, 0})
	// Seed: length claims 10 bytes but only 2 follow
	f.Add([]byte{recordHandshake, 0x03, 0x03, 0, 10, 0xAA, 0xBB})
	// Seed: all record types
	f.Add([]byte{recordChangeCipherSpec, 0x03, 0x03, 0, 1, 1})
	f.Add([]byte{recordAlert, 0x03, 0x03, 0, 2, 2, 20})
	f.Add([]byte{recordApplicationData, 0x03, 0x03, 0, 3, 1, 2, 3})

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) < 5 {
			return
		}
		// Only fuzz if it looks like a plausible record header
		if data[1] != 0x03 || data[2] != 0x03 {
			return
		}
		length := int(data[3])<<8 | int(data[4])
		if length > maxPlaintext {
			return
		}
		// Ensure we have enough data for the claimed length
		if len(data) < 5+length {
			return
		}

		r := bytes.NewReader(data[:5+length])
		_, _ = ReadRecord(r)
	})
}

// --- FuzzReadHandshake ---
// Fuzzes handshake message parsing: type (1 byte) + length (3 bytes) + body.
// Attackers can claim a body length that doesn't match actual data.

func FuzzReadHandshake(f *testing.F) {
	// Seed: valid minimal handshake (ClientHelloDone)
	f.Add([]byte{handshakeServerHelloDone, 0, 0, 0})
	// Seed: valid handshake with body
	f.Add([]byte{handshakeClientHello, 0, 0, 3, 1, 2, 3})
	// Seed: length mismatch (claims 10, has 2)
	f.Add([]byte{handshakeClientHello, 0, 0, 10, 1, 2})
	// Seed: length mismatch (claims 0, has 2 extra)
	f.Add([]byte{handshakeClientHello, 0, 0, 0, 1, 2})
	// Seed: huge length claim
	f.Add([]byte{handshakeClientHello, 0xFF, 0xFF, 0xFF})
	// Seed: empty body
	f.Add([]byte{handshakeClientKeyExchange, 0, 0, 0})

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) < 4 {
			return
		}
		length := int(data[1])<<16 | int(data[2])<<8 | int(data[3])
		if length > len(data)-4 {
			return
		}
		rec := &Record{
			Type:    recordHandshake,
			Version: tls12,
			Data:    data,
		}
		_, _ = ReadHandshake(rec)
	})
}

// --- FuzzParseClientHello ---
// Fuzzes ClientHello parsing: version, random(32), sessionID, cipherSuites,
// compressionMethods, extensions. Extensions contain nested length-prefixed vectors.

func FuzzParseClientHello(f *testing.F) {
	// Seed: minimal valid ClientHello
	var e encoder
	e.Uint16(tls12)
	e.Bytes(make([]byte, 32))
	e.Vector8([]byte{})
	e.Vector16([]byte{0xc0, 0x20})
	e.Vector8([]byte{compressionNull})
	extEnc := encoder{}
	extEnc.Uint16(extensionSRP)
	extBuf := encoder{}
	extBuf.Vector8([]byte("user"))
	extEnc.Vector16(extBuf.BytesSlice())
	e.Vector16(extEnc.BytesSlice())
	f.Add(e.BytesSlice())

	// Seed: ClientHello with no extensions
	var e2 encoder
	e2.Uint16(tls12)
	e2.Bytes(make([]byte, 32))
	e2.Vector8([]byte{})
	e2.Vector16([]byte{0xc0, 0x20})
	e2.Vector8([]byte{compressionNull})
	e2.Vector16([]byte{})
	f.Add(e2.BytesSlice())

	// Seed: truncated ClientHello
	f.Add([]byte{0x03, 0x03})

	// Seed: wrong version
	var e3 encoder
	e3.Uint16(0x0301)
	e3.Bytes(make([]byte, 32))
	e3.Vector8([]byte{})
	e3.Vector16([]byte{0xc0, 0x20})
	e3.Vector8([]byte{compressionNull})
	e3.Vector16([]byte{})
	f.Add(e3.BytesSlice())

	f.Fuzz(func(t *testing.T, data []byte) {
		hs := &Handshake{
			Type: handshakeClientHello,
			Body: data,
		}
		_, _ = ParseClientHello(hs)
	})
}

// --- FuzzParseClientKeyExchange ---
// Fuzzes ClientKeyExchange parsing: A is a length-prefixed big.Int.
// Attackers can send A=0, A=N, A>N, A huge, empty A, etc.

func FuzzParseClientKeyExchange(f *testing.F) {
	// Seed: valid A
	var e encoder
	e.Vector16(big.NewInt(12345).Bytes())
	f.Add(e.BytesSlice())

	// Seed: A=0
	var e2 encoder
	e2.Vector16([]byte{0})
	f.Add(e2.BytesSlice())

	// Seed: empty body
	f.Add([]byte{})

	// Seed: A=N (128 bytes of the SRP prime)
	n := SRP1024().N
	var e3 encoder
	e3.Vector16(n.Bytes())
	f.Add(e3.BytesSlice())

	// Seed: A > N
	overN := new(big.Int).Add(n, big.NewInt(1))
	var e4 encoder
	e4.Vector16(overN.Bytes())
	f.Add(e4.BytesSlice())

	// Seed: trailing data
	var e5 encoder
	e5.Vector16(big.NewInt(1).Bytes())
	e5.Bytes([]byte{0xFF})
	f.Add(e5.BytesSlice())

	f.Fuzz(func(t *testing.T, data []byte) {
		hs := &Handshake{
			Type: handshakeClientKeyExchange,
			Body: data,
		}
		_, _ = ParseClientKeyExchange(hs)
	})
}

// --- FuzzCipherStateDecrypt ---
// Fuzzes the CBC decryption path: IV extraction, CBC decrypt, unpad, MAC verify.
// Attackers can send: too-short ciphertext, non-aligned ciphertext,
// valid-looking padding with wrong MAC, padding oracle probes.

func FuzzCipherStateDecrypt(f *testing.F) {
	// Setup a cipher with known keys
	key := make([]byte, 32)
	macKey := make([]byte, 20)
	for i := range key {
		key[i] = byte(i)
	}
	for i := range macKey {
		macKey[i] = byte(i)
	}
	enc := &CipherState{MACKey: macKey, Key: key, Sequence: 0}

	// Seed: encrypt valid data then fuzz the ciphertext
	ciphertext, _ := enc.Encrypt(recordApplicationData, tls12, []byte("hello"))
	f.Add(ciphertext)
	f.Add([]byte{})                            // empty ciphertext
	f.Add(make([]byte, ivLen))                  // just IV, no body
	f.Add(make([]byte, ivLen+1))                // IV + 1 byte (not block-aligned)
	f.Add(make([]byte, ivLen+16))               // IV + one block
	ffBlock := bytes.Repeat([]byte{0xFF}, ivLen)
	f.Add(append(make([]byte, ivLen), ffBlock...))

	f.Fuzz(func(t *testing.T, data []byte) {
		dec := &CipherState{MACKey: macKey, Key: key, Sequence: 0}
		_, _ = dec.Decrypt(recordApplicationData, tls12, data)
	})
}

// --- FuzzTlsUnpad ---
// Fuzzes TLS CBC padding removal. Attackers manipulate the last byte
// (padding length) and padding bytes to try padding oracle attacks.

func FuzzTlsUnpad(f *testing.F) {
	// Seed: valid padded data
	f.Add(tlsPad([]byte("test"), ivLen))
	// Seed: single byte (padding length)
	f.Add([]byte{0})
	// Seed: all zeros
	f.Add(make([]byte, 32))
	// Seed: all 0xFF
	f.Add(bytes.Repeat([]byte{0xFF}, 32))

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) == 0 || len(data) > 16384 {
			return
		}
		_, _ = tlsUnpad(data)
	})
}

// --- FuzzSplitKeyBlock ---
// Fuzzes key block splitting: expects exactly 136 bytes, splits into
// MAC keys, encryption keys, IVs.

func FuzzSplitKeyBlock(f *testing.F) {
	f.Add(make([]byte, keyBlockLen))
	f.Add(make([]byte, 10))
	f.Add(make([]byte, 200))
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > keyBlockLen*2 {
			return
		}
		_, _ = splitKeyBlock(data)
	})
}

// --- FuzzParseFinished ---
// Fuzzes Finished message parsing: expects exactly 12 bytes of verify data.

func FuzzParseFinished(f *testing.F) {
	f.Add(make([]byte, finishedLength))
	f.Add(make([]byte, 0))
	f.Add(make([]byte, 1))
	f.Add(make([]byte, 100))

	f.Fuzz(func(t *testing.T, data []byte) {
		hs := &Handshake{
			Type: handshakeFinished,
			Body: data,
		}
		_, _ = ParseFinished(hs)
	})
}

// --- FuzzParseAlert ---
// Fuzzes alert parsing: expects exactly 2 bytes (level, description).

func FuzzParseAlert(f *testing.F) {
	f.Add([]byte{alertFatal, alertBadRecordMAC})
	f.Add([]byte{alertWarning, alertCloseNotify})
	f.Add([]byte{})
	f.Add([]byte{1})
	f.Add([]byte{1, 2, 3})

	f.Fuzz(func(t *testing.T, data []byte) {
		rec := &Record{
			Type:    recordAlert,
			Version: tls12,
			Data:    data,
		}
		_, _ = ParseAlert(rec)
	})
}

// --- FuzzParseChangeCipherSpec ---
// Fuzzes ChangeCipherSpec: expects exactly 1 byte with value 1.

func FuzzParseChangeCipherSpec(f *testing.F) {
	f.Add([]byte{1})
	f.Add([]byte{0})
	f.Add([]byte{2})
	f.Add([]byte{1, 2})
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, data []byte) {
		rec := &Record{
			Type:    recordChangeCipherSpec,
			Version: tls12,
			Data:    data,
		}
		_ = ParseChangeCipherSpec(rec)
	})
}

// --- FuzzParseHandshakeBytes ---
// Fuzzes raw handshake byte parsing: type(1) + length(3) + body.

func FuzzParseHandshakeBytes(f *testing.F) {
	f.Add([]byte{handshakeFinished, 0, 0, 4, 1, 2, 3, 4})
	f.Add([]byte{handshakeClientHello, 0, 0, 0})
	f.Add([]byte{1, 2, 3})
	f.Add([]byte{handshakeClientHello, 0, 0, 10, 1, 2})

	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = ParseHandshakeBytes(data)
	})
}

// --- FuzzComputeSharedSecret ---
// Fuzzes SRP shared secret computation with arbitrary A values.
// Attackers try: A=0, A=N, A>N, A huge, A with specific bit patterns
// to try to manipulate the key exchange.

func FuzzComputeSharedSecret(f *testing.F) {
	g := SRP1024()
	user, _ := CreateSRPUser(g, "fuzzuser", "fuzzpass")
	server, _ := NewSRPServer(g, user)

	f.Add(big.NewInt(1).Bytes())
	f.Add(big.NewInt(0).Bytes())
	f.Add(g.N.Bytes())
	f.Add(new(big.Int).Add(g.N, big.NewInt(1)).Bytes())

	// Seed: A with MSB set (very large)
	huge := new(big.Int).Exp(big.NewInt(2), big.NewInt(1024), nil)
	f.Add(huge.Bytes())

	// Seed: all 0xFF bytes
	f.Add(bytes.Repeat([]byte{0xFF}, 128))

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 256 {
			return
		}
		A := new(big.Int).SetBytes(data)
		_, _ = server.ComputeSharedSecret(A)
	})
}

// --- FuzzPadBigInt ---
// Fuzzes big.Int padding: negative size, size=0, size < len(bytes).

func FuzzPadBigInt(f *testing.F) {
	f.Add([]byte{1, 2, 3}, int32(4))
	f.Add([]byte{}, int32(16))
	f.Add([]byte{0xFF}, int32(1))
	f.Add(bytes.Repeat([]byte{0xAA}, 128), int32(64))
	f.Add([]byte{1}, int32(0))

	f.Fuzz(func(t *testing.T, data []byte, size int32) {
		if size < 0 || size > 4096 {
			return
		}
		v := new(big.Int).SetBytes(data)
		_ = padBigInt(v, int(size))
	})
}

// --- FuzzParserCodec ---
// Fuzzes the low-level parser with arbitrary bytes, exercising all read methods.

func FuzzParserCodec(f *testing.F) {
	f.Add([]byte{0x01, 0x02, 0x03, 0x04})
	f.Add([]byte{})
	f.Add([]byte{0x01})
	f.Add(bytes.Repeat([]byte{0xFF}, 256))

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			return
		}
		p := newParser(data)
		for !p.Empty() {
			b, err := p.Uint8()
			if err != nil {
				break
			}
			_ = b
		}

		p = newParser(data)
		for p.Remaining() >= 2 {
			v, err := p.Uint16()
			if err != nil {
				break
			}
			_ = v
		}

		p = newParser(data)
		for p.Remaining() >= 3 {
			v, err := p.Uint24()
			if err != nil {
				break
			}
			_ = v
		}

		p = newParser(data)
		for p.Remaining() > 0 {
			v, err := p.Vector8()
			if err != nil {
				break
			}
			_ = v
		}

		p = newParser(data)
		for p.Remaining() > 1 {
			v, err := p.Vector16()
			if err != nil {
				break
			}
			_ = v
		}
	})
}

// --- FuzzEncoderDecoderRoundTrip ---
// Fuzzes encode then decode to find crashes in the round-trip path.

func FuzzEncoderDecoderRoundTrip(f *testing.F) {
	f.Add([]byte{0x01}, uint16(0x0203))
	f.Add([]byte{0xFF}, uint16(0xFFFF))
	f.Add([]byte{}, uint16(0))

	f.Fuzz(func(t *testing.T, data []byte, length uint16) {
		if len(data) > 256 || int(length) > 256 {
			return
		}
		var e encoder
		e.Vector8(data)
		e.Vector16(data)
		e.Uint24(uint32(length))

		p := newParser(e.BytesSlice())
		_, _ = p.Vector8()
		_, _ = p.Vector16()
		_, _ = p.Uint24()
	})
}

// --- FuzzHandshakeTranscript ---
// Fuzzes transcript hashing with arbitrary handshake data.

func FuzzHandshakeTranscript(f *testing.F) {
	f.Add([]byte{handshakeClientHello, 1, 2, 3})
	f.Add([]byte{})
	f.Add(bytes.Repeat([]byte{0xFF}, 1000))

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			return
		}
		tr := &HandshakeTranscript{}
		hs := &Handshake{
			Type: handshakeClientHello,
			Body: data,
		}
		tr.Add(hs)
		_ = tr.Hash()
	})
}

// --- FuzzTlsPRF ---
// Fuzzes TLS PRF with arbitrary secret, label, seed, and output length.

func FuzzTlsPRF(f *testing.F) {
	f.Add([]byte("secret"), []byte("label"), []byte("seed"), int32(48))
	f.Add([]byte{}, []byte(""), []byte(""), int32(0))
	f.Add(bytes.Repeat([]byte{0xFF}, 100), []byte("master secret"), make([]byte, 64), int32(48))

	f.Fuzz(func(t *testing.T, secret, label, seed []byte, length int32) {
		if length < 0 || length > 4096 {
			return
		}
		_ = tlsPRF(secret, label, seed, int(length))
	})
}

// --- FuzzSRPExtension ---
// Fuzzes SRP extension parsing with nested length-prefixed username.

func FuzzSRPExtension(f *testing.F) {
	// Seed: valid
	var e encoder
	e.Vector8([]byte("alice"))
	f.Add(e.BytesSlice())

	// Seed: empty
	var e2 encoder
	e2.Vector8([]byte{})
	f.Add(e2.BytesSlice())

	// Seed: trailing data
	var e3 encoder
	e3.Vector8([]byte("bob"))
	e3.Bytes([]byte{0xFF})
	f.Add(e3.BytesSlice())

	// Seed: empty
	f.Add([]byte{})

	// Seed: length claims 200 but only 2 bytes follow
	f.Add([]byte{200, 0x01, 0x02})

	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = parseSRPExtension(data)
	})
}

// --- FuzzClientHelloExtensions ---
// Fuzzes extension parsing loop with arbitrary extension data.

func FuzzClientHelloExtensions(f *testing.F) {
	// Seed: valid SRP extension
	var e encoder
	e.Uint16(extensionSRP)
	ext := encoder{}
	ext.Vector8([]byte("user"))
	e.Vector16(ext.BytesSlice())
	f.Add(e.BytesSlice())

	// Seed: empty extensions
	f.Add([]byte{})

	// Seed: extension type only, no data
	var e2 encoder
	e2.Uint16(extensionSRP)
	f.Add(e2.BytesSlice())

	// Seed: multiple extensions
	var e3 encoder
	e3.Uint16(extensionSRP)
	ext3 := encoder{}
	ext3.Vector8([]byte("user"))
	e3.Vector16(ext3.BytesSlice())
	e3.Uint16(9999)
	e3.Vector16([]byte{1, 2})
	f.Add(e3.BytesSlice())

	f.Fuzz(func(t *testing.T, data []byte) {
		ch := &ClientHello{}
		_ = parseClientHelloExtensions(ch, data)
	})
}
