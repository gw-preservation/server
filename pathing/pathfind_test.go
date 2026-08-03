package pathing

import (
	"testing"

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
		_, ok := d.TrapezoidAt(wp.X, wp.Y, wp.Plane)
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
			_, ok := d.TrapezoidAt(mx, my, a.Plane)
			assert.True(t, ok, "segment waypoints %d->%d must stay on the navmesh (midpoint %v)", i, i+1, f)
		}
	}
}

func TestFindPathSameTrap(t *testing.T) {
	d := testPathData()
	assignTrapIDs(d)

	wps, ok := d.FindPath(0, 50, 0, 50, 40, 0)
	assert.True(t, ok)
	assert.Equal(t, []Waypoint{{X: 50, Y: 40, Plane: 0, TrapID: 0}}, wps)
}

func TestFindPathStraightCorridor(t *testing.T) {
	d := testPathData()
	assignTrapIDs(d)

	wps, ok := d.FindPath(0, 50, 0, 0, -250, 0)
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

	assert.False(t, d.LineOfSight(-50, 150, 50, 150, 0))

	wps, ok := d.FindPath(-50, 150, 0, 50, 150, 0)
	assert.True(t, ok)
	assertPathInvariants(t, d, wps)

	last := wps[len(wps)-1]
	assert.Equal(t, float32(50), last.X)
	assert.Equal(t, float32(150), last.Y)
}

func TestFindPathAcrossPortal(t *testing.T) {
	d := portalData()
	assignTrapIDs(d)

	wps, ok := d.FindPath(0, 50, 0, 0, 50, 1)
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

	_, ok := d.FindPath(0, 50, 0, 0, 50, 1)
	assert.False(t, ok)
}

func TestFindPathRejectsUnreachable(t *testing.T) {
	d := portalData()
	assignTrapIDs(d)

	_, ok := d.FindPath(0, 50, 0, 0, 50, 2)
	assert.False(t, ok)
}

func TestFindPathRejectsOffGridEndpoints(t *testing.T) {
	d := testPathData()
	assignTrapIDs(d)

	_, ok := d.FindPath(0, 250, 0, 0, -250, 0)
	assert.False(t, ok, "start off-grid")

	_, ok = d.FindPath(0, 50, 0, 0, 250, 0)
	assert.False(t, ok, "target off-grid")

	_, ok = d.FindPath(0, 50, 0, 0, -250, 1)
	assert.False(t, ok, "bad target plane")

	_, ok = d.FindPath(0, 50, 3, 0, -250, 0)
	assert.False(t, ok, "bad start plane")
}

func TestFindPathIslandUnreachable(t *testing.T) {
	d := testPathData()
	assignTrapIDs(d)

	_, ok := d.FindPath(0, 50, 0, 400, 50, 0)
	assert.False(t, ok)
}

func TestFindPathWalkFromIslandDown(t *testing.T) {
	d := testPathData()
	assignTrapIDs(d)

	wps, ok := d.FindPath(-50, 150, 0, 0, -250, 0)
	assert.True(t, ok)
	assertPathInvariants(t, d, wps)
	last := wps[len(wps)-1]
	assert.Equal(t, float32(0), last.X)
	assert.Equal(t, float32(-250), last.Y)
}
