package gameservice

import (
	"gw1/server/pathing"
	"testing"

	"github.com/stretchr/testify/assert"
)

// assignTrapIDs assigns dense global TrapIDs (as the Gw.dat importer does) so
// hand-built navmeshes can be searched by FindPath.
func assignTrapIDs(d *pathing.PathData) {
	var id uint32
	for pi := range d.Planes {
		for ti := range d.Planes[pi].Trapezoids {
			d.Planes[pi].Trapezoids[ti].TrapID = id
			id++
		}
	}
	d.TrapsCount = id
}

// moveNavData: corridor t0 y[0,100] over t1 y[-100,0] (x[-100,100]), plus an
// unconnected island t2 at x[300,500].
func moveNavData() *pathing.PathData {
	rect := func(yt, yb, xl, xr float32, tl, tr, bl, br int) pathing.Trapezoid {
		return pathing.Trapezoid{
			YT: yt, YB: yb, XTL: xl, XTR: xr, XBL: xl, XBR: xr,
			NeighborTL: tl, NeighborTR: tr, NeighborBL: bl, NeighborBR: br,
		}
	}
	d := &pathing.PathData{Planes: []pathing.Plane{{
		PlaneID: 0,
		Trapezoids: []pathing.Trapezoid{
			rect(100, 0, -100, 100, -1, -1, 1, -1),
			rect(0, -100, -100, 100, 0, -1, -1, -1),
			rect(100, 0, 300, 500, -1, -1, -1, -1),
		},
	}}}
	assignTrapIDs(d)
	return d
}

func withMoveNav(t *testing.T, fn func(inst *Instance)) {
	t.Helper()
	old := instancePathStore
	defer func() { instancePathStore = old }()

	store := pathing.NewStore()
	store.Set(0x340c6, moveNavData())
	instancePathStore = store

	fn(newHeadlessInstance(t, 3))
}

func TestStartPlayerMoveValidMove(t *testing.T) {
	withMoveNav(t, func(inst *Instance) {
		bot, botSink := newTestPlayer("Bot")
		watcher, watcherSink := newTestPlayer("Watcher")
		inst.AddPlayer(bot)
		inst.AddPlayer(watcher)
		bot.posX, bot.posY, bot.plane = 0, 50, 0
		bot.baseSpeed = 288
		botSink.reset()
		watcherSink.reset()

		assert.True(t, inst.startPlayerMove(bot, 50, 50, 0))
		assert.Equal(t, float32(50), bot.destX)
		assert.Equal(t, float32(50), bot.destY)
		assert.Equal(t, 0, bot.destPlane)
		assert.NotEmpty(t, bot.waypoints)
		assert.True(t, botSink.hasOpcode(0x29), "expected MarshalMoveToPointS2C in mover sink")
		assert.True(t, watcherSink.hasOpcode(0x29), "expected MarshalMoveToPointS2C in watcher sink")
	})
}

func TestStartPlayerMoveValidMoveDownCorridor(t *testing.T) {
	withMoveNav(t, func(inst *Instance) {
		bot, _ := newTestPlayer("Bot")
		inst.AddPlayer(bot)
		bot.posX, bot.posY, bot.plane = 0, 50, 0
		bot.baseSpeed = 288

		assert.True(t, inst.startPlayerMove(bot, 0, -50, 0))
		assert.NotEmpty(t, bot.waypoints)
		assert.Equal(t, float32(-50), bot.destY)
	})
}

func TestStartPlayerMoveRejectsWallMove(t *testing.T) {
	withMoveNav(t, func(inst *Instance) {
		bot, _ := newTestPlayer("Bot")
		watcher, watcherSink := newTestPlayer("Watcher")
		inst.AddPlayer(bot)
		inst.AddPlayer(watcher)
		bot.posX, bot.posY, bot.plane = 0, 50, 0
		bot.baseSpeed = 288
		watcherSink.reset()

		assert.False(t, inst.startPlayerMove(bot, 400, 50, 0))
		assert.Empty(t, bot.waypoints)
		assert.Equal(t, float32(0), bot.posX)
		assert.Equal(t, float32(50), bot.posY)
		assert.True(t, watcherSink.hasOpcode(0x2c), "reject should rebroadcast current position")
	})
}

func TestStartPlayerMoveAcceptsDetourMove(t *testing.T) {
	withObstacleNav(t, func(inst *Instance) {
		bot, _ := newTestPlayer("Bot")
		inst.AddPlayer(bot)
		bot.posX, bot.posY, bot.plane = -80, 50, 0
		bot.baseSpeed = 288

		assert.False(t, inst.path.LineOfSight(-80, 50, 80, 50, 0), "straight line must cross the wall")
		assert.True(t, inst.startPlayerMove(bot, 80, 50, 0))
		assert.Greater(t, len(bot.waypoints), 1, "detour should produce multiple waypoints")
	})
}

// obstacleNavData: t0 and t2 side-by-side with a gap, joined below by t1 (the detour).
func obstacleNavData() *pathing.PathData {
	rect := func(yt, yb, xl, xr float32, tl, tr, bl, br int) pathing.Trapezoid {
		return pathing.Trapezoid{
			YT: yt, YB: yb, XTL: xl, XTR: xr, XBL: xl, XBR: xr,
			NeighborTL: tl, NeighborTR: tr, NeighborBL: bl, NeighborBR: br,
		}
	}
	d := &pathing.PathData{Planes: []pathing.Plane{{
		PlaneID: 0,
		Trapezoids: []pathing.Trapezoid{
			rect(100, 0, -100, -20, -1, -1, 1, -1),
			rect(0, -100, -100, 100, 0, -1, 2, -1),
			rect(100, 0, 20, 100, -1, -1, 1, -1),
		},
	}}}
	assignTrapIDs(d)
	return d
}

func withObstacleNav(t *testing.T, fn func(inst *Instance)) {
	t.Helper()
	old := instancePathStore
	defer func() { instancePathStore = old }()

	store := pathing.NewStore()
	store.Set(0x340c6, obstacleNavData())
	instancePathStore = store

	fn(newHeadlessInstance(t, 3))
}

func TestStartPlayerMoveRejectsOffGridTarget(t *testing.T) {
	withMoveNav(t, func(inst *Instance) {
		bot, _ := newTestPlayer("Bot")
		inst.AddPlayer(bot)
		bot.posX, bot.posY, bot.plane = 0, 50, 0
		bot.baseSpeed = 288

		assert.False(t, inst.startPlayerMove(bot, 0, 150, 0))
		assert.Empty(t, bot.waypoints)
	})
}

func TestStartPlayerMoveAllowsOffGridStart(t *testing.T) {
	withMoveNav(t, func(inst *Instance) {
		bot, _ := newTestPlayer("Bot")
		inst.AddPlayer(bot)
		// Off the navmesh (spawn): moves allowed so spawns don't lock players in.
		bot.posX, bot.posY, bot.plane = 0, 150, 0
		bot.baseSpeed = 288

		assert.True(t, inst.startPlayerMove(bot, 50, 50, 0))
		assert.Equal(t, 1, len(bot.waypoints))
		assert.Equal(t, float32(50), bot.destX)
		assert.Equal(t, float32(50), bot.destY)
	})
}

// portalNavData: planes 0 and 1 linked by a portal pair; plane 2 isolated.
func portalNavData() *pathing.PathData {
	rect := func(yt, yb, xl, xr float32, tl, tr, bl, br int) pathing.Trapezoid {
		return pathing.Trapezoid{
			YT: yt, YB: yb, XTL: xl, XTR: xr, XBL: xl, XBR: xr,
			NeighborTL: tl, NeighborTR: tr, NeighborBL: bl, NeighborBR: br,
			PortalLeft: 0xFFFF, PortalRight: 0xFFFF,
		}
	}
	plane0 := pathing.Plane{
		PlaneID:    0,
		Trapezoids: []pathing.Trapezoid{rect(100, 0, -100, 100, -1, -1, -1, -1)},
		Portals: []pathing.Portal{{
			PortalPlaneID: 0, NeighborPlaneID: 1, Flags: 0,
			Traps: []int{0}, PairPlane: 1, PairPortal: 0,
		}},
	}
	plane0.Trapezoids[0].PortalRight = 0
	plane1 := pathing.Plane{
		PlaneID:    1,
		Trapezoids: []pathing.Trapezoid{rect(100, 0, -100, 100, -1, -1, -1, -1)},
		Portals: []pathing.Portal{{
			PortalPlaneID: 1, NeighborPlaneID: 0, Flags: 0,
			Traps: []int{0}, PairPlane: 0, PairPortal: 0,
		}},
	}
	plane1.Trapezoids[0].PortalLeft = 0
	d := &pathing.PathData{Planes: []pathing.Plane{
		plane0,
		plane1,
		{
			PlaneID:    2,
			Trapezoids: []pathing.Trapezoid{rect(100, 0, -100, 100, -1, -1, -1, -1)},
			Portals:    []pathing.Portal{},
		},
	}}
	assignTrapIDs(d)
	return d
}

func withPortalNav(t *testing.T, fn func(inst *Instance)) {
	t.Helper()
	old := instancePathStore
	defer func() { instancePathStore = old }()

	store := pathing.NewStore()
	store.Set(0x340c6, portalNavData())
	instancePathStore = store

	fn(newHeadlessInstance(t, 3))
}

func TestStartPlayerMoveAllowsPortalTraversal(t *testing.T) {
	withPortalNav(t, func(inst *Instance) {
		bot, botSink := newTestPlayer("Bot")
		watcher, watcherSink := newTestPlayer("Watcher")
		inst.AddPlayer(bot)
		inst.AddPlayer(watcher)
		bot.posX, bot.posY, bot.plane = 0, 50, 0
		bot.baseSpeed = 288
		botSink.reset()
		watcherSink.reset()

		assert.True(t, inst.startPlayerMove(bot, 0, 50, 1))
		assert.NotEmpty(t, bot.waypoints)
		assert.Equal(t, 1, bot.waypoints[len(bot.waypoints)-1].Plane, "final waypoint on the destination plane")
		assert.True(t, watcherSink.hasOpcode(0x29), "expected MarshalMoveToPointS2C in watcher sink")
		assert.True(t, botSink.hasOpcode(0x29), "expected MarshalMoveToPointS2C in moving player sink")
	})
}

func TestStartPlayerMoveRejectsUnreachablePlane(t *testing.T) {
	withPortalNav(t, func(inst *Instance) {
		bot, _ := newTestPlayer("Bot")
		inst.AddPlayer(bot)
		bot.posX, bot.posY, bot.plane = 0, 50, 0
		bot.baseSpeed = 288

		assert.False(t, inst.startPlayerMove(bot, 0, 50, 2))
		assert.Empty(t, bot.waypoints)
		assert.Equal(t, float32(0), bot.posX)
		assert.Equal(t, float32(50), bot.posY)
		assert.Equal(t, 0, bot.plane)
	})
}
