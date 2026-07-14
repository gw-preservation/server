package srp

import (
	"crypto/sha256"
	"fmt"
)

const finishedLength = 12

type Finished struct {
	VerifyData []byte
}

func ParseFinished(hs *Handshake) (*Finished, error) {
	if hs.Type != handshakeFinished {
		return nil, fmt.Errorf("expected Finished")
	}

	if len(hs.Body) != finishedLength {
		return nil, fmt.Errorf("invalid Finished length")
	}

	return &Finished{
		VerifyData: hs.Body,
	}, nil

}

func GenerateFinished(
	masterSecret []byte,
	label string,
	transcriptHash []byte,
) *Finished {

	verify := tlsPRF(
		masterSecret,
		[]byte(label),
		transcriptHash,
		finishedLength,
	)

	return &Finished{
		VerifyData: verify,
	}

}

func (f *Finished) Encode() *Handshake {
	return &Handshake{
		Type: handshakeFinished,
		Body: f.VerifyData,
	}
}

// Handshake transcript hashing

type HandshakeTranscript struct {
	data []byte
}

func (t *HandshakeTranscript) Add(hs *Handshake) {
	t.data = append(t.data, hs.Bytes()...)
}

func (t *HandshakeTranscript) Hash() []byte {
	h := sha256.New()
	h.Write(t.data)

	return h.Sum(nil)

}
