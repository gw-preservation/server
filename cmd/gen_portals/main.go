// Usage: go run ./cmd/gen_portals [-gwdat ./Gw.dat] [-o portal_definitions_new.go] [expansion]
//
// Reads instance definitions to find unique MapFileIds for the given expansion
// (default "presearing"), extracts portal prop geometry from each map file in
// the Gw.dat archive, and writes a mapTransitions variable to the output file.
package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"math"
	"os"
	"sort"

	"gw1/server/gameservice"
	"gw1/server/pathing"
)

const (
	chunkIDPropsInfo      = 0x20000004
	chunkIDPropsFilenames = 0x21000004
)

const (
	portalModelBaseWidth = 465.0
	portalModelDepth     = 40.0
	portalProjectDist    = 200.0
)

var portalFileIDs = map[uint32]bool{
	0x0000a825: true,
	0x0000e723: true,
}

type PropInfo struct {
	FilenameIndex  uint16
	X, Y, Z        float32
	F4, F5, F6     float32
	SinAngle       float32
	CosAngle       float32
	F9             float32
	ScalingFactor  float32
	F11            float32
	F12            uint8
	NumSomeStructs uint8
	SomeStructs    []byte
}

type PropFilename struct {
	ID0, ID1 uint16
	F1       uint16
}

type PortalQuad struct {
	X1, Z1 float64
	X2, Z2 float64
	X3, Z3 float64
	X4, Z4 float64
}

func decodeFilename(id0, id1 uint16) uint32 {
	return (uint32(id0) - 0xFF00FF) + (uint32(id1) * 0xFF00)
}

func portalQuad(prop PropInfo, direction int) PortalQuad {
	faceX := float64(prop.SinAngle)
	faceZ := float64(prop.CosAngle)
	dirX := faceX * float64(direction)
	dirZ := faceZ * float64(direction)
	widthX := -faceZ
	widthZ := faceX

	halfWidth := float64(prop.ScalingFactor) * portalModelBaseWidth / 2
	proj := portalProjectDist

	surfX := float64(prop.X) - faceX*portalModelDepth
	surfZ := float64(prop.Z) - faceZ*portalModelDepth

	cx := surfX + dirX*proj
	cz := surfZ + dirZ*proj

	return PortalQuad{
		X1: cx + widthX*halfWidth, Z1: cz + widthZ*halfWidth,
		X2: cx - widthX*halfWidth, Z2: cz - widthZ*halfWidth,
		X3: surfX - widthX*halfWidth, Z3: surfZ - widthZ*halfWidth,
		X4: surfX + widthX*halfWidth, Z4: surfZ + widthZ*halfWidth,
	}
}

func parsePropsChunk(data []byte) ([]PropInfo, error) {
	if len(data) < 10 {
		return nil, fmt.Errorf("props chunk too short for header")
	}
	_ = binary.LittleEndian.Uint32(data[0:4])
	_ = binary.LittleEndian.Uint16(data[4:6])
	propArraySize := binary.LittleEndian.Uint32(data[6:10])

	offset := 10
	if offset+int(propArraySize) > len(data) {
		return nil, fmt.Errorf("prop array size %d exceeds chunk data %d", propArraySize, len(data))
	}

	arrayData := data[offset : offset+int(propArraySize)]
	return parsePropArray(arrayData)
}

func parsePropArray(data []byte) ([]PropInfo, error) {
	if len(data) < 2 {
		return nil, fmt.Errorf("prop array too short")
	}

	numProps := binary.LittleEndian.Uint16(data[0:2])
	offset := 2

	props := make([]PropInfo, numProps)
	for i := uint16(0); i < numProps; i++ {
		prop, consumed, err := parsePropInfo(data[offset:])
		if err != nil {
			return nil, fmt.Errorf("prop %d: %w", i, err)
		}
		props[i] = prop
		offset += consumed
	}

	return props, nil
}

func parsePropInfo(data []byte) (PropInfo, int, error) {
	const fixedSize = 48
	if len(data) < fixedSize {
		return PropInfo{}, 0, fmt.Errorf("prop info too short (need %d, have %d)", fixedSize, len(data))
	}

	prop := PropInfo{
		FilenameIndex:  binary.LittleEndian.Uint16(data[0:2]),
		X:              math.Float32frombits(binary.LittleEndian.Uint32(data[2:6])),
		Z:              math.Float32frombits(binary.LittleEndian.Uint32(data[6:10])),
		Y:              -math.Float32frombits(binary.LittleEndian.Uint32(data[10:14])),
		F4:             math.Float32frombits(binary.LittleEndian.Uint32(data[14:18])),
		F5:             math.Float32frombits(binary.LittleEndian.Uint32(data[18:22])),
		F6:             math.Float32frombits(binary.LittleEndian.Uint32(data[22:26])),
		SinAngle:       math.Float32frombits(binary.LittleEndian.Uint32(data[26:30])),
		CosAngle:       math.Float32frombits(binary.LittleEndian.Uint32(data[30:34])),
		F9:             math.Float32frombits(binary.LittleEndian.Uint32(data[34:38])),
		ScalingFactor:  math.Float32frombits(binary.LittleEndian.Uint32(data[38:42])),
		F11:            math.Float32frombits(binary.LittleEndian.Uint32(data[42:46])),
		F12:            data[46],
		NumSomeStructs: data[47],
	}

	offset := fixedSize
	someStructsSize := int(prop.NumSomeStructs) * 8
	if someStructsSize > 0 {
		if len(data) < offset+someStructsSize {
			return PropInfo{}, 0, fmt.Errorf("prop some_structs truncated")
		}
		prop.SomeStructs = make([]byte, someStructsSize)
		copy(prop.SomeStructs, data[offset:offset+someStructsSize])
		offset += someStructsSize
	}

	return prop, offset, nil
}

func parsePropsFilenamesChunk(data []byte) ([]PropFilename, error) {
	if len(data) < 5 {
		return nil, fmt.Errorf("filenames chunk too short")
	}

	_ = binary.LittleEndian.Uint32(data[0:4])
	_ = data[4]

	offset := 5
	elementSize := 6
	numElements := (len(data) - offset) / elementSize

	filenames := make([]PropFilename, numElements)
	for i := 0; i < numElements; i++ {
		filenames[i] = PropFilename{
			ID0: binary.LittleEndian.Uint16(data[offset : offset+2]),
			ID1: binary.LittleEndian.Uint16(data[offset+2 : offset+4]),
			F1:  binary.LittleEndian.Uint16(data[offset+4 : offset+6]),
		}
		offset += elementSize
	}

	return filenames, nil
}

func extractPortals(archive *pathing.Archive, mapFileId uint32) ([]gameservice.MapPortalDefinition, error) {
	content, err := archive.GetFile(mapFileId)
	if err != nil {
		return nil, fmt.Errorf("failed to read map file 0x%08x: %v", mapFileId, err)
	}

	chunks, err := pathing.ParseRiffChunks(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse RIFF chunks: %v", err)
	}

	var filenames []PropFilename
	if fc := pathing.FindChunk(chunks, chunkIDPropsFilenames); fc != nil {
		filenames, err = parsePropsFilenamesChunk(fc.Data)
		if err != nil {
			return nil, fmt.Errorf("failed to parse prop filenames: %v", err)
		}
	}

	pc := pathing.FindChunk(chunks, chunkIDPropsInfo)
	if pc == nil {
		return nil, nil
	}

	props, err := parsePropsChunk(pc.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse props chunk: %v", err)
	}

	var portals []gameservice.MapPortalDefinition
	for _, prop := range props {
		var resolvedFileID uint32
		if int(prop.FilenameIndex) < len(filenames) {
			fn := filenames[prop.FilenameIndex]
			resolvedFileID = decodeFilename(fn.ID0, fn.ID1)
		}

		if !portalFileIDs[resolvedFileID] {
			continue
		}

		faceX := float64(prop.SinAngle)
		faceZ := float64(prop.CosAngle)

		for _, dir := range []int{1, -1} {
			q := portalQuad(prop, dir)
			spawnX := float32(math.Round(float64(prop.X) - faceX*portalModelDepth + faceX*500*float64(dir)))
			spawnY := float32(math.Round(float64(prop.Z) - faceZ*portalModelDepth + faceZ*500*float64(dir)))

			portals = append(portals, gameservice.MapPortalDefinition{
				Quad: gameservice.MapQuad{
					X1: float32(math.Round(q.X1)), Y1: float32(math.Round(q.Z1)),
					X2: float32(math.Round(q.X2)), Y2: float32(math.Round(q.Z2)),
					X3: float32(math.Round(q.X3)), Y3: float32(math.Round(q.Z3)),
					X4: float32(math.Round(q.X4)), Y4: float32(math.Round(q.Z4)),
				},
				SpawnX: spawnX,
				SpawnY: spawnY,
			})
		}
	}

	return portals, nil
}

func main() {
	gwdatPath := flag.String("gwdat", "./Gw.dat", "path to the Gw.dat archive")
	outPath := flag.String("o", "portal_definitions_new.go", "output file path")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: gen_portals [options] [expansion]\n\n")
		fmt.Fprintf(os.Stderr, "Extracts portal definitions from map files for instances\n")
		fmt.Fprintf(os.Stderr, "matching the given expansion (default \"presearing\").\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	expansion := "presearing"
	if flag.NArg() > 0 {
		expansion = flag.Arg(0)
	}

	fileIds := gameservice.GetInstanceMapFileIds(expansion)
	if len(fileIds) == 0 {
		fmt.Fprintf(os.Stderr, "no instances found for expansion %q\n", expansion)
		os.Exit(1)
	}

	mapIdLookup := gameservice.GetMapIdsForMapFileId()

	archive, err := pathing.Open(*gwdatPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open archive: %v\n", err)
		os.Exit(1)
	}
	defer archive.Close()

	type entry struct {
		fileId  uint32
		portals []gameservice.MapPortalDefinition
	}
	var entries []entry

	for mapFileId := range fileIds {
		portals, err := extractPortals(archive, uint32(mapFileId))
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: %v\n", err)
			continue
		}
		if len(portals) > 0 {
			entries = append(entries, entry{fileId: uint32(mapFileId), portals: portals})
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].fileId < entries[j].fileId
	})

	f, err := os.Create(*outPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create output file: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	fmt.Fprintln(f, "// Code generated by cmd/gen_portals; DO NOT EDIT.")
	fmt.Fprintln(f)
	fmt.Fprintln(f, "package gameservice")
	fmt.Fprintln(f)
	fmt.Fprintf(f, "var mapTransitions = map[int][]MapPortalDefinition{\n")
	for _, e := range entries {
		ids := mapIdLookup[int(e.fileId)]
		if len(ids) > 0 {
			comment := "// "
			for i, id := range ids {
				if i > 0 {
					comment += ", "
				}
				comment += fmt.Sprintf("%d: %s", id.MapId, id.Name)
			}
			fmt.Fprintf(f, "\t0x%x: {%s\n", e.fileId, comment)
		} else {
			fmt.Fprintf(f, "\t0x%x: {\n", e.fileId)
		}
		for _, p := range e.portals {
			fmt.Fprintf(f, "\t\t{\n")
			fmt.Fprintf(f, "\t\t\tQuad: MapQuad{\n")
			fmt.Fprintf(f, "\t\t\t\tX1: %d, Y1: %d,\n", int(p.Quad.X1), int(p.Quad.Y1))
			fmt.Fprintf(f, "\t\t\t\tX2: %d, Y2: %d,\n", int(p.Quad.X2), int(p.Quad.Y2))
			fmt.Fprintf(f, "\t\t\t\tX3: %d, Y3: %d,\n", int(p.Quad.X3), int(p.Quad.Y3))
			fmt.Fprintf(f, "\t\t\t\tX4: %d, Y4: %d,\n", int(p.Quad.X4), int(p.Quad.Y4))
			fmt.Fprintf(f, "\t\t\t},\n")
			fmt.Fprintf(f, "\t\t\tSpawnX: %d,\n", int(p.SpawnX))
			fmt.Fprintf(f, "\t\t\tSpawnY: %d,\n", int(p.SpawnY))
			fmt.Fprintf(f, "\t\t},\n")
		}
		fmt.Fprintf(f, "\t},\n")
	}
	fmt.Fprintf(f, "}\n")

	fmt.Fprintf(os.Stderr, "wrote %d entries to %s\n", len(entries), *outPath)
}
