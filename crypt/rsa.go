package crypt

import (
	"crypto/rand"
	"encoding/binary"
	"math/big"
)

var serverPrivateKey = []byte{
	0x21, 0x33, 0x6E, 0x83, 0x1E, 0xF1, 0x42, 0x63, 0xC6, 0x02, 0xF1, 0xB7, 0xDD, 0x23, 0x39, 0x6E,
	0xC8, 0x53, 0x76, 0x2B, 0xD2, 0x0D, 0xA9, 0x6E, 0x2D, 0xB6, 0x66, 0x5F, 0xE7, 0x3B, 0x9D, 0xB7,
	0xC1, 0xFF, 0x70, 0x09, 0x00, 0x7E, 0x58, 0x3F, 0x33, 0x0F, 0x4F, 0x4C, 0x8F, 0x12, 0x54, 0x7D,
	0xEA, 0xD0, 0xF7, 0xD1, 0xDD, 0xB8, 0x66, 0xA8, 0xF3, 0x8F, 0x6D, 0x45, 0x5E, 0x7D, 0x96, 0x62,
}
var serverPrivateKeyBI = bytesToBI(serverPrivateKey)

// ServerPubKey and SharedPrime are baked into the client
var serverPubKey = []byte{
	0x5d, 0xcb, 0x5b, 0x03, 0x49, 0x50, 0x63, 0xc0, 0xf1, 0x8a, 0x4a, 0xa5, 0x8d, 0x9d, 0x88, 0x00,
	0x0f, 0x41, 0x81, 0xbb, 0xc3, 0x43, 0x03, 0xfd, 0x85, 0x6a, 0x7c, 0x51, 0x6c, 0x24, 0x09, 0x2a,
	0xb6, 0x5d, 0x82, 0x13, 0x28, 0x14, 0x44, 0x7e, 0x57, 0xca, 0xc7, 0x3d, 0x82, 0x91, 0x3e, 0x59,
	0x2b, 0xa2, 0xb5, 0xfa, 0x7b, 0xe2, 0x97, 0xb2, 0x82, 0xe9, 0xe8, 0x9a, 0x01, 0xe1, 0xe2, 0x88,
}
var serverPubKeyBI = bytesToBI(serverPubKey)

// Note this is the same as the one currently in the Client
var sharedPrime = byteSwap([]byte{
	0xF1, 0x2F, 0x1E, 0xDF, 0x1F, 0xFD, 0x3E, 0x05, 0xD6, 0xED, 0x4E, 0x44, 0x73, 0x78, 0x05, 0x6B,
	0x30, 0xE5, 0x72, 0xED, 0x17, 0x05, 0x20, 0x12, 0x12, 0x09, 0x9B, 0x67, 0x30, 0xA1, 0x86, 0x36,
	0x5A, 0x90, 0xFF, 0x69, 0x89, 0x15, 0xC6, 0xE2, 0x4C, 0xD3, 0xF2, 0x55, 0xAE, 0x55, 0x90, 0x05,
	0x28, 0xA2, 0x37, 0x42, 0x4A, 0xA2, 0x8A, 0xA8, 0x22, 0xC9, 0xF9, 0xE3, 0xDF, 0x59, 0x2C, 0xFD,
})
var sharedPrimeBI = bytesToBI(sharedPrime)

func bytesToBI(src []byte) *big.Int {
	i := big.NewInt(0)
	return i.SetBytes(src)
}
func modPow(base, exp, mod *big.Int) *big.Int {
	return new(big.Int).Exp(base, exp, mod)
}

func byteSwap(data []byte) []byte {
	// Create a new slice with the same length as the input slice
	reversed := make([]byte, len(data))

	// Copy elements from the original slice to the reversed slice
	for i, j := 0, len(data)-1; i < len(data); i, j = i+1, j-1 {
		reversed[i] = data[j]
	}

	return reversed
}

// KeyDerivation borrows SHA-1's ARX skeleton — ROL5, ROL30, 5×32-bit state,
// and the round constant 0x6ED9EBA1 (SHA-1 rounds 20–39) — but replaces the
// round function with a custom nonlinear function.
func KeyDerivation(data [20]byte) [20]byte {
	a := binary.LittleEndian.Uint32(data[0:4])
	b := binary.LittleEndian.Uint32(data[4:8])
	c := binary.LittleEndian.Uint32(data[8:12])
	d := binary.LittleEndian.Uint32(data[12:16])
	e := binary.LittleEndian.Uint32(data[16:20])

	// Blend a, b with round constants.
	// ab and bb are pre-mix values used throughout; a, b are updated in-place.
	ab := a + 0x9FB498B3
	a += 0x16745230
	bb := b + 0x66B0CD0D + rol(ab, 5)
	b -= 0x61032548

	// Accumulate into c through the nonlinear function.
	nl := ^(ab & 0x22222222) & 0x7BF36AE2
	acc := rol(bb, 5) + c + nl + 0xF33D5697
	c += acc

	// Cross-mix into d via XOR feedback.
	fb := rol(ab, 30) + 0xA90303AC + d + (bb ^ acc ^ 0x7BF36AE2)

	// Cascade: d absorbs the remainder using original e, then e absorbs feedback.
	d += rol(fb, 5) + e + (acc ^ 0xC72D9278) + 0x6ED9EBA1 + bb
	e += fb

	var out [20]byte
	binary.LittleEndian.PutUint32(out[0:4], a)
	binary.LittleEndian.PutUint32(out[4:8], b)
	binary.LittleEndian.PutUint32(out[8:12], c)
	binary.LittleEndian.PutUint32(out[12:16], d)
	binary.LittleEndian.PutUint32(out[16:20], e)
	return out
}

// rol performs a left rotation on a 32-bit unsigned integer.
func rol(x uint32, n uint) uint32 {
	return (x << n) | (x >> (32 - n))
}

func GenerateEncryptionKeyWithRandomBytes(clientBytes [64]byte, randomBytes [20]byte) ([20]byte, [20]byte) {
	// Reverse clientBytes (little-endian → big-endian for SetBytes)
	buf := byteSwap(clientBytes[:])
	seedBI := new(big.Int).SetBytes(buf)
	secretKey := modPow(seedBI, serverPrivateKeyBI, sharedPrimeBI).Bytes()

	// Now we gotta do the hash thing on top of those bytes
	rc4Key := KeyDerivation(randomBytes)
	var xored [20]byte
	for i := range 20 {
		// secretKey is big-endian; reverse-index to get little-endian byte i.
		// If the DH result is short (leading zeros stripped), treat missing bytes as 0.
		secretIdx := len(secretKey) - 1 - i
		var sk byte
		if secretIdx >= 0 {
			sk = secretKey[secretIdx]
		}
		xored[i] = randomBytes[i] ^ sk
	}
	return rc4Key, xored
}

func GenerateEncryptionKey(clientBytes [64]byte) ([20]byte, [20]byte) {
	var randomBytes [20]byte
	rand.Read(randomBytes[:])
	return GenerateEncryptionKeyWithRandomBytes(clientBytes, randomBytes)
}
