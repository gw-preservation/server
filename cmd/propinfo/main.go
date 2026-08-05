// Usage: go run ./cmd/propinfo -gwdat ./Gw.dat 0x12345678
package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"math"
	"os"
	"strconv"

	"gw1/server/pathing"
)

// Chunk IDs from FFNA_MapFile.h
const (
	chunkIDPropsInfo      = 0x20000004
	chunkIDPropsFilenames = 0x21000004
)

// Portal model constants and projection settings.
const (
	portalModelBaseWidth = 465.0 // model-space width of the portal prop (X extent)
	portalModelDepth     = 40.0  // model-space depth offset from prop origin to visual surface
	portalProjectDist    = 200.0 // how far to project the zone forward from the portal
)

// portalFileIDs is the set of file_ids that represent portal props.
var portalFileIDs = map[uint32]bool{
	0x0000a825: true,
	0x0000e723: true,
}

// decodeFilename converts an (id0, id1) pair into a file_id.
// Formula from pch.h: (id0 - 0xFF00FF) + (id1 * 0xFF00)
func decodeFilename(id0, id1 uint16) uint32 {
	return (uint32(id0) - 0xFF00FF) + (uint32(id1) * 0xFF00)
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
	X1, Z1 float64 // right-forward corner
	X2, Z2 float64 // left-forward corner
	X3, Z3 float64 // left-back corner
	X4, Z4 float64 // right-back corner
}

func portalQuad(prop PropInfo, direction int) PortalQuad {
	// Face normal = (sin(angle), cos(angle)) = (SinAngle, CosAngle).
	faceX := float64(prop.SinAngle)
	faceZ := float64(prop.CosAngle)
	// Walk-through direction is along the face normal (through the portal).
	dirX := faceX * float64(direction)
	dirZ := faceZ * float64(direction)
	// Width is 90° from face normal (along the portal surface).
	widthX := -faceZ
	widthZ := faceX

	halfWidth := float64(prop.ScalingFactor) * portalModelBaseWidth / 2
	proj := portalProjectDist

	// Offset prop position by model depth to get to the visual surface.
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

func main() {
	gwdatPath := flag.String("gwdat", "./Gw.dat", "path to the Gw.dat archive")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: propinfo [options] <map_file_id>\n\n")
		fmt.Fprintf(os.Stderr, "Reads prop data for the given map file id from the\n")
		fmt.Fprintf(os.Stderr, "Gw.dat archive and logs it to stdout.\n")
		fmt.Fprintf(os.Stderr, "Flags must come before the map file id.\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(2)
	}
	fileID64, err := strconv.ParseUint(flag.Arg(0), 0, 32)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid map file id %q\n", flag.Arg(0))
		os.Exit(2)
	}
	fileID := uint32(fileID64)

	archive, err := pathing.Open(*gwdatPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open archive: %v\n", err)
		os.Exit(1)
	}
	defer archive.Close()

	content, err := archive.GetFile(fileID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read map file 0x%08x: %v\n", fileID, err)
		os.Exit(1)
	}

	chunks, err := pathing.ParseRiffChunks(content)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse RIFF chunks: %v\n", err)
		os.Exit(1)
	}

	// Parse prop filenames first so we can resolve names
	var filenames []PropFilename
	if fc := pathing.FindChunk(chunks, chunkIDPropsFilenames); fc != nil {
		filenames, err = parsePropsFilenamesChunk(fc.Data)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to parse prop filenames: %v\n", err)
		}
	}

	// Find and parse the props info chunk
	pc := pathing.FindChunk(chunks, chunkIDPropsInfo)
	if pc == nil {
		fmt.Fprintf(os.Stderr, "no props chunk (0x%08x) found in map file 0x%08x\n", chunkIDPropsInfo, fileID)
		os.Exit(1)
	}

	props, err := parsePropsChunk(pc.Data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse props chunk: %v\n", err)
		os.Exit(1)
	}

	// Log output
	fmt.Printf("Map File: 0x%08x\n", fileID)
	fmt.Printf("Props Count: %d\n", len(props))
	fmt.Printf("Prop Filenames Count: %d\n", len(filenames))
	fmt.Println()

	for i, prop := range props {
		var resolvedFileID uint32
		if int(prop.FilenameIndex) < len(filenames) {
			fn := filenames[prop.FilenameIndex]
			resolvedFileID = decodeFilename(fn.ID0, fn.ID1)
		}

		if !portalFileIDs[resolvedFileID] {
			continue
		}

		fmt.Printf("Prop #%d:\n", i)
		fmt.Printf("  Filename Index: %d  -> file_id=0x%08x\n", prop.FilenameIndex, resolvedFileID)
		fmt.Printf("  Position: (%.4f, %.4f, %.4f)\n", prop.X, prop.Y, prop.Z)
		fmt.Printf("  Rotation: sin=%.4f cos=%.4f", prop.SinAngle, prop.CosAngle)
		if prop.SinAngle != 0 || prop.CosAngle != 0 {
			angle := math.Atan2(float64(prop.SinAngle), float64(prop.CosAngle))
			fmt.Printf("  angle=%.2f deg", angle*180/math.Pi)
		}
		fmt.Println()
		fmt.Printf("  Scale: %.4f\n", prop.ScalingFactor)
		if prop.F4 != 0 || prop.F5 != 0 || prop.F6 != 0 {
			fmt.Printf("  Unknown: f4=%.4f f5=%.4f f6=%.4f\n", prop.F4, prop.F5, prop.F6)
		}
		if prop.F9 != 0 {
			fmt.Printf("  f9: %.4f\n", prop.F9)
		}
		if prop.F11 != 0 {
			fmt.Printf("  f11: %.4f\n", prop.F11)
		}
		if prop.F12 != 0 {
			fmt.Printf("  f12: %d\n", prop.F12)
		}
		if prop.NumSomeStructs > 0 {
			fmt.Printf("  SomeStructs: %d entries (%d bytes)\n", prop.NumSomeStructs, len(prop.SomeStructs))
		}

		// Forward direction: walk through portal from front to back
		q1 := portalQuad(prop, 1)
		faceX := float64(prop.SinAngle)
		faceZ := float64(prop.CosAngle)
		spawnX1 := int(math.Round(float64(prop.X) - faceX*portalModelDepth + faceX*500))
		spawnY1 := int(math.Round(float64(prop.Z) - faceZ*portalModelDepth + faceZ*500))
		fmt.Printf("  From back:\n")
		fmt.Printf("Quad: MapQuad{\n")
		fmt.Printf("\t\t\t\tX1: %d, Y1: %d,\n", int(math.Round(q1.X1)), int(math.Round(q1.Z1)))
		fmt.Printf("\t\t\t\tX2: %d, Y2: %d,\n", int(math.Round(q1.X2)), int(math.Round(q1.Z2)))
		fmt.Printf("\t\t\t\tX3: %d, Y3: %d,\n", int(math.Round(q1.X3)), int(math.Round(q1.Z3)))
		fmt.Printf("\t\t\t\tX4: %d, Y4: %d,\n", int(math.Round(q1.X4)), int(math.Round(q1.Z4)))
		fmt.Printf("\t\t\t},\n")
		fmt.Printf("\t\t\tSpawnX: %d,\n", spawnX1)
		fmt.Printf("\t\t\tSpawnY: %d,\n", spawnY1)

		// Backward direction: walk through portal from back to front
		q2 := portalQuad(prop, -1)
		spawnX2 := int(math.Round(float64(prop.X) - faceX*portalModelDepth - faceX*500))
		spawnY2 := int(math.Round(float64(prop.Z) - faceZ*portalModelDepth - faceZ*500))
		fmt.Printf("  From front:\n")
		fmt.Printf("Quad: MapQuad{\n")
		fmt.Printf("\t\t\t\tX1: %d, Y1: %d,\n", int(math.Round(q2.X1)), int(math.Round(q2.Z1)))
		fmt.Printf("\t\t\t\tX2: %d, Y2: %d,\n", int(math.Round(q2.X2)), int(math.Round(q2.Z2)))
		fmt.Printf("\t\t\t\tX3: %d, Y3: %d,\n", int(math.Round(q2.X3)), int(math.Round(q2.Z3)))
		fmt.Printf("\t\t\t\tX4: %d, Y4: %d,\n", int(math.Round(q2.X4)), int(math.Round(q2.Z4)))
		fmt.Printf("\t\t\t},\n")
		fmt.Printf("\t\t\tSpawnX: %d,\n", spawnX2)
		fmt.Printf("\t\t\tSpawnY: %d,\n", spawnY2)
		fmt.Println()
	}
}

func parsePropsChunk(data []byte) ([]PropInfo, error) {
	if len(data) < 10 {
		return nil, fmt.Errorf("props chunk too short for header")
	}

	// Skip header: magic_number (4) + magic_number2 (2) + prop_array_size_in_bytes (4)
	magicNumber := binary.LittleEndian.Uint32(data[0:4])
	magicNumber2 := binary.LittleEndian.Uint16(data[4:6])
	propArraySize := binary.LittleEndian.Uint32(data[6:10])

	_ = magicNumber
	_ = magicNumber2

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
	// Fixed-size portion: 48 bytes
	const fixedSize = 48
	if len(data) < fixedSize {
		return PropInfo{}, 0, fmt.Errorf("prop info too short (need %d, have %d)", fixedSize, len(data))
	}

	prop := PropInfo{
		FilenameIndex:  binary.LittleEndian.Uint16(data[0:2]),
		X:              math.Float32frombits(binary.LittleEndian.Uint32(data[2:6])),
		Z:              math.Float32frombits(binary.LittleEndian.Uint32(data[6:10])),   // note: z and y swapped
		Y:              -math.Float32frombits(binary.LittleEndian.Uint32(data[10:14])), // y negated
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

	// Chunk4DataHeader: signature (4) + version (1)
	_ = binary.LittleEndian.Uint32(data[0:4]) // signature
	_ = data[4]                               // version

	offset := 5
	elementSize := 6 // FileName (4) + f1 (2)
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
