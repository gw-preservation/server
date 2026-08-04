package gameservice

import (
	"testing"
	"time"

	"gw1/server/packet"
	"gw1/server/pathing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sinkPacket returns the first recorded packet with the given opcode, with the
// opcode header already consumed, so fields can be decoded directly.
func sinkPacket(sink *headlessSink, op int) (packet.In, bool) {
	for _, raw := range sink.packetsSent() {
		if len(raw) >= 2 && int(raw[0])|(int(raw[1])<<8) == op {
			in := packet.NewIn(raw)
			in.Uint16()
			return in, true
		}
	}
	return packet.In{}, false
}

func TestSimulateMovementAdvancesAndArrives(t *testing.T) {
	withMoveNav(t, func(inst *Instance) {
		bot, _ := newTestPlayer("Bot")
		inst.AddPlayer(bot)
		bot.posX, bot.posY, bot.plane = 0, 50, 0
		bot.baseSpeed = 288

		assert.True(t, inst.startPlayerMove(bot, 0, -50, 0))
		assert.NotEmpty(t, bot.waypoints)

		// Advance a full 500ms sim step (288 u/s * 0.5s = 144 units).
		inst.lastMovementAdvanceAt = time.Now().Add(-500 * time.Millisecond)
		inst.flushMovement(time.Now())

		assert.Equal(t, float32(0), bot.posX)
		assert.Equal(t, float32(-50), bot.posY)
		assert.Empty(t, bot.waypoints, "movement must end on arrival")
		assert.Equal(t, 0, bot.waypointIdx)
	})
}

func TestSimulateMovementPartialStep(t *testing.T) {
	withMoveNav(t, func(inst *Instance) {
		bot, _ := newTestPlayer("Bot")
		inst.AddPlayer(bot)
		bot.posX, bot.posY, bot.plane = 0, 50, 0
		bot.baseSpeed = 288

		inst.startPlayerMove(bot, 0, -50, 0)

		inst.lastMovementAdvanceAt = time.Now().Add(-100 * time.Millisecond)
		inst.flushMovement(time.Now())

		assert.Equal(t, float32(0), bot.posX)
		assert.InDelta(t, float32(50-28.8), bot.posY, 0.01)
		assert.NotEmpty(t, bot.waypoints, "still moving")
	})
}

func TestTickMovementRespectsCadence(t *testing.T) {
	withMoveNav(t, func(inst *Instance) {
		bot, botSink := newTestPlayer("Bot")
		inst.AddPlayer(bot)
		bot.posX, bot.posY, bot.plane = 0, 50, 0
		bot.baseSpeed = 288

		inst.startPlayerMove(bot, 0, -50, 0)
		botSink.reset()

		inst.lastMovementAdvanceAt = time.Now().Add(-250 * time.Millisecond)
		inst.tickMovement()
		assert.False(t, botSink.hasOpcode(0x1e), "no tick below cadence")
		assert.Equal(t, float32(50), bot.posY)

		inst.lastMovementAdvanceAt = time.Now().Add(-500 * time.Millisecond)
		inst.tickMovement()
		assert.True(t, botSink.hasOpcode(0x1e), "expected MarshalInstanceMovementTick")
		assert.Equal(t, float32(-50), bot.posY, "player should have arrived")
		assert.Empty(t, bot.waypoints)
	})
}

func TestStartPlayerMoveFlushesPriorMovement(t *testing.T) {
	withMoveNav(t, func(inst *Instance) {
		bot, _ := newTestPlayer("Bot")
		inst.AddPlayer(bot)
		bot.posX, bot.posY, bot.plane = 0, 50, 0
		bot.baseSpeed = 288

		inst.startPlayerMove(bot, 0, -50, 0)

		inst.lastMovementAdvanceAt = time.Now().Add(-250 * time.Millisecond)
		assert.True(t, inst.startPlayerMove(bot, 50, 50, 0))

		assert.InDelta(t, float32(50-72), bot.posY, 0.01, "prior leg advanced 72 units before retarget")
		assert.Equal(t, float32(50), bot.destX)
		assert.Equal(t, float32(50), bot.destY)
	})
}

func TestApplyLastPosCorrection(t *testing.T) {
	withMoveNav(t, func(inst *Instance) {
		bot, botSink := newTestPlayer("Bot")
		watcher, watcherSink := newTestPlayer("Watcher")
		inst.AddPlayer(bot)
		inst.AddPlayer(watcher)
		bot.posX, bot.posY, bot.plane = 0, 50, 0
		bot.baseSpeed = 288

		inst.startPlayerMove(bot, 0, -50, 0)
		botSink.reset()
		watcherSink.reset()

		assert.True(t, inst.applyLastPosCorrection(bot, 5, 45, 0))
		assert.Equal(t, float32(5), bot.posX)
		assert.Equal(t, float32(45), bot.posY)
		assert.Empty(t, bot.waypoints, "movement must be cancelled")
		assert.True(t, watcherSink.hasOpcode(0x2c), "expected position rebroadcast")
	})
}

func TestApplyLastPosCorrectionRejectsLargeJump(t *testing.T) {
	withMoveNav(t, func(inst *Instance) {
		bot, _ := newTestPlayer("Bot")
		inst.AddPlayer(bot)
		bot.posX, bot.posY, bot.plane = 0, 50, 0
		bot.baseSpeed = 288

		assert.False(t, inst.applyLastPosCorrection(bot, 500, 500, 0))
		assert.Equal(t, float32(0), bot.posX)
		assert.Equal(t, float32(50), bot.posY)

		assert.False(t, inst.applyLastPosCorrection(bot, 0, 50, 1))
	})
}

func TestApplyMovementFacing(t *testing.T) {
	bot, _ := newTestPlayer("Bot")

	bot.applyMovementFacing(3, 4)
	assert.InDelta(t, float32(0.6), bot.facingX, 1e-6)
	assert.InDelta(t, float32(0.8), bot.facingY, 1e-6)

	bot.applyMovementFacing(0, 0)
	assert.InDelta(t, float32(0.6), bot.facingX, 1e-6)
}

func TestApplyDirMovementStartsDirectionalMovement(t *testing.T) {
	withMoveNav(t, func(inst *Instance) {
		bot, botSink := newTestPlayer("Bot")
		watcher, watcherSink := newTestPlayer("Watcher")
		inst.AddPlayer(bot)
		inst.AddPlayer(watcher)
		bot.posX, bot.posY, bot.plane = 0, 50, 0
		bot.baseSpeed = 288
		botSink.reset()
		watcherSink.reset()

		inst.applyDirMovement(bot, 0, 50, 0, -1, 1)

		assert.True(t, bot.dirMove)
		assert.Empty(t, bot.waypoints)
		assert.Equal(t, float32(0), bot.facingX)
		assert.Equal(t, float32(-1), bot.facingY)
		assert.Equal(t, 1, bot.curMoveType)
		assert.Equal(t, float32(0), bot.destX)
		assert.Equal(t, float32(50-5000), bot.destY)
		assert.True(t, watcherSink.hasOpcode(0x29), "expected MarshalMoveToPointS2C in watcher sink")
		assert.True(t, watcherSink.hasOpcode(0x25), "expected MarshalAgentUpdateDirection in watcher sink")
		assert.True(t, watcherSink.hasOpcode(0x2b), "expected MarshalAgentUpdateSpeed in watcher sink")
		assert.False(t, botSink.hasOpcode(0x29), "keyboard mover must not receive its own move-to-point")
		assert.False(t, botSink.hasOpcode(0x25), "keyboard mover must not receive its own direction")
		assert.False(t, botSink.hasOpcode(0x2b), "keyboard mover must not receive its own speed")
	})
}

func TestDirMoveSpeedByMoveType(t *testing.T) {
	withMoveNav(t, func(inst *Instance) {
		bot, _ := newTestPlayer("Bot")
		inst.AddPlayer(bot)
		bot.posX, bot.posY, bot.plane = 0, 50, 0
		bot.baseSpeed = 288

		for _, dir := range []int{1, 2, 3, 7, 8} {
			inst.applyDirMovement(bot, 0, 50, 0, -1, dir)
			assert.Equal(t, dir, bot.curMoveType)
			assert.InDelta(t, float32(288), bot.effectiveSpeed(), 1e-6)
		}

		// Backward / backward-strafe dirs move at 0.66x base.
		for _, dir := range []int{4, 5, 6} {
			inst.applyDirMovement(bot, 0, 50, 0, -1, dir)
			assert.Equal(t, dir, bot.curMoveType)
			assert.InDelta(t, float32(288*0.66), bot.effectiveSpeed(), 1e-6)
			assert.InDelta(t, float32(0.66), bot.speedMult(), 1e-6)
		}
	})
}

func TestDirMoveBroadcastBackwardSpeed(t *testing.T) {
	withMoveNav(t, func(inst *Instance) {
		bot, _ := newTestPlayer("Bot")
		watcher, watcherSink := newTestPlayer("Watcher")
		inst.AddPlayer(bot)
		inst.AddPlayer(watcher)
		bot.posX, bot.posY, bot.plane = 0, 50, 0
		bot.baseSpeed = 288
		watcherSink.reset()

		inst.applyDirMovement(bot, 0, 50, 0, -1, 4)

		dir, ok := sinkPacket(watcherSink, 0x25)
		require.True(t, ok, "expected MarshalAgentUpdateDirection")
		agentId, _ := dir.Uint32()
		facingX, _ := dir.Float32()
		facingY, _ := dir.Float32()
		moveType, _ := dir.Uint8()
		assert.Equal(t, bot.agentId, agentId)
		assert.InDelta(t, float32(0), facingX, 1e-6)
		assert.InDelta(t, float32(-1), facingY, 1e-6)
		assert.Equal(t, 4, moveType, "moveTypeCardinal must be Backward")

		speed, ok := sinkPacket(watcherSink, 0x2b)
		require.True(t, ok, "expected MarshalAgentUpdateSpeed")
		agentId, _ = speed.Uint32()
		speedValue, _ := speed.Float32()
		moveType, _ = speed.Uint8()
		assert.Equal(t, bot.agentId, agentId)
		assert.InDelta(t, float32(0.66), speedValue, 1e-6, "speed broadcast must be relative to base speed")
		assert.Equal(t, 4, moveType, "speed packet moveTypeCardinal must be Backward")
	})
}

func TestStopClearsMoveTypeButKeepsEnvMultiplier(t *testing.T) {
	withMoveNav(t, func(inst *Instance) {
		bot, _ := newTestPlayer("Bot")
		inst.AddPlayer(bot)
		bot.posX, bot.posY, bot.plane = 0, 50, 0
		bot.baseSpeed = 288
		bot.speedMultiplier = 2.0 // hypothetical environment effect

		inst.applyDirMovement(bot, 0, 50, 0, -1, 4)
		assert.Equal(t, 4, bot.curMoveType)
		assert.InDelta(t, float32(288*2*0.66), bot.effectiveSpeed(), 1e-6)

		assert.True(t, inst.applyLastPosCorrection(bot, 5, 45, 0))
		assert.False(t, bot.dirMove)
		assert.Equal(t, 0, bot.curMoveType, "stop must clear the move type")
		assert.InDelta(t, float32(2.0), bot.speedMultiplier, 1e-6, "environment multiplier must persist")
		assert.InDelta(t, float32(288*2), bot.effectiveSpeed(), 1e-6)
	})
}

func TestClickMoveClearsMoveType(t *testing.T) {
	withMoveNav(t, func(inst *Instance) {
		bot, _ := newTestPlayer("Bot")
		inst.AddPlayer(bot)
		bot.posX, bot.posY, bot.plane = 0, 50, 0
		bot.baseSpeed = 288

		inst.applyDirMovement(bot, 0, 50, 0, -1, 4)
		assert.Equal(t, 4, bot.curMoveType)

		assert.True(t, inst.startPlayerMove(bot, 0, -50, 0))
		assert.False(t, bot.dirMove)
		assert.Equal(t, 0, bot.curMoveType)
		assert.InDelta(t, float32(288), bot.effectiveSpeed(), 1e-6, "click-move advances at base speed")
	})
}

func TestDirMoveTimeoutClearsMoveType(t *testing.T) {
	withMoveNav(t, func(inst *Instance) {
		bot, _ := newTestPlayer("Bot")
		inst.AddPlayer(bot)
		bot.posX, bot.posY, bot.plane = 0, 50, 0
		bot.baseSpeed = 288

		inst.applyDirMovement(bot, 0, 50, 0, -1, 4)
		bot.lastDirUpdate = time.Now().Add(-3 * time.Second)

		inst.tickMovement()

		assert.False(t, bot.dirMove)
		assert.Equal(t, 0, bot.curMoveType, "timeout must clear the move type")
	})
}

func TestApplyDirMovementSnapsWithinTolerance(t *testing.T) {
	withMoveNav(t, func(inst *Instance) {
		bot, _ := newTestPlayer("Bot")
		inst.AddPlayer(bot)
		bot.posX, bot.posY, bot.plane = 0, 50, 0
		bot.baseSpeed = 288

		inst.applyDirMovement(bot, 5, 45, 0, 1, 1)
		assert.Equal(t, float32(5), bot.posX)
		assert.Equal(t, float32(45), bot.posY)
		assert.True(t, bot.dirMove)

		inst.applyDirMovement(bot, 200, 250, 0, 1, 1)
		assert.Equal(t, float32(200), bot.posX)
		assert.Equal(t, float32(250), bot.posY)
		assert.True(t, bot.dirMove)

		inst.applyDirMovement(bot, 50000, 50000, 0, 1, 1)
		assert.Equal(t, float32(200), bot.posX)
		assert.Equal(t, float32(250), bot.posY)
	})
}

func TestAdvanceDirMovement(t *testing.T) {
	withMoveNav(t, func(inst *Instance) {
		bot, _ := newTestPlayer("Bot")
		inst.AddPlayer(bot)
		bot.posX, bot.posY, bot.plane = 0, 50, 0
		bot.baseSpeed = 288

		inst.applyDirMovement(bot, 0, 50, 0, -1, 1)

		inst.lastMovementAdvanceAt = time.Now().Add(-500 * time.Millisecond)
		inst.flushMovement(time.Now())
		assert.Equal(t, float32(0), bot.posX)
		assert.InDelta(t, float32(50-144), bot.posY, 0.1)
		assert.True(t, bot.dirMove)
	})
}

func TestAdvanceDirMovementRefreshesVirtualTarget(t *testing.T) {
	withMoveNav(t, func(inst *Instance) {
		bot, _ := newTestPlayer("Bot")
		watcher, watcherSink := newTestPlayer("Watcher")
		inst.AddPlayer(bot)
		inst.AddPlayer(watcher)
		bot.posX, bot.posY, bot.plane = 0, 50, 0
		bot.baseSpeed = 288

		inst.applyDirMovement(bot, 0, 50, 0, -1, 1)
		watcherSink.reset()

		bot.destY = -50

		inst.lastMovementAdvanceAt = time.Now().Add(-500 * time.Millisecond)
		inst.flushMovement(time.Now())

		assert.True(t, watcherSink.hasOpcode(0x29), "refresh should re-broadcast move-to-point")
		assert.Equal(t, float32(0), bot.destX)
		assert.Equal(t, float32(bot.posY-5000), bot.destY, "virtual target recomputed from current position")
	})
}

func TestAdvanceDirMovementClampsAtWall(t *testing.T) {
	// A single isolated trapezoid (y in [0,100]) with nothing below it, so
	// walking south leaves the navmesh.
	d := &pathing.PathData{Planes: []pathing.Plane{{
		PlaneID: 0,
		Trapezoids: []pathing.Trapezoid{{
			YT: 100, YB: 0, XTL: -100, XTR: 100, XBL: -100, XBR: 100,
			NeighborTL: -1, NeighborTR: -1, NeighborBL: -1, NeighborBR: -1,
			PortalLeft: 0xFFFF, PortalRight: 0xFFFF,
		}},
	}}}
	assignTrapIDs(d)

	old := instancePathStore
	defer func() { instancePathStore = old }()
	store := pathing.NewStore()
	store.Set(0x340c6, d)
	instancePathStore = store

	inst := newHeadlessInstance(t, 3)
	bot, _ := newTestPlayer("Bot")
	inst.AddPlayer(bot)
	bot.posX, bot.posY, bot.plane = 0, 5, 0
	bot.baseSpeed = 288

	inst.applyDirMovement(bot, 0, 5, 0, -1, 1)
	assert.True(t, bot.dirMove)

	// A full 500ms step (144 units south) would exit the trap; the clamp stops
	// it at the navmesh edge (y = 0, within tolerance).
	inst.lastMovementAdvanceAt = time.Now().Add(-500 * time.Millisecond)
	inst.flushMovement(time.Now())
	assert.InDelta(t, float32(0), bot.posY, 2, "dir movement must clamp at the navmesh edge")
	assert.True(t, bot.dirMove, "clamping does not end keyboard movement; the next sync reconciles")
}

func TestDirMoveTimeout(t *testing.T) {
	withMoveNav(t, func(inst *Instance) {
		bot, botSink := newTestPlayer("Bot")
		inst.AddPlayer(bot)
		bot.posX, bot.posY, bot.plane = 0, 50, 0
		bot.baseSpeed = 288

		inst.applyDirMovement(bot, 0, 50, 0, -1, 1)
		bot.lastDirUpdate = time.Now().Add(-3 * time.Second)
		botSink.reset()

		inst.tickMovement()

		assert.False(t, bot.dirMove, "stale keyboard movement must time out")
		assert.True(t, botSink.hasOpcode(0x28), "expected MarshalAgentStopMoving on timeout")
	})
}

func TestStartPlayerMoveClearsDirMove(t *testing.T) {
	withMoveNav(t, func(inst *Instance) {
		bot, _ := newTestPlayer("Bot")
		inst.AddPlayer(bot)
		bot.posX, bot.posY, bot.plane = 0, 50, 0
		bot.baseSpeed = 288

		inst.applyDirMovement(bot, 0, 50, 0, -1, 1)
		assert.True(t, bot.dirMove)

		assert.True(t, inst.startPlayerMove(bot, 0, -50, 0))
		assert.False(t, bot.dirMove)
		assert.NotEmpty(t, bot.waypoints)
	})
}

func TestCancelClearsDirMove(t *testing.T) {
	withMoveNav(t, func(inst *Instance) {
		bot, _ := newTestPlayer("Bot")
		inst.AddPlayer(bot)
		bot.posX, bot.posY, bot.plane = 0, 50, 0
		bot.baseSpeed = 288

		inst.applyDirMovement(bot, 0, 50, 0, -1, 1)
		assert.True(t, bot.dirMove)

		assert.True(t, inst.applyLastPosCorrection(bot, 5, 45, 0))
		assert.False(t, bot.dirMove)
		assert.Equal(t, float32(5), bot.posX)
		assert.Equal(t, float32(45), bot.posY)
	})
}

func TestGameTickAdvancesMovementAndBroadcastsSimTick(t *testing.T) {
	withMoveNav(t, func(inst *Instance) {
		bot, botSink := newTestPlayer("Bot")
		inst.AddPlayer(bot)
		bot.posX, bot.posY, bot.plane = 0, 50, 0
		bot.baseSpeed = 288

		inst.startPlayerMove(bot, 0, -50, 0)
		botSink.reset()

		inst.lastMovementAdvanceAt = time.Now().Add(-500 * time.Millisecond)
		inst.gameTick()

		assert.True(t, botSink.hasOpcode(0x1e), "expected MarshalInstanceMovementTick")
		assert.Equal(t, float32(-50), bot.posY, "player should have arrived")
		assert.Empty(t, bot.waypoints)
	})
}

func TestPointInTransitionTrapezoid(t *testing.T) {
	trap := &TransitionTrapezoid{
		X1: -50, Y1: 100,
		X2: 50, Y2: 100,
		X3: 100, Y3: 0,
		X4: -100, Y4: 0,
	}

	inside := []struct {
		x, y float32
		name string
	}{
		{0, 50, "center"},
		{-90, 1, "near bottom-left edge"},
		{90, 1, "near bottom-right edge"},
		{-45, 99, "near top-left edge"},
		{45, 99, "near top-right edge"},
		{0, 0, "bottom center"},
		{0, 100, "top center"},
	}
	for _, tc := range inside {
		assert.True(t, pointInTransitionTrapezoid(trap, tc.x, tc.y), tc.name)
	}

	outside := []struct {
		x, y float32
		name string
	}{
		{0, -1, "below bottom"},
		{0, 101, "above top"},
		{-101, 50, "left of bottom edge"},
		{101, 50, "right of bottom edge"},
		{-51, 100, "left of top edge"},
		{51, 100, "right of top edge"},
		{0, -1000, "far below"},
		{0, 1000, "far above"},
	}
	for _, tc := range outside {
		assert.False(t, pointInTransitionTrapezoid(trap, tc.x, tc.y), tc.name)
	}
}

func TestPointInTransitionTrapezoidRectangle(t *testing.T) {
	trap := &TransitionTrapezoid{
		X1: -50, Y1: 100,
		X2: 50, Y2: 100,
		X3: 50, Y3: 0,
		X4: -50, Y4: 0,
	}

	assert.True(t, pointInTransitionTrapezoid(trap, 0, 50))
	assert.True(t, pointInTransitionTrapezoid(trap, -50, 0))
	assert.True(t, pointInTransitionTrapezoid(trap, 50, 100))
	assert.False(t, pointInTransitionTrapezoid(trap, -51, 50))
	assert.False(t, pointInTransitionTrapezoid(trap, 51, 50))
}

func withTransitionNav(t *testing.T, fn func(inst *Instance)) {
	t.Helper()
	old := instancePathStore
	defer func() { instancePathStore = old }()

	store := pathing.NewStore()
	store.Set(0x340c6, moveNavData())
	store.Set(0x2fed, &pathing.PathData{})
	instancePathStore = store

	fn(newHeadlessInstance(t, 3))
}

func TestCheckMapTransitionDirect(t *testing.T) {
	withTransitionNav(t, func(inst *Instance) {
		old := mapTransitions
		mapTransitions = map[int][]TransitionDefinition{
			3: {{
				Trapezoid: TransitionTrapezoid{X1: -100, Y1: -50, X2: 100, Y2: -50, X3: 100, Y3: -100, X4: -100, Y4: -100},
				Plane:     0,
				DestMapId: 2,
			}},
		}
		defer func() { mapTransitions = old }()
		inst.transitions = mapTransitions[3]

		bot, _ := newTestPlayer("Bot")
		inst.AddPlayer(bot)
		bot.posX, bot.posY, bot.plane = 0, -75, 0

		inst.checkMapTransition(bot)
		assert.True(t, bot.conn.IsClosed(), "player in transition zone should be disconnected")
	})
}

func TestKeyboardMoveTriggersTransition(t *testing.T) {
	withTransitionNav(t, func(inst *Instance) {
		old := mapTransitions
		mapTransitions = map[int][]TransitionDefinition{
			3: {{
				Trapezoid: TransitionTrapezoid{X1: -200, Y1: 50, X2: 200, Y2: 50, X3: 200, Y3: -200, X4: -200, Y4: -200},
				Plane:     0,
				DestMapId: 2,
			}},
		}
		defer func() { mapTransitions = old }()
		inst.transitions = mapTransitions[3]

		bot, _ := newTestPlayer("Bot")
		inst.AddPlayer(bot)
		bot.posX, bot.posY, bot.plane = 0, 10, 0
		bot.baseSpeed = 288

		inst.applyDirMovement(bot, 0, 10, 0, -1, 1)

		inst.lastMovementAdvanceAt = time.Now().Add(-500 * time.Millisecond)
		inst.flushMovement(time.Now())

		assert.True(t, bot.conn.IsClosed(), "player should be disconnected after transition")
	})
}

func TestClickMoveTriggersTransition(t *testing.T) {
	withTransitionNav(t, func(inst *Instance) {
		old := mapTransitions
		mapTransitions = map[int][]TransitionDefinition{
			3: {{
				Trapezoid: TransitionTrapezoid{X1: -200, Y1: 50, X2: 200, Y2: 50, X3: 200, Y3: -100, X4: -200, Y4: -100},
				Plane:     0,
				DestMapId: 2,
			}},
		}
		defer func() { mapTransitions = old }()
		inst.transitions = mapTransitions[3]

		bot, _ := newTestPlayer("Bot")
		inst.AddPlayer(bot)
		bot.posX, bot.posY, bot.plane = 0, 50, 0
		bot.baseSpeed = 288

		inst.startPlayerMove(bot, 0, -50, 0)

		inst.lastMovementAdvanceAt = time.Now().Add(-500 * time.Millisecond)
		inst.flushMovement(time.Now())

		assert.True(t, bot.conn.IsClosed(), "player should be disconnected after transition")
	})
}

func TestTransitionIgnoredOnWrongPlane(t *testing.T) {
	withTransitionNav(t, func(inst *Instance) {
		old := mapTransitions
		mapTransitions = map[int][]TransitionDefinition{
			3: {{
				Trapezoid: TransitionTrapezoid{X1: -200, Y1: 50, X2: 200, Y2: 50, X3: 200, Y3: -100, X4: -200, Y4: -100},
				Plane:     1,
				DestMapId: 2,
			}},
		}
		defer func() { mapTransitions = old }()
		inst.transitions = mapTransitions[3]

		bot, _ := newTestPlayer("Bot")
		inst.AddPlayer(bot)
		bot.posX, bot.posY, bot.plane = 0, -50, 0
		bot.baseSpeed = 288

		inst.checkMapTransition(bot)
		assert.False(t, bot.conn.IsClosed(), "no transition on wrong plane")
	})
}

func TestNoTransitionOutsideTrapezoid(t *testing.T) {
	withTransitionNav(t, func(inst *Instance) {
		old := mapTransitions
		mapTransitions = map[int][]TransitionDefinition{
			3: {{
				Trapezoid: TransitionTrapezoid{X1: -200, Y1: 50, X2: 200, Y2: 50, X3: 200, Y3: 20, X4: -200, Y4: 20},
				Plane:     0,
				DestMapId: 2,
			}},
		}
		defer func() { mapTransitions = old }()
		inst.transitions = mapTransitions[3]

		bot, _ := newTestPlayer("Bot")
		inst.AddPlayer(bot)
		bot.posX, bot.posY, bot.plane = 0, 80, 0
		bot.baseSpeed = 288

		inst.checkMapTransition(bot)
		assert.False(t, bot.conn.IsClosed(), "no transition outside trapezoid")
	})
}
