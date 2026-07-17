// This package defines the Appearance struct and provides functions to parse and build appearance bits.

package GameService

// Appearance represents the attributes of a game character's appearance.
// The struct includes bits for gender, height, skin color, hair color,
// face style, primary profession, hair style, and campaign.
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

// ParseAppearanceBits converts a uint32 of bits into an Appearance struct.
// Each bit represents a specific attribute of the character's appearance.
func ParseAppearanceBits(bits uint32) Appearance {
	return Appearance{
		Female:            (bits & 0x1) != 0,          // Bit 0: Female (1 for female, 0 for male)
		Height:            uint8((bits >> 1) & 0xF),   // Bits 1-4: Height (0-15)
		SkinColor:         uint8((bits >> 5) & 0x1F),  // Bits 5-9: Skin color (0-31)
		HairColor:         uint8((bits >> 10) & 0x1F), // Bits 10-15: Hair color (0-31)
		FaceStyle:         uint8((bits >> 15) & 0x1F), // Bits 16-23: Face style (0-31)
		PrimaryProfession: uint8((bits >> 20) & 0xF),  // Bits 24-27: Primary profession (0-15)
		HairStyle:         uint8((bits >> 24) & 0x3F), // Bits 28-35: Hair style (0-63)
		Campaign:          uint8((bits >> 30) & 0x3),  // Bits 36-38: Campaign (0-3)
	}
}

// BuildAppearanceBits creates a uint32 of bits representing the character's appearance.
// The bits are constructed by shifting the corresponding attributes into their respective positions.
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
		bits |= 1 // Bit 0: Female (1 for female, 0 for male)
	}

	bits |= (uint32(height) & 0xF) << 1             // Bits 1-4: Height (0-15)
	bits |= (uint32(skinColor) & 0x1F) << 5         // Bits 5-9: Skin color (0-31)
	bits |= (uint32(hairColor) & 0x1F) << 10        // Bits 10-15: Hair color (0-31)
	bits |= (uint32(faceStyle) & 0x1F) << 15        // Bits 16-23: Face style (0-31)
	bits |= (uint32(primaryProfession) & 0xF) << 20 // Bits 24-27: Primary profession (0-15)
	bits |= (uint32(hairStyle) & 0x3F) << 24        // Bits 28-35: Hair style (0-63)
	bits |= (uint32(campaign) & 0x3) << 30          // Bits 36-38: Campaign (0-3)
	return bits
}
