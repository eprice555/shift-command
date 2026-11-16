package world

import (
	"math"

	"github.com/eprice555/shift-command/internal/geom"
)

// Visibility is a tile's fog-of-war state for one player.
type Visibility uint8

const (
	Unexplored Visibility = iota // never seen: black
	Explored                     // seen before: dim, shows terrain only
	Visible                      // currently in sight
)

// Fog tracks one player's visibility per tile. Each sim tick, call Begin and
// then Reveal for every unit the player owns.
type Fog struct {
	W, H  int
	state []Visibility
}

// NewFog returns a fully unexplored fog for a w×h map.
func NewFog(w, h int) *Fog { return &Fog{W: w, H: h, state: make([]Visibility, w*h)} }

// Begin demotes every visible tile to explored.
func (f *Fog) Begin() {
	for i, v := range f.state {
		if v == Visible {
			f.state[i] = Explored
		}
	}
}

// Reveal marks tiles whose centers are within r of c as visible.
func (f *Fog) Reveal(c geom.Vec, r float64) {
	x0, x1 := max(int(math.Floor(c.X-r)), 0), min(int(math.Ceil(c.X+r)), f.W-1)
	y0, y1 := max(int(math.Floor(c.Y-r)), 0), min(int(math.Ceil(c.Y+r)), f.H-1)
	r2 := r * r
	for y := y0; y <= y1; y++ {
		dy := float64(y) + 0.5 - c.Y
		row := f.state[y*f.W:]
		for x := x0; x <= x1; x++ {
			dx := float64(x) + 0.5 - c.X
			if dx*dx+dy*dy <= r2 {
				row[x] = Visible
			}
		}
	}
}

// At returns the visibility of tile (x, y); out-of-bounds is Unexplored.
func (f *Fog) At(x, y int) Visibility {
	if x < 0 || y < 0 || x >= f.W || y >= f.H {
		return Unexplored
	}
	return f.state[y*f.W+x]
}

// VisibleAt reports whether the tile containing p is currently visible.
func (f *Fog) VisibleAt(p geom.Vec) bool { return f.At(TileOf(p)) == Visible }
