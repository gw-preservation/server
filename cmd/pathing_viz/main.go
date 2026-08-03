// Usage: go run ./cmd/pathing_viz -gwdat ./Gw.dat 0x1b97d -o out.bmp
package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"

	"gw1/server/pathing"
)

const maxDim = 10000

var (
	background = [3]int{0, 0, 0}
	fillColor  = [3]int{0, 255, 0}
	edgeColor  = [3]int{160, 255, 160}
	xnodeColor = [3]int{255, 64, 64}
	ynodeColor = [3]int{64, 128, 255}
)

type trap struct {
	plane         int
	yt, yb, xtl   float64
	xtr, xbl, xbr float64
}

func (t trap) corners() [4][2]float64 {
	return [4][2]float64{
		{t.xtl, t.yt},
		{t.xtr, t.yt},
		{t.xbr, t.yb},
		{t.xbl, t.yb},
	}
}

func main() {
	gwdatPath := flag.String("gwdat", "./Gw.dat", "path to the Gw.dat archive")
	outPath := flag.String("o", "", "output BMP path (default <fileid>.bmp)")
	width := flag.Int("width", 2000, "target image width in px")
	scale := flag.Float64("scale", 0, "pixels per world unit (overrides --width)")
	colorize := flag.Bool("colorize-planes", false, "shade each plane with a distinct color")
	noFill := flag.Bool("no-fill", false, "skip trapezoid fill")
	noNodes := flag.Bool("no-nodes", false, "skip node markers")
	noOutline := flag.Bool("no-outline", false, "skip trapezoid outlines")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: pathing_viz [options] <map_file_id>\n\n")
		fmt.Fprintf(os.Stderr, "Reads pathing data for the given map file id out of the\n")
		fmt.Fprintf(os.Stderr, "Gw.dat archive and renders it to a BMP image.\n")
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
	sd, err := pathing.ParsePathData(content)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse pathing data for 0x%08x: %v\n", fileID, err)
		os.Exit(1)
	}

	render(sd, fileID, *outPath, *width, *scale, *colorize, *noFill, *noNodes, *noOutline)
}

func render(sd *pathing.PathData, fileID uint32, outPath string, width int, scale float64, colorize, noFill, noNodes, noOutline bool) {
	traps, xnodes, ynodes, planeIDs := extract(sd)

	traps, xnodes, ynodes = dropNonFinite(traps, xnodes, ynodes)
	if len(traps) == 0 {
		fmt.Fprintf(os.Stderr, "no trapezoids found in map file 0x%08x\n", fileID)
		os.Exit(1)
	}

	minX, maxX, minY, maxY := computeBounds(traps, xnodes, ynodes)
	xrange := maxX - minX
	yrange := maxY - minY

	if scale == 0 {
		scale = float64(width) / xrange
	}
	w := int(math.Round(xrange * scale))
	h := int(math.Round(yrange * scale))
	if max(w, h) > maxDim {
		scale *= maxDim / float64(max(w, h))
		w = int(math.Round(xrange * scale))
		h = int(math.Round(yrange * scale))
	}
	if w < 1 || h < 1 {
		fmt.Fprintln(os.Stderr, "image too small")
		os.Exit(1)
	}

	px := func(x float64) float64 { return (x - minX) * scale }
	py := func(y float64) float64 { return (maxY - y) * scale }

	fmt.Fprintf(os.Stderr, "planes: %d, traps: %d, xnodes: %d, ynodes: %d, image: %dx%d\n",
		len(planeIDs), len(traps), len(xnodes), len(ynodes), w, h)

	im := newImage(w, h)
	im.fill(background)

	palette := buildPalette(max(len(planeIDs), 1))
	sortedPlanes := make([]int, 0, len(planeIDs))
	for p := range planeIDs {
		sortedPlanes = append(sortedPlanes, p)
	}
	sort.Ints(sortedPlanes)
	planeColor := make(map[int][3]int, len(sortedPlanes))
	for i, p := range sortedPlanes {
		planeColor[p] = palette[i]
	}

	if !noFill {
		for _, tr := range traps {
			color := fillColor
			if colorize {
				color = planeColor[tr.plane]
			}
			var pts [4][2]float64
			cs := tr.corners()
			for i, c := range cs {
				pts[i] = [2]float64{px(c[0]), py(c[1])}
			}
			im.fillPolygon(pts[:], color)
		}
	}

	if !noOutline {
		for _, tr := range traps {
			cs := tr.corners()
			for i := range cs {
				x0, y0 := px(cs[i][0]), py(cs[i][1])
				x1, y1 := px(cs[(i+1)%4][0]), py(cs[(i+1)%4][1])
				im.drawLine(int(x0), int(y0), int(x1), int(y1), edgeColor)
			}
		}
	}

	if !noNodes {
		r := max(1, int(math.Round(scale*2)))
		for _, n := range xnodes {
			cx, cy := int(px(n[0])), int(py(n[1]))
			im.drawCircle(cx, cy, r, xnodeColor)
			d := max(scale*6.0, 2.0)
			vlen := math.Hypot(n[2], n[3])
			if vlen > 0 {
				im.drawLine(cx, cy,
					int(float64(cx)+n[2]/vlen*d), int(float64(cy)-n[3]/vlen*d), xnodeColor)
			}
		}
		for _, n := range ynodes {
			im.drawCircle(int(px(n[0])), int(py(n[1])), r, ynodeColor)
		}
	}

	if outPath == "" {
		outPath = fmt.Sprintf("0x%x.bmp", fileID)
	}
	if err := writeBMP(outPath, im); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write %s: %v\n", outPath, err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "wrote %s\n", outPath)
}

func extract(sd *pathing.PathData) ([]trap, [][4]float64, [][2]float64, map[int]bool) {
	var traps []trap
	var xnodes [][4]float64
	var ynodes [][2]float64
	planes := make(map[int]bool)

	for _, pl := range sd.Planes {
		planes[int(pl.PlaneID)] = true
		for i := range pl.Trapezoids {
			tr := pl.Trapezoids[i]
			traps = append(traps, trap{
				plane: int(pl.PlaneID),
				yt:    float64(tr.YT), yb: float64(tr.YB),
				xtl: float64(tr.XTL), xtr: float64(tr.XTR),
				xbl: float64(tr.XBL), xbr: float64(tr.XBR),
			})
		}
		for i := uint32(0); i < pl.NumXNodes; i++ {
			n := pl.Nodes[i]
			xnodes = append(xnodes, [4]float64{float64(n.X), float64(n.Y), float64(n.DirX), float64(n.DirY)})
		}
		for i := pl.NumXNodes; i < pl.NumXNodes+pl.NumYNodes; i++ {
			n := pl.Nodes[i]
			ynodes = append(ynodes, [2]float64{float64(n.X), float64(n.Y)})
		}
	}
	return traps, xnodes, ynodes, planes
}

func dropNonFinite(traps []trap, xnodes [][4]float64, ynodes [][2]float64) ([]trap, [][4]float64, [][2]float64) {
	var t []trap
	for _, tr := range traps {
		if finite(tr.yt) && finite(tr.yb) && finite(tr.xtl) && finite(tr.xtr) && finite(tr.xbl) && finite(tr.xbr) {
			t = append(t, tr)
		}
	}
	var x [][4]float64
	for _, n := range xnodes {
		if finite(n[0]) && finite(n[1]) && finite(n[2]) && finite(n[3]) {
			x = append(x, n)
		}
	}
	var y [][2]float64
	for _, n := range ynodes {
		if finite(n[0]) && finite(n[1]) {
			y = append(y, n)
		}
	}
	return t, x, y
}

func finite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

func computeBounds(traps []trap, xnodes [][4]float64, ynodes [][2]float64) (minX, maxX, minY, maxY float64) {
	xs := make([]float64, 0, len(traps)*4+len(xnodes)+len(ynodes))
	ys := make([]float64, 0, len(traps)*4+len(xnodes)+len(ynodes))
	for _, tr := range traps {
		for _, c := range tr.corners() {
			xs = append(xs, c[0])
			ys = append(ys, c[1])
		}
	}
	for _, n := range xnodes {
		xs = append(xs, n[0])
		ys = append(ys, n[1])
	}
	for _, n := range ynodes {
		xs = append(xs, n[0])
		ys = append(ys, n[1])
	}
	if len(xs) == 0 {
		return 0, 1, 0, 1
	}
	minX, maxX = xs[0], xs[0]
	minY, maxY = ys[0], ys[0]
	for i := range xs {
		minX = min(minX, xs[i])
		maxX = max(maxX, xs[i])
		minY = min(minY, ys[i])
		maxY = max(maxY, ys[i])
	}
	padX := (maxX - minX) * 0.01
	padY := (maxY - minY) * 0.01
	if padX == 0 {
		padX = 1.0
	}
	if padY == 0 {
		padY = 1.0
	}
	return minX - padX, maxX + padX, minY - padY, maxY + padY
}

func hsvToRGB(h, s, v float64) (int, int, int) {
	i := int(h * 6)
	f := h*6 - float64(i)
	p := v * (1 - s)
	q := v * (1 - s*f)
	t := v * (1 - s*(1-f))
	var r, g, b float64
	switch i % 6 {
	case 0:
		r, g, b = v, t, p
	case 1:
		r, g, b = q, v, p
	case 2:
		r, g, b = p, v, t
	case 3:
		r, g, b = p, q, v
	case 4:
		r, g, b = t, p, v
	default:
		r, g, b = v, p, q
	}
	return int(r * 255), int(g * 255), int(b * 255)
}

func buildPalette(n int) [][3]int {
	var candidates [][3]int
	for hh := 0; hh < 36; hh++ {
		h := float64(hh) / 36.0
		for _, s := range []float64{0.55, 0.75, 1.0} {
			for _, v := range []float64{0.6, 0.8, 1.0} {
				r, g, b := hsvToRGB(h, s, v)
				candidates = append(candidates, [3]int{r, g, b})
			}
		}
	}
	palette := [][3]int{candidates[0]}
	candidates = candidates[1:]
	for len(palette) < n && len(candidates) > 0 {
		bestIdx, bestDist := 0, -1.0
		for ci, c := range candidates {
			nearest := math.MaxFloat64
			for _, p := range palette {
				d := float64(c[0]-p[0])*float64(c[0]-p[0]) +
					float64(c[1]-p[1])*float64(c[1]-p[1]) +
					float64(c[2]-p[2])*float64(c[2]-p[2])
				nearest = min(nearest, d)
			}
			if nearest > bestDist {
				bestDist = nearest
				bestIdx = ci
			}
		}
		palette = append(palette, candidates[bestIdx])
		candidates = append(candidates[:bestIdx], candidates[bestIdx+1:]...)
	}
	return palette
}

type image struct {
	w, h int
	buf  []byte
}

func newImage(w, h int) *image {
	return &image{w: w, h: h, buf: make([]byte, w*h*3)}
}

func (im *image) setPixel(x, y int, color [3]int) {
	if x < 0 || x >= im.w || y < 0 || y >= im.h {
		return
	}
	off := (y*im.w + x) * 3
	im.buf[off] = byte(color[0])
	im.buf[off+1] = byte(color[1])
	im.buf[off+2] = byte(color[2])
}

func (im *image) fill(color [3]int) {
	for off := 0; off < len(im.buf); off += 3 {
		im.buf[off] = byte(color[0])
		im.buf[off+1] = byte(color[1])
		im.buf[off+2] = byte(color[2])
	}
}

func (im *image) fillPolygon(pts [][2]float64, color [3]int) {
	if len(pts) == 0 {
		return
	}
	minY, maxY := pts[0][1], pts[0][1]
	for _, p := range pts {
		minY = min(minY, p[1])
		maxY = max(maxY, p[1])
	}
	y0 := max(0, int(math.Floor(minY)))
	y1 := min(im.h-1, int(math.Ceil(maxY)))
	for y := y0; y <= y1; y++ {
		yf := float64(y) + 0.5
		var crossings []float64
		for i := range pts {
			x1, y1p := pts[i][0], pts[i][1]
			x2, y2p := pts[(i+1)%len(pts)][0], pts[(i+1)%len(pts)][1]
			if (y1p <= yf && yf < y2p) || (y2p <= yf && yf < y1p) {
				t := (yf - y1p) / (y2p - y1p)
				crossings = append(crossings, x1+t*(x2-x1))
			}
		}
		sort.Float64s(crossings)
		for i := 0; i+1 < len(crossings); i += 2 {
			xa := max(0, int(math.Ceil(crossings[i])))
			xb := min(im.w-1, int(math.Floor(crossings[i+1])))
			for x := xa; x <= xb; x++ {
				im.setPixel(x, y, color)
			}
		}
	}
}

func (im *image) drawLine(x0, y0, x1, y1 int, color [3]int) {
	dx := abs(x1 - x0)
	dy := -abs(y1 - y0)
	sx := 1
	if x0 >= x1 {
		sx = -1
	}
	sy := 1
	if y0 >= y1 {
		sy = -1
	}
	err := dx + dy
	x, y := x0, y0
	for {
		im.setPixel(x, y, color)
		if x == x1 && y == y1 {
			return
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x += sx
		}
		if e2 <= dx {
			err += dx
			y += sy
		}
	}
}

func (im *image) drawCircle(cx, cy, r int, color [3]int) {
	for dy := -r; dy <= r; dy++ {
		for dx := -r; dx <= r; dx++ {
			if dx*dx+dy*dy <= r*r {
				im.setPixel(cx+dx, cy+dy, color)
			}
		}
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func writeBMP(path string, im *image) error {
	rowSize := im.w * 3
	pad := (4 - (rowSize % 4)) % 4
	padded := rowSize + pad
	imageSize := padded * im.h
	fileSize := 14 + 40 + imageSize

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	header := make([]byte, 14+40)
	copy(header[0:2], "BM")
	binary.LittleEndian.PutUint32(header[2:6], uint32(fileSize))
	binary.LittleEndian.PutUint32(header[10:14], 14+40)
	binary.LittleEndian.PutUint32(header[14:18], 40)
	binary.LittleEndian.PutUint32(header[18:22], uint32(im.w))
	binary.LittleEndian.PutUint32(header[22:26], uint32(im.h))
	binary.LittleEndian.PutUint16(header[26:28], 1)
	binary.LittleEndian.PutUint16(header[28:30], 24)
	binary.LittleEndian.PutUint32(header[34:38], uint32(imageSize))
	if _, err := f.Write(header); err != nil {
		return err
	}

	row := make([]byte, padded)
	for y := im.h - 1; y >= 0; y-- {
		src := im.buf[y*rowSize : (y+1)*rowSize]
		for x := 0; x < im.w; x++ {
			row[x*3] = src[x*3+2] // B
			row[x*3+1] = src[x*3+1]
			row[x*3+2] = src[x*3] // R
		}
		if _, err := f.Write(row); err != nil {
			return err
		}
	}
	return nil
}
