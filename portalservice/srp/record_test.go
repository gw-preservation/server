package srp

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecord_ReadWrite_RoundTrip(t *testing.T) {
	rec := &Record{
		Type:    recordHandshake,
		Version: tls12,
		Data:    []byte{0x01, 0x02, 0x03, 0x04, 0x05},
	}

	var buf bytes.Buffer
	err := WriteRecord(&buf, rec)
	require.NoError(t, err)

	readRec, err := ReadRecord(&buf)
	require.NoError(t, err)
	assert.Equal(t, rec.Type, readRec.Type)
	assert.Equal(t, rec.Version, readRec.Version)
	assert.Equal(t, rec.Data, readRec.Data)
}

func TestRecord_ReadWrite_EmptyData(t *testing.T) {
	rec := &Record{
		Type:    recordAlert,
		Version: tls12,
		Data:    []byte{},
	}

	var buf bytes.Buffer
	err := WriteRecord(&buf, rec)
	require.NoError(t, err)

	readRec, err := ReadRecord(&buf)
	require.NoError(t, err)
	assert.Equal(t, rec.Type, readRec.Type)
	assert.Empty(t, readRec.Data)
}

func TestReadRecord_WrongVersion(t *testing.T) {
	// Manually craft a record with wrong version
	hdr := []byte{recordHandshake, 0x03, 0x01, 0, 0} // TLS 1.0
	var buf bytes.Buffer
	buf.Write(hdr)

	_, err := ReadRecord(&buf)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrUnsupportedVersion)
}

func TestReadRecord_TooLarge(t *testing.T) {
	hdr := []byte{recordHandshake, 0x03, 0x03, 0x80, 0x00} // length = 32768
	var buf bytes.Buffer
	buf.Write(hdr)

	_, err := ReadRecord(&buf)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrRecordTooLarge)
}

func TestWriteRecord_WrongVersion(t *testing.T) {
	rec := &Record{
		Type:    recordHandshake,
		Version: 0x0301, // TLS 1.0
		Data:    []byte{0x01},
	}

	var buf bytes.Buffer
	err := WriteRecord(&buf, rec)
	assert.Error(t, err)
}

func TestWriteRecord_TooLarge(t *testing.T) {
	rec := &Record{
		Type:    recordHandshake,
		Version: tls12,
		Data:    make([]byte, maxPlaintext+1),
	}

	var buf bytes.Buffer
	err := WriteRecord(&buf, rec)
	assert.Error(t, err)
}

func TestRecord_HeaderLength(t *testing.T) {
	rec := &Record{
		Type:    recordApplicationData,
		Version: tls12,
		Data:    []byte{0xDE, 0xAD},
	}

	var buf bytes.Buffer
	err := WriteRecord(&buf, rec)
	require.NoError(t, err)

	// 5-byte header + 2-byte data = 7 bytes total
	assert.Equal(t, 7, buf.Len())
}

func TestRecord_AllRecordTypes(t *testing.T) {
	types := []uint8{
		recordChangeCipherSpec,
		recordAlert,
		recordHandshake,
		recordApplicationData,
	}

	for _, typ := range types {
		rec := &Record{
			Type:    typ,
			Version: tls12,
			Data:    []byte{0x01},
		}

		var buf bytes.Buffer
		err := WriteRecord(&buf, rec)
		require.NoError(t, err)

		readRec, err := ReadRecord(&buf)
		require.NoError(t, err)
		assert.Equal(t, typ, readRec.Type)
	}
}

func TestRecord_LargeData(t *testing.T) {
	data := make([]byte, 16384) // maxPlaintext
	for i := range data {
		data[i] = byte(i % 256)
	}

	rec := &Record{
		Type:    recordApplicationData,
		Version: tls12,
		Data:    data,
	}

	var buf bytes.Buffer
	err := WriteRecord(&buf, rec)
	require.NoError(t, err)

	readRec, err := ReadRecord(&buf)
	require.NoError(t, err)
	assert.Equal(t, data, readRec.Data)
}
