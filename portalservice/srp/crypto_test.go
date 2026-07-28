package srp

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- tlsPad / tlsUnpad ---

func TestTlsPadUnpad_RoundTrip(t *testing.T) {
	original := []byte("hello world")
	padded := tlsPad(original, ivLen)
	assert.Equal(t, 0, len(padded)%ivLen, "padded data must be block-aligned")

	// Unpad by removing the last byte's padding
	unpadded, err := tlsUnpad(padded)
	require.NoError(t, err)
	assert.Equal(t, original, unpadded)
}

func TestTlsPadUnpad_Empty(t *testing.T) {
	original := []byte{}
	padded := tlsPad(original, ivLen)
	unpadded, err := tlsUnpad(padded)
	require.NoError(t, err)
	assert.Equal(t, original, unpadded)
}

func TestTlsPadUnpadBlockSize(t *testing.T) {
	// Pad a 1-byte message to 16-byte block
	padded := tlsPad([]byte{0x01}, ivLen)
	// 1 byte data + padding + 1 byte padding-length
	// 1 + 14 + 1 = 16 (one full block)
	assert.Equal(t, 16, len(padded))
	assert.Equal(t, byte(14), padded[len(padded)-1]) // padding value
}

func TestTlsUnpad_InvalidPadding(t *testing.T) {
	data := make([]byte, 16)
	data[15] = 5 // claims 5 bytes of padding, but none match
	_, err := tlsUnpad(data)
	assert.Error(t, err)
}

func TestTlsUnpad_PaddingTooLong(t *testing.T) {
	data := make([]byte, 2)
	data[1] = 0xFF // padding length exceeds data
	_, err := tlsUnpad(data)
	assert.Error(t, err)
}

func TestTlsUnpad_Empty(t *testing.T) {
	_, err := tlsUnpad([]byte{})
	assert.Error(t, err)
}

// --- concat ---

func TestConcat(t *testing.T) {
	a := []byte{1, 2}
	b := []byte{3, 4, 5}
	c := []byte{6}
	result := concat(a, b, c)
	assert.Equal(t, []byte{1, 2, 3, 4, 5, 6}, result)
}

func TestConcat_Empty(t *testing.T) {
	result := concat()
	assert.Empty(t, result)
}

func TestConcat_Single(t *testing.T) {
	result := concat([]byte{0xAA})
	assert.Equal(t, []byte{0xAA}, result)
}

// --- tlsPRF ---

func TestTlsPRF_Deterministic(t *testing.T) {
	secret := []byte("secret")
	label := []byte("test label")
	seed := []byte("seed data")

	r1 := tlsPRF(secret, label, seed, 48)
	r2 := tlsPRF(secret, label, seed, 48)
	assert.Equal(t, r1, r2)
}

func TestTlsPRF_DifferentLengths(t *testing.T) {
	secret := []byte("secret")
	label := []byte("label")
	seed := []byte("seed")

	r16 := tlsPRF(secret, label, seed, 16)
	r48 := tlsPRF(secret, label, seed, 48)
	// First 16 bytes should be the same (PRF is streaming)
	assert.Equal(t, r16, r48[:16])
	// But total lengths differ
	assert.Equal(t, 16, len(r16))
	assert.Equal(t, 48, len(r48))
}

func TestTlsPRF_DifferentSecrets(t *testing.T) {
	label := []byte("label")
	seed := []byte("seed")

	r1 := tlsPRF([]byte("secret1"), label, seed, 48)
	r2 := tlsPRF([]byte("secret2"), label, seed, 48)
	assert.NotEqual(t, r1, r2)
}

// --- pHash ---

func TestPHash_Deterministic(t *testing.T) {
	secret := []byte("key")
	seed := []byte("seed")
	r1 := pHash(secret, seed, 32, sha256.New)
	r2 := pHash(secret, seed, 32, sha256.New)
	assert.Equal(t, r1, r2)
}

func TestPHash_ZeroLength(t *testing.T) {
	secret := []byte("key")
	seed := []byte("seed")
	r := pHash(secret, seed, 0, sha256.New)
	assert.Empty(t, r)
}

// --- hmacHash ---

func TestHmacHash_Deterministic(t *testing.T) {
	secret := []byte("secret")
	data := []byte("data")
	r1 := hmacHash(secret, data, sha256.New)
	r2 := hmacHash(secret, data, sha256.New)
	assert.Equal(t, r1, r2)
	assert.Equal(t, 32, len(r1)) // SHA-256 output
}

func TestHmacHash_DifferentKeys(t *testing.T) {
	data := []byte("data")
	r1 := hmacHash([]byte("key1"), data, sha256.New)
	r2 := hmacHash([]byte("key2"), data, sha256.New)
	assert.NotEqual(t, r1, r2)
}

// --- deriveMasterSecret ---

func TestDeriveMasterSecret(t *testing.T) {
	premaster := make([]byte, 48)
	rand.Read(premaster)
	clientRandom := make([]byte, 32)
	rand.Read(clientRandom)
	serverRandom := make([]byte, 32)
	rand.Read(serverRandom)

	ms := deriveMasterSecret(premaster, clientRandom, serverRandom)
	assert.Equal(t, masterLen, len(ms))
}

func TestDeriveMasterSecret_Deterministic(t *testing.T) {
	premaster := []byte("premaster secret here for testing!!")
	clientRandom := []byte("client random 32 bytes padding!!!!!!")
	serverRandom := []byte("server random 32 bytes padding!!!!!!")

	ms1 := deriveMasterSecret(premaster, clientRandom, serverRandom)
	ms2 := deriveMasterSecret(premaster, clientRandom, serverRandom)
	assert.Equal(t, ms1, ms2)
}

// --- deriveKeyBlock ---

func TestDeriveKeyBlock(t *testing.T) {
	masterSecret := make([]byte, 48)
	rand.Read(masterSecret)
	clientRandom := make([]byte, 32)
	rand.Read(clientRandom)
	serverRandom := make([]byte, 32)
	rand.Read(serverRandom)

	block := deriveKeyBlock(masterSecret, clientRandom, serverRandom)
	assert.Equal(t, keyBlockLen, len(block))
}

// --- splitKeyBlock ---

func TestSplitKeyBlock(t *testing.T) {
	block := make([]byte, keyBlockLen)
	rand.Read(block)

	kb, err := splitKeyBlock(block)
	require.NoError(t, err)
	assert.Equal(t, macKeyLen, len(kb.ClientMACKey))
	assert.Equal(t, macKeyLen, len(kb.ServerMACKey))
	assert.Equal(t, encKeyLen, len(kb.ClientWriteKey))
	assert.Equal(t, encKeyLen, len(kb.ServerWriteKey))
	assert.Equal(t, ivLen, len(kb.ClientIV))
	assert.Equal(t, ivLen, len(kb.ServerIV))

	// Verify the total adds up
	total := len(kb.ClientMACKey) + len(kb.ServerMACKey) +
		len(kb.ClientWriteKey) + len(kb.ServerWriteKey) +
		len(kb.ClientIV) + len(kb.ServerIV)
	assert.Equal(t, keyBlockLen, total)
}

func TestSplitKeyBlock_WrongLength(t *testing.T) {
	_, err := splitKeyBlock(make([]byte, 10))
	assert.Error(t, err)
}

// --- CipherState Encrypt/Decrypt ---

func TestCipherState_EncryptDecrypt(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	macKey := make([]byte, 20)
	rand.Read(macKey)

	enc := &CipherState{MACKey: macKey, Key: key, Sequence: 0}
	dec := &CipherState{MACKey: macKey, Key: key, Sequence: 0}

	plaintext := []byte("hello TLS world")
	ciphertext, err := enc.Encrypt(recordApplicationData, tls12, plaintext)
	require.NoError(t, err)

	recovered, err := dec.Decrypt(recordApplicationData, tls12, ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, recovered)
}

func TestCipherState_EncryptDecrypt_Empty(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	macKey := make([]byte, 20)
	rand.Read(macKey)

	enc := &CipherState{MACKey: macKey, Key: key, Sequence: 0}
	dec := &CipherState{MACKey: macKey, Key: key, Sequence: 0}

	plaintext := []byte{}
	ciphertext, err := enc.Encrypt(recordApplicationData, tls12, plaintext)
	require.NoError(t, err)

	recovered, err := dec.Decrypt(recordApplicationData, tls12, ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, recovered)
}

func TestCipherState_EncryptDecrypt_LargeData(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	macKey := make([]byte, 20)
	rand.Read(macKey)

	enc := &CipherState{MACKey: macKey, Key: key, Sequence: 0}
	dec := &CipherState{MACKey: macKey, Key: key, Sequence: 0}

	plaintext := make([]byte, 4096)
	rand.Read(plaintext)

	ciphertext, err := enc.Encrypt(recordApplicationData, tls12, plaintext)
	require.NoError(t, err)

	recovered, err := dec.Decrypt(recordApplicationData, tls12, ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, recovered)
}

func TestCipherState_EncryptIncrementsSequence(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	macKey := make([]byte, 20)
	rand.Read(macKey)

	enc := &CipherState{MACKey: macKey, Key: key, Sequence: 0}
	assert.Equal(t, uint64(0), enc.Sequence)

	enc.Encrypt(recordApplicationData, tls12, []byte("a"))
	assert.Equal(t, uint64(1), enc.Sequence)

	enc.Encrypt(recordApplicationData, tls12, []byte("b"))
	assert.Equal(t, uint64(2), enc.Sequence)
}

func TestCipherState_DecryptIncrementsSequence(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	macKey := make([]byte, 20)
	rand.Read(macKey)

	enc := &CipherState{MACKey: macKey, Key: key, Sequence: 0}
	dec := &CipherState{MACKey: macKey, Key: key, Sequence: 0}

	c1, _ := enc.Encrypt(recordApplicationData, tls12, []byte("a"))
	c2, _ := enc.Encrypt(recordApplicationData, tls12, []byte("b"))

	dec.Decrypt(recordApplicationData, tls12, c1)
	assert.Equal(t, uint64(1), dec.Sequence)

	dec.Decrypt(recordApplicationData, tls12, c2)
	assert.Equal(t, uint64(2), dec.Sequence)
}

func TestCipherState_Decrypt_TooShort(t *testing.T) {
	key := make([]byte, 32)
	macKey := make([]byte, 20)
	dec := &CipherState{MACKey: macKey, Key: key, Sequence: 0}

	_, err := dec.Decrypt(recordApplicationData, tls12, make([]byte, 5))
	assert.Error(t, err)
}

func TestCipherState_Decrypt_InvalidMAC(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	macKey := make([]byte, 20)
	rand.Read(macKey)

	enc := &CipherState{MACKey: macKey, Key: key, Sequence: 0}

	ciphertext, _ := enc.Encrypt(recordApplicationData, tls12, []byte("secret"))

	// Tamper with ciphertext
	tampered := make([]byte, len(ciphertext))
	copy(tampered, ciphertext)
	tampered[ivLen] ^= 0xFF

	dec := &CipherState{MACKey: macKey, Key: key, Sequence: 0}
	_, err := dec.Decrypt(recordApplicationData, tls12, tampered)
	assert.ErrorIs(t, err, ErrBadRecordMAC)
}

func TestCipherState_Decrypt_WrongKey(t *testing.T) {
	key1 := make([]byte, 32)
	rand.Read(key1)
	key2 := make([]byte, 32)
	rand.Read(key2)
	macKey := make([]byte, 20)
	rand.Read(macKey)

	enc := &CipherState{MACKey: macKey, Key: key1, Sequence: 0}
	dec := &CipherState{MACKey: macKey, Key: key2, Sequence: 0}

	ciphertext, _ := enc.Encrypt(recordApplicationData, tls12, []byte("secret"))
	_, err := dec.Decrypt(recordApplicationData, tls12, ciphertext)
	assert.ErrorIs(t, err, ErrBadRecordMAC)
}

func TestCipherState_EncryptProducesDifferentCiphertexts(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	macKey := make([]byte, 20)
	rand.Read(macKey)

	enc := &CipherState{MACKey: macKey, Key: key, Sequence: 0}

	c1, _ := enc.Encrypt(recordApplicationData, tls12, []byte("same"))
	c2, _ := enc.Encrypt(recordApplicationData, tls12, []byte("same"))
	// Random IV should make ciphertexts different even for same plaintext
	assert.False(t, bytes.Equal(c1, c2))
}

func TestCipherState_DifferentContentTypes(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	macKey := make([]byte, 20)
	rand.Read(macKey)

	enc := &CipherState{MACKey: macKey, Key: key, Sequence: 0}
	dec := &CipherState{MACKey: macKey, Key: key, Sequence: 0}

	plaintext := []byte("test data")
	ciphertext, _ := enc.Encrypt(recordHandshake, tls12, plaintext)

	// Decrypting with wrong content type should fail MAC check
	_, err := dec.Decrypt(recordApplicationData, tls12, ciphertext)
	assert.ErrorIs(t, err, ErrBadRecordMAC)
}
