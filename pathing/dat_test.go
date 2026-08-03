package pathing

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildSyntheticDat writes a minimal .dat from file id -> raw content pairs.
func buildSyntheticDat(t *testing.T, files map[uint32][]byte) string {
	t.Helper()

	const entryCount = 0x10 + 8
	tocOffset := 56 + entryCount*mftEntrySize
	fileOffsets := make(map[uint32]uint32)

	buf := &bytes.Buffer{}
	buf.Write(u32le(fileArchiveMagic))
	buf.Write(u32le(0)) // header_size
	buf.Write(u32le(0)) // block_size
	buf.Write(u32le(0)) // checksum
	writeLE64(buf, 32)  // mft_offset
	buf.Write(u32le(0)) // mft_size
	buf.Write(u32le(0)) // flags

	buf.Write(u32le(0))          // file_id
	buf.Write(u32le(0))          // h0004
	buf.Write(u32le(0))          // h0008
	buf.Write(u32le(entryCount)) // entry_count
	buf.Write(u32le(0))          // h0010
	buf.Write(u32le(0))          // h0014

	fileIDs := make([]uint32, 0, len(files))
	for id := range files {
		fileIDs = append(fileIDs, id)
	}
	sort.Slice(fileIDs, func(i, j int) bool { return fileIDs[i] < fileIDs[j] })
	offset := uint32(tocOffset + len(fileIDs)*mftFileNameSize)
	for _, id := range fileIDs {
		fileOffsets[id] = offset
		offset += uint32(len(files[id]))
	}

	for i := uint32(0); i < entryCount; i++ {
		switch {
		case i == indexTOC:
			writeLE64(buf, uint64(tocOffset))
			buf.Write(u32le(uint32(len(fileIDs)) * mftFileNameSize))
			buf.Write(u16le(0))
			buf.Write([]byte{0, 0})
			buf.Write(u32le(0))
			buf.Write(u32le(0))
		case i >= indexFirstFile-1 && i < indexFirstFile-1+uint32(len(fileIDs)):
			idx := int(i) - (indexFirstFile - 1)
			id := fileIDs[idx]
			writeLE64(buf, uint64(fileOffsets[id]))
			buf.Write(u32le(uint32(len(files[id]))))
			buf.Write(u16le(0)) // compressed: stored
			buf.Write([]byte{0, 0})
			buf.Write(u32le(0))
			buf.Write(u32le(0))
		default:
			writeLE64(buf, 0)
			buf.Write(u32le(0))
			buf.Write(u16le(0))
			buf.Write([]byte{0, 0})
			buf.Write(u32le(0))
			buf.Write(u32le(0))
		}
	}
	// TOC: file id -> 1-based MFT index (0x10 + position)
	for idx, id := range fileIDs {
		buf.Write(u32le(id))
		buf.Write(u32le(indexFirstFile + uint32(idx)))
	}
	for _, id := range fileIDs {
		buf.Write(files[id])
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "Gw.dat")
	require.NoError(t, os.WriteFile(path, buf.Bytes(), 0o644))
	return path
}

func TestArchiveSynthetic(t *testing.T) {
	fileData := []byte("hello guild wars")
	path := buildSyntheticDat(t, map[uint32][]byte{0x2fed: fileData})

	a, err := Open(path)
	require.NoError(t, err)
	defer a.Close()

	got, err := a.GetFile(0x2fed)
	require.NoError(t, err)
	assert.Equal(t, fileData, got)

	_, err = a.GetFile(0xdead)
	assert.Error(t, err)
}

func TestLazyStoreLoadsOnDemand(t *testing.T) {
	validMap := wrapMap(buildPathBytes(buildPlaneBytes(
		0, 1, 1, 1, 0, 1, 0, 0,
		concat(f32le(10), f32le(20)),
		concat(u32le(0xFFFFFFFF), u32le(0xFFFFFFFF), u32le(0xFFFFFFFF), u32le(0xFFFFFFFF),
			u16le(0xFFFF), u16le(0xFFFF),
			f32le(100), f32le(0), f32le(-50), f32le(50), f32le(-50), f32le(50)),
		concat(u32le(0), u32le(0), u32le(0xFFFFFFFF), u32le(0xFFFFFFFF)),
		nil, concat(u32le(0)), nil, nil, 0,
	)))
	path := buildSyntheticDat(t, map[uint32][]byte{0x1b97d: validMap})

	store, err := NewLazyStore(path)
	require.NoError(t, err)

	assert.Nil(t, store.PathDataForFileID(0x1b97d), "nothing loaded until requested")

	sd, err := store.EnsureLoaded(0x1b97d)
	require.NoError(t, err)
	require.NotNil(t, sd, "requested map should be parsed on demand")
	assert.Same(t, sd, store.PathDataForFileID(0x1b97d))

	sdAgain, err := store.EnsureLoaded(0x1b97d)
	require.NoError(t, err)
	assert.Same(t, sd, sdAgain)
}

func TestNewLazyStoreFailsWhenArchiveMissing(t *testing.T) {
	_, err := NewLazyStore("/nonexistent/Gw.dat")
	require.Error(t, err)
}

func writeLE64(buf *bytes.Buffer, v uint64) {
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], v)
	buf.Write(b[:])
}
