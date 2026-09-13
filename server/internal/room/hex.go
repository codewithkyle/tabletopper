package room

import "math"

const hexNudge = 1e-6

var sqrt3 = math.Sqrt(3)

func hexSize(g Grid) float64 { return float64(max(1, g.CellSize)) }
func hexOrigin(g Grid) (float64, float64) {
	s := hexSize(g)
	return float64(g.OffsetX) + s/2, float64(g.OffsetY) + s/2
}
func hexCentreF(g Grid, q, r float64) (float64, float64) {
	s := hexSize(g)
	ox, oy := hexOrigin(g)
	if g.Type == GridHexFlat {
		return ox + s*(sqrt3/2)*q, oy + s*(q/2+r)
	}
	return ox + s*(q+r/2), oy + s*(sqrt3/2)*r
}
func hexCentre(g Grid, q, r int) (int, int) {
	x, y := hexCentreF(g, float64(q), float64(r))
	return int(math.Round(x)), int(math.Round(y))
}
func hexFractional(g Grid, x, y int) (float64, float64) {
	s := hexSize(g)
	ox, oy := hexOrigin(g)
	px, py := float64(x)-ox, float64(y)-oy
	if g.Type == GridHexFlat {
		return 2 * px / (s * sqrt3), py/s - px/(s*sqrt3)
	}
	return px/s - py/(s*sqrt3), 2 * py / (s * sqrt3)
}
func hexRound(fq, fr float64) (int, int) {
	x, z := fq, fr
	y := -x - z
	rx, ry, rz := math.Round(x), math.Round(y), math.Round(z)
	dx, dy, dz := math.Abs(rx-x), math.Abs(ry-y), math.Abs(rz-z)
	switch {
	case dx > dy && dx > dz:
		rx = -ry - rz
	case dy > dz:
		ry = -rx - rz
	default:
		rz = -rx - ry
	}
	return int(rx), int(rz)
}
func hexAt(g Grid, x, y int) (int, int) {
	return hexRound(hexFractional(g, x, y))
}
func hexDistance(q0, r0, q1, r1 int) int {
	dq, dr := q1-q0, r1-r0
	return (abs(dq) + abs(dq+dr) + abs(dr)) / 2
}
func hexLine(q0, r0, q1, r1 int, out []int) []int {
	out = out[:0]
	steps := hexDistance(q0, r0, q1, r1)
	if steps == 0 {
		return append(out, q0, r0)
	}
	for i := 0; i <= steps && len(out) < PathCellsMax*2; i++ {
		t := float64(i) / float64(steps)
		fq := (float64(q0) + hexNudge) + ((float64(q1)+hexNudge)-(float64(q0)+hexNudge))*t
		fr := (float64(r0) + hexNudge) + ((float64(r1)+hexNudge)-(float64(r0)+hexNudge))*t
		q, r := hexRound(fq, fr)
		out = append(out, q, r)
	}
	return out
}
func hexCorners(g Grid, q, r int, out []int) []int {
	out = out[:0]
	cx, cy := hexCentreF(g, float64(q), float64(r))
	radius := hexSize(g) / sqrt3
	start := -30.0
	if g.Type == GridHexFlat {
		start = 0
	}
	for i := range 6 {
		a := (start + 60*float64(i)) * math.Pi / 180
		out = append(out,
			int(math.Round(cx+radius*math.Cos(a))),
			int(math.Round(cy+radius*math.Sin(a))),
		)
	}
	return out
}
func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
