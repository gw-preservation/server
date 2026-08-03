package pathing

import (
	"encoding/binary"
	"fmt"
	"os"
)

// fileArchiveMagic is the multichar literal '\x1ANA3'.
const (
	fileArchiveMagic = 0x1A4E4133

	indexTOC       = 1
	indexFirstFile = 0x10

	faHeaderSize    = 32
	mftHeaderSize   = 24
	mftEntrySize    = 24
	mftFileNameSize = 8
)

type faHeader struct {
	magic      uint32
	headerSize uint32
	blockSize  uint32
	checksum   uint32
	mftOffset  uint64
	mftSize    uint32
	flags      uint32
}

type mftEntry struct {
	offset     uint64
	size       uint32
	compressed uint16 // 0 = stored, 8 = compressed
	flags      uint8
	stream     uint8
	nextStream uint32
	checksum   uint32
}

type Archive struct {
	file    *os.File
	size    int64
	header  faHeader
	entries []mftEntry
	hash    map[uint32]uint32 // file id -> 1-based MFT file index
}

func Open(path string) (*Archive, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("stat archive: %w", err)
	}
	a := &Archive{file: f, size: st.Size(), hash: make(map[uint32]uint32)}
	if err := a.readHeader(); err != nil {
		f.Close()
		return nil, err
	}
	if err := a.readMFT(); err != nil {
		f.Close()
		return nil, err
	}
	if err := a.readTOC(); err != nil {
		f.Close()
		return nil, err
	}
	return a, nil
}

func (a *Archive) Close() error {
	if a.file == nil {
		return nil
	}
	err := a.file.Close()
	a.file = nil
	return err
}

func (a *Archive) readHeader() error {
	var buf [faHeaderSize]byte
	if _, err := a.file.ReadAt(buf[:], 0); err != nil {
		return fmt.Errorf("read archive header: %w", err)
	}
	h := faHeader{
		magic:      binary.LittleEndian.Uint32(buf[0:4]),
		headerSize: binary.LittleEndian.Uint32(buf[4:8]),
		blockSize:  binary.LittleEndian.Uint32(buf[8:12]),
		checksum:   binary.LittleEndian.Uint32(buf[12:16]),
		mftOffset:  binary.LittleEndian.Uint64(buf[16:24]),
		mftSize:    binary.LittleEndian.Uint32(buf[24:28]),
		flags:      binary.LittleEndian.Uint32(buf[28:32]),
	}
	if h.magic != fileArchiveMagic {
		return fmt.Errorf("invalid archive header (expected 0x%08x, got 0x%08x)", fileArchiveMagic, h.magic)
	}
	a.header = h
	return nil
}

func (a *Archive) readMFT() error {
	if a.header.mftOffset > uint64(a.size) || uint64(a.size)-a.header.mftOffset < mftHeaderSize {
		return fmt.Errorf("MFT header at 0x%x beyond end of file (%d bytes)", a.header.mftOffset, a.size)
	}
	var buf [mftHeaderSize]byte
	if _, err := a.file.ReadAt(buf[:], int64(a.header.mftOffset)); err != nil {
		return fmt.Errorf("read MFT header: %w", err)
	}
	entryCount := binary.LittleEndian.Uint32(buf[12:16])
	if entryCount <= indexTOC {
		return fmt.Errorf("MFT too small (%d entries)", entryCount)
	}
	if avail := uint64(a.size) - a.header.mftOffset - mftHeaderSize; uint64(entryCount) > avail/mftEntrySize {
		return fmt.Errorf("MFT claims %d entries, but the file only fits %d", entryCount, avail/mftEntrySize)
	}

	a.entries = make([]mftEntry, entryCount)
	data := make([]byte, int(entryCount)*mftEntrySize)
	if _, err := a.file.ReadAt(data, int64(a.header.mftOffset)+mftHeaderSize); err != nil {
		return fmt.Errorf("read MFT entries: %w", err)
	}
	for i := range a.entries {
		off := i * mftEntrySize
		a.entries[i] = mftEntry{
			offset:     binary.LittleEndian.Uint64(data[off : off+8]),
			size:       binary.LittleEndian.Uint32(data[off+8 : off+12]),
			compressed: binary.LittleEndian.Uint16(data[off+12 : off+14]),
			flags:      data[off+14],
			stream:     data[off+15],
			nextStream: binary.LittleEndian.Uint32(data[off+16 : off+20]),
			checksum:   binary.LittleEndian.Uint32(data[off+20 : off+24]),
		}
	}
	return nil
}

func (a *Archive) readTOC() error {
	toc := a.entries[indexTOC]
	if toc.offset > uint64(a.size) || uint64(a.size)-toc.offset < uint64(toc.size) {
		return fmt.Errorf("TOC at 0x%x needs %d bytes but the file is %d bytes", toc.offset, toc.size, a.size)
	}
	count := toc.size / mftFileNameSize
	data := make([]byte, toc.size)
	if _, err := a.file.ReadAt(data, int64(toc.offset)); err != nil {
		return fmt.Errorf("read TOC: %w", err)
	}
	for i := uint32(0); i < count; i++ {
		off := i * mftFileNameSize
		fileID := binary.LittleEndian.Uint32(data[off : off+4])
		fileIndex := binary.LittleEndian.Uint32(data[off+4 : off+8])
		a.hash[fileID] = fileIndex
	}
	return nil
}

func (a *Archive) GetFile(fileID uint32) ([]byte, error) {
	fileIndex, ok := a.hash[fileID]
	if !ok || fileIndex == 0 {
		return nil, fmt.Errorf("no such file 0x%08x", fileID)
	}
	idx := fileIndex - 1
	if idx >= uint32(len(a.entries)) {
		return nil, fmt.Errorf("file 0x%08x: entry index %d out of range", fileID, idx)
	}
	entry := a.entries[idx]
	if entry.offset > uint64(a.size) || uint64(a.size)-entry.offset < uint64(entry.size) {
		return nil, fmt.Errorf("file 0x%08x: entry at 0x%x needs %d bytes but the file is %d bytes", fileID, entry.offset, entry.size, a.size)
	}
	buf := make([]byte, entry.size)
	if _, err := a.file.ReadAt(buf, int64(entry.offset)); err != nil {
		return nil, fmt.Errorf("read file 0x%08x: %w", fileID, err)
	}
	if entry.compressed == 0 {
		return buf, nil
	}
	if len(buf) < 4 {
		return nil, fmt.Errorf("file 0x%08x: too short to hold decompressed size", fileID)
	}
	outSize := binary.LittleEndian.Uint32(buf[len(buf)-4:])
	if uint64(outSize) > uint64(a.size) {
		return nil, fmt.Errorf("file 0x%08x: decompressed size %d exceeds archive size %d", fileID, outSize, a.size)
	}
	out, err := decompress(buf[:len(buf)-4], int(outSize))
	if err != nil {
		return nil, fmt.Errorf("decompress file 0x%08x: %w", fileID, err)
	}
	return out, nil
}
