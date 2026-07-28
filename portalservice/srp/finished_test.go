package srp

import (
	"crypto/sha256"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Finished ---

func TestFinished_Encode(t *testing.T) {
	f := &Finished{
		VerifyData: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12},
	}

	hs := f.Encode()
	assert.Equal(t, handshakeFinished, hs.Type)
	assert.Equal(t, f.VerifyData, hs.Body)
}

func TestParseFinished_Valid(t *testing.T) {
	hs := &Handshake{
		Type: handshakeFinished,
		Body: make([]byte, finishedLength),
	}

	f, err := ParseFinished(hs)
	require.NoError(t, err)
	assert.Equal(t, hs.Body, f.VerifyData)
}

func TestParseFinished_WrongType(t *testing.T) {
	hs := &Handshake{
		Type: handshakeClientHello,
		Body: make([]byte, finishedLength),
	}

	_, err := ParseFinished(hs)
	assert.Error(t, err)
}

func TestParseFinished_WrongLength(t *testing.T) {
	hs := &Handshake{
		Type: handshakeFinished,
		Body: make([]byte, 8), // wrong length
	}

	_, err := ParseFinished(hs)
	assert.Error(t, err)
}

func TestFinished_RoundTrip(t *testing.T) {
	original := &Finished{
		VerifyData: make([]byte, finishedLength),
	}

	hs := original.Encode()
	parsed, err := ParseFinished(hs)
	require.NoError(t, err)
	assert.Equal(t, original.VerifyData, parsed.VerifyData)
}

// --- GenerateFinished ---

func TestGenerateFinished_Deterministic(t *testing.T) {
	master := make([]byte, 48)
	for i := range master {
		master[i] = byte(i)
	}
	transcript := sha256.Sum256([]byte("handshake data"))

	f1 := GenerateFinished(master, "client finished", transcript[:])
	f2 := GenerateFinished(master, "client finished", transcript[:])
	assert.Equal(t, f1.VerifyData, f2.VerifyData)
}

func TestGenerateFinished_DifferentLabels(t *testing.T) {
	master := make([]byte, 48)
	transcript := sha256.Sum256([]byte("data"))

	client := GenerateFinished(master, "client finished", transcript[:])
	server := GenerateFinished(master, "server finished", transcript[:])
	assert.NotEqual(t, client.VerifyData, server.VerifyData)
}

func TestGenerateFinished_Length(t *testing.T) {
	master := make([]byte, 48)
	transcript := make([]byte, 32)

	f := GenerateFinished(master, "test", transcript)
	assert.Equal(t, finishedLength, len(f.VerifyData))
}

// --- HandshakeTranscript ---

func TestHandshakeTranscript_Empty(t *testing.T) {
	tr := &HandshakeTranscript{}
	hash := tr.Hash()
	assert.Equal(t, sha256.Size, len(hash))
}

func TestHandshakeTranscript_Add(t *testing.T) {
	tr := &HandshakeTranscript{}

	hs1 := &Handshake{Type: handshakeClientHello, Body: []byte{1, 2, 3}}
	hs2 := &Handshake{Type: handshakeServerHello, Body: []byte{4, 5, 6}}

	tr.Add(hs1)
	h1 := tr.Hash()

	tr.Add(hs2)
	h2 := tr.Hash()

	// Hashes should differ as data accumulates
	assert.NotEqual(t, h1, h2)
}

func TestHandshakeTranscript_Deterministic(t *testing.T) {
	tr1 := &HandshakeTranscript{}
	tr2 := &HandshakeTranscript{}

	hs := &Handshake{Type: handshakeClientHello, Body: []byte{1, 2, 3}}
	tr1.Add(hs)
	tr2.Add(hs)

	assert.Equal(t, tr1.Hash(), tr2.Hash())
}

func TestHandshakeTranscript_Order(t *testing.T) {
	hs1 := &Handshake{Type: handshakeClientHello, Body: []byte{1}}
	hs2 := &Handshake{Type: handshakeServerHello, Body: []byte{2}}

	tr1 := &HandshakeTranscript{}
	tr1.Add(hs1)
	tr1.Add(hs2)

	tr2 := &HandshakeTranscript{}
	tr2.Add(hs2)
	tr2.Add(hs1)

	// Different order should produce different hash
	assert.NotEqual(t, tr1.Hash(), tr2.Hash())
}
