package pathing

import (
	"encoding/binary"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func u16le(v uint16) []byte  { b := make([]byte, 2); binary.LittleEndian.PutUint16(b, v); return b }
func u32le(v uint32) []byte  { b := make([]byte, 4); binary.LittleEndian.PutUint32(b, v); return b }
func f32le(v float32) []byte { return u32le(math.Float32bits(v)) }

func concat(parts ...[]byte) []byte {
	var out []byte
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

func tagged(tag uint8, payload []byte) []byte {
	return concat([]byte{tag}, u32le(uint32(len(payload))), payload)
}

func buildPathBytes(planes ...[]byte) []byte {
	planesPayload := u32le(uint32(len(planes)))
	for _, p := range planes {
		planesPayload = append(planesPayload, p...)
	}
	return concat(
		u32le(pathSignature), u32le(pathVersion), u32le(0),
		[]byte{0}, u32le(0), // step-2 blob, empty
		tagged(8, planesPayload),
	)
}

func buildPlaneBytes(h000C, vectors, traps, xnodes, ynodes, sinks, portals, portalTraps uint32,
	vectorData, trapData, xnodeData, ynodeData, sinkData, portalTrapData, portalData []byte, rootType uint8) []byte {
	return concat(
		tagged(0, concat(
			u32le(h000C), u32le(vectors), u32le(traps), u32le(xnodes), u32le(ynodes), u32le(sinks), u32le(portals), u32le(portalTraps),
		)),
		tagged(11, make([]byte, h000C*8)),
		tagged(1, vectorData),
		tagged(2, trapData),
		tagged(3, []byte{rootType}),
		tagged(4, xnodeData),
		tagged(5, ynodeData),
		tagged(6, sinkData),
		tagged(10, portalTrapData),
		tagged(9, portalData),
	)
}

func wrapMap(buildPathBytes []byte) []byte {
	chunk := concat(u32le(pathChunkID), u32le(uint32(len(buildPathBytes))), buildPathBytes)
	return concat([]byte(riffSignature), []byte{riffMap1}, chunk)
}

func TestParsePathDataMinimalPlane(t *testing.T) {
	plane := buildPlaneBytes(
		0, 1, 1, 1, 0, 1, 0, 0,
		concat(f32le(10), f32le(20)), // vectors
		concat( // 1 trap, 44 bytes
			u32le(0xFFFFFFFF), u32le(0xFFFFFFFF), u32le(0xFFFFFFFF), u32le(0xFFFFFFFF), // neighbors: none
			u16le(0xFFFF), u16le(0xFFFF), // portals: none
			f32le(100), f32le(0), f32le(-50), f32le(50), f32le(-50), f32le(50),
		),
		concat(u32le(0), u32le(0), u32le(0xFFFFFFFF), u32le(0xFFFFFFFF)), // xnode pos0,pos1,left,right
		nil,              // ynodes
		concat(u32le(0)), // sink -> trap 0
		nil,              // portal traps
		nil,              // portals
		0,                // root type: xnode
	)

	sd, err := ParsePathData(wrapMap(buildPathBytes(plane)))
	require.NoError(t, err)
	require.Len(t, sd.Planes, 1)
	assert.Equal(t, uint32(1), sd.TrapsCount)

	p := sd.Planes[0]
	require.Len(t, p.Trapezoids, 1)
	trap := p.Trapezoids[0]
	assert.Equal(t, uint32(0), trap.TrapID)
	assert.Equal(t, -1, trap.NeighborTL)
	assert.Equal(t, -1, trap.NeighborTR)
	assert.Equal(t, -1, trap.NeighborBL)
	assert.Equal(t, -1, trap.NeighborBR)
	assert.Equal(t, uint16(0xFFFF), trap.PortalLeft)
	assert.Equal(t, uint16(0xFFFF), trap.PortalRight)
	assert.Equal(t, float32(100), trap.YT)
	assert.Equal(t, float32(0), trap.YB)
	assert.Equal(t, float32(50), trap.XBR)

	require.Len(t, p.Nodes, 2)
	xnode := p.Nodes[0]
	assert.Equal(t, NodeTypeXNode, xnode.Type)
	assert.Equal(t, float32(10), xnode.X)
	assert.Equal(t, float32(20), xnode.Y)
	assert.Equal(t, float32(0), xnode.DirX)
	assert.Equal(t, float32(0), xnode.DirY)
	assert.Equal(t, -1, xnode.Left)
	assert.Equal(t, -1, xnode.Right)

	sink := p.Nodes[1]
	assert.Equal(t, NodeTypeSink, sink.Type)
	assert.Equal(t, 0, sink.Trap)

	assert.Equal(t, 0, p.RootNode)
}

func TestParsePathDataNodeNamespaceAndDirs(t *testing.T) {
	v0 := concat(f32le(0), f32le(0))  // vector 0
	v1 := concat(f32le(10), f32le(0)) // vector 1
	plane0 := buildPlaneBytes(
		0, 2, 1, 2, 1, 1, 0, 0,
		concat(v0, v1),
		concat(
			u32le(0xFFFFFFFF), u32le(0xFFFFFFFF), u32le(0xFFFFFFFF), u32le(0xFFFFFFFF),
			u16le(0xFFFF), u16le(0xFFFF),
			f32le(10), f32le(-10), f32le(-5), f32le(5), f32le(-5), f32le(5),
		),
		concat(
			u32le(0), u32le(1), u32le(0xFFFFFFFF), u32le(firstYNodeIndex),
			u32le(1), u32le(0), u32le(0), u32le(0xFFFFFFFF),
		),
		concat(u32le(0), u32le(0xFFFFFFFF), u32le(firstSinkNodeIndex)),
		concat(u32le(0)),
		nil,
		nil,
		0,
	)
	sd, err := ParsePathData(wrapMap(buildPathBytes(plane0)))
	require.NoError(t, err)
	require.Len(t, sd.Planes, 1)
	p := sd.Planes[0]
	require.Len(t, p.Nodes, 4) // 2 xnodes + 1 ynode + 1 sink

	x0 := p.Nodes[0]
	assert.Equal(t, float32(10), x0.DirX)
	assert.Equal(t, -1, x0.Left)
	assert.Equal(t, 2, x0.Right, "xnode right should point at ynode index 2")

	x1 := p.Nodes[1]
	assert.Equal(t, 0, x1.Left, "xnode left should point at xnode 0")

	yn := p.Nodes[2]
	assert.Equal(t, NodeTypeYNode, yn.Type)
	assert.Equal(t, 3, yn.Below, "ynode below should point at sink index 3")
	assert.Equal(t, -1, yn.Above)
}

func TestParsePathDataPortalPairing(t *testing.T) {
	mkPlane := func(neighborPlane uint16) []byte {
		return buildPlaneBytes(
			0, 1, 1, 1, 0, 0, 1, 1,
			concat(f32le(0), f32le(0)),
			concat(
				u32le(0xFFFFFFFF), u32le(0xFFFFFFFF), u32le(0xFFFFFFFF), u32le(0xFFFFFFFF),
				u16le(0), u16le(0),
				f32le(1), f32le(0), f32le(-1), f32le(1), f32le(-1), f32le(1),
			),
			concat(u32le(0), u32le(0), u32le(0xFFFFFFFF), u32le(0xFFFFFFFF)),
			nil,
			nil,
			concat(u32le(0)), // portal trap 0 -> trap 0
			concat(u16le(1), u16le(0), u16le(neighborPlane), u16le(5), []byte{0x00}),
			0,
		)
	}

	sd, err := ParsePathData(wrapMap(buildPathBytes(mkPlane(1), mkPlane(0))))
	require.NoError(t, err)
	require.Len(t, sd.Planes, 2)

	p0 := sd.Planes[0]
	require.Len(t, p0.Portals, 1)
	assert.Equal(t, uint16(1), p0.Portals[0].NeighborPlaneID)
	assert.Equal(t, 1, p0.Portals[0].PairPlane)
	assert.Equal(t, 0, p0.Portals[0].PairPortal)
	assert.Equal(t, []int{0}, p0.Portals[0].Traps)

	p1 := sd.Planes[1]
	require.Len(t, p1.Portals, 1)
	assert.Equal(t, uint16(0), p1.Portals[0].NeighborPlaneID)
	assert.Equal(t, 0, p1.Portals[0].PairPlane)
	assert.Equal(t, 0, p1.Portals[0].PairPortal)
}

func TestParsePathDataErrors(t *testing.T) {
	good := wrapMap(buildPathBytes(buildPlaneBytes(
		0, 1, 1, 1, 0, 1, 0, 0,
		concat(f32le(10), f32le(20)),
		concat(u32le(0xFFFFFFFF), u32le(0xFFFFFFFF), u32le(0xFFFFFFFF), u32le(0xFFFFFFFF),
			u16le(0xFFFF), u16le(0xFFFF),
			f32le(100), f32le(0), f32le(-50), f32le(50), f32le(-50), f32le(50)),
		concat(u32le(0), u32le(0), u32le(0xFFFFFFFF), u32le(0xFFFFFFFF)),
		nil, concat(u32le(0)), nil, nil, 0,
	)))

	t.Run("not a map", func(t *testing.T) {
		_, err := ParsePathData([]byte("ffna\x00"))
		assert.Error(t, err)
	})
	t.Run("wrong riff type", func(t *testing.T) {
		_, err := ParsePathData(append([]byte("ffna\x00"), good[5:]...))
		assert.Error(t, err)
	})
	t.Run("bad path signature", func(t *testing.T) {
		bad := make([]byte, len(good))
		copy(bad, good)
		copy(bad[13:], u32le(0xDEADBEEF)) // first 5 bytes of the path chunk are ffna+type; path blob starts at 13
		_, err := ParsePathData(bad)
		assert.Error(t, err)
	})
	t.Run("missing path chunk", func(t *testing.T) {
		chunk := concat(u32le(0x1234), u32le(0), nil)
		content := concat([]byte(riffSignature), []byte{riffMap1}, chunk)
		_, err := ParsePathData(content)
		assert.Error(t, err)
	})
	t.Run("truncated", func(t *testing.T) {
		_, err := ParsePathData(good[:len(good)-5])
		assert.Error(t, err)
	})
	t.Run("trap neighbor out of range", func(t *testing.T) {
		badPlane := buildPlaneBytes(
			0, 1, 1, 1, 0, 1, 0, 0,
			concat(f32le(10), f32le(20)),
			concat(u32le(1), u32le(0xFFFFFFFF), u32le(0xFFFFFFFF), u32le(0xFFFFFFFF),
				u16le(0xFFFF), u16le(0xFFFF),
				f32le(100), f32le(0), f32le(-50), f32le(50), f32le(-50), f32le(50)),
			concat(u32le(0), u32le(0), u32le(0xFFFFFFFF), u32le(0xFFFFFFFF)),
			nil, concat(u32le(0)), nil, nil, 0,
		)
		_, err := ParsePathData(wrapMap(buildPathBytes(badPlane)))
		assert.Error(t, err)
	})
}
