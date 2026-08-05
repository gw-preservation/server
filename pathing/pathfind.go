package pathing

import (
	"gw1/server/geom"
	"math"
)

// Waypoint is a point on a path produced by FindPath; TrapID is the containing
// trapezoid's global id.
type Waypoint struct {
	geom.Pos2P
	TrapID uint32
}

type pathPoint struct {
	x, y  float32
	plane int
	trap  *Trapezoid
}

type pathNode struct {
	closed bool
	cost   float32
	point  pathPoint
	next   *pathNode
}

type heapEntry struct {
	cost   float32
	trapID uint32
}

type pathBuildStep struct {
	x, y     float32
	dx, dy   float32
	plane    int
	nextTrap *Trapezoid
}

type pathContext struct {
	d     *PathData
	nodes []pathNode
	prioq binaryHeap
	steps []pathBuildStep
}

// pathNodeCount: TrapsCount, or the largest TrapID present when it is unset
// (hand-built navmeshes).
func (d *PathData) pathNodeCount() int {
	if n := int(d.TrapsCount); n > 0 {
		return n
	}
	var maxID uint32
	for pi := range d.Planes {
		for ti := range d.Planes[pi].Trapezoids {
			if id := d.Planes[pi].Trapezoids[ti].TrapID; id > maxID {
				maxID = id
			}
		}
	}
	return int(maxID) + 1
}

func (ctx *pathContext) addNode(parent *pathNode, trap *Trapezoid, x, y float32, plane int, currentCost, estimatedCost float32) {
	n := &ctx.nodes[trap.TrapID]
	n.closed = false
	n.cost = currentCost
	n.point.x = x
	n.point.y = y
	n.point.plane = plane
	n.point.trap = trap
	n.next = parent
	ctx.prioq.push(heapEntry{cost: estimatedCost, trapID: trap.TrapID})
}

func (ctx *pathContext) visitTrap(curr *pathNode, dstX, dstY float32, neighbor *Trapezoid, crossX, crossY float32, maxCost float32) {
	d := vec2Dist(curr.point.x, curr.point.y, crossX, crossY)
	costToTrap := curr.cost + d
	if maxCost <= costToTrap {
		return
	}

	bottomX := clampf(dstX, neighbor.XBL, neighbor.XBR)
	topX := clampf(dstX, neighbor.XTL, neighbor.XTR)
	bottomDx := bottomX - dstX
	bottomDy := neighbor.YB - dstY
	topDx := topX - dstX
	topDy := neighbor.YT - dstY
	bottomSq := bottomDx*bottomDx + bottomDy*bottomDy
	topSq := topDx*topDx + topDy*topDy
	bestSq := bottomSq
	if topSq < bottomSq {
		bestSq = topSq
	}
	if neighbor.YB <= dstY && dstY <= neighbor.YT {
		bestSq -= 0.0001
	}
	bestEstimate := float32(math.Sqrt(float64(bestSq)))

	ctx.addNode(curr, neighbor, crossX, crossY, curr.point.plane, costToTrap, costToTrap+bestEstimate)
}

func (ctx *pathContext) visitAbove(curr *pathNode, dstX, dstY float32, neighbor *Trapezoid, maxCost float32) {
	t := curr.point.trap
	xl := maxf(neighbor.XBL, t.XTL)
	xr := minf(neighbor.XBR, t.XTR)
	cx, cy := pointOnNextTrap(xl, xr, neighbor.YB, curr.point.x, curr.point.y, dstX, dstY)
	ctx.visitTrap(curr, dstX, dstY, neighbor, cx, cy, maxCost)
}

func (ctx *pathContext) visitBelow(curr *pathNode, dstX, dstY float32, neighbor *Trapezoid, maxCost float32) {
	t := curr.point.trap
	xl := maxf(neighbor.XTL, t.XBL)
	xr := minf(neighbor.XTR, t.XBR)
	cx, cy := pointOnNextTrap(xl, xr, neighbor.YT, curr.point.x, curr.point.y, dstX, dstY)
	ctx.visitTrap(curr, dstX, dstY, neighbor, cx, cy, maxCost)
}

// visitPortalLeft expands the traps beyond the left portal; blocked portals
// (flag 0x4) are skipped.
func (ctx *pathContext) visitPortalLeft(curr *pathNode, dstX, dstY float32, portalID uint16, maxCost float32) {
	pl := &ctx.d.Planes[curr.point.plane]
	if int(portalID) >= len(pl.Portals) {
		return
	}
	portal := &pl.Portals[portalID]
	if portal.Flags&0x4 != 0 || portal.PairPlane < 0 || portal.PairPortal < 0 {
		return
	}
	pair := &ctx.d.Planes[portal.PairPlane]
	if portal.PairPortal >= len(pair.Portals) {
		return
	}
	twin := &pair.Portals[portal.PairPortal]

	for _, trapIdx := range twin.Traps {
		if trapIdx < 0 || trapIdx >= len(pair.Trapezoids) {
			continue
		}
		portalTrap := &pair.Trapezoids[trapIdx]
		if ctx.nodes[portalTrap.TrapID].closed {
			continue
		}

		var p1x, p1y float32
		if portalTrap.YT <= curr.point.trap.YT {
			p1x, p1y = portalTrap.XTR, portalTrap.YT
		} else {
			p1x, p1y = curr.point.trap.XTL, curr.point.trap.YT
		}

		var p2x, p2y float32
		if curr.point.trap.YB <= portalTrap.YB {
			p2x, p2y = portalTrap.XBR, portalTrap.YB
		} else {
			p2x, p2y = curr.point.trap.XBL, curr.point.trap.YB
		}

		if p2y <= p1y {
			cx, cy := pickNextPoint(p1x, p1y, p2x, p2y, curr.point.x, curr.point.y, dstX, dstY)
			ctx.visitTrap(curr, dstX, dstY, portalTrap, cx, cy, maxCost)
		}
	}
}

// visitPortalRight mirrors visitPortalLeft for the right edge.
func (ctx *pathContext) visitPortalRight(curr *pathNode, dstX, dstY float32, portalID uint16, maxCost float32) {
	pl := &ctx.d.Planes[curr.point.plane]
	if int(portalID) >= len(pl.Portals) {
		return
	}
	portal := &pl.Portals[portalID]
	if portal.Flags&0x4 != 0 || portal.PairPlane < 0 || portal.PairPortal < 0 {
		return
	}
	pair := &ctx.d.Planes[portal.PairPlane]
	if portal.PairPortal >= len(pair.Portals) {
		return
	}
	twin := &pair.Portals[portal.PairPortal]

	for _, trapIdx := range twin.Traps {
		if trapIdx < 0 || trapIdx >= len(pair.Trapezoids) {
			continue
		}
		portalTrap := &pair.Trapezoids[trapIdx]
		if ctx.nodes[portalTrap.TrapID].closed {
			continue
		}

		var p1x, p1y float32
		if portalTrap.YT <= curr.point.trap.YT {
			p1x, p1y = portalTrap.XTL, portalTrap.YT
		} else {
			p1x, p1y = curr.point.trap.XTR, curr.point.trap.YT
		}

		var p2x, p2y float32
		if curr.point.trap.YB <= portalTrap.YB {
			p2x, p2y = portalTrap.XBL, portalTrap.YB
		} else {
			p2x, p2y = curr.point.trap.XBR, curr.point.trap.YB
		}

		if p2y <= p1y {
			cx, cy := pickNextPoint(p1x, p1y, p2x, p2y, curr.point.x, curr.point.y, dstX, dstY)
			ctx.visitTrap(curr, dstX, dstY, portalTrap, cx, cy, maxCost)
		}
	}
}

func reversePath(head *pathNode) *pathNode {
	var prev *pathNode
	for head != nil {
		next := head.next
		head.next = prev
		prev = head
		head = next
	}
	return prev
}

func vec2Sub(ax, ay, bx, by float32) (float32, float32) {
	return ax - bx, ay - by
}

func vec2Cross(ax, ay, bx, by float32) float32 {
	return ax*by - ay*bx
}

func vec2Dist(ax, ay, bx, by float32) float32 {
	dx := ax - bx
	dy := ay - by
	return float32(math.Sqrt(float64(dx*dx + dy*dy)))
}

func vec2Unit(x, y float32) (float32, float32) {
	tmp := float32(math.Sqrt(float64(x*x + y*y)))
	return x / tmp, y / tmp
}

func clampf(v, lo, hi float32) float32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func pointOnNextTrap(minLeftX, maxRightX, nextY float32, srcX, srcY, dstX, dstY float32) (float32, float32) {
	percent := float32(-1)
	if srcY != dstY {
		percent = (nextY - srcY) / (dstY - srcY)
	}
	if percent < 0 {
		srcXC := clampf(srcX, minLeftX, maxRightX)
		dstXC := clampf(dstX, minLeftX, maxRightX)
		return srcXC*0.9 + dstXC*0.1, nextY
	}
	newX := (dstX-srcX)*percent + srcX
	return clampf(newX, minLeftX, maxRightX), nextY
}

func intersectionPoint(s1x, s1y, d1x, d1y, s2x, s2y, d2x, d2y float32, t1, t2 *float32) bool {
	d := vec2Cross(d1x, d1y, d2x, d2y)
	if d == 0 {
		return false
	}
	vx, vy := vec2Sub(s1x, s1y, s2x, s2y)
	*t1 = vec2Cross(d2x, d2y, vx, vy) / d
	*t2 = vec2Cross(d1x, d1y, vx, vy) / d
	return true
}

func pickNextPoint(p1x, p1y, p2x, p2y, curX, curY, dstX, dstY float32) (float32, float32) {
	curToDstX := dstX - curX
	curToDstY := dstY - curY
	p1ToP2X := p2x - p1x
	p1ToP2Y := p2y - p1y

	var t1, t2 float32
	if intersectionPoint(curX, curY, curToDstX, curToDstY, p1x, p1y, p1ToP2X, p1ToP2Y, &t1, &t2) && t2 >= 0 {
		t2 = minf(t2, 1)
		return p1x + p1ToP2X*t2, p1y + p1ToP2Y*t2
	}

	norm2 := p1ToP2X*p1ToP2X + p1ToP2Y*p1ToP2Y
	// The game divides by zero here; with Go, +Inf reproduces its clamped result.
	divisor := 1 / norm2
	t1 = (p1x*p1ToP2X + p1y*p1ToP2Y) * divisor
	t2 = (p2x*p1ToP2X + p2y*p1ToP2Y) * divisor
	t1 = clampf(t1, 0, 1)
	t2 = clampf(t2, 0, 1)
	t := t2*0.1 + t1*0.9
	return p1x + p1ToP2X*t, p1y + p1ToP2Y*t
}

// preciseCross is the funnel's cross product; near-zero and near-parallel
// inputs collapse to 0.
func preciseCross(v1x, v1y, v2x, v2y float32) float32 {
	if v1x == 0 && v1y == 0 {
		return 0
	}
	if v2x == 0 && v2y == 0 {
		return 0
	}
	c := vec2Cross(v1x, v1y, v2x, v2y)
	if -0.01 < c && c < 0.01 {
		return 0
	}
	if c <= -1 || 1 <= c {
		return c
	}
	u1x, u1y := vec2Unit(v1x, v1y)
	u2x, u2y := vec2Unit(v2x, v2y)
	c = vec2Cross(u1x, u1y, u2x, u2y)
	if c <= -0.01 || 0.01 <= c {
		return c
	}
	return 0
}

func (ctx *pathContext) createWaypoints(src, dst pathPoint) []Waypoint {
	const maxLoop = 0xBB8

	helper := &pathBuildHelper{
		ctx:          ctx,
		leftBound:    pathBuildBound{point: src},
		rightBound:   pathBuildBound{point: src},
		currStart:    src,
		currentPlane: src.plane,
		srcPoint:     src,
		dstPoint:     dst,
	}
	ctx.steps = ctx.steps[:0]
	currNode := &ctx.nodes[src.trap.TrapID]

	for loop := 0; ; loop++ {
		if loop >= maxLoop {
			break
		}

		currTrap := currNode.point.trap
		nextNode := currNode.next

		if nextNode == nil || currTrap == dst.trap {
			if helper.buildAddLast(&currNode, dst) {
				break
			}
			continue
		}

		nextTrap := currNode.next.point.trap
		pl := &ctx.d.Planes[currNode.point.plane]

		switch {
		case trapIsNeighbor(nextTrap, pl, currTrap.NeighborTL) || trapIsNeighbor(nextTrap, pl, currTrap.NeighborTR):
			leftX := maxf(currTrap.XTL, nextTrap.XBL)
			rightX := minf(currTrap.XTR, nextTrap.XBR)
			helper.buildProcessNext(leftX, currTrap.YT, rightX, currTrap.YT, nextTrap, &currNode)
		case trapIsNeighbor(nextTrap, pl, currTrap.NeighborBL) || trapIsNeighbor(nextTrap, pl, currTrap.NeighborBR):
			leftX := maxf(currTrap.XBL, nextTrap.XTL)
			rightX := minf(currTrap.XBR, nextTrap.XTR)
			helper.buildProcessNext(rightX, currTrap.YB, leftX, currTrap.YB, nextTrap, &currNode)
		case helper.portalAdjacentToTrap(currTrap.PortalRight, nextTrap) != nil:
			var topX, topY, bottomX, bottomY float32
			if nextTrap.YT <= currTrap.YT {
				topX, topY = nextTrap.XTL, nextTrap.YT
			} else {
				topX, topY = currTrap.XTR, currTrap.YT
			}
			if currTrap.YB <= nextTrap.YB {
				bottomX, bottomY = nextTrap.XBL, nextTrap.YB
			} else {
				bottomX, bottomY = currTrap.XBR, currTrap.YB
			}
			helper.currentPlane = int(helper.portalAdjacentToTrap(currTrap.PortalRight, nextTrap).NeighborPlaneID)
			ctx.steps = append(ctx.steps, pathBuildStep{
				x: topX, y: topY,
				dx: bottomX - topX, dy: bottomY - topY,
				plane:    helper.currentPlane,
				nextTrap: nextTrap,
			})
			helper.buildProcessNext(topX, topY, bottomX, bottomY, nextTrap, &currNode)
		case helper.portalAdjacentToTrap(currTrap.PortalLeft, nextTrap) != nil:
			var topX, topY, bottomX, bottomY float32
			if nextTrap.YT <= currTrap.YT {
				topX, topY = nextTrap.XTR, nextTrap.YT
			} else {
				topX, topY = currTrap.XTL, currTrap.YT
			}
			if currTrap.YB <= nextTrap.YB {
				bottomX, bottomY = nextTrap.XBR, nextTrap.YB
			} else {
				bottomX, bottomY = currTrap.XBL, currTrap.YB
			}
			helper.currentPlane = int(helper.portalAdjacentToTrap(currTrap.PortalLeft, nextTrap).NeighborPlaneID)
			ctx.steps = append(ctx.steps, pathBuildStep{
				x: topX, y: topY,
				dx: bottomX - topX, dy: bottomY - topY,
				plane:    helper.currentPlane,
				nextTrap: nextTrap,
			})
			helper.buildProcessNext(bottomX, bottomY, topX, topY, nextTrap, &currNode)
		}
	}

	return helper.waypoints
}

func trapIsNeighbor(nextTrap *Trapezoid, pl *Plane, idx int) bool {
	return idx >= 0 && idx < len(pl.Trapezoids) && &pl.Trapezoids[idx] == nextTrap
}

// binaryHeap is a min-heap of heapEntry stored directly as a slice with no
// interface{} boxing.  This eliminates the allocation overhead of container/heap.
type binaryHeap struct {
	data []heapEntry
}

func (h *binaryHeap) reset(n int) {
	h.data = h.data[:0]
	if n > 0 && n <= cap(h.data) {
		return
	}
	h.data = make([]heapEntry, 0, n)
}

func (h *binaryHeap) len() int { return len(h.data) }

func (h *binaryHeap) push(e heapEntry) {
	h.data = append(h.data, e)
	h.siftUp(len(h.data) - 1)
}

func (h *binaryHeap) pop() heapEntry {
	top := h.data[0]
	n := len(h.data) - 1
	h.data[0] = h.data[n]
	h.data = h.data[:n]
	if n > 0 {
		h.siftDown(0)
	}
	return top
}

func (h *binaryHeap) siftUp(i int) {
	d := h.data
	e := d[i]
	for i > 0 {
		parent := (i - 1) >> 1
		if d[parent].cost <= e.cost {
			break
		}
		d[i] = d[parent]
		i = parent
	}
	d[i] = e
}

func (h *binaryHeap) siftDown(i int) {
	d := h.data
	n := len(d)
	e := d[i]
	for {
		l := 2*i + 1
		if l >= n {
			break
		}
		r := l + 1
		small := l
		if r < n && d[r].cost < d[l].cost {
			small = r
		}
		if d[small].cost >= e.cost {
			break
		}
		d[i] = d[small]
		i = small
	}
	d[i] = e
}

// FindPath runs A* over the trapezoid navmesh using a custom binary heap
// (no interface{} boxing) and a reduced-sqrt heuristic, and returns a waypoint
// list.
func (d *PathData) FindPath(src, dst geom.Pos2P) ([]Waypoint, bool) {
	const maxCost = 10000.0

	srcTrapIdx, ok := d.TrapezoidAt(src)
	if !ok {
		return nil, false
	}
	dstTrapIdx, ok := d.TrapezoidAt(dst)
	if !ok {
		return nil, false
	}

	srcTrap := &d.Planes[src.Plane].Trapezoids[srcTrapIdx]
	dstTrap := &d.Planes[dst.Plane].Trapezoids[dstTrapIdx]

	if srcTrap == dstTrap {
		return []Waypoint{{Pos2P: dst, TrapID: dstTrap.TrapID}}, true
	}

	nc := d.pathNodeCount()
	ctx := pathContext{d: d, nodes: make([]pathNode, nc)}
	ctx.prioq.reset(nc)

	ctx.addNode(nil, srcTrap, src.X, src.Y, src.Plane, 0, float32(math.Inf(1)))

	for ctx.prioq.len() > 0 {
		top := ctx.prioq.pop()
		if int(top.trapID) >= len(ctx.nodes) {
			continue
		}
		curr := &ctx.nodes[top.trapID]
		curr.closed = true
		currTrap := curr.point.trap

		if currTrap == dstTrap {
			reversePath(curr)
			srcPoint := pathPoint{x: src.X, y: src.Y, plane: src.Plane, trap: srcTrap}
			dstPoint := pathPoint{x: dst.X, y: dst.Y, plane: dst.Plane, trap: dstTrap}
			return ctx.createWaypoints(srcPoint, dstPoint), true
		}

		pl := &ctx.d.Planes[curr.point.plane]
		for _, nb := range [2]int{currTrap.NeighborTL, currTrap.NeighborTR} {
			if nb < 0 || nb >= len(pl.Trapezoids) {
				continue
			}
			if !ctx.nodes[pl.Trapezoids[nb].TrapID].closed {
				ctx.visitAbove(curr, dst.X, dst.Y, &pl.Trapezoids[nb], maxCost)
			}
		}
		for _, nb := range [2]int{currTrap.NeighborBL, currTrap.NeighborBR} {
			if nb < 0 || nb >= len(pl.Trapezoids) {
				continue
			}
			if !ctx.nodes[pl.Trapezoids[nb].TrapID].closed {
				ctx.visitBelow(curr, dst.X, dst.Y, &pl.Trapezoids[nb], maxCost)
			}
		}
		if currTrap.PortalLeft != noPortal && int(currTrap.PortalLeft) < len(pl.Portals) {
			ctx.visitPortalLeft(curr, dst.X, dst.Y, currTrap.PortalLeft, maxCost)
		}
		if currTrap.PortalRight != noPortal && int(currTrap.PortalRight) < len(pl.Portals) {
			ctx.visitPortalRight(curr, dst.X, dst.Y, currTrap.PortalRight, maxCost)
		}
	}

	return nil, false
}

type pathBuildBound struct {
	point  pathPoint
	stepID int
	vecX   float32
	vecY   float32
}

type pathBuildHelper struct {
	ctx          *pathContext
	leftBound    pathBuildBound
	rightBound   pathBuildBound
	currStart    pathPoint
	currentPlane int
	lastPos      pathPoint
	srcPoint     pathPoint
	dstPoint     pathPoint
	waypoints    []Waypoint
}

func (h *pathBuildHelper) buildAddWaypointAndReduce(newPoint pathPoint, fromX, fromY float32) {
	steps := h.ctx.steps
	for _, step := range steps {
		var bx, by float32
		if step.dx != 0 || step.dy != 0 {
			var t, s float32
			toX := newPoint.x - fromX
			toY := newPoint.y - fromY
			intersectionPoint(step.x, step.y, step.dx, step.dy, fromX, fromY, toX, toY, &t, &s)
			switch {
			case t > 1.01:
				bx = step.x + step.dx
				by = step.y + step.dy
			case t < -0.01:
				bx = step.x
				by = step.y
			default:
				bx = step.x + t*step.dx
				by = step.y + t*step.dy
			}
		} else {
			bx = step.x
			by = step.y
		}

		if h.lastPos.x != bx || h.lastPos.y != by || h.lastPos.plane != step.plane {
			h.lastPos.x = bx
			h.lastPos.y = by
			h.lastPos.plane = step.plane
			if n := len(h.waypoints); n != 0 && h.waypoints[n-1].X == bx && h.waypoints[n-1].Y == by {
				h.waypoints = h.waypoints[:n-1]
			}
			h.waypoints = append(h.waypoints, Waypoint{Pos2P: geom.Pos2P{X: bx, Y: by, Plane: step.plane}, TrapID: step.nextTrap.TrapID})
		}
	}

	h.ctx.steps = h.ctx.steps[:0]
	if h.lastPos.x != newPoint.x || h.lastPos.y != newPoint.y {
		h.lastPos = newPoint
		if n := len(h.waypoints); n != 0 && h.waypoints[n-1].X == newPoint.x && h.waypoints[n-1].Y == newPoint.y {
			h.waypoints = h.waypoints[:n-1]
		}
		h.waypoints = append(h.waypoints, Waypoint{Pos2P: geom.Pos2P{X: newPoint.x, Y: newPoint.y, Plane: newPoint.plane}, TrapID: newPoint.trap.TrapID})
	}
}

func (h *pathBuildHelper) buildAddWaypoint(leftX, leftY, rightX, rightY float32, node **pathNode) {
	var boundUsed *pathBuildBound
	switch {
	case leftX == h.leftBound.point.x && leftY == h.leftBound.point.y:
		boundUsed = &h.leftBound
		h.rightBound.point = h.leftBound.point
		h.rightBound.stepID = h.leftBound.stepID
		*node = &h.ctx.nodes[h.leftBound.point.trap.TrapID]
	case rightX == h.rightBound.point.x && rightY == h.rightBound.point.y:
		boundUsed = &h.rightBound
		h.leftBound.point = h.rightBound.point
		h.leftBound.stepID = h.rightBound.stepID
		*node = &h.ctx.nodes[h.rightBound.point.trap.TrapID]
	default:
		dx := leftX - h.currStart.x
		dy := leftY - h.currStart.y
		lval := dy*dy + dx*dx
		dx = rightX - h.currStart.x
		dy = rightY - h.currStart.y
		rval := dy*dy + dx*dx
		if lval < rval {
			boundUsed = &h.leftBound
		} else {
			boundUsed = &h.rightBound
		}
	}

	h.ctx.steps = h.ctx.steps[:boundUsed.stepID]
	h.buildAddWaypointAndReduce(boundUsed.point, h.currStart.x, h.currStart.y)
	h.currStart = boundUsed.point
	h.rightBound.vecX = 0
	h.rightBound.vecY = 0
	h.leftBound.vecX = 0
	h.leftBound.vecY = 0
	h.currentPlane = boundUsed.point.plane
}

func (h *pathBuildHelper) buildProcessNext(leftX, leftY, rightX, rightY float32, nextTrap *Trapezoid, node **pathNode) {
	toLeftX := leftX - h.currStart.x
	toLeftY := leftY - h.currStart.y
	toRightX := rightX - h.currStart.x
	toRightY := rightY - h.currStart.y

	var newLeft pathBuildBound
	if preciseCross(toLeftX, toLeftY, h.leftBound.vecX, h.leftBound.vecY) < 0 {
		toLeftX = h.leftBound.vecX
		toLeftY = h.leftBound.vecY
		newLeft = h.leftBound
	} else {
		newLeft.vecX = toLeftX
		newLeft.vecY = toLeftY
		newLeft.point.x = leftX
		newLeft.point.y = leftY
		newLeft.point.plane = h.currentPlane
		newLeft.point.trap = nextTrap
		newLeft.stepID = len(h.ctx.steps)
	}

	var newRight pathBuildBound
	if 0 < preciseCross(toRightX, toRightY, h.rightBound.vecX, h.rightBound.vecY) {
		toRightX = h.rightBound.vecX
		toRightY = h.rightBound.vecY
		newRight = h.rightBound
	} else {
		newRight.vecX = toRightX
		newRight.vecY = toRightY
		newRight.point.x = rightX
		newRight.point.y = rightY
		newRight.point.plane = h.currentPlane
		newRight.point.trap = nextTrap
		newRight.stepID = len(h.ctx.steps)
	}

	if (h.leftBound.vecX == 0 && h.leftBound.vecY == 0) || preciseCross(toLeftX, toLeftY, toRightX, toRightY) <= 0 {
		h.leftBound = newLeft
		h.rightBound = newRight
		*node = (*node).next
	} else {
		h.buildAddWaypoint(newLeft.point.x, newLeft.point.y, newRight.point.x, newRight.point.y, node)
	}
}

func (h *pathBuildHelper) buildAddLast(node **pathNode, newPoint pathPoint) bool {
	toX := newPoint.x - h.currStart.x
	toY := newPoint.y - h.currStart.y
	if 0 <= preciseCross(toX, toY, h.leftBound.vecX, h.leftBound.vecY) {
		if preciseCross(toX, toY, h.rightBound.vecX, h.rightBound.vecY) <= 0 {
			h.buildAddWaypointAndReduce(newPoint, h.currStart.x, h.currStart.y)
			return true
		}
		// [left, right, newPoint]
		h.rightBound.vecX = 0
		h.rightBound.vecY = 0
		h.leftBound.point = h.rightBound.point
		*node = &h.ctx.nodes[h.rightBound.point.trap.TrapID]
		h.ctx.steps = h.ctx.steps[:h.rightBound.stepID]
		h.buildAddWaypointAndReduce(h.rightBound.point, h.currStart.x, h.currStart.y)
		h.currStart = h.rightBound.point
		h.currentPlane = h.rightBound.point.plane
		return false
	}
	// [newPoint, left, right]
	h.leftBound.vecX = 0
	h.leftBound.vecY = 0
	h.rightBound.point = h.leftBound.point
	*node = &h.ctx.nodes[h.leftBound.point.trap.TrapID]
	h.ctx.steps = h.ctx.steps[:h.leftBound.stepID]
	h.buildAddWaypointAndReduce(h.leftBound.point, h.currStart.x, h.currStart.y)
	h.currStart = h.leftBound.point
	h.currentPlane = h.leftBound.point.plane
	return false
}

func (h *pathBuildHelper) portalAdjacentToTrap(portalID uint16, trap *Trapezoid) *Portal {
	if portalID == noPortal {
		return nil
	}
	pl := &h.ctx.d.Planes[h.currentPlane]
	if int(portalID) >= len(pl.Portals) {
		return nil
	}
	portal := &pl.Portals[portalID]
	if portal.PairPlane < 0 || portal.PairPortal < 0 {
		return nil
	}
	pair := &h.ctx.d.Planes[portal.PairPlane]
	if portal.PairPortal >= len(pair.Portals) {
		return nil
	}
	twin := &pair.Portals[portal.PairPortal]
	for _, idx := range twin.Traps {
		if idx >= 0 && idx < len(pair.Trapezoids) && &pair.Trapezoids[idx] == trap {
			return portal
		}
	}
	return nil
}
