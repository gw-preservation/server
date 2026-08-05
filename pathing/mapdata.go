// Package pathing reads Guild Wars navmesh data from Gw.dat archives.
package pathing

import "gw1/server/geom"

// Trapezoid: horizontal walkable cell with sloped edges; corners in order
// (xtl,yt) (xtr,yt) (xbr,yb) (xbl,yb).
type Trapezoid struct {
	TrapID      uint32
	NeighborTL  int // -1 = wall
	NeighborTR  int
	NeighborBL  int
	NeighborBR  int
	PortalLeft  uint16
	PortalRight uint16
	YT          float32
	YB          float32
	XTL         float32
	XTR         float32
	XBL         float32
	XBR         float32
}

type NodeType uint8

const (
	NodeTypeXNode NodeType = 0
	NodeTypeYNode NodeType = 1
	NodeTypeSink  NodeType = 2
)

// Node: Nodes is laid out [xnodes][ynodes][sinks], mirroring the node-id
// namespaces collapsed into dense indices. Only the fields of the node's Type
// are meaningful.
type Node struct {
	Type         NodeType
	X            float32
	Y            float32
	DirX, DirY   float32
	Left, Right  int
	Above, Below int
	Trap         int
}

type Portal struct {
	PortalPlaneID   uint16
	NeighborPlaneID uint16
	Flags           uint8
	TrapsCount      uint32
	Traps           []int // indices into Plane.Trapezoids
	PairPlane       int   // index into PathData.Planes, -1 = none
	PairPortal      int   // index into Plane.Portals, -1 = none
}

type Plane struct {
	PlaneID      uint16
	NumXNodes    uint32
	NumYNodes    uint32
	Vectors      []geom.Vec2
	Trapezoids   []Trapezoid
	Nodes        []Node
	RootNode     int // index into Nodes, -1 = none (per-plane entry point)
	PortalsTraps []int
	Portals      []Portal
	// grid is the point-lookup bucket index built by buildTrapGrid; nil = linear scan.
	grid *trapGrid
}

type PathData struct {
	Planes     []Plane
	TrapsCount uint32
}
