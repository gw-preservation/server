package pathing

import "testing"

var benchWaypoints []Waypoint

func BenchmarkFindPathSameTrap(b *testing.B) {
	d := testPathData()
	assignTrapIDs(d)
	var wps []Waypoint
	var ok bool
	for b.Loop() {
		wps, ok = d.FindPath(0, 50, 0, 50, 40, 0)
	}
	benchWaypoints, _ = wps, ok
}

func BenchmarkFindPathStraightCorridor(b *testing.B) {
	d := testPathData()
	assignTrapIDs(d)
	var wps []Waypoint
	var ok bool
	for b.Loop() {
		wps, ok = d.FindPath(0, 50, 0, 0, -250, 0)
	}
	benchWaypoints, _ = wps, ok
}

func BenchmarkFindPathDetour(b *testing.B) {
	d := testPathData()
	assignTrapIDs(d)
	var wps []Waypoint
	var ok bool
	for b.Loop() {
		wps, ok = d.FindPath(-50, 150, 0, 50, 150, 0)
	}
	benchWaypoints, _ = wps, ok
}

func BenchmarkFindPathAcrossPortal(b *testing.B) {
	d := portalData()
	assignTrapIDs(d)
	var wps []Waypoint
	var ok bool
	for b.Loop() {
		wps, ok = d.FindPath(0, 50, 0, 0, 50, 1)
	}
	benchWaypoints, _ = wps, ok
}

func BenchmarkFindPathUnreachable(b *testing.B) {
	d := portalData()
	assignTrapIDs(d)
	var ok bool
	for b.Loop() {
		_, ok = d.FindPath(0, 50, 0, 0, 50, 2)
	}
	sinkBool = ok
}

// connectedGridData builds a cols×rows grid with both vertical and horizontal
// neighbors so that FindPath can route in any direction.
func connectedGridData(cols, rows int, cellW, cellH float32) *PathData {
	ts := make([]Trapezoid, cols*rows)
	idx := func(c, r int) int { return r*cols + c }
	for r := 0; r < rows; r++ {
		yt := float32(rows-r) * cellH
		yb := yt - cellH
		for c := 0; c < cols; c++ {
			xl := float32(c) * cellW
			xr := xl + cellW
			t := Trapezoid{YT: yt, YB: yb, XTL: xl, XTR: xr, XBL: xl, XBR: xr,
				NeighborTL: -1, NeighborTR: -1, NeighborBL: -1, NeighborBR: -1}
			if r > 0 {
				t.NeighborTL, t.NeighborTR = idx(c, r-1), idx(c, r-1)
			}
			if r < rows-1 {
				t.NeighborBL, t.NeighborBR = idx(c, r+1), idx(c, r+1)
			}
			ts[idx(c, r)] = t
		}
	}
	// Add horizontal neighbors: link each cell to the one left and right.
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			i := idx(c, r)
			if c > 0 {
				// Left neighbor shares the left edge (TL/BL).
				ts[i].NeighborTL = idx(c-1, r)
				ts[i].NeighborBL = idx(c-1, r)
			}
			if c < cols-1 {
				// Right neighbor shares the right edge (TR/BR).
				ts[i].NeighborTR = idx(c+1, r)
				ts[i].NeighborBR = idx(c+1, r)
			}
		}
	}
	pl := Plane{PlaneID: 0, Trapezoids: ts}
	buildTrapGrid(&pl)
	return &PathData{Planes: []Plane{pl}}
}

func BenchmarkFindPathLargeShort(b *testing.B) {
	d := connectedGridData(50, 50, 10, 10)
	assignTrapIDs(d)
	var wps []Waypoint
	var ok bool
	for b.Loop() {
		wps, ok = d.FindPath(5, 495, 0, 45, 455, 0)
	}
	benchWaypoints, _ = wps, ok
}

func BenchmarkFindPathLargeDiagonal(b *testing.B) {
	d := connectedGridData(15, 15, 10, 10)
	assignTrapIDs(d)
	var wps []Waypoint
	var ok bool
	for b.Loop() {
		wps, ok = d.FindPath(5, 145, 0, 145, 5, 0)
	}
	benchWaypoints, _ = wps, ok
}

func BenchmarkFindPathLargeUnreachable(b *testing.B) {
	d := connectedGridData(15, 15, 10, 10)
	assignTrapIDs(d)
	var ok bool
	for b.Loop() {
		_, ok = d.FindPath(5, 145, 0, 145, 5, 1)
	}
	sinkBool = ok
}
