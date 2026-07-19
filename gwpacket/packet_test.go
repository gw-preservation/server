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

func TestInPosition(t *testing.T) {
	p := NewIn([]byte{0x01, 0x02, 0x03, 0x04})
	assert.Equal(t, p.Position(), 0)
	p.Uint16()
	assert.Equal(t, p.Position(), 2)
}

func TestInBool(t *testing.T) {
	p := NewIn([]byte{0x01, 0x00})
	val, err := p.Bool()
	assert.NoError(t, err)
	assert.True(t, val)
	val, err = p.Bool()
	assert.NoError(t, err)
	assert.False(t, val)
	val, err = p.Bool()
	assert.Error(t, err)
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
func TestInFloat32(t *testing.T) {
	bts := []byte{0x00, 0x00, 0x80, 0x7f}
	r := NewIn(bts)
	f32, err := r.Float32()
	assert.NoError(t, err)
	assert.Equal(t, f32, float32(math.Inf(+1)))
	r = NewIn([]byte{0x00, 0x00, 0x00})
	_, err = r.Float32()
	assert.Error(t, err)
}

func TestInUint64(t *testing.T) {
	p := NewIn(([]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x00}))
	val, err := p.Uint64()
	assert.NoError(t, err)
	assert.Equal(t, uint64(0x807060504030201), val)
	val, err = p.Uint64()
	assert.Error(t, err)
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

	// UTF16 string too large
	p = NewIn([]byte{0xff, 0xff})
	val, err = p.UTF16WithLengthPrefix()
	assert.Error(t, err)
}

func TestInBytes(t *testing.T) {
	p := NewIn([]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08})
	val, err := p.Bytes(1)
	assert.NoError(t, err)
	assert.Len(t, val, 1)
	assert.Equal(t, p.Position(), 1)
	val, err = p.Bytes(100)
	assert.Error(t, err)
}

func TestInOpcode(t *testing.T) {
	p := NewIn([]byte{0x01, 0x02, 0x03, 0x04})
	assert.Equal(t, p.Opcode(), 0x201)
}

func TestInString(t *testing.T) {
	p := NewIn([]byte{0x01, 0x02, 0x03, 0x04})
	assert.Equal(t, "[0201] with 4 bytes", p.String())
}

func TestNewOut(t *testing.T) {
	p := NewOut(0x8020)
	assert.IsType(t, Out{}, p)
	assert.Equal(t, []byte{0x20, 0x80}, p.buf.Bytes())
}

func TestNewOutRaw(t *testing.T) {
	p := NewOutRaw()
	assert.IsType(t, Out{}, p)
	assert.Len(t, p.buf.Bytes(), 0)
}

func TestOutBool(t *testing.T) {
	p := NewOut(0x1234)
	p.Bool(true)
	p.Bool(false)
	p.Bool(true)
	p.Bool(true)
	p.Bool(false)
	assert.Equal(t, []byte{0x34, 0x12, 0x01, 0x00, 0x01, 0x01, 0x00}, p.buf.Bytes())
}

func TestOutUint8(t *testing.T) {
	p := NewOut(0x1234)
	p.Uint8(0xcc)
	p.Uint8(0xd1)
	p.Uint8(0x0a)
	assert.Equal(t, []byte{0x34, 0x12, 0xcc, 0xd1, 0x0a}, p.buf.Bytes())
}

func TestOutUint16(t *testing.T) {
	p := NewOut(0x1234)
	p.Uint16(0xcafe)
	p.Uint16(0xbabe)
	assert.Equal(t, []byte{0x34, 0x12, 0xfe, 0xca, 0xbe, 0xba}, p.buf.Bytes())
}
func TestOutUint32(t *testing.T) {
	p := NewOut(0x1234)
	p.Uint32(0xdeadbeef)
	p.Uint32(0xb008b135)
	assert.Equal(t, []byte{0x34, 0x12, 0xef, 0xbe, 0xad, 0xde, 0x35, 0xb1, 0x08, 0xb0}, p.buf.Bytes())
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
	assert.Equal(t, []byte{0x34, 0x12, 0x00, 0x80, 0xec, 0xc3, 0x7e, 0x28, 0x0e, 0x46}, p.buf.Bytes())
}

func TestOutBytes(t *testing.T) {
	p := NewOut(0x1234)
	p.Bytes([]byte{0xca, 0xfe})
	assert.Equal(t, []byte{0x34, 0x12, 0xca, 0xfe}, p.buf.Bytes())
}

func TestGetBytes(t *testing.T) {
	p := NewOut(0x1234)
	p.Uint16(0xffff)
	p.Uint32(0xcccccccc)
	assert.Equal(t, []byte{0x34, 0x12, 0xff, 0xff, 0xcc, 0xcc, 0xcc, 0xcc}, p.GetBytes())
}

func TestOutUTF16WithLengthPrefix(t *testing.T) {
	p := NewOut(0x1234)
	p.UTF16WithLengthPrefix("hello")
	assert.Equal(t, []byte{0x34, 0x12, 0x05, 0x00, 0x68, 0x00, 0x65, 0x00, 0x6c, 0x00, 0x6c, 0x00, 0x6f, 0x00}, p.buf.Bytes())

	// Now make one with a non-ASCII character
	p = NewOut(0x1234)
	p.UTF16WithLengthPrefix("\u0155")
	result := p.buf.Bytes()
	assert.Equal(t, []byte{0x34, 0x12, 0x01, 0x00, 0x55, 0x01}, result)
	// Length is in runes, NOT bytes, so check the length prefix (uint16) is correct:
	assert.Equal(t, uint16(1), uint16(result[2])|uint16(result[3])<<8)
}

func TestOutMerge(t *testing.T) {
	a := NewOut(0xbeba)
	b := NewOut(0xfeca)
	b.Merge(a)
	assert.Equal(t, []byte{0xca, 0xfe, 0xba, 0xbe}, b.buf.Bytes())
}

func TestOutReset(t *testing.T) {
	p := NewOut(0x1234)
	assert.Len(t, p.buf.Bytes(), 2)
	p.Reset()
	assert.Len(t, p.buf.Bytes(), 0)
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
