package ui

import (
	"image/color"
	"math"
	"testing"
)

func TestEmblemPointsRadiusBounds(t *testing.T) {
	const r = 100.0
	for _, sides := range []int{0, 3, 6} {
		pts := EmblemPoints(nil, sides, 96, r, 0.3)
		if len(pts) != 96 {
			t.Fatalf("sides=%d: got %d points", sides, len(pts))
		}
		apothem := r
		if sides >= 3 {
			apothem = r * math.Cos(math.Pi/float64(sides))
		}
		for _, p := range pts {
			d := math.Hypot(p.X, p.Y)
			if d > r+1e-9 || d < apothem-1e-9 {
				t.Fatalf("sides=%d: point at distance %v outside [%v,%v]", sides, d, apothem, r)
			}
		}
	}
}

func TestEmblemPointsHitsCorners(t *testing.T) {
	pts := EmblemPoints(nil, 3, 96, 50, 0)
	corners := 0
	for _, p := range pts {
		if math.Abs(math.Hypot(p.X, p.Y)-50) < 1e-9 {
			corners++
		}
	}
	if corners != 3 {
		t.Errorf("triangle sampled %d corners, want 3", corners)
	}
}

func TestLerpPointsEndpoints(t *testing.T) {
	a := EmblemPoints(nil, 6, 12, 10, 0)
	b := EmblemPoints(nil, 0, 12, 20, 0)
	if got := LerpPoints(nil, a, b, 0); got[5] != a[5] {
		t.Errorf("t=0: %v != %v", got[5], a[5])
	}
	if got := LerpPoints(nil, a, b, 1); got[5] != b[5] {
		t.Errorf("t=1: %v != %v", got[5], b[5])
	}
}

func TestEaseInOutCubic(t *testing.T) {
	for _, c := range []struct{ in, want float64 }{{0, 0}, {0.5, 0.5}, {1, 1}} {
		if got := EaseInOutCubic(c.in); math.Abs(got-c.want) > 1e-12 {
			t.Errorf("ease(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestLerpColor(t *testing.T) {
	a, b := color.RGBA{0, 0, 0, 255}, color.RGBA{200, 100, 50, 255}
	if got := LerpColor(a, b, 0.5); got != (color.RGBA{100, 50, 25, 255}) {
		t.Errorf("got %v", got)
	}
}
