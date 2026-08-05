package gameservice

import "math"

type MapQuad struct {
	X1, Y1 float32
	X2, Y2 float32
	X3, Y3 float32
	X4, Y4 float32
}

type Portal struct {
	Pos           Pos2D
	Facing        Pos2D
	ScalingFactor float32
	LinkMapIdA    int
	LinkSpawnA    Pos3P
	LinkMapIdB    int
	LinkSpawnB    Pos3P
}

type MapPortalZone struct {
	Quad                MapQuad
	FromMapId           int
	ToMapId             int
	Spawn               Pos3P
	OriginalPortalIndex int
	ZoneSide            string
}

const (
	portalModelBaseWidth = 465.0
	portalModelDepth     = 40.0
	portalProjectDist    = 500.0
)

var mapZones map[int][]MapPortalZone

func derivePortalZones(portals []Portal) []MapPortalZone {
	var zones []MapPortalZone
	for portalIdx, p := range portals {
		for _, dir := range []int{1, -1} {
			var side string
			var toMap int
			var fromMap int
			var spawn Pos3P
			if dir == 1 {
				side = "A"
				toMap = p.LinkMapIdA
				fromMap = p.LinkMapIdB
				spawn = p.LinkSpawnA
			} else {
				side = "B"
				toMap = p.LinkMapIdB
				fromMap = p.LinkMapIdA
				spawn = p.LinkSpawnB
			}
			faceX := float64(p.Facing.X)
			faceZ := float64(p.Facing.Y)
			dirX := faceX * float64(dir)
			dirZ := faceZ * float64(dir)
			widthX := -faceZ
			widthZ := faceX

			halfWidth := float64(p.ScalingFactor) * portalModelBaseWidth / 2
			proj := portalProjectDist

			surfX := float64(p.Pos.X) - faceX*portalModelDepth
			surfZ := float64(p.Pos.Y) - faceZ*portalModelDepth

			cx := surfX + dirX*proj
			cz := surfZ + dirZ*proj

			zones = append(zones, MapPortalZone{
				Quad: MapQuad{
					X1: float32(math.Round(cx + widthX*halfWidth)),
					Y1: float32(math.Round(cz + widthZ*halfWidth)),
					X2: float32(math.Round(cx - widthX*halfWidth)),
					Y2: float32(math.Round(cz - widthZ*halfWidth)),
					X3: float32(math.Round(surfX - widthX*halfWidth)),
					Y3: float32(math.Round(surfZ - widthZ*halfWidth)),
					X4: float32(math.Round(surfX + widthX*halfWidth)),
					Y4: float32(math.Round(surfZ + widthZ*halfWidth)),
				},
				ToMapId:             toMap,
				FromMapId:           fromMap,
				Spawn:               spawn,
				OriginalPortalIndex: portalIdx,
				ZoneSide:            side,
			})
		}
	}
	return zones
}

var portals = map[int][]Portal{
	0x1b97d: { // 148: Pre Ascalon City, 164: Ashford Abbey, 146: Lakeside County
		{Pos: Pos2D{X: 7378, Y: 5429}, Facing: Pos2D{X: 0.70710677, Y: 0.70710677}, ScalingFactor: 1.1362},
		{Pos: Pos2D{X: 4384, Y: -19844}, Facing: Pos2D{X: -0.9996988, Y: 0.02454131}, ScalingFactor: 0.99612427},
		{Pos: Pos2D{X: -13897, Y: -20108}, Facing: Pos2D{X: 1, Y: -4.371139e-08}, ScalingFactor: 0.99612427,
			LinkMapIdA: 146,
			LinkMapIdB: 161,
			LinkSpawnB: Pos3P{10380, 19817, 0}},
		{Pos: Pos2D{X: -5488, Y: 13494}, Facing: Pos2D{X: 0, Y: 1}, ScalingFactor: 0.99612427},
		{Pos: Pos2D{X: -14540, Y: 10054}, Facing: Pos2D{X: 0.98527765, Y: 0.17096186}, ScalingFactor: 0.99612427},
		{Pos: Pos2D{X: -11309, Y: -6226}, Facing: Pos2D{X: -0.99879545, Y: 0.049067825}, ScalingFactor: 0.8015747,
			LinkMapIdA: 164,
			LinkSpawnA: Pos3P{-11548, -6240, 0},
			LinkMapIdB: 146,
			LinkSpawnB: Pos3P{-10988, -6255, 0}},
	},
	0x1ba26: { // 163: The Barradin Estate
		{Pos: Pos2D{X: 22967, Y: -17036}, Facing: Pos2D{X: 1, Y: -4.371139e-08}, ScalingFactor: 0.99612427},
		{Pos: Pos2D{X: 22324, Y: 13126}, Facing: Pos2D{X: 0.98527765, Y: 0.17096186}, ScalingFactor: 0.99612427},
		{Pos: Pos2D{X: 25555, Y: -3154}, Facing: Pos2D{X: -0.99879545, Y: 0.049067825}, ScalingFactor: 0.8015747},
		{Pos: Pos2D{X: 6822, Y: -17963}, Facing: Pos2D{X: 0.63439333, Y: 0.77301043}, ScalingFactor: 0.99612427},
		{Pos: Pos2D{X: -6793, Y: -16845}, Facing: Pos2D{X: 0, Y: 1}, ScalingFactor: 0.99612427},
		{Pos: Pos2D{X: -3250, Y: 9209}, Facing: Pos2D{X: 0.9495282, Y: 0.31368166}, ScalingFactor: 0.99612427},
		{Pos: Pos2D{X: -7361, Y: 1435}, Facing: Pos2D{X: -0.9996988, Y: -0.024541287}, ScalingFactor: 0.73153687},
	},
	0x1bacb: { // 165: Foible's Fair
		{Pos: Pos2D{X: 10679, Y: 19828}, Facing: Pos2D{X: 1, Y: -4.371139e-08}, ScalingFactor: 0.99612427,
			LinkMapIdA: 146,
			LinkSpawnA: Pos3P{-12914, -20062, 0},
			LinkMapIdB: 161},
		{Pos: Pos2D{X: 20040, Y: -10113}, Facing: Pos2D{X: 0.9238795, Y: 0.38268343}, ScalingFactor: 0.9883423},
		{Pos: Pos2D{X: -5466, Y: 18901}, Facing: Pos2D{X: 0.63439333, Y: 0.77301043}, ScalingFactor: 0.99612427},
		{Pos: Pos2D{X: 426, Y: 7688}, Facing: Pos2D{X: -0.40524137, Y: 0.9142097}, ScalingFactor: 0.99612427,
			LinkMapIdA: 165,
			LinkSpawnA: Pos3P{159, 8456, 0},
			LinkMapIdB: 161,
			LinkSpawnB: Pos3P{633, 7270, 0}},
		{Pos: Pos2D{X: -19081, Y: 20019}, Facing: Pos2D{X: 0, Y: 1}, ScalingFactor: 0.99612427},
	},
	0x1bb1d: { // 166: Pre Fort Ranik, 162: Regent Valley
		{Pos: Pos2D{X: -17120, Y: 17020}, Facing: Pos2D{X: -0.9996988, Y: 0.02454131}, ScalingFactor: 0.99612427},
		{Pos: Pos2D{X: -26040, Y: -13185}, Facing: Pos2D{X: 0.9238795, Y: 0.38268343}, ScalingFactor: 0.9883423},
		{Pos: Pos2D{X: 22595, Y: 7257}, Facing: Pos2D{X: -0.0735646, Y: 0.99729043}, ScalingFactor: 0.99612427},
	},
	0x1c530: { // 145: The Catacombs
		{Pos: Pos2D{X: 16823, Y: -10892}, Facing: Pos2D{X: 1, Y: -4.371139e-08}, ScalingFactor: 0.99612427},
		{Pos: Pos2D{X: 19411, Y: 2990}, Facing: Pos2D{X: -0.99879545, Y: 0.049067825}, ScalingFactor: 0.8015747},
		{Pos: Pos2D{X: 5961, Y: -22656}, Facing: Pos2D{X: 0.29028466, Y: 0.95694035}, ScalingFactor: 0.99612427},
	},
	0x1c539: { // 147: The Northlands
		{Pos: Pos2D{X: 26113, Y: 4767}, Facing: Pos2D{X: 0.9757021, Y: 0.21910122}, ScalingFactor: 0.6225891},
		{Pos: Pos2D{X: -11632, Y: -17226}, Facing: Pos2D{X: 0, Y: 1}, ScalingFactor: 0.99612427},
		{Pos: Pos2D{X: -20684, Y: -20666}, Facing: Pos2D{X: 0.98527765, Y: 0.17096186}, ScalingFactor: 0.99612427},
	},
}
