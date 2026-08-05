package gameservice

import (
	"time"

	"gw1/server/geom"
	"gw1/server/pathing"
)

type Agent struct {
	agentId             int
	definitionIndex     int
	isPlayer            bool
	name                string
	Pos                 geom.Pos2P
	Facing              geom.Vec2
	modelId             int
	allegianceFlags     int
	encName             string
	primaryProfession   int
	secondaryProfession int
	level               int
	fileId              int
	unkPropertiesBytes  string
	uuid                uint64

	// Movement state: path mode (waypoints + dest* leg target) or keyboard mode
	// (dirMove in facing; dest* is a broadcast-only virtual target).
	Dest          geom.Pos2P
	waypoints     []pathing.Waypoint
	waypointIdx   int
	dirMove       bool
	lastDirUpdate time.Time

	// Speed: baseSpeed * speedMultiplier, times the moveType factor while dirMove.
	baseSpeed       float32
	speedMultiplier float32
	curMoveType     int
}

func (a *Agent) setDestination(wp pathing.Waypoint) {
	a.Dest = wp.Pos2P
	dx := a.Dest.X - a.Pos.X
	dy := a.Dest.Y - a.Pos.Y
	norm := vec2Length(dx, dy)
	if norm > 0 {
		a.Facing = geom.Vec2{X: dx / norm, Y: dy / norm}
	}
}

// moveTypeMultiplier: keyboard dir speed factor (0.66 for backward/backward-strafe).
func moveTypeMultiplier(dir int) float32 {
	switch dir {
	case 4, 5, 6:
		return 0.66
	case 0:
		return 0.0
	default:
		return 1.0
	}
}

// speedMult is the base-relative speed observers are told via AgentUpdateSpeedBase.
func (a *Agent) speedMult() float32 {
	mult := a.speedMultiplier
	if a.dirMove {
		mult *= moveTypeMultiplier(a.curMoveType)
	}
	return mult
}

func (a *Agent) effectiveSpeed() float32 {
	return a.baseSpeed * a.speedMult()
}

func (a *Agent) resetMovementType() {
	a.curMoveType = 0
}
