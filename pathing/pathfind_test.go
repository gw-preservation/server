package pathing

import (
	"testing"

	"gw1/server/geom"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func assignTrapIDs(d *PathData) {
	var id uint32
	for pi := range d.Planes {
		for ti := range d.Planes[pi].Trapezoids {
			d.Planes[pi].Trapezoids[ti].TrapID = id
			id++
		}
	}
	d.TrapsCount = id
}

// assertPathInvariants checks that every waypoint, and the midpoints between
// consecutive same-plane waypoints, stay on the navmesh (midpoints are sampled
// because LineOfSight treats a boundary-start walk as leaving the trap).
func assertPathInvariants(t *testing.T, d *PathData, wps []Waypoint) {
	t.Helper()
	require.NotEmpty(t, wps)
	for _, wp := range wps {
		_, ok := d.TrapezoidAt(wp.Pos2P)
		assert.True(t, ok, "waypoint (%v, %v) on plane %d must be on the navmesh", wp.X, wp.Y, wp.Plane)
	}
	for i := 0; i+1 < len(wps); i++ {
		a, b := wps[i], wps[i+1]
		if a.Plane != b.Plane {
			continue
		}
		for _, f := range []float32{0.25, 0.5, 0.75} {
			mx := a.X + (b.X-a.X)*f
			my := a.Y + (b.Y-a.Y)*f
			_, ok := d.TrapezoidAt(geom.Pos2P{X: mx, Y: my, Plane: a.Plane})
			assert.True(t, ok, "segment waypoints %d->%d must stay on the navmesh (midpoint %v)", i, i+1, f)
		}
	}
}

func TestFindPathSameTrap(t *testing.T) {
	d := testPathData()
	assignTrapIDs(d)

	wps, ok := d.FindPath(geom.Pos2P{X: 0, Y: 50, Plane: 0}, geom.Pos2P{X: 50, Y: 40, Plane: 0})
	assert.True(t, ok)
	assert.Equal(t, []Waypoint{{Pos2P: geom.Pos2P{X: 50, Y: 40, Plane: 0}, TrapID: 0}}, wps)
}

func TestFindPathStraightCorridor(t *testing.T) {
	d := testPathData()
	assignTrapIDs(d)

	wps, ok := d.FindPath(geom.Pos2P{X: 0, Y: 50, Plane: 0}, geom.Pos2P{X: 0, Y: -250, Plane: 0})
	assert.True(t, ok)
	assertPathInvariants(t, d, wps)

	last := wps[len(wps)-1]
	assert.Equal(t, float32(0), last.X)
	assert.Equal(t, float32(-250), last.Y)
	assert.Equal(t, 0, last.Plane)
}

func TestFindPathDetoursAroundWall(t *testing.T) {
	d := testPathData()
	assignTrapIDs(d)

	assert.False(t, d.LineOfSight(geom.Pos2P{X: -50, Y: 150}, geom.Pos2P{X: 50, Y: 150}))

	wps, ok := d.FindPath(geom.Pos2P{X: -50, Y: 150, Plane: 0}, geom.Pos2P{X: 50, Y: 150, Plane: 0})
	assert.True(t, ok)
	assertPathInvariants(t, d, wps)

	last := wps[len(wps)-1]
	assert.Equal(t, float32(50), last.X)
	assert.Equal(t, float32(150), last.Y)
}

func TestFindPathAcrossPortal(t *testing.T) {
	d := portalData()
	assignTrapIDs(d)

	wps, ok := d.FindPath(geom.Pos2P{X: 0, Y: 50, Plane: 0}, geom.Pos2P{X: 0, Y: 50, Plane: 1})
	assert.True(t, ok)
	assertPathInvariants(t, d, wps)

	last := wps[len(wps)-1]
	assert.Equal(t, 1, last.Plane)
	assert.Equal(t, float32(0), last.X)
	assert.Equal(t, float32(50), last.Y)
}

func TestFindPathRejectsBlockedPortal(t *testing.T) {
	d := portalData()
	assignTrapIDs(d)
	d.Planes[0].Portals[0].Flags |= 0x4

	_, ok := d.FindPath(geom.Pos2P{X: 0, Y: 50, Plane: 0}, geom.Pos2P{X: 0, Y: 50, Plane: 1})
	assert.False(t, ok)
}

func TestFindPathRejectsUnreachable(t *testing.T) {
	d := portalData()
	assignTrapIDs(d)

	_, ok := d.FindPath(geom.Pos2P{X: 0, Y: 50, Plane: 0}, geom.Pos2P{X: 0, Y: 50, Plane: 2})
	assert.False(t, ok)
}

func TestFindPathRejectsOffGridEndpoints(t *testing.T) {
	d := testPathData()
	assignTrapIDs(d)

	_, ok := d.FindPath(geom.Pos2P{X: 0, Y: 250, Plane: 0}, geom.Pos2P{X: 0, Y: -250, Plane: 0})
	assert.False(t, ok, "start off-grid")

	_, ok = d.FindPath(geom.Pos2P{X: 0, Y: 50, Plane: 0}, geom.Pos2P{X: 0, Y: 250, Plane: 0})
	assert.False(t, ok, "target off-grid")

	_, ok = d.FindPath(geom.Pos2P{X: 0, Y: 50, Plane: 0}, geom.Pos2P{X: 0, Y: -250, Plane: 1})
	assert.False(t, ok, "bad target plane")

	_, ok = d.FindPath(geom.Pos2P{X: 0, Y: 50, Plane: 3}, geom.Pos2P{X: 0, Y: -250, Plane: 0})
	assert.False(t, ok, "bad start plane")
}

func TestFindPathIslandUnreachable(t *testing.T) {
	d := testPathData()
	assignTrapIDs(d)

	_, ok := d.FindPath(geom.Pos2P{X: 0, Y: 50, Plane: 0}, geom.Pos2P{X: 400, Y: 50, Plane: 0})
	assert.False(t, ok)
}

func TestFindPathWalkFromIslandDown(t *testing.T) {
	d := testPathData()
	assignTrapIDs(d)

	wps, ok := d.FindPath(geom.Pos2P{X: -50, Y: 150, Plane: 0}, geom.Pos2P{X: 0, Y: -250, Plane: 0})
	assert.True(t, ok)
	assertPathInvariants(t, d, wps)
	last := wps[len(wps)-1]
	assert.Equal(t, float32(0), last.X)
	assert.Equal(t, float32(-250), last.Y)
}
