package geom

// Pos2 is a 2D position.
type Pos2 struct {
	X, Y float32
}

func (p *Pos2) IsEmpty() bool {
	return p.X == 0 && p.Y == 0
}

// Vec2 is a 2D direction or velocity vector.
type Vec2 struct {
	X, Y float32
}

// Pos2P is a 2D position with a plane index.
type Pos2P struct {
	X, Y  float32
	Plane int
}

func (p *Pos2P) IsEmpty() bool {
	return p.X == 0 && p.Y == 0 && p.Plane == 0
}
