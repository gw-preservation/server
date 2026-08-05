package gameservice

import (
	"fmt"
	"math"
	"time"

	"gw1/server/geom"
	"gw1/server/pathing"
)

// maxMovementCorrection caps how far a click-mover's stop position may differ
const maxMovementCorrection = 100.0

const movementTickInterval = 500 * time.Millisecond

// Keyboard movement: client reports MovementUpdate (0x803c) on keypress and
// ~once/sec; between syncs the server advances the mover. Observers get
// 0x25/0x2b and a far virtual target (0x29) to interpolate against.
const (
	virtualTargetDist  = 5000.0
	virtualRefreshDist = 500.0
	dirMoveTimeout     = 2 * time.Second
	dirMoveSnapGuard   = 1000.0
)

func vec2Length(dx, dy float32) float32 {
	return float32(math.Sqrt(float64(dx*dx + dy*dy)))
}

func (p *Player) applyMovementFacing(fx, fy float32) {
	n := vec2Length(fx, fy)
	if n > 0 {
		p.facingX = fx / n
		p.facingY = fy / n
	}
}

// applyDirMovement handles a keyboard MovementUpdate (0x803c): the mover is
// client-authoritative, and observers are pointed at the virtual target.
func (i *Instance) applyDirMovement(p *Player, pos geom.Pos2P, fx, fy float32, moveType int) {
	i.assertActor()
	p.applyMovementFacing(fx, fy)
	p.waypoints = nil
	p.waypointIdx = 0
	p.dirMove = true
	p.lastDirUpdate = time.Now()
	p.curMoveType = moveType

	if vec2Length(p.Pos.X-pos.X, p.Pos.Y-pos.Y) <= dirMoveSnapGuard {
		p.Pos.X, p.Pos.Y = pos.X, pos.Y
	} else {
		p.log.Warn().Float32("x", pos.X).Float32("y", pos.Y).Msg("keyboard movement sync rejected (teleport guard)")
	}

	p.setDirTarget()
	i.broadcastMoveToPointOthers(&p.Agent, p)
	for _, other := range i.players {
		if other == p {
			continue
		}
		other.EnqueuePacket(MarshalAgentUpdateDirection(p.agentId, p.facingX, p.facingY, p.curMoveType))
		other.EnqueuePacket(MarshalAgentUpdateSpeed(p.agentId, p.speedMult(), p.curMoveType))
	}
}

func (a *Agent) setDirTarget() {
	a.Dest.X = a.Pos.X + a.facingX*virtualTargetDist
	a.Dest.Y = a.Pos.Y + a.facingY*virtualTargetDist
	a.Dest.Plane = a.Pos.Plane
}

func (i *Instance) broadcastMoveToPointOthers(a *Agent, except *Player) {
	for _, other := range i.players {
		if other == except {
			continue
		}
		other.EnqueuePacket(MarshalMoveToPointS2C(a.agentId, a.Dest.X, a.Dest.Y, a.Dest.Plane, a.Pos.Plane))
	}
}

func transitionCross(ax, ay, bx, by, px, py float32) float32 {
	return (bx-ax)*(py-ay) - (by-ay)*(px-ax)
}

func pointInMapQuad(t *MapQuad, x, y float32) bool {
	s1 := transitionCross(t.X1, t.Y1, t.X2, t.Y2, x, y)
	s2 := transitionCross(t.X2, t.Y2, t.X3, t.Y3, x, y)
	s3 := transitionCross(t.X3, t.Y3, t.X4, t.Y4, x, y)
	s4 := transitionCross(t.X4, t.Y4, t.X1, t.Y1, x, y)
	return (s1 >= 0 && s2 >= 0 && s3 >= 0 && s4 >= 0) ||
		(s1 <= 0 && s2 <= 0 && s3 <= 0 && s4 <= 0)
}

func (i *Instance) checkMapTransition(p *Player) {
	for idx := range i.transitions {
		t := &i.transitions[idx]
		if pointInMapQuad(&t.Quad, p.Pos.X, p.Pos.Y) {
			if t.ToMapId == 0 || t.Spawn.IsEmpty() {
				p.SendChat(fmt.Sprintf("incomplete portal: portalIndex=%d to: %d x: %f y:%f side=%s, curMapId=%d", t.OriginalPortalIndex, t.ToMapId, t.Spawn.X, t.Spawn.Y, t.ZoneSide, i.mapId), 3)
			} else {
				if t.FromMapId == i.mapId && t.ToMapId != i.mapId {
					i.cancelPlayerMovement(p)
					i.TransferPlayerToNewMap(p, t.ToMapId, t.Spawn.X, t.Spawn.Y, t.Spawn.Plane)
				}
			}
			return
		}
	}
}

func (i *Instance) advanceDirClamped(a *Agent, dist float32) {
	nx := a.Pos.X + a.facingX*dist
	ny := a.Pos.Y + a.facingY*dist
	if _, ok := i.path.TrapezoidAt(geom.Pos2P{X: nx, Y: ny, Plane: a.Pos.Plane}); ok {
		a.Pos.X, a.Pos.Y = nx, ny
		return
	}

	lo, hi := float32(0), dist
	for k := 0; k < 12; k++ {
		mid := (lo + hi) / 2
		mx := a.Pos.X + a.facingX*mid
		my := a.Pos.Y + a.facingY*mid
		if _, ok := i.path.TrapezoidAt(geom.Pos2P{X: mx, Y: my, Plane: a.Pos.Plane}); ok {
			lo = mid
		} else {
			hi = mid
		}
	}
	a.Pos.X += a.facingX * lo
	a.Pos.Y += a.facingY * lo
}

func (i *Instance) checkDirMoveTimeouts(now time.Time) {
	i.assertActor()
	for _, p := range i.players {
		if p.conn.IsClosed() {
			continue
		}
		if p.dirMove && now.Sub(p.lastDirUpdate) > dirMoveTimeout {
			p.log.Warn().Msg("keyboard movement timed out")
			i.cancelPlayerMovement(p)
		}
	}
}

func (i *Instance) tickMovement() {
	i.assertActor()
	now := time.Now()
	i.checkDirMoveTimeouts(now)
	if i.lastMovementAdvanceAt.IsZero() {
		i.lastMovementAdvanceAt = now
		return
	}
	if now.Sub(i.lastMovementAdvanceAt) < movementTickInterval {
		return
	}
	i.flushMovement(now)
}

func (i *Instance) flushMovement(now time.Time) {
	i.assertActor()
	if i.lastMovementAdvanceAt.IsZero() {
		i.lastMovementAdvanceAt = now
		return
	}
	delta := now.Sub(i.lastMovementAdvanceAt)
	if delta <= 0 {
		i.lastMovementAdvanceAt = now
		return
	}
	i.lastMovementAdvanceAt = now

	ms := int(delta.Milliseconds())
	if ms <= 0 {
		ms = 1
	}
	for _, p := range i.players {
		if p.conn.IsClosed() {
			continue
		}
		p.EnqueuePacket(MarshalInstanceMovementTick(ms))
	}

	d := float32(delta.Seconds())
	for _, p := range i.players {
		if p.conn.IsClosed() {
			continue
		}
		i.advanceAgent(p, d)
	}
}

func (i *Instance) advanceAgent(p *Player, delta float32) {
	a := &p.Agent
	if a.dirMove {
		i.advanceDirClamped(a, delta*a.effectiveSpeed())
		if vec2Length(a.Pos.X-a.Dest.X, a.Pos.Y-a.Dest.Y) <= virtualRefreshDist {
			a.setDirTarget()
			i.broadcastMoveToPointOthers(a, p)
		}
	} else if len(a.waypoints) > 0 {
		dist := delta * a.effectiveSpeed()
		distToDest := vec2Length(a.Pos.X-a.Dest.X, a.Pos.Y-a.Dest.Y)

		if distToDest <= dist {
			dist -= distToDest
			a.Pos = a.Dest
			if a.waypointIdx < len(a.waypoints) {
				a.setDestination(a.waypoints[a.waypointIdx])
				a.waypointIdx++
				i.broadcastMoveToPoint(a)
			} else {
				dist = 0
			}
		}

		if dist > 0 {
			a.Pos.X += a.facingX * dist
			a.Pos.Y += a.facingY * dist
		}

		if a.Pos.Plane == a.Dest.Plane && a.Pos.X == a.Dest.X && a.Pos.Y == a.Dest.Y {
			a.waypoints = nil
			a.waypointIdx = 0
		}
	}

	i.checkMapTransition(p)
}

// startPlayerMove: A* over the navmesh is the sole validator; movers off the
// navmesh (spawn areas) walk straight to the target.
func (i *Instance) startPlayerMove(p *Player, dst geom.Pos2P) bool {
	i.assertActor()

	i.flushMovement(time.Now())

	p.dirMove = false
	p.waypoints = p.waypoints[:0]
	p.waypointIdx = 0
	p.resetMovementType()

	waypoints, ok := i.path.FindPath(p.Pos, dst)
	if !ok {
		if _, onGrid := i.path.TrapezoidAt(p.Pos); !onGrid {
			waypoints = []pathing.Waypoint{{Pos2P: dst}}
		} else {
			i.log.Warn().Float32("x", dst.X).Float32("y", dst.Y).Int("plane", dst.Plane).Msg("no path to requested target")
			i.broadcastPlayerPos(p)
			p.EnqueuePacket(MarshalMoveToPointS2C(p.agentId, p.Pos.X, p.Pos.Y, p.Pos.Plane, p.Pos.Plane))
			return false
		}
	}

	p.waypoints = waypoints
	p.setDestination(waypoints[0])
	p.waypointIdx = 1
	i.log.Info().Float32("x", dst.X).Float32("y", dst.Y).Int("plane", dst.Plane).Int("waypoints", len(waypoints)).Msg("accepted move request")
	i.broadcastMoveToPoint(&p.Agent)
	return true
}

func (i *Instance) cancelPlayerMovement(p *Player) {
	i.assertActor()
	p.waypoints = nil
	p.waypointIdx = 0
	p.dirMove = false
	p.resetMovementType()
	i.broadcastPlayerPos(p)
	i.BroadcastGeneric(MarshalAgentStopMoving(p.agentId))
}

func (i *Instance) applyLastPosCorrection(p *Player, dst geom.Pos2P) bool {
	i.assertActor()
	limit := float32(maxMovementCorrection)
	if p.dirMove {
		limit = float32(dirMoveSnapGuard)
	}
	if dst.Plane != p.Pos.Plane || limit < vec2Length(p.Pos.X-dst.X, p.Pos.Y-dst.Y) {
		i.log.Warn().Msg("player tried to correct position by more than allowed")
		return false
	}
	p.Pos = dst
	i.cancelPlayerMovement(p)
	return true
}

func (i *Instance) broadcastMoveToPoint(a *Agent) {
	for _, other := range i.players {
		other.EnqueuePacket(MarshalMoveToPointS2C(a.agentId, a.Dest.X, a.Dest.Y, a.Dest.Plane, a.Pos.Plane))
	}
}
