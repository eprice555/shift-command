package world

import "github.com/eprice555/shift-command/internal/geom"

// Map is a playable map: terrain plus player start locations.
type Map struct {
	*Grid
	Name   string
	Starts [2]geom.Vec
}

// Skirmish map size in tiles.
const (
	SkirmishW = 80
	SkirmishH = 80
)

// NewSkirmishMap builds the M1 test map: a 2-player map with 180° rotational
// symmetry. Each player has a walled main base with one ramp leading to a
// natural expansion area, and rock formations break up the center.
func NewSkirmishMap() *Map {
	g := NewGrid(SkirmishW, SkirmishH)
	b := mirrorBuilder{g}

	// Player 1 main (bottom-left), open only through the ramp at x 16..20.
	b.rect(0, 54, 16, 56)
	b.rect(21, 54, 26, 56)
	b.rect(24, 56, 26, SkirmishH)
	// Natural expansion shelf above the ramp.
	b.rect(0, 34, 12, 36)
	b.circle(7.5, 45.5, 1.6)
	// Center and lanes.
	b.circle(40, 40, 4.2)
	b.circle(31, 31, 2.8)
	b.rect(34, 60, 46, 62)
	b.circle(58, 60, 3)
	b.rect(56, 44, 60, 52)

	return &Map{
		Grid:   g,
		Name:   "Neon Crossing",
		Starts: [2]geom.Vec{geom.V(12, 68), geom.V(SkirmishW-12, SkirmishH-68)},
	}
}

// mirrorBuilder blocks tiles and their 180°-rotated twins.
type mirrorBuilder struct{ g *Grid }

func (b mirrorBuilder) block(x, y int) {
	b.g.SetBlocked(x, y, true)
	b.g.SetBlocked(b.g.W-1-x, b.g.H-1-y, true)
}

// rect blocks tiles in [x0, x1) × [y0, y1).
func (b mirrorBuilder) rect(x0, y0, x1, y1 int) {
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			b.block(x, y)
		}
	}
}

// circle blocks tiles whose centers are within r of (cx, cy).
func (b mirrorBuilder) circle(cx, cy, r float64) {
	for y := int(cy - r - 1); y <= int(cy+r+1); y++ {
		for x := int(cx - r - 1); x <= int(cx+r+1); x++ {
			if TileCenter(x, y).Dist2(geom.V(cx, cy)) <= r*r {
				b.block(x, y)
			}
		}
	}
}
