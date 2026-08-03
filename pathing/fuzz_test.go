package pathing

import (
	"os"
	"path/filepath"
	"testing"
)

// FuzzParsePathData ensures malformed map files return errors, not panics.
func FuzzParsePathData(f *testing.F) {
	seed := wrapMap(buildPathBytes(buildPlaneBytes(
		0, 1, 1, 1, 0, 1, 0, 0,
		concat(f32le(10), f32le(20)),
		concat(u32le(0xFFFFFFFF), u32le(0xFFFFFFFF), u32le(0xFFFFFFFF), u32le(0xFFFFFFFF),
			u16le(0xFFFF), u16le(0xFFFF),
			f32le(100), f32le(0), f32le(-50), f32le(50), f32le(-50), f32le(50)),
		concat(u32le(0), u32le(0), u32le(0xFFFFFFFF), u32le(0xFFFFFFFF)),
		nil, concat(u32le(0)), nil, nil, 0,
	)))
	f.Add(seed)
	f.Add([]byte("ffna"))
	f.Add([]byte("ffna\x00garbage"))
	f.Add(seed[:20])

	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = ParsePathData(data)
	})
}

func FuzzDecompress(f *testing.F) {
	f.Add([]byte{0xff, 0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66})
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = decompress(data, 4096)
	})
}

func FuzzArchive(f *testing.F) {
	for _, seed := range [][]byte{
		{},
		[]byte("hello"),
		make([]byte, 32),
		make([]byte, 4096),
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		path := filepath.Join(t.TempDir(), "Gw.dat")
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
		a, err := Open(path)
		if err != nil {
			return
		}
		defer a.Close()
		for _, id := range []uint32{0, 1, indexFirstFile, 0xFFFFFFFF} {
			_, _ = a.GetFile(id)
		}
	})
}
