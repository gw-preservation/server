package pathing

import (
	"gw1/server/geom"
	"testing"

	"github.com/stretchr/testify/assert"
)

func buildRectangularTrapezoid(yt, yb, xl, xr float32, tl, tr, bl, br int) Trapezoid {
	return Trapezoid{
		YT: yt, YB: yb, XTL: xl, XTR: xr, XBL: xl, XBR: xr,
		NeighborTL: tl, NeighborTR: tr, NeighborBL: bl, NeighborBR: br,
	}
}

func testPathData() *PathData {
	return &PathData{
		Planes: []Plane{
			{
				PlaneID: 0,
				Trapezoids: []Trapezoid{
					buildRectangularTrapezoid(100, 0, -100, 100, 5, 6, 2, -1),
					buildRectangularTrapezoid(100, 0, 300, 500, -1, -1, -1, -1),
					buildRectangularTrapezoid(0, -100, -100, 100, 0, -1, 3, -1),
					buildRectangularTrapezoid(-100, -200, -100, 100, 2, -1, 4, -1),
					buildRectangularTrapezoid(-200, -300, -100, 100, 3, -1, -1, -1),
					buildRectangularTrapezoid(200, 100, -100, -10, -1, -1, 0, -1),
					buildRectangularTrapezoid(200, 100, 10, 100, -1, -1, 0, -1),
				},
			},
		},
	}
}

func TestTrapezoidAt(t *testing.T) {
	d := testPathData()

	idx, ok := d.TrapezoidAt(geom.Pos2P{X: 0, Y: 50, Plane: 0})
	assert.True(t, ok)
	assert.Equal(t, 0, idx)

	idx, ok = d.TrapezoidAt(geom.Pos2P{X: 400, Y: 50, Plane: 0})
	assert.True(t, ok)
	assert.Equal(t, 1, idx, "island trap")

	_, ok = d.TrapezoidAt(geom.Pos2P{X: -5, Y: 150, Plane: 0})
	assert.False(t, ok, "wall gap between top islands")

	_, ok = d.TrapezoidAt(geom.Pos2P{X: 0, Y: 250, Plane: 0})
	assert.False(t, ok, "above the navmesh")

	_, ok = d.TrapezoidAt(geom.Pos2P{X: 0, Y: 50, Plane: 1})
	assert.False(t, ok, "unknown plane")
}

func TestLineOfSightSameTrap(t *testing.T) {
	d := testPathData()
	assert.True(t, d.LineOfSight(geom.Pos2P{X: -50, Y: 20, Plane: 0}, geom.Pos2P{X: 80, Y: 40, Plane: 0}), "both points in trap 0")
}

func TestLineOfSightDownTheCorridor(t *testing.T) {
	d := testPathData()
	assert.True(t, d.LineOfSight(geom.Pos2P{X: 0, Y: 50, Plane: 0}, geom.Pos2P{X: 0, Y: -250, Plane: 0}))
	assert.True(t, d.LineOfSight(geom.Pos2P{X: -90, Y: 50, Plane: 0}, geom.Pos2P{X: 90, Y: -250, Plane: 0}), "diagonal across the corridor")
}

func TestLineOfSightBlockedByWall(t *testing.T) {
	d := testPathData()
	assert.False(t, d.LineOfSight(geom.Pos2P{X: 0, Y: 50, Plane: 0}, geom.Pos2P{X: 400, Y: 50, Plane: 0}))
}

func TestLineOfSightEndpointsOffGrid(t *testing.T) {
	d := testPathData()
	assert.False(t, d.LineOfSight(geom.Pos2P{X: 0, Y: 250, Plane: 0}, geom.Pos2P{X: 0, Y: -250, Plane: 0}), "start off-grid")
	assert.False(t, d.LineOfSight(geom.Pos2P{X: 0, Y: 50, Plane: 0}, geom.Pos2P{X: 0, Y: 250, Plane: 0}), "target off-grid")
	assert.False(t, d.LineOfSight(geom.Pos2P{X: 0, Y: 50, Plane: 99}, geom.Pos2P{X: 0, Y: -250, Plane: 99}), "bad plane")
}

func TestLineOfSightTopLeftRightSplit(t *testing.T) {
	d := testPathData()
	assert.True(t, d.LineOfSight(geom.Pos2P{X: -50, Y: 50, Plane: 0}, geom.Pos2P{X: -50, Y: 150, Plane: 0}))
	assert.True(t, d.LineOfSight(geom.Pos2P{X: 50, Y: 50, Plane: 0}, geom.Pos2P{X: 50, Y: 150, Plane: 0}))
	assert.False(t, d.LineOfSight(geom.Pos2P{X: -50, Y: 150, Plane: 0}, geom.Pos2P{X: 50, Y: 150, Plane: 0}))
}

func TestLineOfSightDegenerateTrap(t *testing.T) {
	d := &PathData{Planes: []Plane{{
		PlaneID: 0,
		Trapezoids: []Trapezoid{
			buildRectangularTrapezoid(100, 50, -100, 100, -1, -1, 1, -1), // 0
			buildRectangularTrapezoid(50, 50, -100, 100, 0, -1, 2, -1),   // 1 zero-height
			buildRectangularTrapezoid(50, 0, -100, 100, 1, -1, -1, -1),   // 2
		},
	}}}
	assert.True(t, d.LineOfSight(geom.Pos2P{X: 0, Y: 75, Plane: 0}, geom.Pos2P{X: 0, Y: 25, Plane: 0}), "through the zero-height cell")
	assert.False(t, d.LineOfSight(geom.Pos2P{X: 0, Y: 75, Plane: 99}, geom.Pos2P{X: 0, Y: 25, Plane: 99}), "bad plane")
}

func portalData() *PathData {
	rect := func(yt, yb, xl, xr float32) Trapezoid {
		t := buildRectangularTrapezoid(yt, yb, xl, xr, -1, -1, -1, -1)
		t.PortalLeft, t.PortalRight = noPortal, noPortal
		return t
	}
	p0 := Plane{
		PlaneID:    0,
		Trapezoids: []Trapezoid{rect(100, 0, -100, 100)},
		Portals: []Portal{{
			PortalPlaneID: 0, NeighborPlaneID: 1, Flags: 0,
			Traps: []int{0}, PairPlane: 1, PairPortal: 0,
		}},
	}
	p0.Trapezoids[0].PortalRight = 0
	p1 := Plane{
		PlaneID:    1,
		Trapezoids: []Trapezoid{rect(100, 0, -100, 100)},
		Portals: []Portal{{
			PortalPlaneID: 1, NeighborPlaneID: 0, Flags: 0,
			Traps: []int{0}, PairPlane: 0, PairPortal: 0,
		}},
	}
	p1.Trapezoids[0].PortalLeft = 0
	return &PathData{Planes: []Plane{
		p0,
		p1,
		{PlaneID: 2, Trapezoids: []Trapezoid{rect(100, 0, -100, 100)}},
	}}
}

func TestReachableAcrossPortal(t *testing.T) {
	d := portalData()
	assert.True(t, d.Reachable(geom.Pos2P{X: 0, Y: 50, Plane: 0}, geom.Pos2P{X: 0, Y: 50, Plane: 1}), "cross-plane via portal")
	assert.True(t, d.Reachable(geom.Pos2P{X: 0, Y: 50, Plane: 1}, geom.Pos2P{X: 0, Y: 50, Plane: 0}), "and back")
	assert.True(t, d.Reachable(geom.Pos2P{X: 0, Y: 50, Plane: 0}, geom.Pos2P{X: 0, Y: 50, Plane: 0}), "same plane, same trap")
	assert.False(t, d.Reachable(geom.Pos2P{X: 0, Y: 50, Plane: 0}, geom.Pos2P{X: 0, Y: 50, Plane: 2}), "plane 2 is not connected")
}

func TestReachableBlockedPortal(t *testing.T) {
	d := portalData()
	d.Planes[0].Portals[0].Flags |= 0x4
	assert.False(t, d.Reachable(geom.Pos2P{X: 0, Y: 50, Plane: 0}, geom.Pos2P{X: 0, Y: 50, Plane: 1}), "blocked portal must not be traversable")
}

func TestReachableOffGrid(t *testing.T) {
	d := portalData()
	assert.False(t, d.Reachable(geom.Pos2P{X: 0, Y: 150, Plane: 0}, geom.Pos2P{X: 0, Y: 50, Plane: 1}), "start off-grid")
	assert.False(t, d.Reachable(geom.Pos2P{X: 0, Y: 50, Plane: 0}, geom.Pos2P{X: 0, Y: 150, Plane: 1}), "target off-grid")
	assert.False(t, d.Reachable(geom.Pos2P{X: 0, Y: 50, Plane: 3}, geom.Pos2P{X: 0, Y: 50, Plane: 1}), "bad start plane")
	assert.False(t, d.Reachable(geom.Pos2P{X: 0, Y: 50, Plane: 0}, geom.Pos2P{X: 0, Y: 50, Plane: 3}), "bad target plane")
}

func TestTrapezoidAtIndexed(t *testing.T) {
	pl := gridPlane(20, 20, 10, 10)
	buildTrapGrid(&pl)
	d := &PathData{Planes: []Plane{pl}}
	assert.NotNil(t, d.Planes[0].grid)

	for _, c := range [][2]float32{
		{5, 195},   // top-left
		{195, 5},   // bottom-right
		{105, 95},  // middle
		{0, 200},   // exact top-left corner
		{200, 0},   // exact bottom-right corner
		{10, 0},    // on bottom edge
		{-5, 100},  // off-grid left
		{205, 100}, // off-grid right
		{100, 205}, // off-grid top
		{100, -5},  // off-grid bottom
		{10, 50},   // on a cell boundary (x=10)
		{50, 10},   // on a cell boundary (y=10)
	} {
		want, wantOK := trapezoidAt(&pl, c[0], c[1])
		got, gotOK := d.TrapezoidAt(geom.Pos2P{X: c[0], Y: c[1], Plane: 0})
		assert.Equal(t, wantOK, gotOK, "point %v ok mismatch", c)
		assert.Equal(t, want, got, "point %v index mismatch", c)
	}
}
