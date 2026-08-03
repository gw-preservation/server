package pathing

import "testing"

var (
	sinkInt  int
	sinkBool bool
)

func BenchmarkTrapezoidAtHit(b *testing.B) {
	d := testPathData()

	for b.Loop() {
		sinkInt, sinkBool = d.TrapezoidAt(0, 50, 0)
	}
}

func BenchmarkTrapezoidAtMiss(b *testing.B) {
	d := testPathData()
	b.ResetTimer()
	for b.Loop() {
		sinkInt, sinkBool = d.TrapezoidAt(0, 250, 0)
	}
}

func BenchmarkLineOfSightSameTrap(b *testing.B) {
	d := testPathData()
	b.ResetTimer()
	for b.Loop() {
		sinkBool = d.LineOfSight(-50, 20, 80, 40, 0)
	}
}

func BenchmarkLineOfSightDownCorridor(b *testing.B) {
	d := testPathData()
	b.ResetTimer()
	for b.Loop() {
		sinkBool = d.LineOfSight(0, 50, 0, -250, 0)
	}
}

func BenchmarkLineOfSightBlocked(b *testing.B) {
	d := testPathData()
	b.ResetTimer()
	for b.Loop() {
		sinkBool = d.LineOfSight(0, 50, 400, 50, 0)
	}
}

func BenchmarkReachableAcrossPortal(b *testing.B) {
	d := portalData()
	b.ResetTimer()
	for b.Loop() {
		sinkBool = d.Reachable(0, 50, 0, 0, 50, 1)
	}
}

func BenchmarkReachableUnreachable(b *testing.B) {
	d := portalData()
	b.ResetTimer()
	for b.Loop() {
		sinkBool = d.Reachable(0, 50, 0, 0, 50, 2)
	}
}

// gridPlane builds a cols x rows grid, each cell linked to the one above/below.
func gridPlane(cols, rows int, cellW, cellH float32) Plane {
	ts := make([]Trapezoid, cols*rows)
	idx := func(c, r int) int { return r*cols + c }
	for r := 0; r < rows; r++ {
		yt := float32(rows-r) * cellH
		yb := yt - cellH
		for c := 0; c < cols; c++ {
			xl := float32(c) * cellW
			xr := xl + cellW
			t := Trapezoid{YT: yt, YB: yb, XTL: xl, XTR: xr, XBL: xl, XBR: xr}
			t.NeighborTL, t.NeighborTR = -1, -1
			t.NeighborBL, t.NeighborBR = -1, -1
			if r > 0 {
				t.NeighborTL, t.NeighborTR = idx(c, r-1), idx(c, r-1)
			}
			if r < rows-1 {
				t.NeighborBL, t.NeighborBR = idx(c, r+1), idx(c, r+1)
			}
			ts[idx(c, r)] = t
		}
	}
	return Plane{PlaneID: 0, Trapezoids: ts}
}

func gridData(cols, rows int, cellW, cellH float32) *PathData {
	pl := gridPlane(cols, rows, cellW, cellH)
	buildTrapGrid(&pl)
	return &PathData{Planes: []Plane{pl}}
}

func BenchmarkTrapezoidAtLarge(b *testing.B) {
	d := gridData(100, 100, 10, 10)
	b.ResetTimer()
	for b.Loop() {
		sinkInt, sinkBool = d.TrapezoidAt(995, 5, 0)
	}
}

func BenchmarkLineOfSightLarge(b *testing.B) {
	d := gridData(100, 100, 10, 10)
	b.ResetTimer()
	for b.Loop() {
		sinkBool = d.LineOfSight(5, 995, 995, 5, 0)
	}
}

func BenchmarkReachableLarge(b *testing.B) {
	d := gridData(100, 100, 10, 10)
	b.ResetTimer()
	for b.Loop() {
		sinkBool = d.Reachable(5, 995, 0, 995, 5, 0)
	}
}
