package srp

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"hash"
)

const (
	macKeyLen       = sha1.Size
	encKeyLen       = 32
	ivLen           = 16
	masterLen       = 48
	keyBlockLen     = 136
	recordHeaderLen = 13
)

// TLS PRF

func tlsPRF(secret, label, seed []byte, length int) []byte {
	combined := concat(label, seed)
	return pHash(
		secret,
		combined,
		length,
		sha256.New,
	)

}

func concat(parts ...[]byte) []byte {
	total := 0
	for _, p := range parts {
		total += len(p)
	}

	out := make([]byte, 0, total)
	for _, p := range parts {
		out = append(out, p...)
	}

	return out
}

func pHash(secret, seed []byte, length int, hashFunc func() hash.Hash) []byte {
	result := make([]byte, 0, length)

	a := seed

	for len(result) < length {
		a = hmacHash(secret, a, hashFunc)
		block := hmacHash(secret, concat(a, seed), hashFunc)
		result = append(result, block...)
	}

	return result[:length]

}

func hmacHash(secret, data []byte, hashFunc func() hash.Hash) []byte {
	h := hmac.New(hashFunc, secret)
	h.Write(data)

	return h.Sum(nil)

}

// Key expansion

type KeyBlock struct {
	ClientMACKey []byte
	ServerMACKey []byte

	ClientWriteKey []byte
	ServerWriteKey []byte

	ClientIV []byte
	ServerIV []byte
}

func deriveMasterSecret(
	premaster []byte,
	clientRandom []byte,
	serverRandom []byte,
) []byte {
	seed := concat(clientRandom, serverRandom)

	return tlsPRF(
		premaster,
		[]byte("master secret"),
		seed,
		masterLen,
	)

}

func deriveKeyBlock(
	masterSecret []byte,
	clientRandom []byte,
	serverRandom []byte,
) []byte {
	seed := concat(serverRandom, clientRandom)

	return tlsPRF(
		masterSecret,
		[]byte("key expansion"),
		seed,
		keyBlockLen,
	)

}

func splitKeyBlock(block []byte) (*KeyBlock, error) {
	if len(block) != keyBlockLen {
		return nil, fmt.Errorf("invalid key block")
	}
	offset := 0

	next := func(n int) []byte {
		v := block[offset : offset+n]
		offset += n
		return v
	}

	return &KeyBlock{
		ClientMACKey:   next(macKeyLen),
		ServerMACKey:   next(macKeyLen),
		ClientWriteKey: next(encKeyLen),
		ServerWriteKey: next(encKeyLen),
		ClientIV:       next(ivLen),
		ServerIV:       next(ivLen),
	}, nil

}

// CBC record protection

type CipherState struct {
	MACKey []byte
	Key    []byte

	Sequence uint64
}

func (c *CipherState) mac(
	contentType uint8,
	version uint16,
	data []byte,
) []byte {
	header := make([]byte, recordHeaderLen)

	binary.BigEndian.PutUint64(header[0:], c.Sequence)
	header[8] = contentType
	binary.BigEndian.PutUint16(header[9:], version)
	binary.BigEndian.PutUint16(header[11:], uint16(len(data)))

	h := hmac.New(sha1.New, c.MACKey)
	h.Write(header)
	h.Write(data)

	return h.Sum(nil)

}

func (c *CipherState) Encrypt(
	contentType uint8,
	version uint16,
	plaintext []byte,
) ([]byte, error) {

	mac := c.mac(
		contentType,
		version,
		plaintext,
	)

	c.Sequence++

	data := concat(plaintext, mac)

	data = tlsPad(
		data,
		ivLen,
	)

	block, err := aes.NewCipher(c.Key)
	if err != nil {
		return nil, err
	}

	iv := make([]byte, ivLen)

	if _, err := rand.Read(iv); err != nil {
		return nil, err
	}

	out := make([]byte, len(data))

	mode := cipher.NewCBCEncrypter(
		block,
		iv,
	)

	mode.CryptBlocks(out, data)

	return append(iv, out...), nil

}

func (c *CipherState) Decrypt(
	contentType uint8,
	version uint16,
	ciphertext []byte,
) ([]byte, error) {

	if len(ciphertext) < ivLen {
		return nil, fmt.Errorf("ciphertext too short")
	}

	iv := ciphertext[:ivLen]
	body := ciphertext[ivLen:]

	if len(body)%ivLen != 0 {
		return nil, fmt.Errorf("invalid ciphertext length")
	}

	block, err := aes.NewCipher(c.Key)
	if err != nil {
		return nil, err
	}

	plaintext := make([]byte, len(body))

	mode := cipher.NewCBCDecrypter(
		block,
		iv,
	)

	mode.CryptBlocks(plaintext, body)

	plaintext, err = tlsUnpad(plaintext)
	if err != nil {
		return nil, ErrBadRecordMAC
	}

	if len(plaintext) < macKeyLen {
		return nil, ErrBadRecordMAC
	}

	dataLen := len(plaintext) - macKeyLen

	data := plaintext[:dataLen]
	receivedMAC := plaintext[dataLen:]

	expectedMAC := c.mac(
		contentType,
		version,
		data,
	)

	if !hmac.Equal(receivedMAC, expectedMAC) {
		return nil, ErrBadRecordMAC
	}

	c.Sequence++

	return data, nil

}

// TLS CBC padding as defined in RFC 5246 §6.2.3.2.
func tlsPad(data []byte, blockSize int) []byte {
	padding := blockSize - ((len(data) + 1) % blockSize)

	out := make([]byte, len(data)+padding+1)

	copy(out, data)

	for i := len(data); i < len(out); i++ {
		out[i] = byte(padding)
	}

	return out

}

func tlsUnpad(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty padded data")
	}

	padding := int(data[len(data)-1])

	paddingLength := padding + 1

	if paddingLength > len(data) {
		return nil, fmt.Errorf("invalid padding")
	}

	for i := len(data) - paddingLength; i < len(data); i++ {
		if int(data[i]) != padding {
			return nil, fmt.Errorf("invalid padding bytes")
		}
	}

	return data[:len(data)-paddingLength], nil

}
