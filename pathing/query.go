package pathing

import "gw1/server/geom"

import "math"

// noPortal = 0xFFFF in the dat.
const noPortal = 0xFFFF

const trapTol = 1.0

const entryTol = 1e-2

const (
	trapEdgeTop = iota
	trapEdgeRight
	trapEdgeBottom
	trapEdgeLeft
)

func (d *PathData) TrapezoidAt(p geom.Pos2P) (int, bool) {
	if p.Plane < 0 || p.Plane >= len(d.Planes) {
		return -1, false
	}
	return trapezoidAt(&d.Planes[p.Plane], p.X, p.Y)
}

func trapezoidAt(pl *Plane, x, y float32) (int, bool) {
	if g := pl.grid; g != nil {
		c, r, ok := g.bucket(x, y)
		if !ok {
			return -1, false
		}
		for _, i := range g.buckets[r*g.w+c] {
			if pointInTrapezoid(&pl.Trapezoids[i], x, y) {
				return i, true
			}
		}
		return -1, false
	}
	for i := range pl.Trapezoids {
		if pointInTrapezoid(&pl.Trapezoids[i], x, y) {
			return i, true
		}
	}
	return -1, false
}

// trapGrid buckets each trapezoid by bbox so a point lookup scans only nearby cells.
type trapGrid struct {
	w                        int
	minX, minY, spanX, spanY float32
	buckets                  [][]int
}

func (g *trapGrid) bucket(x, y float32) (c, r int, ok bool) {
	if x < g.minX || x > g.minX+g.spanX || y < g.minY || y > g.minY+g.spanY {
		return 0, 0, false
	}
	c = int((x - g.minX) / g.spanX * float32(g.w))
	r = int((y - g.minY) / g.spanY * float32(g.w))
	if c < 0 {
		c = 0
	}
	if r < 0 {
		r = 0
	}
	if c >= g.w {
		c = g.w - 1
	}
	if r >= g.w {
		r = g.w - 1
	}
	return c, r, true
}

func buildTrapGrid(pl *Plane) {
	n := len(pl.Trapezoids)
	if n < 16 {
		return
	}
	minX, minY := float32(math.Inf(1)), float32(math.Inf(1))
	maxX, maxY := float32(math.Inf(-1)), float32(math.Inf(-1))
	for i := range pl.Trapezoids {
		t := &pl.Trapezoids[i]
		if t.YT < t.YB || !finite32(t.YT) || !finite32(t.YB) || !finite32(t.XTL) || !finite32(t.XTR) || !finite32(t.XBL) || !finite32(t.XBR) {
			return
		}
		minX = minf(minX, minf(t.XTL, t.XBL))
		maxX = maxf(maxX, maxf(t.XTR, t.XBR))
		minY = minf(minY, t.YB)
		maxY = maxf(maxY, t.YT)
	}
	minX -= trapTol
	minY -= trapTol
	maxX += trapTol
	maxY += trapTol
	if maxX <= minX || maxY <= minY {
		return
	}

	dim := int(math.Sqrt(float64(n)))
	if dim < 8 {
		dim = 8
	}
	if dim > 128 {
		dim = 128
	}
	g := &trapGrid{w: dim, minX: minX, minY: minY, spanX: maxX - minX, spanY: maxY - minY}
	g.buckets = make([][]int, dim*dim)
	bucketIdx := func(coord, lo, span float32) int {
		c := int((coord - lo) / span * float32(dim))
		if c < 0 {
			c = 0
		}
		if c >= dim {
			c = dim - 1
		}
		return c
	}
	for i := range pl.Trapezoids {
		t := &pl.Trapezoids[i]
		c0 := bucketIdx(minf(t.XTL, t.XBL), minX, g.spanX)
		c1 := bucketIdx(maxf(t.XTR, t.XBR), minX, g.spanX)
		r0 := bucketIdx(t.YB, minY, g.spanY)
		r1 := bucketIdx(t.YT, minY, g.spanY)
		for r := r0; r <= r1; r++ {
			for c := c0; c <= c1; c++ {
				g.buckets[r*dim+c] = append(g.buckets[r*dim+c], i)
			}
		}
	}
	pl.grid = g
}

func finite32(f float32) bool {
	return !math.IsInf(float64(f), 0) && !math.IsNaN(float64(f))
}

// LineOfSight reports whether the segment stays in the walkable trapezoids; both endpoints must be on the navmesh.
func (d *PathData) LineOfSight(from, to geom.Pos2P) bool {
	if from.Plane < 0 || from.Plane >= len(d.Planes) {
		return false
	}
	pl := &d.Planes[from.Plane]
	if len(pl.Trapezoids) == 0 {
		return false
	}
	start, ok := trapezoidAt(pl, from.X, from.Y)
	if !ok {
		return false
	}
	end, ok := trapezoidAt(pl, to.X, to.Y)
	if !ok {
		return false
	}
	if start == end {
		return true
	}

	// The walk always advances, so it cannot cycle; enterX/enterY and prev stop re-crossing the entry edge.
	enterX, enterY := from.X, from.Y
	prev := -1
	for step := 0; step <= len(pl.Trapezoids); step++ {
		ex, ey, e1, e2, n, ok := trapExit(&pl.Trapezoids[start], from.X, from.Y, to.X, to.Y, enterX, enterY)
		if !ok {
			return start == end
		}
		for k := range n {
			e := e1
			if k == 1 {
				e = e2
			}
			next := neighborAcross(pl, start, e, ex, ey)
			if next < 0 || next == start || next == prev {
				continue
			}
			if next == end {
				return true
			}
			enterX, enterY = ex, ey
			prev, start = start, next
			goto advanced
		}
		return false
	advanced:
	}
	return false
}

// Reachable reports whether (x1,y1,fromPlane) reaches (x2,y2,toPlane) via the walkable graph (neighbors plus unblocked portals).
func (d *PathData) Reachable(from, to geom.Pos2P) bool {
	if from.Plane < 0 || from.Plane >= len(d.Planes) || to.Plane < 0 || to.Plane >= len(d.Planes) {
		return false
	}
	startPlane := &d.Planes[from.Plane]
	start, ok := trapezoidAt(startPlane, from.X, from.Y)
	if !ok {
		return false
	}
	endPlane := &d.Planes[to.Plane]
	end, ok := trapezoidAt(endPlane, to.X, to.Y)
	if !ok {
		return false
	}
	if from.Plane == to.Plane && start == end {
		return true
	}

	type cell struct {
		plane, trap int
	}
	visited := make(map[cell]bool)
	visited[cell{from.Plane, start}] = true
	queue := []cell{{from.Plane, start}}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur == (cell{to.Plane, end}) {
			return true
		}

		pl := &d.Planes[cur.plane]
		t := &pl.Trapezoids[cur.trap]

		for _, n := range [...]int{t.NeighborTL, t.NeighborTR, t.NeighborBL, t.NeighborBR} {
			if n < 0 || n >= len(pl.Trapezoids) {
				continue
			}
			c := cell{cur.plane, n}
			if !visited[c] {
				visited[c] = true
				queue = append(queue, c)
			}
		}

		for _, portalID := range [...]uint16{t.PortalLeft, t.PortalRight} {
			if portalID == noPortal || int(portalID) >= len(pl.Portals) {
				continue
			}
			portal := &pl.Portals[portalID]
			// 0x4 marks a blocked portal.
			if portal.Flags&0x4 != 0 || portal.PairPlane < 0 || portal.PairPortal < 0 {
				continue
			}
			pairPlane := portal.PairPlane
			if pairPlane >= len(d.Planes) {
				continue
			}
			pair := &d.Planes[pairPlane]
			if portal.PairPortal >= len(pair.Portals) {
				continue
			}
			twin := &pair.Portals[portal.PairPortal]
			for _, n := range twin.Traps {
				if n < 0 || n >= len(pair.Trapezoids) {
					continue
				}
				c := cell{pairPlane, n}
				if !visited[c] {
					visited[c] = true
					queue = append(queue, c)
				}
			}
		}
	}
	return false
}

func neighborAcross(pl *Plane, start, edge int, ex, ey float32) int {
	t := &pl.Trapezoids[start]
	switch edge {
	case trapEdgeTop:
		if n := t.NeighborTL; n >= 0 && n < len(pl.Trapezoids) {
			nb := &pl.Trapezoids[n]
			if ex >= nb.XBL-trapTol && ex <= nb.XBR+trapTol {
				return n
			}
		}
		if n := t.NeighborTR; n >= 0 && n < len(pl.Trapezoids) {
			nb := &pl.Trapezoids[n]
			if ex >= nb.XBL-trapTol && ex <= nb.XBR+trapTol {
				return n
			}
		}
		return -1
	case trapEdgeBottom:
		if n := t.NeighborBL; n >= 0 && n < len(pl.Trapezoids) {
			nb := &pl.Trapezoids[n]
			if ex >= nb.XTL-trapTol && ex <= nb.XTR+trapTol {
				return n
			}
		}
		if n := t.NeighborBR; n >= 0 && n < len(pl.Trapezoids) {
			nb := &pl.Trapezoids[n]
			if ex >= nb.XTL-trapTol && ex <= nb.XTR+trapTol {
				return n
			}
		}
		return -1
	default:
		return -1
	}
}

// trapExit returns where the segment leaves the trapezoid; entry-edge crossings are skipped.
func trapExit(t *Trapezoid, x1, y1, x2, y2, enterX, enterY float32) (ex, ey float32, e1, e2 int, n int, ok bool) {
	if t.YT == t.YB {
		if y1 == y2 {
			return 0, 0, 0, 0, 0, false
		}
		tt := (t.YT - y1) / (y2 - y1)
		if tt <= 0 || tt > 1 {
			return 0, 0, 0, 0, 0, false
		}
		ex = x1 + (x2-x1)*tt
		ey = t.YT
		if ex < t.XBL-trapTol || ex > t.XBR+trapTol {
			return 0, 0, 0, 0, 0, false
		}
		return ex, ey, trapEdgeTop, trapEdgeBottom, 2, true
	}

	cx := [4][2]float32{
		{t.XTL, t.YT},
		{t.XTR, t.YT},
		{t.XBR, t.YB},
		{t.XBL, t.YB},
	}
	var hits [4]struct {
		edge int
		tt   float32
		px   float32
		py   float32
	}
	nhits := 0
	for i := range 4 {
		if tt, hit := segCrossT(x1, y1, x2, y2, cx[i][0], cx[i][1], cx[(i+1)%4][0], cx[(i+1)%4][1]); hit && tt > 0 && tt <= 1 {
			hits[nhits].edge = i
			hits[nhits].tt = tt
			hits[nhits].px = x1 + (x2-x1)*tt
			hits[nhits].py = y1 + (y2-y1)*tt
			nhits++
		}
	}
	if nhits == 0 {
		return 0, 0, 0, 0, 0, false
	}

	closest := 0
	for i := 1; i < nhits; i++ {
		if hits[i].tt < hits[closest].tt {
			closest = i
		}
	}
	exitT := hits[closest].tt
	if nearPoint(hits[closest].px, hits[closest].py, enterX, enterY) && nhits > 1 {
		next := float32(math.Inf(1))
		for i := 0; i < nhits; i++ {
			if hits[i].tt > hits[closest].tt+1e-4 && hits[i].tt < next {
				next = hits[i].tt
			}
		}
		if !math.IsInf(float64(next), 1) {
			exitT = next
		}
	}

	ex = x1 + (x2-x1)*exitT
	ey = y1 + (y2-y1)*exitT
	for i := 0; i < nhits; i++ {
		if math.Abs(float64(hits[i].tt-exitT)) <= 1e-4 {
			if n == 0 {
				e1 = hits[i].edge
			} else {
				e2 = hits[i].edge
			}
			n++
		}
	}
	if n == 0 {
		return 0, 0, 0, 0, 0, false
	}
	return ex, ey, e1, e2, n, true
}

func nearPoint(ax, ay, bx, by float32) bool {
	return math.Abs(float64(ax-bx)) <= entryTol && math.Abs(float64(ay-by)) <= entryTol
}

func segCrossT(ax, ay, bx, by, p1x, p1y, p2x, p2y float32) (t float32, ok bool) {
	dx, dy := bx-ax, by-ay
	ex, ey := p2x-p1x, p2y-p1y
	denom := dx*ey - dy*ex
	if denom == 0 {
		return 0, false
	}
	t = ((p1x-ax)*ey - (p1y-ay)*ex) / denom
	u := ((p1x-ax)*dy - (p1y-ay)*dx) / denom
	if t <= 0 || t > 1 || u < 0 || u > 1 {
		return 0, false
	}
	return t, true
}

func pointInTrapezoid(t *Trapezoid, x, y float32) bool {
	if y < t.YB-trapTol || y > t.YT+trapTol {
		return false
	}
	xl, xr := trapezoidXSpan(t, y)
	return x >= xl-trapTol && x <= xr+trapTol
}

func trapezoidXSpan(t *Trapezoid, y float32) (float32, float32) {
	if t.YT == t.YB {
		return minf(t.XTL, t.XBL), maxf(t.XTR, t.XBR)
	}
	f := (t.YT - y) / (t.YT - t.YB) // 0 at the top edge, 1 at the bottom
	return t.XBL*f + t.XTL*(1-f), t.XBR*f + t.XTR*(1-f)
}

func minf(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func maxf(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}
