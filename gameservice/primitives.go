package gameservice

// 2 dimensional position
type Pos2D struct {
	X, Y float32
}

func (p *Pos2D) IsEmpty() bool {
	return p.X == 0 && p.Y == 0
}

// Full 3D position
type Pos3D struct {
	X, Y, Z float32
}

// like Pos2D but with a Plane
type Pos3P struct {
	X, Y  float32
	Plane int
}

func (p *Pos3P) IsEmpty() bool {
	return p.X == 0 && p.Y == 0 && p.Plane == 0
}
