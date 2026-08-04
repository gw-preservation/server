package gameservice

import (
	"math"
	"time"

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
func (i *Instance) applyDirMovement(p *Player, x, y, fx, fy float32, moveType int) {
	i.assertActor()
	p.applyMovementFacing(fx, fy)
	p.waypoints = nil
	p.waypointIdx = 0
	p.dirMove = true
	p.lastDirUpdate = time.Now()
	p.curMoveType = moveType

	if vec2Length(p.posX-x, p.posY-y) <= dirMoveSnapGuard {
		p.posX, p.posY = x, y
	} else {
		p.log.Warn().Float32("x", x).Float32("y", y).Msg("keyboard movement sync rejected (teleport guard)")
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
	a.destX = a.posX + a.facingX*virtualTargetDist
	a.destY = a.posY + a.facingY*virtualTargetDist
	a.destPlane = a.plane
}

func (i *Instance) broadcastMoveToPointOthers(a *Agent, except *Player) {
	for _, other := range i.players {
		if other == except {
			continue
		}
		other.EnqueuePacket(MarshalMoveToPointS2C(a.agentId, a.destX, a.destY, a.destPlane, a.plane))
	}
}

func transitionCross(ax, ay, bx, by, px, py float32) float32 {
	return (bx-ax)*(py-ay) - (by-ay)*(px-ax)
}

func pointInTransitionTrapezoid(t *TransitionTrapezoid, x, y float32) bool {
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
		if t.Plane != p.plane {
			continue
		}
		if pointInTransitionTrapezoid(&t.Trapezoid, p.posX, p.posY) {
			i.cancelPlayerMovement(p)
			i.TransferPlayerToNewMap(p, t.DestMapId, t.SpawnX, t.SpawnY, t.SpawnPlane)
			return
		}
	}
}

func (i *Instance) advanceDirClamped(a *Agent, dist float32) {
	nx := a.posX + a.facingX*dist
	ny := a.posY + a.facingY*dist
	if _, ok := i.path.TrapezoidAt(nx, ny, a.plane); ok {
		a.posX, a.posY = nx, ny
		return
	}

	lo, hi := float32(0), dist
	for k := 0; k < 12; k++ {
		mid := (lo + hi) / 2
		mx := a.posX + a.facingX*mid
		my := a.posY + a.facingY*mid
		if _, ok := i.path.TrapezoidAt(mx, my, a.plane); ok {
			lo = mid
		} else {
			hi = mid
		}
	}
	a.posX += a.facingX * lo
	a.posY += a.facingY * lo
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
		if vec2Length(a.posX-a.destX, a.posY-a.destY) <= virtualRefreshDist {
			a.setDirTarget()
			i.broadcastMoveToPointOthers(a, p)
		}
	} else if len(a.waypoints) > 0 {
		dist := delta * a.effectiveSpeed()
		distToDest := vec2Length(a.posX-a.destX, a.posY-a.destY)

		if distToDest <= dist {
			dist -= distToDest
			a.posX, a.posY, a.plane = a.destX, a.destY, a.destPlane
			if a.waypointIdx < len(a.waypoints) {
				a.setDestination(a.waypoints[a.waypointIdx])
				a.waypointIdx++
				i.broadcastMoveToPoint(a)
			} else {
				dist = 0
			}
		}

		if dist > 0 {
			a.posX += a.facingX * dist
			a.posY += a.facingY * dist
		}

		if a.plane == a.destPlane && a.posX == a.destX && a.posY == a.destY {
			a.waypoints = nil
			a.waypointIdx = 0
		}
	}

	i.checkMapTransition(p)
}

// startPlayerMove: A* over the navmesh is the sole validator; movers off the
// navmesh (spawn areas) walk straight to the target.
func (i *Instance) startPlayerMove(p *Player, x, y float32, plane int) bool {
	i.assertActor()

	i.flushMovement(time.Now())

	p.dirMove = false
	p.waypoints = p.waypoints[:0]
	p.waypointIdx = 0
	p.resetMovementType()

	waypoints, ok := i.path.FindPath(p.posX, p.posY, p.plane, x, y, plane)
	if !ok {
		if _, onGrid := i.path.TrapezoidAt(p.posX, p.posY, p.plane); !onGrid {
			waypoints = []pathing.Waypoint{{X: x, Y: y, Plane: plane}}
		} else {
			i.log.Warn().Float32("x", x).Float32("y", y).Int("plane", plane).Msg("no path to requested target")
			i.broadcastPlayerPos(p)
			p.EnqueuePacket(MarshalMoveToPointS2C(p.agentId, p.posX, p.posY, p.plane, p.plane))
			return false
		}
	}

	p.waypoints = waypoints
	p.setDestination(waypoints[0])
	p.waypointIdx = 1
	i.log.Info().Float32("x", x).Float32("y", y).Int("plane", plane).Int("waypoints", len(waypoints)).Msg("accepted move request")
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

func (i *Instance) applyLastPosCorrection(p *Player, x, y float32, plane int) bool {
	i.assertActor()
	limit := float32(maxMovementCorrection)
	if p.dirMove {
		limit = float32(dirMoveSnapGuard)
	}
	if plane != p.plane || limit < vec2Length(p.posX-x, p.posY-y) {
		i.log.Warn().Msg("player tried to correct position by more than allowed")
		return false
	}
	p.posX, p.posY, p.plane = x, y, plane
	i.cancelPlayerMovement(p)
	return true
}

func (i *Instance) broadcastMoveToPoint(a *Agent) {
	for _, other := range i.players {
		other.EnqueuePacket(MarshalMoveToPointS2C(a.agentId, a.destX, a.destY, a.destPlane, a.plane))
	}
}
