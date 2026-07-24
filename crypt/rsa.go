package crypt

import (
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

// Seed is generated each run. For some cases we can use a static one for debugging RSA.
var staticSeed = byteSwap([]byte{
	0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F,
	0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19, 0x1A, 0x1B, 0x1C, 0x1D, 0x1E, 0x1F,
	0x20, 0x21, 0x22, 0x23, 0x24, 0x25, 0x26, 0x27, 0x28, 0x29, 0x2A, 0x2B, 0x2C, 0x2D, 0x2E, 0x2F,
	0x30, 0x31, 0x32, 0x33, 0x34, 0x35, 0x36, 0x37, 0x38, 0x39, 0x3A, 0x3B, 0x3C, 0x3D, 0x3E, 0x3F,
})
var staticSeedBI = bytesToBI(staticSeed)

// Four is the base used in creating shared key
var fourBI = big.NewInt(4)

func bytesToBI(src []byte) *big.Int {
	i := big.NewInt(0)
	return i.SetBytes(src)
}

func modPow(base, exp, mod *big.Int) *big.Int {
	// Create new variables to avoid mutating the original arguments
	baseCopy := new(big.Int).Set(base)
	expCopy := new(big.Int).Set(exp)
	modCopy := new(big.Int).Set(mod)

	result := big.NewInt(1)
	baseCopy.Mod(baseCopy, modCopy) // Ensure base is within the modulus

	for expCopy.Cmp(big.NewInt(0)) > 0 { // While exp > 0
		if new(big.Int).And(expCopy, big.NewInt(1)).Cmp(big.NewInt(1)) == 0 { // If exp is odd
			result.Mul(result, baseCopy)
			result.Mod(result, modCopy)
		}
		baseCopy.Mul(baseCopy, baseCopy)    // Square the base
		baseCopy.Mod(baseCopy, modCopy)     // Take mod
		expCopy.Div(expCopy, big.NewInt(2)) // exp = exp / 2
	}

	return result
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

func KeyDerivationOld(data [20]byte) [20]byte {
	var first, second, third, fourth, fifth uint32
	first = binary.LittleEndian.Uint32(data[0:4])
	second = binary.LittleEndian.Uint32(data[4:8])
	third = binary.LittleEndian.Uint32(data[8:12])
	fourth = binary.LittleEndian.Uint32(data[12:16])
	fifth = binary.LittleEndian.Uint32(data[16:20])
	var eax, ebx, ecx, edx, edi, esi, arg1 uint32
	eax = 0
	ebx = 0
	ecx = 0
	edx = 0
	edi = 0
	esi = 0
	eax = 0
	arg1 = 0

	ecx = first                  // MOV ECX,DWORD PTR SS:[LOCAL.6]
	eax = 0x67452301             // MOV EAX,67452301
	eax = rol(eax, 5)            // rol EAX,5
	edi = second                 // MOV EDI,DWORD PTR SS:[LOCAL.5]
	esi = ecx + eax + 0xB7103887 // LEA ESI,[ECX+EAX+B7103887]
	eax = 0xEFCDAB89             // MOV EAX,EFCDAB89
	eax = rol(eax, 0x1E)         // rol EAX,1E
	edx = esi                    // MOV EDX,ESI
	ecx = eax                    // MOV ECX,EAX
	edx = rol(edx, 5)            // rol EDX,5
	ecx &= 0x67452301            // AND ECX,67452301
	edi += edx                   // ADD EDI,EDX
	ecx ^= 0x98BADCFE            // XOR ECX,98BADCFE
	edx = 0x67452301             // MOV EDX,67452301
	edx = rol(edx, 0x1E)         // rol EDX,1E
	edi = edi + ecx + 0x6AB4CE0F // LEA EDI,[EDI+ECX+nvd3dum.6AB4CE0F]
	ebx = eax                    // MOV EBX,EAX
	ecx = edi                    // MOV ECX,EDI
	ebx ^= edx                   // XOR EBX,EDX
	ecx = rol(ecx, 5)            // rol ECX,5
	ecx += third                 // ADD ECX,DWORD PTR SS:[LOCAL.4]
	ebx &= esi                   // AND EBX,ESI
	ebx ^= eax                   // XOR EBX,EAX
	arg1 = edi                   // MOV DWORD PTR SS:[ARG.1],EDI
	esi = rol(esi, 0x1E)         // rol ESI,1E
	ecx = ecx + ebx + 0xF33D5697 // LEA ECX,[ECX+EBX+F33D5697]
	ebx = edx                    // MOV EBX,EDX
	ebx = rol(ebx, 5)            // rol EBX,5
	ebx += fourth                // ADD EBX,DWORD PTR SS:[LOCAL.3]
	edi ^= ecx                   // XOR EDI,ECX
	edi ^= eax                   // XOR EDI,EAX
	ebx += esi                   // ADD EBX,ESI
	eax = rol(eax, 0x1E)         // rol EAX,1E
	esi = ebx + edi + 0x6ED9EBA1 // LEA ESI,[EBX+EDI+6ED9EBA1]
	ebx = first                  // MOV EBX,DWORD PTR SS:[LOCAL.6]
	edi = edx                    // MOV EDI,EDX
	edi = rol(edi, 0x1E)         // rol EDI,1E
	ebx += edi                   // ADD EBX,EDI
	edi = third                  // MOV EDI,DWORD PTR SS:[LOCAL.4]
	first = ebx                  // MOV DWORD PTR SS:[LOCAL.6],EBX
	ebx = second                 // MOV EBX,DWORD PTR SS:[LOCAL.5]
	edi += ecx                   // ADD EDI,ECX
	ebx += eax                   // ADD EBX,EAX
	third = edi                  // MOV DWORD PTR SS:[LOCAL.4],EDI
	ecx ^= eax                   // XOR ECX,EAX
	eax = fifth                  // MOV EAX,DWORD PTR SS:[LOCAL.2]
	edi = esi                    // MOV EDI,ESI
	ecx ^= edx                   // XOR ECX,EDX
	edx = eax                    // MOV EDX,EAX
	edi = rol(edi, 5)            // rol EDI,5
	edx += edi                   // ADD EDX,EDI
	eax += esi                   // ADD EAX,ESI
	ecx += edx                   // ADD ECX,EDX
	edx = arg1                   // MOV EDX,DWORD PTR SS:[ARG.1]
	ecx += edx                   // ADD ECX,EDX
	edx = fourth                 // MOV EDX,DWORD PTR SS:[LOCAL.3]
	fifth = eax                  // MOV DWORD PTR SS:[LOCAL.2],EAX
	// eax = fifth//MOV EAX,DWORD PTR SS:[LOCAL.1]
	ecx = ecx + edx + 0x6ED9EBA1 // LEA ECX,[ECX+EDX+6ED9EBA1]
	edx ^= edx                   // XOR EDX,EDX
	fourth = ecx                 // MOV DWORD PTR SS:[LOCAL.3],ECX
	second = ebx                 // MOV DWORD PTR SS:[LOCAL.5],EBX

	out := make([]byte, 20)
	binary.LittleEndian.PutUint32(out[0:4], first)
	binary.LittleEndian.PutUint32(out[4:8], second)
	binary.LittleEndian.PutUint32(out[8:12], third)
	binary.LittleEndian.PutUint32(out[12:16], fourth)
	binary.LittleEndian.PutUint32(out[16:20], fifth)
	return [20]byte(out)
}

func KeyDerivationNew(data [20]byte) [20]byte {
	var first, second, third, fourth, fifth uint32
	first = binary.LittleEndian.Uint32(data[0:4])
	second = binary.LittleEndian.Uint32(data[4:8])
	third = binary.LittleEndian.Uint32(data[8:12])
	fourth = binary.LittleEndian.Uint32(data[12:16])
	fifth = binary.LittleEndian.Uint32(data[16:20])
	var eax, ebx, ecx, edx, edi, esi uint32
	eax = 0
	ebx = 0
	ecx = 0
	edx = 0
	edi = 0
	esi = 0
	eax = 0

	// MOV EDI,DWORD PTR SS:[EBP-18]            ;  uint32_t[0] goes into EDI
	edi = first
	// MOV EBX,DWORD PTR SS:[EBP-14]            ;  uint32_t[1] goes into EBX
	ebx = second
	// ADD EDI,9FB498B3
	edi += 0x9FB498B3
	// MOV EDX,DWORD PTR SS:[EBP-10]            ;  uint32_t[2] goes into EDX
	edx = third
	// MOV EAX,EDI
	eax = edi
	// ROL EAX,5
	eax = rol(eax, 5)
	// ADD DWORD PTR SS:[EBP-18],16745230
	first += 0x16745230
	// LEA ESI,DWORD PTR DS:[EBX+66B0CD0D]
	esi = ebx + 0x66B0CD0D // VERIFY
	// SUB EBX,61032548
	ebx -= 0x61032548
	// ADD ESI,EAX
	esi += eax
	// MOV DWORD PTR SS:[EBP-14],EBX
	second = ebx
	// MOV EAX,EDI
	eax = edi
	// MOV ECX,ESI
	ecx = esi
	// AND EAX,22222222
	eax &= 0x22222222
	// ROL ECX,5
	ecx = rol(ecx, 5)
	// NOT EAX
	eax = ^eax
	// ROL EDI,1E
	edi = rol(edi, 0x1E)
	// AND EAX,7BF36AE2
	eax &= 0x7BF36AE2
	// ADD ECX,EDX
	ecx += edx
	// ADD EAX,F33D5697
	eax += 0xF33D5697
	// ADD EDI,A90303AC
	edi += 0xA90303AC
	// ADD EDI,DWORD PTR SS:[EBP-C]             ;  uint32_t[3] read here
	edi += fourth
	// ADD ECX,EAX
	ecx += eax
	// MOV EAX,ESI
	eax = esi
	// MOV EBX,ECX
	ebx = ecx
	// XOR EAX,ECX
	eax = eax ^ ecx
	// ADD EDX,EBX
	edx += ebx
	// MOV ECX,DWORD PTR SS:[EBP-8]             ;  uint32_t[4] goes into ECX
	ecx = fifth
	// XOR EAX,7BF36AE2
	eax = eax ^ 0x7BF36AE2
	// ADD EDI,EAX
	edi += eax
	// MOV DWORD PTR SS:[EBP-10],EDX
	third = edx
	// MOV EAX,EDI
	eax = edi
	// XOR EBX,C72D9278
	ebx = ebx ^ 0xC72D9278
	// ROL EAX,5
	eax = rol(eax, 5)
	// ADD EBX,6ED9EBA1
	ebx += 0x6ED9EBA1
	// ADD EAX,ECX
	eax += ecx
	// ADD ECX,EDI
	ecx += edi
	// ADD EAX,EBX
	eax += ebx
	// MOV DWORD PTR SS:[EBP-8],ECX
	fifth = ecx
	// MOV EBX,DWORD PTR SS:[EBP-24]
	// ADD EAX,ESI
	eax += esi
	// ADD DWORD PTR SS:[EBP-C],EAX
	fourth += eax
	out := make([]byte, 20)
	binary.LittleEndian.PutUint32(out[0:4], first)
	binary.LittleEndian.PutUint32(out[4:8], second)
	binary.LittleEndian.PutUint32(out[8:12], third)
	binary.LittleEndian.PutUint32(out[12:16], fourth)
	binary.LittleEndian.PutUint32(out[16:20], fifth)
	return [20]byte(out)
}

// rol performs a left rotation on a 32-bit unsigned integer.
func rol(x uint32, n uint) uint32 {
	return (x << n) | (x >> (32 - n))
}

func GenerateEncryptionKeyWithRandomBytes(clientBytes [64]byte, randomBytes [20]byte) ([20]byte, [20]byte) {
	// clientBytes are received LittleEndian, we use BigEndian
	seedBI := bytesToBI(byteSwap(clientBytes[:]))
	secretKey := modPow(seedBI, serverPrivateKeyBI, sharedPrimeBI).Bytes()
	secretKeyByteSwapped := byteSwap(secretKey)

	// Now we gotta do the hash thing on top of those bytes
	rc4Key := KeyDerivationOld(randomBytes)
	xored := make([]byte, len(randomBytes))
	for i := range len(xored) {
		xored[i] = randomBytes[i] ^ secretKeyByteSwapped[i]
	}
	return [20]byte(rc4Key), [20]byte(xored)
}

func GenerateEncryptionKey(clientBytes [64]byte) ([20]byte, [20]byte) {
	//var randomBytes [20]byte
	//rand.Read(randomBytes[:])
	return GenerateEncryptionKeyWithRandomBytes(clientBytes, [20]byte(staticSeed))
}
