package ui

import (
	"image/color"
	"math"

	"github.com/eprice555/shift-command/internal/geom"
)

// Vec is a 2D point.
type Vec = geom.Vec

// EmblemPoints appends n points sampled evenly by angle around a regular
// polygon with the given side count (0 = circle), circumradius r, rotated by
// rot radians. Sampling every shape with the same n lets shapes tween
// point-by-point; n should be a multiple of every side count used so corners
// land exactly on samples.
func EmblemPoints(dst []Vec, sides, n int, r, rot float64) []Vec {
	for i := range n {
		th := 2 * math.Pi * float64(i) / float64(n)
		rr := r
		if sides >= 3 {
			seg := 2 * math.Pi / float64(sides)
			local := math.Mod(th, seg) - seg/2
			rr = r * math.Cos(math.Pi/float64(sides)) / math.Cos(local)
		}
		s, c := math.Sincos(th + rot)
		dst = append(dst, geom.V(c*rr, s*rr))
	}
	return dst
}

// LerpPoints appends the point-wise interpolation of a and b at t.
func LerpPoints(dst, a, b []Vec, t float64) []Vec {
	for i := range min(len(a), len(b)) {
		dst = append(dst, a[i].Lerp(b[i], t))
	}
	return dst
}

// EaseInOutCubic maps t in [0,1] to an eased [0,1].
func EaseInOutCubic(t float64) float64 {
	if t < 0.5 {
		return 4 * t * t * t
	}
	u := -2*t + 2
	return 1 - u*u*u/2
}

// LerpColor interpolates two opaque colors.
func LerpColor(a, b color.RGBA, t float64) color.RGBA {
	l := func(x, y uint8) uint8 { return uint8(math.Round(float64(x) + (float64(y)-float64(x))*t)) }
	return color.RGBA{l(a.R, b.R), l(a.G, b.G), l(a.B, b.B), l(a.A, b.A)}
}

// Whiten blends c toward white by t, for "white-hot" neon tube cores.
func Whiten(c color.RGBA, t float64) color.RGBA {
	return LerpColor(c, color.RGBA{0xFF, 0xFF, 0xFF, c.A}, t)
}
