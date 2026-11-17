package world

import "github.com/eprice555/shift-command/internal/geom"

// Map is a playable map: terrain, resource sites, and player start
// locations.
type Map struct {
	*Grid
	Name string
	// Starts are each player's HQ footprint centers.
	Starts [2]geom.Vec
	// Expansions are open HQ sites (footprint centers) beyond the mains.
	Expansions []geom.Vec
	Resources  []ResourceSite
}

// ResourceKind distinguishes the two resources.
type ResourceKind uint8

const (
	Crystal ResourceKind = iota // 1×1 node, mined directly
	Flux                        // 2×2 vent, needs an Extractor on top
)

// ResourceSize is the footprint of a resource node in tiles.
func (k ResourceKind) Size() int {
	if k == Flux {
		return 2
	}
	return 1
}

// ResourceSite is a resource node's placement: footprint top-left tile.
type ResourceSite struct {
	Kind ResourceKind
	X, Y int
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
	b := &mirrorBuilder{g: g}

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

	// Main: a crystal line west of the HQ and a vent to the south-east.
	for _, t := range [][2]int{{5, 63}, {4, 64}, {4, 65}, {4, 66}, {4, 67}, {4, 68}, {4, 69}, {5, 70}} {
		b.resource(Crystal, t[0], t[1])
	}
	b.resource(Flux, 17, 73)
	// Natural: crystals along the west edge, vent tucked under the shelf.
	for _, t := range [][2]int{{2, 37}, {1, 38}, {1, 39}, {1, 40}, {1, 41}, {2, 42}} {
		b.resource(Crystal, t[0], t[1])
	}
	b.resource(Flux, 13, 36)

	natural := geom.V(9, 41)
	return &Map{
		Grid:       g,
		Name:       "Neon Crossing",
		Starts:     [2]geom.Vec{geom.V(12, 68), b.mirror(geom.V(12, 68))},
		Expansions: []geom.Vec{natural, b.mirror(natural)},
		Resources:  b.res,
	}
}

// mirrorBuilder blocks tiles and their 180°-rotated twins.
type mirrorBuilder struct {
	g   *Grid
	res []ResourceSite
}

// mirror returns p's 180°-rotated twin.
func (b *mirrorBuilder) mirror(p geom.Vec) geom.Vec {
	return geom.V(float64(b.g.W)-p.X, float64(b.g.H)-p.Y)
}

// resource adds a resource site at tile (x, y) and its twin.
func (b *mirrorBuilder) resource(k ResourceKind, x, y int) {
	n := k.Size()
	b.res = append(b.res, ResourceSite{k, x, y}, ResourceSite{k, b.g.W - n - x, b.g.H - n - y})
}

func (b *mirrorBuilder) block(x, y int) {
	b.g.SetBlocked(x, y, true)
	b.g.SetBlocked(b.g.W-1-x, b.g.H-1-y, true)
}

// rect blocks tiles in [x0, x1) × [y0, y1).
func (b *mirrorBuilder) rect(x0, y0, x1, y1 int) {
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			b.block(x, y)
		}
	}
}

// circle blocks tiles whose centers are within r of (cx, cy).
func (b *mirrorBuilder) circle(cx, cy, r float64) {
	for y := int(cy - r - 1); y <= int(cy+r+1); y++ {
		for x := int(cx - r - 1); x <= int(cx+r+1); x++ {
			if TileCenter(x, y).Dist2(geom.V(cx, cy)) <= r*r {
				b.block(x, y)
			}
		}
	}
}
