package srp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- parser tests ---

func TestParser_Uint8(t *testing.T) {
	p := newParser([]byte{0xAB})
	v, err := p.Uint8()
	assert.NoError(t, err)
	assert.Equal(t, uint8(0xAB), v)
	assert.True(t, p.Empty())
}

func TestParser_Uint8_Underflow(t *testing.T) {
	p := newParser([]byte{})
	_, err := p.Uint8()
	assert.ErrorIs(t, err, ErrBufferUnderflow)
}

func TestParser_Uint16(t *testing.T) {
	p := newParser([]byte{0x01, 0x02})
	v, err := p.Uint16()
	assert.NoError(t, err)
	assert.Equal(t, uint16(0x0102), v)
}

func TestParser_Uint16_Underflow(t *testing.T) {
	p := newParser([]byte{0x01})
	_, err := p.Uint16()
	assert.ErrorIs(t, err, ErrBufferUnderflow)
}

func TestParser_Uint24(t *testing.T) {
	p := newParser([]byte{0x01, 0x02, 0x03})
	v, err := p.Uint24()
	assert.NoError(t, err)
	assert.Equal(t, uint32(0x010203), v)
}

func TestParser_Uint24_Underflow(t *testing.T) {
	p := newParser([]byte{0x01, 0x02})
	_, err := p.Uint24()
	assert.ErrorIs(t, err, ErrBufferUnderflow)
}

func TestParser_Bytes(t *testing.T) {
	p := newParser([]byte{0xAA, 0xBB, 0xCC, 0xDD})
	b, err := p.Bytes(3)
	assert.NoError(t, err)
	assert.Equal(t, []byte{0xAA, 0xBB, 0xCC}, b)
	assert.Equal(t, 1, p.Remaining())
}

func TestParser_Bytes_Underflow(t *testing.T) {
	p := newParser([]byte{0x01})
	_, err := p.Bytes(2)
	assert.ErrorIs(t, err, ErrBufferUnderflow)
}

func TestParser_Bytes_NegativeLength(t *testing.T) {
	p := newParser([]byte{0x01})
	_, err := p.Bytes(-1)
	assert.Error(t, err)
}

func TestParser_Vector8(t *testing.T) {
	p := newParser([]byte{3, 0xAA, 0xBB, 0xCC})
	b, err := p.Vector8()
	assert.NoError(t, err)
	assert.Equal(t, []byte{0xAA, 0xBB, 0xCC}, b)
}

func TestParser_Vector16(t *testing.T) {
	p := newParser([]byte{0, 4, 0xDE, 0xAD, 0xBE, 0xEF})
	b, err := p.Vector16()
	assert.NoError(t, err)
	assert.Equal(t, []byte{0xDE, 0xAD, 0xBE, 0xEF}, b)
}

func TestParser_Vector24(t *testing.T) {
	p := newParser([]byte{0, 0, 3, 0x01, 0x02, 0x03})
	b, err := p.Vector24()
	assert.NoError(t, err)
	assert.Equal(t, []byte{0x01, 0x02, 0x03}, b)
}

func TestParser_Vector16Uint(t *testing.T) {
	p := newParser([]byte{0, 6, 0x00, 0x0A, 0x00, 0x14, 0x00, 0x1E})
	u, err := p.Vector16Uint()
	assert.NoError(t, err)
	assert.Equal(t, []uint16{10, 20, 30}, u)
}

func TestParser_Vector16Uint_OddLength(t *testing.T) {
	p := newParser([]byte{0, 3, 0x01, 0x02, 0x03})
	_, err := p.Vector16Uint()
	assert.Error(t, err)
}

func TestParser_Vector8Uint(t *testing.T) {
	p := newParser([]byte{2, 0x0A, 0x14})
	u, err := p.Vector8Uint()
	assert.NoError(t, err)
	assert.Equal(t, []uint8{0x0A, 0x14}, u)
}

func TestParser_Sequential(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}
	p := newParser(data)

	v8, _ := p.Uint8()
	v16, _ := p.Uint16()
	v24, _ := p.Uint24()

	assert.Equal(t, uint8(0x01), v8)
	assert.Equal(t, uint16(0x0203), v16)
	assert.Equal(t, uint32(0x040506), v24)
}

func TestParser_Empty(t *testing.T) {
	p := newParser([]byte{})
	assert.True(t, p.Empty())
	assert.Equal(t, 0, p.Remaining())
}

// --- encoder tests ---

func TestEncoder_Uint8(t *testing.T) {
	var e encoder
	e.Uint8(0xAB)
	assert.Equal(t, []byte{0xAB}, e.BytesSlice())
}

func TestEncoder_Uint16(t *testing.T) {
	var e encoder
	e.Uint16(0x0102)
	assert.Equal(t, []byte{0x01, 0x02}, e.BytesSlice())
}

func TestEncoder_Uint24(t *testing.T) {
	var e encoder
	e.Uint24(0x010203)
	assert.Equal(t, []byte{0x01, 0x02, 0x03}, e.BytesSlice())
}

func TestEncoder_Vector8(t *testing.T) {
	var e encoder
	e.Vector8([]byte{0xDE, 0xAD})
	assert.Equal(t, []byte{2, 0xDE, 0xAD}, e.BytesSlice())
}

func TestEncoder_Vector16(t *testing.T) {
	var e encoder
	e.Vector16([]byte{0xBE, 0xEF})
	assert.Equal(t, []byte{0, 2, 0xBE, 0xEF}, e.BytesSlice())
}

func TestEncoder_Vector24(t *testing.T) {
	var e encoder
	e.Vector24([]byte{0x01, 0x02, 0x03})
	assert.Equal(t, []byte{0, 0, 3, 0x01, 0x02, 0x03}, e.BytesSlice())
}

func TestEncoder_Bytes(t *testing.T) {
	var e encoder
	e.Bytes([]byte{0x01, 0x02, 0x03})
	assert.Equal(t, []byte{0x01, 0x02, 0x03}, e.BytesSlice())
}

func TestEncoder_Sequential(t *testing.T) {
	var e encoder
	e.Uint8(0x01)
	e.Uint16(0x0203)
	e.Uint24(0x040506)
	expected := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}
	assert.Equal(t, expected, e.BytesSlice())
}

// --- round-trip tests ---

func TestCodecRoundTrip_Uint8(t *testing.T) {
	var e encoder
	e.Uint8(42)

	p := newParser(e.BytesSlice())
	v, err := p.Uint8()
	require.NoError(t, err)
	assert.Equal(t, uint8(42), v)
}

func TestCodecRoundTrip_Uint16(t *testing.T) {
	var e encoder
	e.Uint16(12345)

	p := newParser(e.BytesSlice())
	v, err := p.Uint16()
	require.NoError(t, err)
	assert.Equal(t, uint16(12345), v)
}

func TestCodecRoundTrip_Uint24(t *testing.T) {
	var e encoder
	e.Uint24(1234567)

	p := newParser(e.BytesSlice())
	v, err := p.Uint24()
	require.NoError(t, err)
	assert.Equal(t, uint32(1234567), v)
}

func TestCodecRoundTrip_Vector8(t *testing.T) {
	original := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	var e encoder
	e.Vector8(original)

	p := newParser(e.BytesSlice())
	v, err := p.Vector8()
	require.NoError(t, err)
	assert.Equal(t, original, v)
}

func TestParser_Vector24_Underflow(t *testing.T) {
	p := newParser([]byte{0x01, 0x02})
	_, err := p.Vector24()
	assert.Error(t, err)
}

func TestParser_Vector24_DataUnderflow(t *testing.T) {
	p := newParser([]byte{0, 0, 5, 0x01, 0x02}) // claims 5 bytes, has 2
	_, err := p.Vector24()
	assert.Error(t, err)
}

func TestParser_Vector8Uint_Underflow(t *testing.T) {
	p := newParser([]byte{}) // empty
	_, err := p.Vector8Uint()
	assert.Error(t, err)
}

func TestParser_Vector16Uint_Empty(t *testing.T) {
	p := newParser([]byte{0, 0}) // length 0
	u, err := p.Vector16Uint()
	require.NoError(t, err)
	assert.Empty(t, u)
}

func TestEncoder_Empty(t *testing.T) {
	var e encoder
	assert.Empty(t, e.BytesSlice())
}

func TestEncoder_Vector8_Empty(t *testing.T) {
	var e encoder
	e.Vector8([]byte{})
	assert.Equal(t, []byte{0}, e.BytesSlice())
}

func TestEncoder_Vector16_Empty(t *testing.T) {
	var e encoder
	e.Vector16([]byte{})
	assert.Equal(t, []byte{0, 0}, e.BytesSlice())
}

func TestEncoder_Vector24_Empty(t *testing.T) {
	var e encoder
	e.Vector24([]byte{})
	assert.Equal(t, []byte{0, 0, 0}, e.BytesSlice())
}

func TestCodecRoundTrip_Vector16(t *testing.T) {
	original := []byte{0xDE, 0xAD, 0xBE, 0xEF, 0xCA, 0xFE}
	var e encoder
	e.Vector16(original)

	p := newParser(e.BytesSlice())
	v, err := p.Vector16()
	require.NoError(t, err)
	assert.Equal(t, original, v)
}

func TestCodecRoundTrip_Vector24(t *testing.T) {
	original := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	var e encoder
	e.Vector24(original)

	p := newParser(e.BytesSlice())
	v, err := p.Vector24()
	require.NoError(t, err)
	assert.Equal(t, original, v)
}

func TestCodecRoundTrip_Vector8Uint(t *testing.T) {
	original := []uint8{0x0A, 0x14, 0x1E}
	var e encoder
	e.Vector8([]byte(original))

	p := newParser(e.BytesSlice())
	v, err := p.Vector8Uint()
	require.NoError(t, err)
	assert.Equal(t, original, v)
}
