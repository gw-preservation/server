package gameservice

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseAppearanceBits_Male(t *testing.T) {
	a := ParseAppearanceBits(0)
	assert.False(t, a.Female)
}

func TestParseAppearanceBits_Female(t *testing.T) {
	a := ParseAppearanceBits(0x1)
	assert.True(t, a.Female)
}

func TestParseAppearanceBits_Height(t *testing.T) {
	a := ParseAppearanceBits(0x2) // height = 1
	assert.Equal(t, uint8(1), a.Height)

	a = ParseAppearanceBits(0x10) // height = 8
	assert.Equal(t, uint8(8), a.Height)
}

func TestParseAppearanceBits_SkinColor(t *testing.T) {
	a := ParseAppearanceBits(0xA0) // skinColor = 5
	assert.Equal(t, uint8(5), a.SkinColor)
}

func TestParseAppearanceBits_HairColor(t *testing.T) {
	a := ParseAppearanceBits(0x2800) // 10 << 10 = hairColor = 10
	assert.Equal(t, uint8(10), a.HairColor)
}

func TestParseAppearanceBits_FaceStyle(t *testing.T) {
	a := ParseAppearanceBits(0x28000) // faceStyle = 5
	assert.Equal(t, uint8(5), a.FaceStyle)
}

func TestParseAppearanceBits_PrimaryProfession(t *testing.T) {
	a := ParseAppearanceBits(0x300000) // primaryProfession = 3
	assert.Equal(t, uint8(3), a.PrimaryProfession)
}

func TestParseAppearanceBits_HairStyle(t *testing.T) {
	a := ParseAppearanceBits(0x5000000) // hairStyle = 5
	assert.Equal(t, uint8(5), a.HairStyle)
}

func TestParseAppearanceBits_Campaign(t *testing.T) {
	a := ParseAppearanceBits(0xC0000000) // campaign = 3
	assert.Equal(t, uint8(3), a.Campaign)
}

func TestParseAppearanceBits_AllFields(t *testing.T) {
	bits := uint32(0x1)      // female
	bits |= uint32(7) << 1   // height = 7
	bits |= uint32(25) << 5  // skinColor = 25
	bits |= uint32(15) << 10 // hairColor = 15
	bits |= uint32(31) << 15 // faceStyle = 31
	bits |= uint32(12) << 20 // primaryProfession = 12
	bits |= uint32(50) << 24 // hairStyle = 50
	bits |= uint32(2) << 30  // campaign = 2

	a := ParseAppearanceBits(bits)
	assert.True(t, a.Female)
	assert.Equal(t, uint8(7), a.Height)
	assert.Equal(t, uint8(25), a.SkinColor)
	assert.Equal(t, uint8(15), a.HairColor)
	assert.Equal(t, uint8(31), a.FaceStyle)
	assert.Equal(t, uint8(12), a.PrimaryProfession)
	assert.Equal(t, uint8(50), a.HairStyle)
	assert.Equal(t, uint8(2), a.Campaign)
}

func TestBuildAppearanceBits_Male(t *testing.T) {
	bits := BuildAppearanceBits(false, 0, 0, 0, 0, 0, 0, 0)
	assert.Equal(t, uint32(0), bits)
}

func TestBuildAppearanceBits_Female(t *testing.T) {
	bits := BuildAppearanceBits(true, 0, 0, 0, 0, 0, 0, 0)
	assert.Equal(t, uint32(1), bits)
}

func TestBuildAppearanceBits_Height(t *testing.T) {
	bits := BuildAppearanceBits(false, 1, 0, 0, 0, 0, 0, 0)
	assert.Equal(t, uint32(0x2), bits)
}

func TestBuildAppearanceBits_AllFields(t *testing.T) {
	bits := BuildAppearanceBits(true, 7, 25, 15, 31, 12, 50, 2)

	expected := uint32(0x1)
	expected |= uint32(7) << 1
	expected |= uint32(25) << 5
	expected |= uint32(15) << 10
	expected |= uint32(31) << 15
	expected |= uint32(12) << 20
	expected |= uint32(50) << 24
	expected |= uint32(2) << 30
	assert.Equal(t, expected, bits)
}

func TestBuildAppearanceBits_MaxValues(t *testing.T) {
	bits := BuildAppearanceBits(true, 15, 31, 31, 31, 15, 63, 3)
	assert.NotZero(t, bits)

	a := ParseAppearanceBits(bits)
	assert.True(t, a.Female)
	assert.Equal(t, uint8(15), a.Height)
	assert.Equal(t, uint8(31), a.SkinColor)
	assert.Equal(t, uint8(31), a.HairColor)
	assert.Equal(t, uint8(31), a.FaceStyle)
	assert.Equal(t, uint8(15), a.PrimaryProfession)
	assert.Equal(t, uint8(63), a.HairStyle)
	assert.Equal(t, uint8(3), a.Campaign)
}

func TestBuildAndParse_RoundTrip(t *testing.T) {
	original := BuildAppearanceBits(true, 7, 25, 15, 31, 12, 50, 2)
	parsed := ParseAppearanceBits(original)

	assert.True(t, parsed.Female)
	assert.Equal(t, uint8(7), parsed.Height)
	assert.Equal(t, uint8(25), parsed.SkinColor)
	assert.Equal(t, uint8(15), parsed.HairColor)
	assert.Equal(t, uint8(31), parsed.FaceStyle)
	assert.Equal(t, uint8(12), parsed.PrimaryProfession)
	assert.Equal(t, uint8(50), parsed.HairStyle)
	assert.Equal(t, uint8(2), parsed.Campaign)
}

func TestParseAndBuild_RoundTrip(t *testing.T) {
	a := Appearance{
		Female:            false,
		Height:            10,
		SkinColor:         20,
		HairColor:         5,
		FaceStyle:         18,
		PrimaryProfession: 8,
		HairStyle:         42,
		Campaign:          1,
	}

	bits := BuildAppearanceBits(a.Female, a.Height, a.SkinColor, a.HairColor,
		a.FaceStyle, a.PrimaryProfession, a.HairStyle, a.Campaign)
	result := ParseAppearanceBits(bits)

	assert.Equal(t, a, result)
}

func TestBuildAppearanceBits_MasksOverflow(t *testing.T) {
	bits := BuildAppearanceBits(false, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF)
	a := ParseAppearanceBits(bits)

	assert.Equal(t, uint8(15), a.Height)            // 4 bits: 0xFF & 0xF = 0xF
	assert.Equal(t, uint8(31), a.SkinColor)         // 5 bits: 0xFF & 0x1F = 0x1F
	assert.Equal(t, uint8(31), a.HairColor)         // 5 bits
	assert.Equal(t, uint8(31), a.FaceStyle)         // 5 bits
	assert.Equal(t, uint8(15), a.PrimaryProfession) // 4 bits
	assert.Equal(t, uint8(63), a.HairStyle)         // 6 bits: 0xFF & 0x3F = 0x3F
	assert.Equal(t, uint8(3), a.Campaign)           // 2 bits
}

func TestParseAppearanceBits_ZeroBits(t *testing.T) {
	a := ParseAppearanceBits(0)
	assert.False(t, a.Female)
	assert.Equal(t, uint8(0), a.Height)
	assert.Equal(t, uint8(0), a.SkinColor)
	assert.Equal(t, uint8(0), a.HairColor)
	assert.Equal(t, uint8(0), a.FaceStyle)
	assert.Equal(t, uint8(0), a.PrimaryProfession)
	assert.Equal(t, uint8(0), a.HairStyle)
	assert.Equal(t, uint8(0), a.Campaign)
}

func TestParseAppearanceBits_KnownValue(t *testing.T) {
	a := ParseAppearanceBits(0x0744943b)
	assert.True(t, a.Female)
	assert.Equal(t, uint8(13), a.Height)
	assert.Equal(t, uint8(1), a.SkinColor)
	assert.Equal(t, uint8(5), a.HairColor)
	assert.Equal(t, uint8(9), a.FaceStyle)
	assert.Equal(t, uint8(4), a.PrimaryProfession)
	assert.Equal(t, uint8(7), a.HairStyle)
	assert.Equal(t, uint8(0), a.Campaign)
}
