package srp

import (
	"crypto/rand"
	"crypto/sha1"
	"fmt"
	"math/big"
)

type SRPGroup struct {
	N *big.Int
	g *big.Int
}

func SRP1024() *SRPGroup {
	G := new(big.Int).SetBytes([]byte{0x02})
	N := new(big.Int).SetBytes([]byte{
		0xEE, 0xAF, 0x0A, 0xB9, 0xAD, 0xB3, 0x8D, 0xD6, 0x9C, 0x33, 0xF8, 0x0A, 0xFA, 0x8F,
		0xC5, 0xE8, 0x60, 0x72, 0x61, 0x87, 0x75, 0xFF, 0x3C, 0x0B, 0x9E, 0xA2, 0x31, 0x4C,
		0x9C, 0x25, 0x65, 0x76, 0xD6, 0x74, 0xDF, 0x74, 0x96, 0xEA, 0x81, 0xD3, 0x38, 0x3B,
		0x48, 0x13, 0xD6, 0x92, 0xC6, 0xE0, 0xE0, 0xD5, 0xD8, 0xE2, 0x50, 0xB9, 0x8B, 0xE4,
		0x8E, 0x49, 0x5C, 0x1D, 0x60, 0x89, 0xDA, 0xD1, 0x5D, 0xC7, 0xD7, 0xB4, 0x61, 0x54,
		0xD6, 0xB6, 0xCE, 0x8E, 0xF4, 0xAD, 0x69, 0xB1, 0x5D, 0x49, 0x82, 0x55, 0x9B, 0x29,
		0x7B, 0xCF, 0x18, 0x85, 0xC5, 0x29, 0xF5, 0x66, 0x66, 0x0E, 0x57, 0xEC, 0x68, 0xED,
		0xBC, 0x3C, 0x05, 0x72, 0x6C, 0xC0, 0x2F, 0xD4, 0xCB, 0xF4, 0x97, 0x6E, 0xAA, 0x9A,
		0xFD, 0x51, 0x38, 0xFE, 0x83, 0x76, 0x43, 0x5B, 0x9F, 0xC6, 0x1D, 0x2F, 0xC0, 0xEB,
		0x06, 0xE3})

	return &SRPGroup{
		N: N,
		g: G,
	}
}

type SRPLookup func(username string) (*SRPUser, error)

type SRPUser struct {
	Username string
	Salt     []byte
	Verifier *big.Int
}

type SRPServer struct {
	Group *SRPGroup

	User *SRPUser

	b *big.Int
	B *big.Int
}

func (g *SRPGroup) Multiplier() *big.Int {
	nBytes := (g.N.BitLen() + 7) / 8

	N := padBigInt(g.N, nBytes)
	G := padBigInt(g.g, nBytes)

	h := sha1.New()
	h.Write(N)
	h.Write(G)

	return new(big.Int).SetBytes(h.Sum(nil))
}
func padBigInt(v *big.Int, size int) []byte {
	out := make([]byte, size)

	b := v.Bytes()

	copy(out[size-len(b):], b)

	return out
}

func NewSRPServer(group *SRPGroup, user *SRPUser) (*SRPServer, error) {
	b := big.NewInt(123456789)
	//b, err := newRandomBigIntBytes(size)

	k := group.Multiplier()

	gb := new(big.Int).Exp(
		group.g,
		b,
		group.N,
	)

	kv := new(big.Int).Mul(
		k,
		user.Verifier,
	)

	B := new(big.Int).Add(
		kv,
		gb,
	)

	B.Mod(B, group.N)

	if B.Sign() == 0 {
		return nil, fmt.Errorf("invalid B")
	}

	return &SRPServer{
		Group: group,
		User:  user,
		b:     b,
		B:     B,
	}, nil
}
func newRandomBigInt(max *big.Int) (*big.Int, error) {
	for {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return nil, err
		}

		if n.Sign() > 0 {
			return n, nil
		}
	}
}

func newRandomBigIntBytes(size int) (*big.Int, error) {
	buf := make([]byte, size)

	if _, err := rand.Read(buf); err != nil {
		return nil, err
	}

	return new(big.Int).SetBytes(buf), nil
}

func (s *SRPServer) ComputeSharedSecret(A *big.Int) (*big.Int, error) {
	if A == nil {
		return nil, fmt.Errorf("missing A")
	}
	if A.Sign() <= 0 {
		return nil, fmt.Errorf("invalid A")
	}

	// Reject A % N == 0
	aMod := new(big.Int).Mod(new(big.Int).Set(A), s.Group.N)
	if aMod.Sign() == 0 {
		return nil, fmt.Errorf("invalid A")
	}

	size := (s.Group.N.BitLen() + 7) / 8

	ABytes := padBigInt(A, size)
	BBytes := padBigInt(s.B, size)

	h := sha1.New()
	h.Write(ABytes)
	h.Write(BBytes)

	u := new(big.Int).SetBytes(h.Sum(nil))

	// v^u mod N
	vu := new(big.Int).Exp(
		s.User.Verifier,
		u,
		s.Group.N,
	)

	// A * v^u mod N
	base := new(big.Int).Mul(
		A,
		vu,
	)
	base.Mod(base, s.Group.N)

	// (A*v^u)^b mod N
	S := new(big.Int).Exp(
		base,
		s.b,
		s.Group.N,
	)
	return S, nil
}

func (s *SRPServer) PremasterSecret(A *big.Int) ([]byte, error) {
	S, err := s.ComputeSharedSecret(A)
	if err != nil {
		return nil, err
	}

	size := (s.Group.N.BitLen() + 7) / 8

	return padBigInt(S, size), nil
}
