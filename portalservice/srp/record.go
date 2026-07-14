package srp

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const (
	recordChangeCipherSpec uint8 = 20
	recordAlert            uint8 = 21
	recordHandshake        uint8 = 22
	recordApplicationData  uint8 = 23
)

const (
	tls12 uint16 = 0x0303
)

const (
	maxPlaintext = 16384
)

type Record struct {
	Type    uint8
	Version uint16
	Data    []byte
}

var (
	ErrRecordTooLarge     = errors.New("record too large")
	ErrUnsupportedVersion = errors.New("unsupported version")
)

func ReadRecord(r io.Reader) (*Record, error) {
	var hdr [5]byte

	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return nil, err
	}

	rec := &Record{
		Type:    hdr[0],
		Version: binary.BigEndian.Uint16(hdr[1:3]),
	}

	if rec.Version != tls12 {
		return nil, fmt.Errorf("%w: %04x", ErrUnsupportedVersion, rec.Version)
	}

	length := binary.BigEndian.Uint16(hdr[3:5])

	if length > maxPlaintext {
		return nil, fmt.Errorf("%w: %d", ErrRecordTooLarge, length)
	}

	rec.Data = make([]byte, length)

	if _, err := io.ReadFull(r, rec.Data); err != nil {
		return nil, err
	}

	return rec, nil
}

func WriteRecord(w io.Writer, rec *Record) error {
	if rec.Version != tls12 {
		return fmt.Errorf("unsupported TLS version %04x", rec.Version)
	}

	if len(rec.Data) > maxPlaintext {
		return fmt.Errorf("record too large")
	}

	var hdr [5]byte

	hdr[0] = rec.Type
	binary.BigEndian.PutUint16(hdr[1:3], rec.Version)
	binary.BigEndian.PutUint16(hdr[3:5], uint16(len(rec.Data)))

	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}

	_, err := w.Write(rec.Data)
	return err
}
