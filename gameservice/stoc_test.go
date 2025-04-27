package GameService

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewItemGeneralInfo(t *testing.T) {
	pktBytes1 := newItemGeneralInfo(itemGeneralInfo{
		itemLocalId:   1,
		fileId:        2147595574,
		itemType:      3,
		unk1:          1,
		itemFlags:     536875008,
		merchantPrice: 5,
		itemId:        32,
		quantity:      1,
		encNameBytes:  []byte{0xa8, 0x21, 0x57, 0xd1, 0x8f, 0xb5, 0x6f, 0x16},
		unk3:          608703488,
	})

	expBytes1 := []byte{96, 1, 1, 0, 0, 0, 54, 181, 1, 128, 3, 1, 0, 0, 0, 0, 0, 0, 16, 0, 32, 5, 0, 0, 0, 32, 0, 0, 0, 1, 0, 0, 0, 4, 0, 168, 33, 87, 209, 143, 181, 111, 22, 1, 0, 20, 72, 36}
	assert.Equal(t, expBytes1, pktBytes1.GetBytes())
}
