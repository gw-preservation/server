package srp

import (
	"crypto/rand"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSRP1024_Group(t *testing.T) {
	g := SRP1024()
	assert.Equal(t, big.NewInt(2), g.g)
	assert.NotNil(t, g.N)
	assert.Equal(t, 1024, g.N.BitLen())
}

func TestSRP1024_Multiplier(t *testing.T) {
	g := SRP1024()
	k := g.Multiplier()
	assert.NotNil(t, k)
	assert.True(t, k.Sign() > 0)
}

func TestPadBigInt(t *testing.T) {
	v := big.NewInt(0xFF)
	padded := padBigInt(v, 4)
	assert.Equal(t, 4, len(padded))
	assert.Equal(t, byte(0), padded[0])
	assert.Equal(t, byte(0), padded[1])
	assert.Equal(t, byte(0), padded[2])
	assert.Equal(t, byte(0xFF), padded[3])
}

func TestPadBigInt_Large(t *testing.T) {
	v := new(big.Int).SetBytes([]byte{1, 2, 3, 4, 5})
	padded := padBigInt(v, 5)
	assert.Equal(t, []byte{1, 2, 3, 4, 5}, padded)
}

func TestPadBigInt_ZeroPad(t *testing.T) {
	v := big.NewInt(0)
	padded := padBigInt(v, 3)
	assert.Equal(t, []byte{0, 0, 0}, padded)
}

func TestNewSRPServer(t *testing.T) {
	g := SRP1024()
	user := &SRPUser{
		Username: "test",
		Salt:     []byte{1, 2, 3, 4},
		Verifier: big.NewInt(42),
	}

	server, err := NewSRPServer(g, user)
	require.NoError(t, err)
	assert.NotNil(t, server)
	assert.NotNil(t, server.B)
	assert.True(t, server.B.Sign() > 0)
	assert.Equal(t, g, server.Group)
	assert.Equal(t, user, server.User)
}

func TestComputeSharedSecret(t *testing.T) {
	g := SRP1024()
	user := &SRPUser{
		Username: "test",
		Salt:     []byte{1, 2, 3, 4},
		Verifier: big.NewInt(42),
	}

	server, err := NewSRPServer(g, user)
	require.NoError(t, err)

	// Simulate client A value
	A := new(big.Int).Exp(g.g, big.NewInt(999999), g.N)

	S, err := server.ComputeSharedSecret(A)
	require.NoError(t, err)
	assert.NotNil(t, S)
	assert.True(t, S.Sign() > 0)
}

func TestComputeSharedSecret_NilA(t *testing.T) {
	g := SRP1024()
	user := &SRPUser{
		Username: "test",
		Salt:     []byte{1, 2, 3, 4},
		Verifier: big.NewInt(42),
	}

	server, err := NewSRPServer(g, user)
	require.NoError(t, err)

	_, err = server.ComputeSharedSecret(nil)
	assert.Error(t, err)
}

func TestComputeSharedSecret_ZeroA(t *testing.T) {
	g := SRP1024()
	user := &SRPUser{
		Username: "test",
		Salt:     []byte{1, 2, 3, 4},
		Verifier: big.NewInt(42),
	}

	server, err := NewSRPServer(g, user)
	require.NoError(t, err)

	_, err = server.ComputeSharedSecret(big.NewInt(0))
	assert.Error(t, err)
}

func TestComputeSharedSecret_AModNZero(t *testing.T) {
	g := SRP1024()
	user := &SRPUser{
		Username: "test",
		Salt:     []byte{1, 2, 3, 4},
		Verifier: big.NewInt(42),
	}

	server, err := NewSRPServer(g, user)
	require.NoError(t, err)

	// A = N, so A mod N = 0
	_, err = server.ComputeSharedSecret(new(big.Int).Set(g.N))
	assert.Error(t, err)
}

func TestPremasterSecret(t *testing.T) {
	g := SRP1024()
	user := &SRPUser{
		Username: "test",
		Salt:     []byte{1, 2, 3, 4},
		Verifier: big.NewInt(42),
	}

	server, err := NewSRPServer(g, user)
	require.NoError(t, err)

	A := new(big.Int).Exp(g.g, big.NewInt(999999), g.N)

	pm, err := server.PremasterSecret(A)
	require.NoError(t, err)
	assert.Equal(t, 128, len(pm)) // 1024 bits = 128 bytes
}

func TestPremasterSecret_NilA(t *testing.T) {
	g := SRP1024()
	user := &SRPUser{
		Username: "test",
		Salt:     []byte{1, 2, 3, 4},
		Verifier: big.NewInt(42),
	}

	server, err := NewSRPServer(g, user)
	require.NoError(t, err)

	_, err = server.PremasterSecret(nil)
	assert.Error(t, err)
}

func TestNewRandomBigInt(t *testing.T) {
	max := big.NewInt(1000)
	for i := 0; i < 10; i++ {
		n, err := newRandomBigInt(max)
		require.NoError(t, err)
		assert.True(t, n.Sign() > 0)
		assert.True(t, n.Cmp(max) < 0)
	}
}

func TestNewRandomBigIntBytes(t *testing.T) {
	n, err := newRandomBigIntBytes(32)
	require.NoError(t, err)
	assert.NotNil(t, n)
	assert.True(t, n.Sign() > 0)
}

func TestSRPVerifier(t *testing.T) {
	g := SRP1024()
	salt := []byte{0x01, 0x02, 0x03, 0x04}
	v := SRPVerifier("user", "pass", salt, g)
	assert.NotNil(t, v)
	assert.True(t, v.Sign() > 0)
}

func TestSRPVerifier_Deterministic(t *testing.T) {
	g := SRP1024()
	salt := []byte{0x01, 0x02, 0x03, 0x04}
	v1 := SRPVerifier("user", "pass", salt, g)
	v2 := SRPVerifier("user", "pass", salt, g)
	assert.Equal(t, v1, v2)
}

func TestSRPVerifier_DifferentPasswords(t *testing.T) {
	g := SRP1024()
	salt := []byte{0x01, 0x02, 0x03, 0x04}
	v1 := SRPVerifier("user", "pass1", salt, g)
	v2 := SRPVerifier("user", "pass2", salt, g)
	assert.NotEqual(t, v1, v2)
}



func TestComputeSharedSecret_RandomA(t *testing.T) {
	g := SRP1024()
	user, err := CreateSRPUser(g, "testuser", "testpass")
	require.NoError(t, err)

	server, err := NewSRPServer(g, user)
	require.NoError(t, err)

	// Generate a random A like a real client would
	a, err := newRandomBigIntBytes(128)
	require.NoError(t, err)
	A := new(big.Int).Exp(g.g, a, g.N)

	S, err := server.ComputeSharedSecret(A)
	require.NoError(t, err)
	assert.True(t, S.Sign() > 0)
}

func TestPadBigInt_Nil(t *testing.T) {
	padded := padBigInt(nil, 4)
	assert.Equal(t, []byte{0, 0, 0, 0}, padded)
}

func TestPadBigInt_BytesLargerThanSize(t *testing.T) {
	v := new(big.Int).SetBytes([]byte{1, 2, 3, 4, 5})
	padded := padBigInt(v, 3)
	// Should truncate to last 3 bytes
	assert.Equal(t, []byte{3, 4, 5}, padded)
}

func TestNewSRPServer_ZeroVerifier(t *testing.T) {
	g := SRP1024()
	user := &SRPUser{
		Username: "test",
		Salt:     []byte{1, 2, 3, 4},
		Verifier: big.NewInt(0),
	}

	server, err := NewSRPServer(g, user)
	require.NoError(t, err)
	assert.NotNil(t, server)
	assert.True(t, server.B.Sign() > 0)
}

func TestPremasterSecret_InvalidA(t *testing.T) {
	g := SRP1024()
	user := &SRPUser{
		Username: "test",
		Salt:     []byte{1, 2, 3, 4},
		Verifier: big.NewInt(42),
	}

	server, err := NewSRPServer(g, user)
	require.NoError(t, err)

	// A = N (A mod N == 0)
	_, err = server.PremasterSecret(new(big.Int).Set(g.N))
	assert.Error(t, err)

	// A > N
	overN := new(big.Int).Add(g.N, big.NewInt(1))
	_, err = server.PremasterSecret(overN)
	assert.Error(t, err)
}

func TestPadBigInt_Random(t *testing.T) {
	b := make([]byte, 64)
	rand.Read(b)
	v := new(big.Int).SetBytes(b)
	padded := padBigInt(v, 128)
	assert.Equal(t, 128, len(padded))
	// First 64 bytes should be zero padding
	for i := 0; i < 64; i++ {
		assert.Equal(t, byte(0), padded[i])
	}
}
