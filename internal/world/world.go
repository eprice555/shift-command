// Package world holds the map grid, fog of war, and pathfinding (grid A* for
// single units, flow fields for group moves). It is deterministic and has no
// rendering dependencies. Positions are in tile units; tile (x, y) covers
// [x, x+1) × [y, y+1).
package world

import (
	"math"

	"github.com/eprice555/shift-command/internal/geom"
)

// Grid is a passability grid with two layers: static terrain, and an
// occupancy count for things placed during a match (buildings, resource
// nodes). A tile is blocked if either layer blocks it.
type Grid struct {
	W, H    int
	terrain []bool
	occ     []uint8

	scratch *searchScratch // reused A* buffers; Grid is not goroutine-safe
}

// NewGrid returns a fully passable w×h grid.
func NewGrid(w, h int) *Grid {
	return &Grid{W: w, H: h, terrain: make([]bool, w*h), occ: make([]uint8, w*h)}
}

// InBounds reports whether (x, y) is a tile on the grid.
func (g *Grid) InBounds(x, y int) bool { return x >= 0 && y >= 0 && x < g.W && y < g.H }

// Blocked reports whether (x, y) is impassable. Out-of-bounds is blocked.
func (g *Grid) Blocked(x, y int) bool {
	if !g.InBounds(x, y) {
		return true
	}
	i := y*g.W + x
	return g.terrain[i] || g.occ[i] > 0
}

// Terrain reports whether (x, y) is static terrain (walls, rocks), ignoring
// anything placed during a match. Out-of-bounds counts as terrain.
func (g *Grid) Terrain(x, y int) bool { return !g.InBounds(x, y) || g.terrain[y*g.W+x] }

// Occupied reports whether something placed during a match covers (x, y).
func (g *Grid) Occupied(x, y int) bool { return g.InBounds(x, y) && g.occ[y*g.W+x] > 0 }

// Occupy adds (delta > 0) or removes (delta < 0) one occupant from every
// tile in the size×size square at (x, y). Overlapping occupants (an
// Extractor on a vent) stack, so removing one leaves the other blocking.
func (g *Grid) Occupy(x, y, size, delta int) {
	for ty := y; ty < y+size; ty++ {
		for tx := x; tx < x+size; tx++ {
			if !g.InBounds(tx, ty) {
				continue
			}
			i := ty*g.W + tx
			g.occ[i] = uint8(max(0, int(g.occ[i])+delta))
		}
	}
}

// Passable is !Blocked.
func (g *Grid) Passable(x, y int) bool { return !g.Blocked(x, y) }

// SetBlocked marks (x, y) as terrain; out-of-bounds is ignored.
func (g *Grid) SetBlocked(x, y int, b bool) {
	if g.InBounds(x, y) {
		g.terrain[y*g.W+x] = b
	}
}

// TileRect returns the world-space box covered by the size×size square of
// tiles at (x, y).
func TileRect(x, y, size int) geom.Rect {
	return geom.Rect{Min: geom.V(float64(x), float64(y)), Max: geom.V(float64(x+size), float64(y+size))}
}

// TileOf returns the tile containing p.
func TileOf(p geom.Vec) (int, int) { return int(math.Floor(p.X)), int(math.Floor(p.Y)) }

// TileCenter returns the center of tile (x, y).
func TileCenter(x, y int) geom.Vec { return geom.V(float64(x)+0.5, float64(y)+0.5) }

// Clamp keeps p inside the map, inset by margin.
func (g *Grid) Clamp(p geom.Vec, margin float64) geom.Vec {
	p.X = min(max(p.X, margin), float64(g.W)-margin)
	p.Y = min(max(p.Y, margin), float64(g.H)-margin)
	return p
}

// NearestPassable returns the passable tile closest to (x, y) by ring search,
// or ok=false if the grid has none.
func (g *Grid) NearestPassable(x, y int) (int, int, bool) {
	x = min(max(x, 0), g.W-1)
	y = min(max(y, 0), g.H-1)
	if g.Passable(x, y) {
		return x, y, true
	}
	for r := 1; r < max(g.W, g.H); r++ {
		best, bx, by := math.MaxInt, 0, 0
		for dy := -r; dy <= r; dy++ {
			for dx := -r; dx <= r; dx++ {
				if max(abs(dx), abs(dy)) != r || g.Blocked(x+dx, y+dy) {
					continue
				}
				if d := dx*dx + dy*dy; d < best {
					best, bx, by = d, x+dx, y+dy
				}
			}
		}
		if best != math.MaxInt {
			return bx, by, true
		}
	}
	return 0, 0, false
}

// SnapPassable moves p to the nearest passable tile center if p is on a
// blocked tile; otherwise it returns p unchanged.
func (g *Grid) SnapPassable(p geom.Vec) geom.Vec {
	x, y := TileOf(p)
	if g.Passable(x, y) {
		return p
	}
	nx, ny, ok := g.NearestPassable(x, y)
	if !ok {
		return p
	}
	return TileCenter(nx, ny)
}

// ResolveCircle pushes a circle at p with radius r out of blocked tiles and
// the map edge, returning the corrected center.
func (g *Grid) ResolveCircle(p geom.Vec, r float64) geom.Vec {
	return g.resolveCircle(p, r, false)
}

// ResolveCircleIgnoringTerrain resolves a body against map edges and placed
// buildings, but lets it pass through static terrain.
func (g *Grid) ResolveCircleIgnoringTerrain(p geom.Vec, r float64) geom.Vec {
	return g.resolveCircle(p, r, true)
}

func (g *Grid) resolveCircle(p geom.Vec, r float64, ignoreTerrain bool) geom.Vec {
	p = g.Clamp(p, r)
	for range 2 {
		cx, cy := TileOf(p)
		for ty := cy - 1; ty <= cy+1; ty++ {
			for tx := cx - 1; tx <= cx+1; tx++ {
				if !g.InBounds(tx, ty) || (ignoreTerrain && !g.Occupied(tx, ty)) || (!ignoreTerrain && !g.Blocked(tx, ty)) {
					continue
				}
				// Closest point on the tile's box to p.
				qx := min(max(p.X, float64(tx)), float64(tx+1))
				qy := min(max(p.Y, float64(ty)), float64(ty+1))
				d := geom.V(p.X-qx, p.Y-qy)
				l := d.Len()
				switch {
				case l == 0:
					// Center is inside the tile: eject through its nearest face.
					c := TileCenter(tx, ty)
					o := p.Sub(c)
					if math.Abs(o.X) > math.Abs(o.Y) {
						p.X = c.X + math.Copysign(0.5+r, o.X)
					} else {
						p.Y = c.Y + math.Copysign(0.5+r, o.Y)
					}
				case l < r:
					p = p.Add(d.Scale((r - l) / l))
				}
			}
		}
	}
	return g.Clamp(p, r)
}

// LineOfSight reports whether the segment a→b crosses only passable tiles.
// Passing exactly through a corner requires both side tiles to be open.
func (g *Grid) LineOfSight(a, b geom.Vec) bool {
	x, y := TileOf(a)
	ex, ey := TileOf(b)
	if g.Blocked(x, y) || g.Blocked(ex, ey) {
		return false
	}
	d := b.Sub(a)
	stepX, stepY := sign(d.X), sign(d.Y)
	tMaxX, tDeltaX := math.Inf(1), math.Inf(1)
	tMaxY, tDeltaY := math.Inf(1), math.Inf(1)
	if d.X != 0 {
		next := float64(x)
		if stepX > 0 {
			next++
		}
		tMaxX, tDeltaX = (next-a.X)/d.X, math.Abs(1/d.X)
	}
	if d.Y != 0 {
		next := float64(y)
		if stepY > 0 {
			next++
		}
		tMaxY, tDeltaY = (next-a.Y)/d.Y, math.Abs(1/d.Y)
	}
	const eps = 1e-9
	for n := abs(ex-x) + abs(ey-y); n > 0 && (x != ex || y != ey); n-- {
		switch {
		case math.Abs(tMaxX-tMaxY) < eps:
			if g.Blocked(x+stepX, y) || g.Blocked(x, y+stepY) {
				return false
			}
			x += stepX
			y += stepY
			tMaxX += tDeltaX
			tMaxY += tDeltaY
		case tMaxX < tMaxY:
			x += stepX
			tMaxX += tDeltaX
		default:
			y += stepY
			tMaxY += tDeltaY
		}
		if g.Blocked(x, y) {
			return false
		}
	}
	return true
}

// ClearPath is LineOfSight for a body of radius r: the center line and both
// edges must be clear.
func (g *Grid) ClearPath(a, b geom.Vec, r float64) bool {
	if !g.LineOfSight(a, b) {
		return false
	}
	if r <= 0 || a == b {
		return true
	}
	n := b.Sub(a).Norm()
	off := geom.V(-n.Y, n.X).Scale(r)
	return g.LineOfSight(a.Add(off), b.Add(off)) && g.LineOfSight(a.Sub(off), b.Sub(off))
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func sign(v float64) int {
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	}
	return 0
}
