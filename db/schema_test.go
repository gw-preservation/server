package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUUIDStr(t *testing.T) {
	uuid := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F}
	result := UUIDStr(uuid)
	assert.Equal(t, "00010203-0405-0607-0809-0a0b0c0d0e0f", result)
}

func TestUUIDStr_AllZeros(t *testing.T) {
	uuid := make([]byte, 16)
	result := UUIDStr(uuid)
	assert.Equal(t, "00000000-0000-0000-0000-000000000000", result)
}

func TestUUIDStrSwapped(t *testing.T) {
	uuid := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F}
	result := UUIDStrSwapped(uuid)
	assert.Equal(t, "03020100-0504-0706-0809-0a0b0c0d0e0f", result)
}

func TestUUIDStrSwapped_AllZeros(t *testing.T) {
	uuid := make([]byte, 16)
	result := UUIDStrSwapped(uuid)
	assert.Equal(t, "00000000-0000-0000-0000-000000000000", result)
}

func TestUUIDStrSwapped_DiffersFromUUIDStr(t *testing.T) {
	uuid := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F}
	assert.NotEqual(t, UUIDStr(uuid), UUIDStrSwapped(uuid))
}

func TestModifiersArray_Value(t *testing.T) {
	mods := ModifiersArray{1, 2, 3}
	val, err := mods.Value()
	assert.NoError(t, err)
	assert.IsType(t, []byte{}, val)
}

func TestModifiersArray_Scan(t *testing.T) {
	var mods ModifiersArray
	err := mods.Scan([]byte(`[10,20,30]`))
	assert.NoError(t, err)
	assert.Equal(t, ModifiersArray{10, 20, 30}, mods)
}

func TestModifiersArray_ScanInvalidType(t *testing.T) {
	var mods ModifiersArray
	err := mods.Scan("not a byte slice")
	assert.Error(t, err)
}

func TestModifiersArray_Empty(t *testing.T) {
	mods := ModifiersArray{}
	val, err := mods.Value()
	assert.NoError(t, err)

	var result ModifiersArray
	err = result.Scan(val.([]byte))
	assert.NoError(t, err)
	assert.Empty(t, result)
}

func TestModifiersArray_RoundTrip(t *testing.T) {
	original := ModifiersArray{0x23a80014, 0x24b80003}
	val, err := original.Value()
	assert.NoError(t, err)

	var result ModifiersArray
	err = result.Scan(val.([]byte))
	assert.NoError(t, err)
	assert.Equal(t, original, result)
}
