package gameservice

type MapQuad struct {
	X1, Y1 float32
	X2, Y2 float32
	X3, Y3 float32
	X4, Y4 float32
}

type MapPortalDefinition struct {
	Quad       MapQuad
	Plane      int
	DestMapId  int
	SpawnX     float32
	SpawnY     float32
	SpawnPlane int
}

var mapTransitions = map[int][]MapPortalDefinition{
	164: {
		{
			Quad: MapQuad{
				X1: -11265.0, Y1: -6093.0,
				X2: -11150.0, Y2: -6093,
				X3: -11150, Y3: -6360,
				X4: -11265, Y4: -6360,
			},
			Plane:      0,
			DestMapId:  146,
			SpawnX:     -10822,
			SpawnY:     -6274,
			SpawnPlane: 0,
		},
	},
	146: {
		{
			Quad: MapQuad{
				X1: -11411, Y1: -6079,
				X2: -11275, Y2: -6092,
				X3: -11275, Y3: -6355,
				X4: -11373, Y4: -6355,
			},
			Plane:      0,
			DestMapId:  164,
			SpawnX:     -11500,
			SpawnY:     -6295,
			SpawnPlane: 0,
		},
	},
}
