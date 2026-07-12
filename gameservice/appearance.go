package GameService

type Appearance struct {
	Female            bool
	Height            uint8
	SkinColor         uint8
	HairColor         uint8
	FaceStyle         uint8
	PrimaryProfession uint8
	HairStyle         uint8
	Campaign          uint8
}

func ParseAppearanceBits(bits uint32) Appearance {
	return Appearance{
		Female:            (bits & 0x1) != 0,
		Height:            uint8((bits >> 1) & 0xF),
		SkinColor:         uint8((bits >> 5) & 0x1F),
		HairColor:         uint8((bits >> 10) & 0x1F),
		FaceStyle:         uint8((bits >> 15) & 0x1F),
		PrimaryProfession: uint8((bits >> 20) & 0xF),
		HairStyle:         uint8((bits >> 24) & 0x3F),
		Campaign:          uint8((bits >> 30) & 0x3),
	}
}

func BuildAppearanceBits(
	female bool,
	height uint8,
	skinColor uint8,
	hairColor uint8,
	faceStyle uint8,
	primaryProfession uint8,
	hairStyle uint8,
	campaign uint8,
) uint32 {
	var bits uint32

	if female {
		bits |= 1
	}

	bits |= (uint32(height) & 0xF) << 1
	bits |= (uint32(skinColor) & 0x1F) << 5
	bits |= (uint32(hairColor) & 0x1F) << 10
	bits |= (uint32(faceStyle) & 0x1F) << 15
	bits |= (uint32(primaryProfession) & 0xF) << 20
	bits |= (uint32(hairStyle) & 0x3F) << 24
	bits |= (uint32(campaign) & 0x3) << 30

	return bits
}
