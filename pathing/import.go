package pathing

import (
	"encoding/binary"
	"fmt"
	"math"
	"sort"

	"gw1/server/geom"
)

// RIFF container (map files are RIFF with signature "ffna").
const (
	riffSignature = "ffna"
	riffTypes     = 9 // sentinel: valid type bytes are < riffTypes
	riffMap1      = 1
	riffMap2      = 3
)

const (
	pathStage       = 2
	chunkIDPath     = 8
	pathChunkID     = (pathStage << 28) | chunkIDPath
	pathSignature   = 0xEEFE704C
	pathVersion     = 12
	pathSeqFastSave = 0xFFFFFFFF
)

// Node id namespaces (see mapdata.go Node doc).
const (
	firstXNodeIndex    = 0x00000000
	firstYNodeIndex    = 0x40000000
	firstSinkNodeIndex = 0x80000000
	noIndex            = -1
)

// RiffChunk is a parsed RIFF chunk with its ID and payload.
type RiffChunk struct {
	ID   uint32
	Data []byte
}

type cursor struct {
	data []byte
	pos  int
}

func (c *cursor) remaining() int { return len(c.data) - c.pos }

func (c *cursor) skip(n int) bool {
	if n < 0 || c.pos+n > len(c.data) {
		return false
	}
	c.pos += n
	return true
}

func (c *cursor) u8() (uint8, bool) {
	if c.remaining() < 1 {
		return 0, false
	}
	v := c.data[c.pos]
	c.pos++
	return v, true
}

func (c *cursor) u16() (uint16, bool) {
	if c.remaining() < 2 {
		return 0, false
	}
	v := binary.LittleEndian.Uint16(c.data[c.pos:])
	c.pos += 2
	return v, true
}

func (c *cursor) u32() (uint32, bool) {
	if c.remaining() < 4 {
		return 0, false
	}
	v := binary.LittleEndian.Uint32(c.data[c.pos:])
	c.pos += 4
	return v, true
}

func (c *cursor) f32() (float32, bool) {
	v, ok := c.u32()
	return math.Float32frombits(v), ok
}

// ParsePathData parses pathing data from a decompressed RIFF map file ("ffna"
// type 1/3, stage-2 path chunk).
func ParsePathData(content []byte) (*PathData, error) {
	riffType, ok := riffType(content)
	if !ok || (riffType != riffMap1 && riffType != riffMap2) {
		return nil, fmt.Errorf("not a map file (riff type %d)", riffType)
	}

	chunks, err := ParseRiffChunks(content)
	if err != nil {
		return nil, err
	}
	chunk := FindChunk(chunks, pathChunkID)
	if chunk == nil {
		return nil, fmt.Errorf("no path chunk (0x%08x)", pathChunkID)
	}

	sd := &PathData{}
	if err := importPathData(sd, chunk.Data); err != nil {
		return nil, err
	}
	return sd, nil
}

func riffType(content []byte) (uint8, bool) {
	if len(content) < 5 {
		return 0, false
	}
	if string(content[:4]) != riffSignature {
		return 0, false
	}
	t := content[4]
	if t >= riffTypes {
		return 0, false
	}
	return t, true
}

// ParseRiffChunks splits a decompressed RIFF "ffna" file into its chunks.
func ParseRiffChunks(content []byte) ([]RiffChunk, error) {
	if len(content) < 5 || string(content[:4]) != riffSignature || content[4] >= riffTypes {
		return nil, fmt.Errorf("bad riff header")
	}
	data := content[5:]
	var chunks []RiffChunk
	for len(data) >= 8 {
		id := binary.LittleEndian.Uint32(data[0:4])
		size := binary.LittleEndian.Uint32(data[4:8])
		if uint64(len(data)) < 8+uint64(size) {
			return nil, fmt.Errorf("truncated riff chunk")
		}
		chunks = append(chunks, RiffChunk{ID: id, Data: data[8 : 8+size]})
		data = data[8+size:]
	}
	sort.Slice(chunks, func(i, j int) bool { return chunks[i].ID < chunks[j].ID })
	return chunks, nil
}

// FindChunk returns the first chunk with the given ID, or nil if not found.
func FindChunk(chunks []RiffChunk, id uint32) *RiffChunk {
	i := sort.Search(len(chunks), func(i int) bool { return chunks[i].ID >= id })
	if i < len(chunks) && chunks[i].ID == id {
		return &chunks[i]
	}
	return nil
}

func readTagged(c *cursor, tag uint8) (uint32, error) {
	if c.remaining() < 5 {
		return 0, fmt.Errorf("truncated tagged chunk header")
	}
	if c.data[c.pos] != tag {
		return 0, fmt.Errorf("expected tagged chunk %d at offset %d, got %d", tag, c.pos, c.data[c.pos])
	}
	length := binary.LittleEndian.Uint32(c.data[c.pos+1 : c.pos+5])
	c.pos += 5
	if c.remaining() < int(length) {
		return 0, fmt.Errorf("tagged chunk %d payload truncated (need %d, have %d)", tag, length, c.remaining())
	}
	return length, nil
}

func importPathData(sd *PathData, data []byte) error {
	c := &cursor{data: data}

	if c.remaining() < 12 {
		return fmt.Errorf("path data too short for header")
	}
	sig := binary.LittleEndian.Uint32(c.data[0:4])
	ver := binary.LittleEndian.Uint32(c.data[4:8])
	seq := binary.LittleEndian.Uint32(c.data[8:12])
	if sig != pathSignature || ver != pathVersion {
		return fmt.Errorf("bad path data header (sig 0x%08x, version %d)", sig, ver)
	}
	if seq == pathSeqFastSave {
		return fmt.Errorf("path data uses fast-save sequence")
	}
	c.pos += 12

	if c.remaining() < 5 {
		return fmt.Errorf("path data too short for step-2 blob")
	}
	stepSize := binary.LittleEndian.Uint32(c.data[c.pos+1 : c.pos+5])
	if !c.skip(5) || !c.skip(int(stepSize)) {
		return fmt.Errorf("path data step-2 blob truncated")
	}

	// Tag 8: planes.
	if _, err := readTagged(c, 8); err != nil {
		return err
	}
	planeCount, ok := c.u32()
	if !ok {
		return fmt.Errorf("truncated plane count")
	}
	pairs := make(map[uint32]portalRef) // shared across planes
	for i := uint32(0); i < planeCount; i++ {
		if err := importPlane(sd, c, pairs); err != nil {
			return fmt.Errorf("plane %d: %w", i, err)
		}
	}
	return nil
}

type nodeResolver struct {
	numX, numY, numSinks uint32
}

func (r nodeResolver) index(nodeID uint32) int {
	if nodeID < firstYNodeIndex {
		if nodeID < r.numX {
			return int(nodeID)
		}
		return noIndex
	}
	if nodeID < firstSinkNodeIndex {
		idx := nodeID - firstYNodeIndex
		if idx < r.numY {
			return int(r.numX) + int(idx)
		}
		return noIndex
	}
	idx := nodeID - firstSinkNodeIndex
	if idx < r.numSinks {
		return int(r.numX) + int(r.numY) + int(idx)
	}
	return noIndex
}

func importPlane(sd *PathData, c *cursor, pairs map[uint32]portalRef) error {
	planeID := uint16(len(sd.Planes))

	if _, err := readTagged(c, 0); err != nil {
		return err
	}
	if c.remaining() < 32 {
		return fmt.Errorf("plane counts truncated")
	}
	h000C := binary.LittleEndian.Uint32(c.data[c.pos+0:])
	vectorCount := binary.LittleEndian.Uint32(c.data[c.pos+4:])
	trapCount := binary.LittleEndian.Uint32(c.data[c.pos+8:])
	xnodeCount := binary.LittleEndian.Uint32(c.data[c.pos+12:])
	ynodeCount := binary.LittleEndian.Uint32(c.data[c.pos+16:])
	sinkCount := binary.LittleEndian.Uint32(c.data[c.pos+20:])
	portalCount := binary.LittleEndian.Uint32(c.data[c.pos+24:])
	portalTrapsCount := binary.LittleEndian.Uint32(c.data[c.pos+28:])
	c.pos += 32

	needed := uint64(h000C)*8 +
		uint64(vectorCount)*8 +
		uint64(trapCount)*44 +
		uint64(xnodeCount)*16 +
		uint64(ynodeCount)*12 +
		uint64(sinkCount)*4 +
		uint64(portalCount)*9 +
		uint64(portalTrapsCount)*4
	if needed > uint64(c.remaining()) {
		return fmt.Errorf("counts imply %d payload bytes but only %d remain", needed, c.remaining())
	}

	plane := Plane{PlaneID: planeID, NumXNodes: xnodeCount, NumYNodes: ynodeCount}
	plane.Vectors = make([]geom.Vec2, vectorCount)
	plane.Trapezoids = make([]Trapezoid, trapCount)
	plane.Nodes = make([]Node, int(xnodeCount)+int(ynodeCount)+int(sinkCount))
	plane.Portals = make([]Portal, portalCount)
	plane.PortalsTraps = make([]int, portalTrapsCount)
	for i := range plane.PortalsTraps {
		plane.PortalsTraps[i] = noIndex
	}

	resolver := nodeResolver{numX: xnodeCount, numY: ynodeCount, numSinks: sinkCount}

	// Tag 11: h000C*8 bytes of unknown data.
	if length, err := readTagged(c, 11); err != nil {
		return err
	} else if uint64(h000C)*8 > uint64(length) {
		return fmt.Errorf("tag 11 payload too small (h000C=%d, len=%d)", h000C, length)
	}
	if !c.skip(int(h000C) * 8) {
		return fmt.Errorf("tag 11 truncated")
	}

	if length, err := readTagged(c, 1); err != nil {
		return err
	} else if int(length) != int(vectorCount)*8 {
		return fmt.Errorf("vectors length mismatch (have %d, want %d)", length, vectorCount*8)
	}
	for i := range plane.Vectors {
		x, _ := c.f32()
		y, _ := c.f32()
		plane.Vectors[i] = geom.Vec2{X: x, Y: y}
	}

	if length, err := readTagged(c, 2); err != nil {
		return err
	} else if int(length) != int(trapCount)*44 {
		return fmt.Errorf("trapezoids length mismatch (have %d, want %d)", length, trapCount*44)
	}
	for i := range plane.Trapezoids {
		trapID0, _ := c.u32()
		trapID1, _ := c.u32()
		trapID2, _ := c.u32()
		trapID3, _ := c.u32()
		portalLeft, _ := c.u16()
		portalRight, _ := c.u16()
		yt, _ := c.f32()
		yb, _ := c.f32()
		xtl, _ := c.f32()
		xtr, _ := c.f32()
		xbl, _ := c.f32()
		xbr, _ := c.f32()

		trap := &plane.Trapezoids[i]
		trap.TrapID = sd.TrapsCount
		sd.TrapsCount++
		var err error
		if trap.NeighborTL, err = trapRef(&plane, trapID0); err != nil {
			return err
		}
		if trap.NeighborTR, err = trapRef(&plane, trapID1); err != nil {
			return err
		}
		if trap.NeighborBL, err = trapRef(&plane, trapID2); err != nil {
			return err
		}
		if trap.NeighborBR, err = trapRef(&plane, trapID3); err != nil {
			return err
		}
		trap.PortalLeft = portalLeft
		trap.PortalRight = portalRight
		trap.YT, trap.YB, trap.XTL, trap.XTR, trap.XBL, trap.XBR = yt, yb, xtl, xtr, xbl, xbr
	}

	if length, err := readTagged(c, 3); err != nil {
		return err
	} else if length < 1 {
		return fmt.Errorf("root node payload empty")
	}
	rootType := c.data[c.pos]
	c.pos++
	switch rootType {
	case 0:
		plane.RootNode = 0
	case 1:
		plane.RootNode = int(xnodeCount)
	case 2:
		plane.RootNode = int(xnodeCount) + int(ynodeCount)
	default:
		return fmt.Errorf("invalid root node type %d", rootType)
	}

	if length, err := readTagged(c, 4); err != nil {
		return err
	} else if int(length) != int(xnodeCount)*16 {
		return fmt.Errorf("xnodes length mismatch (have %d, want %d)", length, xnodeCount*16)
	}
	for i := uint32(0); i < xnodeCount; i++ {
		pos1, _ := c.u32()
		pos2, _ := c.u32()
		left, _ := c.u32()
		right, _ := c.u32()
		if pos1 >= vectorCount || pos2 >= vectorCount {
			return fmt.Errorf("xnode %d references vector %d/%d out of range (%d)", i, pos1, pos2, vectorCount)
		}
		node := &plane.Nodes[i]
		node.Type = NodeTypeXNode
		p1 := plane.Vectors[pos1]
		p2 := plane.Vectors[pos2]
		node.X, node.Y = p1.X, p1.Y
		node.DirX, node.DirY = p2.X-p1.X, p2.Y-p1.Y
		node.Left = resolver.index(left)
		node.Right = resolver.index(right)
	}

	if length, err := readTagged(c, 5); err != nil {
		return err
	} else if int(length) != int(ynodeCount)*12 {
		return fmt.Errorf("ynodes length mismatch (have %d, want %d)", length, ynodeCount*12)
	}
	for i := uint32(0); i < ynodeCount; i++ {
		pos, _ := c.u32()
		above, _ := c.u32()
		below, _ := c.u32()
		if pos >= vectorCount {
			return fmt.Errorf("ynode %d references vector %d out of range (%d)", i, pos, vectorCount)
		}
		node := &plane.Nodes[int(xnodeCount)+int(i)]
		node.Type = NodeTypeYNode
		v := plane.Vectors[pos]
		node.X, node.Y = v.X, v.Y
		node.Above = resolver.index(above)
		node.Below = resolver.index(below)
	}

	if length, err := readTagged(c, 6); err != nil {
		return err
	} else if int(length) != int(sinkCount)*4 {
		return fmt.Errorf("sinks length mismatch (have %d, want %d)", length, sinkCount*4)
	}
	for i := uint32(0); i < sinkCount; i++ {
		trap, _ := c.u32()
		if trap >= trapCount {
			return fmt.Errorf("sink %d references trapezoid %d out of range (%d)", i, trap, trapCount)
		}
		node := &plane.Nodes[int(xnodeCount)+int(ynodeCount)+int(i)]
		node.Type = NodeTypeSink
		node.Trap = int(trap)
	}

	if length, err := readTagged(c, 10); err != nil {
		return err
	} else if int(length) != int(portalTrapsCount)*4 {
		return fmt.Errorf("portal traps length mismatch (have %d, want %d)", length, portalTrapsCount*4)
	}
	for i := range plane.PortalsTraps {
		trap, _ := c.u32()
		if trap < trapCount {
			plane.PortalsTraps[i] = int(trap)
		}
	}

	// Tag 9: portals (with cross-plane pairing).
	if length, err := readTagged(c, 9); err != nil {
		return err
	} else if int(length) != int(portalCount)*9 {
		return fmt.Errorf("portals length mismatch (have %d, want %d)", length, portalCount*9)
	}
	for i := range plane.Portals {
		trapsCount, _ := c.u16()
		trapsIdx, _ := c.u16()
		neighborPlaneID, _ := c.u16()
		neighborSharedID, _ := c.u16()
		flags, _ := c.u8()

		p := &plane.Portals[i]
		p.PortalPlaneID = planeID
		p.NeighborPlaneID = neighborPlaneID
		p.Flags = flags
		p.TrapsCount = uint32(trapsCount)
		p.PairPlane, p.PairPortal = noIndex, noIndex
		if int(trapsIdx)+int(trapsCount) > len(plane.PortalsTraps) {
			return fmt.Errorf("portal %d traps slice out of range", i)
		}
		p.Traps = plane.PortalsTraps[int(trapsIdx) : int(trapsIdx)+int(trapsCount)]

		myPlaneIdx := len(sd.Planes)
		selfKey := (uint32(planeID) << 16) | uint32(neighborSharedID)
		pairs[selfKey] = portalRef{plane: myPlaneIdx, portal: i}

		twinKey := (uint32(neighborPlaneID) << 16) | uint32(neighborSharedID)
		if twin, ok := pairs[twinKey]; ok {
			twinPortal := portalAt(sd, &plane, twin)
			if twinPortal.PairPlane != noIndex {
				return fmt.Errorf("portal %d already paired", i)
			}
			p.PairPlane = twin.plane
			p.PairPortal = twin.portal
			twinPortal.PairPlane = myPlaneIdx
			twinPortal.PairPortal = i
		}
	}

	buildTrapGrid(&plane)

	sd.Planes = append(sd.Planes, plane)
	return nil
}

type portalRef struct {
	plane  int
	portal int
}

func portalAt(sd *PathData, cur *Plane, ref portalRef) *Portal {
	if ref.plane == len(sd.Planes) {
		return &cur.Portals[ref.portal]
	}
	return &sd.Planes[ref.plane].Portals[ref.portal]
}

func trapRef(plane *Plane, datIdx uint32) (int, error) {
	if datIdx == 0xFFFFFFFF {
		return noIndex, nil
	}
	if datIdx >= uint32(len(plane.Trapezoids)) {
		return 0, fmt.Errorf("trapezoid index %d out of range (%d)", datIdx, len(plane.Trapezoids))
	}
	return int(datIdx), nil
}
