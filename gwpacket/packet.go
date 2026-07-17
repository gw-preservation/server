package GwPacket

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"unicode/utf16"
)

/*
===============================================================================
    In
===============================================================================
*/

type In struct {
	data   []byte
	offset int
}

func NewIn(src []byte) In {
	p := In{
		data:   src,
		offset: 0,
	}
	return p
}

func (p *In) Position() int {
	return p.offset
}
func (p *In) Skip(by int) {
	p.offset += by
}

func (p *In) Remaining() int {
	return len(p.data) - p.offset
}

func (p *In) Uint8() (out int, err error) {
	if p.Remaining() < 1 {
		err = io.ErrUnexpectedEOF
		return
	}
	out = int(p.data[p.offset])
	p.offset++
	return out, err
}

func (p *In) Bool() (out bool, err error) {
	tmp, err := p.Uint8()
	if err != nil {
		return
	}
	if tmp > 0 {
		out = true
	} else {
		out = false
	}
	return
}

func (p *In) Uint16() (out int, err error) {
	if p.Remaining() < 2 {
		err = io.ErrUnexpectedEOF
		return
	}
	out = int(p.data[p.offset]) | (int(p.data[p.offset+1]) << 8)
	p.offset += 2
	return out, err
}

func (p *In) Uint32() (out int, err error) {
	if p.Remaining() < 4 {
		err = io.ErrUnexpectedEOF
		return
	}
	out = int(p.data[p.offset]) |
		(int(p.data[p.offset+1]) << 8) |
		(int(p.data[p.offset+2]) << 16) |
		(int(p.data[p.offset+3]) << 24)
	p.offset += 4
	return out, err
}

func (p *In) Float32() (out float32, err error) {
	asU32, err := p.Uint32()
	if err != nil {
		return
	}
	out = math.Float32frombits(uint32(asU32))
	return
}

func (p *In) Uint64() (out uint64, err error) {
	if p.Remaining() < 8 {
		err = io.ErrUnexpectedEOF
		return
	}
	out = uint64(p.data[p.offset]) |
		(uint64(p.data[p.offset+1]) << 8) |
		(uint64(p.data[p.offset+2]) << 16) |
		(uint64(p.data[p.offset+3]) << 24) |
		(uint64(p.data[p.offset+4]) << 32) |
		(uint64(p.data[p.offset+5]) << 40) |
		(uint64(p.data[p.offset+6]) << 48) |
		(uint64(p.data[p.offset+7]) << 56)
	p.offset += 8
	return out, err
}

func (p *In) UTF16(length int) (out string, err error) {
	var utf16Units []uint16
	var val int
	for range length {
		val, err = p.Uint16()
		if err != nil {
			err = fmt.Errorf("Uint16(): %w", err)
			return
		}
		utf16Units = append(utf16Units, uint16(val))
	}
	// String has `len` UTF-16 codepoints
	out = string(utf16.Decode(utf16Units))
	return
}

func (p *In) UTF16WithLengthPrefix() (out string, err error) {
	var len int
	len, err = p.Uint16()
	if err != nil {
		err = fmt.Errorf("Uint16(): %w", err)
		return
	}
	// Safety check
	if len > 512 {
		err = fmt.Errorf("length > 512: %d", len)
		return
	}
	out, err = p.UTF16(len)
	return
}

func (p *In) Bytes(length int) (out []byte, err error) {
	if p.Remaining() < length {
		err = io.ErrUnexpectedEOF
		return
	}
	out = p.data[p.offset : p.offset+length]
	p.offset += length
	return
}

func (p *In) Opcode() int {
	return int(p.data[0]) | (int(p.data[1]) << 8)
}

func (p In) String() string {
	return fmt.Sprintf("[%04x] with %d bytes", p.Opcode(), len(p.data))
}

/*
===============================================================================
    Out
===============================================================================
*/

type Out struct {
	buf *bytes.Buffer
}

func NewOut(opcode int) Out {
	p := Out{
		buf: &bytes.Buffer{},
	}
	p.Uint16(opcode)
	return p
}
func NewOutRaw() Out {
	p := Out{
		buf: &bytes.Buffer{},
	}
	return p
}

func (p *Out) Bool(val bool) {
	if val {
		p.Uint8(1)
	} else {
		p.Uint8(0)
	}
}

func (p *Out) Uint8(val int) {
	p.buf.Write([]byte{byte(val & 0xff)})
}
func (p *Out) Uint16(val int) {
	p.buf.Write([]byte{byte(val & 0xff), byte((val >> 8 & 0xff))})
}

func (p *Out) Uint32(val int) {
	p.buf.Write([]byte{byte(val & 0xFF), byte((val >> 8) & 0xff), byte((val >> 16) & 0xff), byte((val >> 24) & 0xff)})
}

func (p *Out) UTF16(str string) {
	utf16Units := utf16.Encode([]rune(str))
	for _, unit := range utf16Units {
		p.Uint16(int(unit))
	}
}

func (p *Out) UTF16WithLengthPrefix(str string) {
	p.Uint16(len(str))
	p.UTF16(str)
}

func (p *Out) Float32(val float32) {
	asU32 := math.Float32bits(val)
	p.Uint32(int(asU32))
}

func (p *Out) Bytes(src []byte) {
	p.buf.Write(src)
}

func (p *Out) GetBytes() []byte {
	return p.buf.Bytes()
}

func (p *Out) Merge(src Out) {
	p.Bytes(src.GetBytes())
}

func (p *Out) Reset() {
	p.buf = &bytes.Buffer{}
}
