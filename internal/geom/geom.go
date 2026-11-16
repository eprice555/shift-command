// Package geom has the small 2D vector type shared by the sim, world, and
// renderer. Sim positions are in tile units (1.0 = one map tile).
package geom

import "math"

// Vec is a 2D point or vector.
type Vec struct{ X, Y float64 }

// V is shorthand for Vec{x, y}.
func V(x, y float64) Vec { return Vec{x, y} }

func (a Vec) Add(b Vec) Vec       { return Vec{a.X + b.X, a.Y + b.Y} }
func (a Vec) Sub(b Vec) Vec       { return Vec{a.X - b.X, a.Y - b.Y} }
func (a Vec) Scale(k float64) Vec { return Vec{a.X * k, a.Y * k} }
func (a Vec) Dot(b Vec) float64   { return a.X*b.X + a.Y*b.Y }
func (a Vec) Len() float64        { return math.Hypot(a.X, a.Y) }
func (a Vec) Len2() float64       { return a.X*a.X + a.Y*a.Y }
func (a Vec) Dist(b Vec) float64  { return a.Sub(b).Len() }
func (a Vec) Dist2(b Vec) float64 { return a.Sub(b).Len2() }
func (a Vec) Lerp(b Vec, t float64) Vec {
	return Vec{a.X + (b.X-a.X)*t, a.Y + (b.Y-a.Y)*t}
}

// Norm returns a unit vector in a's direction, or the zero vector.
func (a Vec) Norm() Vec {
	l := a.Len()
	if l == 0 {
		return Vec{}
	}
	return Vec{a.X / l, a.Y / l}
}

// Angle returns the direction of a in radians.
func (a Vec) Angle() float64 { return math.Atan2(a.Y, a.X) }

// Rect is an axis-aligned rectangle.
type Rect struct{ Min, Max Vec }

// RectFrom returns the rectangle spanned by two corners in any order.
func RectFrom(a, b Vec) Rect {
	return Rect{Vec{min(a.X, b.X), min(a.Y, b.Y)}, Vec{max(a.X, b.X), max(a.Y, b.Y)}}
}

// Contains reports whether p is inside r (inclusive).
func (r Rect) Contains(p Vec) bool {
	return p.X >= r.Min.X && p.X <= r.Max.X && p.Y >= r.Min.Y && p.Y <= r.Max.Y
}

func (r Rect) W() float64 { return r.Max.X - r.Min.X }
func (r Rect) H() float64 { return r.Max.Y - r.Min.Y }
