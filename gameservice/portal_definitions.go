package gameservice

type MapQuad struct {
	X1, Y1 float32
	X2, Y2 float32
	X3, Y3 float32
	X4, Y4 float32
}

type MapPortalDefinition struct {
	Quad     MapQuad
	ToMapId  int
	SpawnX   float32
	SpawnY   float32
}

var mapTransitions = map[int][]MapPortalDefinition{
	164: {
		// Ashford Abbey -> Lakeside County
		{
			Quad: MapQuad{
				X1: -11078, Y1: -6424,
				X2: -11060, Y2: -6052,
				X3: -11260, Y3: -6042,
				X4: -11278, Y4: -6414,
			},
			SpawnX:  -10770,
			SpawnY:  -6252,
			ToMapId: 146,
		},
	},
	146: {
		// Lakeside County -> Ashford Abbey
		{
			Quad: MapQuad{
				X1: -11478, Y1: -6404,
				X2: -11460, Y2: -6032,
				X3: -11260, Y3: -6042,
				X4: -11278, Y4: -6414,
			},
			SpawnX:  -11768,
			SpawnY:  -6203,
			ToMapId: 164,
		},
		{
			Quad: MapQuad{
				-13934, -20256,
				-14340, -20256,
				-14327, -19872,
				-13916, -19872,
			},
			ToMapId: 161,
			SpawnX:  802,
			SpawnY:  976,
		},
	},
	139: {
		// Ventari's Refuge -> Ettin's Back
		{
			Quad: MapQuad{
				X1: -15351, Y1: 440,
				X2: -15024, Y2: 113,
				X3: -14883, Y3: 254,
				X4: -15210, Y4: 581,
			},
			SpawnX:  -15400,
			SpawnY:  64,
			ToMapId: 44,
		},
	},
	44: {
		// Ettin's Back -> Ventari's Refuge
		{
			Quad: MapQuad{
				X1: -15069, Y1: 723,
				X2: -14741, Y2: 395,
				X3: -14883, Y3: 254,
				X4: -15210, Y4: 581,
			},
			SpawnX:  -14693,
			SpawnY:  771,
			ToMapId: 139,
		},
	},
}
