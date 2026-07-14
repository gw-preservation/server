package srp

import (
	"encoding/binary"
	"errors"
	"fmt"
)

var ErrBufferUnderflow = errors.New("buffer underflow")

// Decoder

type parser struct {
	buf []byte
	pos int
}

func newParser(b []byte) *parser {
	return &parser{buf: b}
}

func (p *parser) Remaining() int {
	return len(p.buf) - p.pos
}

func (p *parser) Empty() bool {
	return p.Remaining() == 0
}

func (p *parser) Uint8() (uint8, error) {
	if p.Remaining() < 1 {
		return 0, ErrBufferUnderflow
	}

	v := p.buf[p.pos]
	p.pos++

	return v, nil

}

func (p *parser) Uint16() (uint16, error) {
	if p.Remaining() < 2 {
		return 0, ErrBufferUnderflow
	}

	v := binary.BigEndian.Uint16(p.buf[p.pos:])
	p.pos += 2

	return v, nil

}

func (p *parser) Uint24() (uint32, error) {
	if p.Remaining() < 3 {
		return 0, ErrBufferUnderflow
	}

	v := uint32(p.buf[p.pos])<<16 |
		uint32(p.buf[p.pos+1])<<8 |
		uint32(p.buf[p.pos+2])

	p.pos += 3

	return v, nil

}

func (p *parser) Bytes(n int) ([]byte, error) {
	if n < 0 {
		return nil, fmt.Errorf("negative length")
	}

	if p.Remaining() < n {
		return nil, ErrBufferUnderflow
	}

	b := p.buf[p.pos : p.pos+n]
	p.pos += n

	return b, nil

}

func (p *parser) Vector8() ([]byte, error) {
	n, err := p.Uint8()
	if err != nil {
		return nil, err
	}

	return p.Bytes(int(n))

}

func (p *parser) Vector16() ([]byte, error) {
	n, err := p.Uint16()
	if err != nil {
		return nil, err
	}

	return p.Bytes(int(n))

}

func (p *parser) Vector24() ([]byte, error) {
	n, err := p.Uint24()
	if err != nil {
		return nil, err
	}

	return p.Bytes(int(n))

}

func (p *parser) Vector16Uint() ([]uint16, error) {
	b, err := p.Vector16()
	if err != nil {
		return nil, err
	}

	if len(b)%2 != 0 {
		return nil, fmt.Errorf("invalid uint16 vector length")
	}

	out := make([]uint16, len(b)/2)

	for i := range out {
		out[i] = binary.BigEndian.Uint16(b[i*2:])
	}

	return out, nil

}

func (p *parser) Vector8Uint() ([]uint8, error) {
	b, err := p.Vector8()
	if err != nil {
		return nil, err
	}

	return b, nil

}

// Encoder

type encoder struct {
	buf []byte
}

func (e *encoder) Bytes(b []byte) {
	e.buf = append(e.buf, b...)
}

func (e *encoder) Uint8(v uint8) {
	e.buf = append(e.buf, v)
}

func (e *encoder) Uint16(v uint16) {
	var tmp [2]byte

	binary.BigEndian.PutUint16(tmp[:], v)

	e.buf = append(e.buf, tmp[:]...)

}

func (e *encoder) Uint24(v uint32) {
	e.buf = append(
		e.buf,
		byte(v>>16),
		byte(v>>8),
		byte(v),
	)
}

func (e *encoder) Vector8(b []byte) {
	e.Uint8(uint8(len(b)))
	e.Bytes(b)
}

func (e *encoder) Vector16(b []byte) {
	e.Uint16(uint16(len(b)))
	e.Bytes(b)
}

func (e *encoder) Vector24(b []byte) {
	e.Uint24(uint32(len(b)))
	e.Bytes(b)
}

func (e *encoder) BytesSlice() []byte {
	return e.buf
}
