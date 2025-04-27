package GwPacket

import (
	"io"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewIn(t *testing.T) {
	src := []byte{0x01, 0x02, 0x03, 0x04}
	p := NewIn(src)
	assert.IsType(t, In{}, p)
}

func TestInUint8(t *testing.T) {
	p := NewIn([]byte{0x01, 0x02, 0x03, 0x04})
	val, err := p.Uint8()
	assert.NoError(t, err)
	assert.Equal(t, 0x01, val)
	val, err = p.Uint8()
	assert.NoError(t, err)
	assert.Equal(t, 0x02, val)
	val, err = p.Uint8()
	assert.NoError(t, err)
	assert.Equal(t, 0x03, val)
	val, err = p.Uint8()
	assert.NoError(t, err)
	assert.Equal(t, 0x04, val)
	// Out of bounds
	val, err = p.Uint8()
	assert.ErrorIs(t, err, io.ErrUnexpectedEOF)
	assert.Equal(t, val, 0)
}

func TestInUint16(t *testing.T) {
	p := NewIn(([]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}))
	val, err := p.Uint16()
	assert.NoError(t, err)
	assert.Equal(t, 0x0201, val)
	val, err = p.Uint16()
	assert.NoError(t, err)
	assert.Equal(t, 0x0403, val)
	val, err = p.Uint16()
	assert.NoError(t, err)
	assert.Equal(t, 0x0605, val)
	// Out of bounds
	val, err = p.Uint16()
	assert.ErrorIs(t, err, io.ErrUnexpectedEOF)
	assert.Equal(t, val, 0)
}
func TestInUint32(t *testing.T) {
	p := NewIn(([]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}))
	val, err := p.Uint32()
	assert.NoError(t, err)
	assert.Equal(t, 0x04030201, val)
	val, err = p.Uint32()
	assert.NoError(t, err)
	assert.Equal(t, 0x08070605, val)
	// Out of bounds
	val, err = p.Uint32()
	assert.ErrorIs(t, err, io.ErrUnexpectedEOF)
	assert.Equal(t, val, 0)
}

func TestInUTF16(t *testing.T) {
	p := NewIn([]byte{
		0x05, 0x00, 0x48, 0x00, 0x65, 0x00, 0x6C, 0x00, 0x6C, 0x00, 0x6F, 0x00,
	})
	val, err := p.UTF16WithLengthPrefix()
	assert.NoError(t, err)
	assert.Equal(t, "Hello", val)

	// Partial out of bounds
	p = NewIn(([]byte{
		0x02, 0x00, 0x10,
	}))
	val, err = p.UTF16WithLengthPrefix()
	assert.ErrorIs(t, err, io.ErrUnexpectedEOF)
	assert.Equal(t, "", val)

	// Full out of bounds
	val, err = p.UTF16WithLengthPrefix()
	assert.ErrorIs(t, err, io.ErrUnexpectedEOF)
	assert.Equal(t, "", val)
}

func TestInOpcode(t *testing.T) {
	p := NewIn([]byte{0x01, 0x02, 0x03, 0x04})
	assert.Equal(t, 0x201, p.Opcode())
}

func TestNewOut(t *testing.T) {
	p := NewOut(0x8020)
	assert.IsType(t, Out{}, p)
	assert.Equal(t, p.buf.Bytes(), []byte{0x20, 0x80})
}

func TestOutUint8(t *testing.T) {
	p := NewOut(0x1234)
	p.Uint8(0xcc)
	p.Uint8(0xd1)
	p.Uint8(0x0a)
	assert.Equal(t, p.buf.Bytes(), []byte{0x34, 0x12, 0xcc, 0xd1, 0x0a})
}

func TestOutUint16(t *testing.T) {
	p := NewOut(0x1234)
	p.Uint16(0xcafe)
	p.Uint16(0xbabe)
	assert.Equal(t, p.buf.Bytes(), []byte{0x34, 0x12, 0xfe, 0xca, 0xbe, 0xba})
}
func TestOutUint32(t *testing.T) {
	p := NewOut(0x1234)
	p.Uint32(0xdeadbeef)
	p.Uint32(0xb008b135)
	assert.Equal(t, p.buf.Bytes(), []byte{0x34, 0x12, 0xef, 0xbe, 0xad, 0xde, 0x35, 0xb1, 0x08, 0xb0})
}

func TestOutFloat32(t *testing.T) {
	p := NewOut(0x1234)
	p.Float32(-473.000000)
	p.Float32(9098.12345)
	r := NewIn(p.GetBytes())
	r.Skip(2)
	fromReader1, _ := r.Float32()
	fromReader2, _ := r.Float32()
	assert.Equal(t, float32(-473.000000), fromReader1)
	assert.Equal(t, float32(9098.12345), fromReader2)
	assert.Equal(t, p.buf.Bytes(), []byte{0x34, 0x12, 0x00, 0x80, 0xec, 0xc3, 0x7e, 0x28, 0x0e, 0x46})
}

func TestOutUTF16WithLengthPrefix(t *testing.T) {
	p := NewOut(0x1234)
	p.UTF16WithLengthPrefix("hello")
	assert.Equal(t, p.buf.Bytes(), []byte{0x34, 0x12, 0x05, 0x00, 0x68, 0x00, 0x65, 0x00, 0x6c, 0x00, 0x6c, 0x00, 0x6f, 0x00})
}

func BenchmarkOutUint16(b *testing.B) {
	p := NewOut(0x1234)
	for b.Loop() {
		p.Uint16(0xcafe)
	}
}

func BenchmarkOutUTF16(b *testing.B) {
	p := NewOut(0x1234)
	for b.Loop() {
		p.UTF16WithLengthPrefix("hello, 你好")
	}
}

func TestDecodeFloatFromUint32(t *testing.T) {
	bts := []byte{0x00, 0x00, 0x80, 0x7f}
	r := NewIn(bts)
	f32, _ := r.Float32()
	t.Fatalf("val = %f", f32)
}

func TestInf(t *testing.T) {
	infFloat := float32(math.Inf(1))
	w := NewOutRaw()
	w.Float32(infFloat)
	r := NewIn(w.GetBytes())
	f32, _ := r.Float32()
	t.Fatalf("val = %f", f32)
}
